package subscription

import (
	stdctx "context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/xxcheng123/cloudpan189-share/internal/consts"
	appContext "github.com/xxcheng123/cloudpan189-share/internal/framework/context"
	"github.com/xxcheng123/cloudpan189-share/internal/framework/httpcontext"
	"github.com/xxcheng123/cloudpan189-share/internal/repository/models"
	"github.com/xxcheng123/cloudpan189-share/internal/services/cloudbridge"
	"github.com/xxcheng123/cloudpan189-share/internal/services/storagefacade"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type mockSubscriptionStorageFacade struct {
	storagefacade.Service
	req *storagefacade.CreateStorageRequest
}

func (m *mockSubscriptionStorageFacade) CreateStorage(ctx appContext.Context, req *storagefacade.CreateStorageRequest) (int64, error) {
	m.req = req

	return 12345, nil
}

type mockSubscriptionCloudBridge struct {
	cloudbridge.Service
}

type failingRoundTripper struct {
	err error
}

func (t failingRoundTripper) RoundTrip(*http.Request) (*http.Response, error) {
	return nil, t.err
}

func (m *mockSubscriptionCloudBridge) GetShareInfo(ctx appContext.Context, shareCode string, accessCode string) (*cloudbridge.ShareInfo, error) {
	return &cloudbridge.ShareInfo{
		Name:     "share-name",
		IsFolder: true,
		ShareId:  67890,
		ID:       "share-file-id",
	}, nil
}

func setupSubscriptionHandlerTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}

	if err := db.AutoMigrate(&Setting{}, &models.MountPoint{}); err != nil {
		t.Fatalf("migrate test db: %v", err)
	}

	return db
}

func closeSubscriptionHandlerTestDB(t *testing.T, db *gorm.DB) {
	t.Helper()

	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get sql db: %v", err)
	}

	if err := sqlDB.Close(); err != nil {
		t.Fatalf("close sql db: %v", err)
	}
}

func TestGetConfigReturnsDefaultsWithoutCreatingSetting(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db := setupSubscriptionHandlerTestDB(t)
	handler := NewHandler(db, nil, nil, nil, nil, zap.NewNop(), nil)
	router := gin.New()
	wrapper := httpcontext.NewHandlerFuncWrapper(zap.NewNop())
	router.GET("/config", wrapper.Wrap(handler.GetConfig()))

	req := httptest.NewRequestWithContext(stdctx.Background(), http.MethodGet, "/config", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected ok, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	var response struct {
		Code int                `json:"code"`
		Data SubscriptionConfig `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}

	if response.Data.PanSearchURL == "" {
		t.Fatal("expected default pan search URL")
	}

	if response.Data.CronExpression != "0 2 * * *" {
		t.Fatalf("expected default cron expression, got %q", response.Data.CronExpression)
	}

	var count int64
	if err := db.Model(&Setting{}).Where("name = ?", "subscription_config").Count(&count).Error; err != nil {
		t.Fatalf("count settings: %v", err)
	}

	if count != 0 {
		t.Fatalf("expected GET config not to create setting, got count %d", count)
	}
}

func TestUpdateConfigUpdatesExistingSettingWithoutDuplicate(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db := setupSubscriptionHandlerTestDB(t)
	handler := NewHandler(db, nil, nil, nil, nil, zap.NewNop(), nil)

	existing := &Setting{
		Name: "subscription_config",
		Value: models.SubscriptionConfig{
			EnableTMDB:       true,
			EnableDouban:     true,
			PanSearchURL:     "https://old.example.com/api/search",
			CronExpression:   "0 1 * * *",
			DefaultMountPath: "/old",
			AutoMount:        true,
		},
	}
	if err := db.Create(existing).Error; err != nil {
		t.Fatalf("create setting: %v", err)
	}

	router := gin.New()
	wrapper := httpcontext.NewHandlerFuncWrapper(zap.NewNop())
	router.POST("/config", wrapper.Wrap(handler.UpdateConfig()))

	req := httptest.NewRequestWithContext(
		stdctx.Background(),
		http.MethodPost,
		"/config",
		strings.NewReader(`{"enableTMDB":false,"enableDouban":false,"panSearchURL":"https://new.example.com/api/search","defaultMountPath":"/new","autoMount":false,"cronExpression":"0 3 * * *","tmdbAPIKey":"","openaiAPIKey":"","openaiBaseURL":"https://api.openai.com","openaiModel":"gpt-4o-mini"}`),
	)
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected ok, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	var count int64
	if err := db.Model(&Setting{}).Where("name = ?", "subscription_config").Count(&count).Error; err != nil {
		t.Fatalf("count settings: %v", err)
	}

	if count != 1 {
		t.Fatalf("expected one setting, got %d", count)
	}

	var updated Setting
	if err := db.First(&updated, existing.ID).Error; err != nil {
		t.Fatalf("query updated setting: %v", err)
	}

	if updated.Value.EnableTMDB {
		t.Fatal("expected enable TMDB to be false")
	}

	if updated.Value.AutoMount {
		t.Fatal("expected auto mount to be false")
	}

	if updated.Value.PanSearchURL != "https://new.example.com/api/search" {
		t.Fatalf("expected pan search URL updated, got %q", updated.Value.PanSearchURL)
	}
}

func TestCheckSubscriptionSettingUpdateResultAllowsExistingNoop(t *testing.T) {
	db := setupSubscriptionHandlerTestDB(t)
	handler := NewHandler(db, nil, nil, nil, nil, zap.NewNop(), nil)

	setting := &Setting{
		Name: "subscription_config",
		Value: models.SubscriptionConfig{
			PanSearchURL:   "https://search.example.com/api/search",
			CronExpression: "0 2 * * *",
		},
	}
	if err := db.Create(setting).Error; err != nil {
		t.Fatalf("create setting: %v", err)
	}

	if err := handler.checkSubscriptionSettingUpdateResult(&gorm.DB{RowsAffected: 0}, setting.ID); err != nil {
		t.Fatalf("expected existing no-op setting update to pass, got %v", err)
	}
}

func TestCheckSubscriptionSettingUpdateResultReturnsNotFoundWhenMissing(t *testing.T) {
	db := setupSubscriptionHandlerTestDB(t)
	handler := NewHandler(db, nil, nil, nil, nil, zap.NewNop(), nil)

	err := handler.checkSubscriptionSettingUpdateResult(&gorm.DB{RowsAffected: 0}, 999)
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("expected record not found, got %v", err)
	}
}

func TestMountSubscriptionPassesCurrentUserIDToStorageFacade(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db := setupSubscriptionHandlerTestDB(t)
	storageSvc := &mockSubscriptionStorageFacade{}
	handler := NewHandler(db, nil, nil, storageSvc, &mockSubscriptionCloudBridge{}, zap.NewNop(), nil)

	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(consts.CtxKeyUserId, int64(77))
		c.Next()
	})

	wrapper := httpcontext.NewHandlerFuncWrapper(zap.NewNop())
	router.POST("/mount", wrapper.Wrap(handler.MountSubscription()))

	req := httptest.NewRequestWithContext(
		stdctx.Background(),
		http.MethodPost,
		"/mount",
		strings.NewReader(`{"title":"测试资源","shareUrl":"https://cloud.189.cn/t/abc123","shareCode":"abc123","mountPath":"/热门订阅/测试资源"}`),
	)
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected ok, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	if storageSvc.req == nil {
		t.Fatal("expected storage facade to be called")
	}

	if storageSvc.req.CreatorUserID != 77 {
		t.Fatalf("expected creator user id 77, got %d", storageSvc.req.CreatorUserID)
	}

	if storageSvc.req.FileId != "share-file-id" {
		t.Fatalf("expected share file id, got %q", storageSvc.req.FileId)
	}
}

func TestMountSubscriptionRejectsExistingPathOwnedByOtherUser(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db := setupSubscriptionHandlerTestDB(t)
	if err := db.Create(&models.MountPoint{
		FileId:        999,
		Name:          "测试资源",
		FullPath:      "/热门订阅/测试资源",
		OsType:        models.OsTypeSubscribeShareFolder,
		TokenId:       0,
		CreatorUserID: 88,
	}).Error; err != nil {
		t.Fatalf("create mount point: %v", err)
	}

	storageSvc := &mockSubscriptionStorageFacade{}
	handler := NewHandler(db, nil, nil, storageSvc, &mockSubscriptionCloudBridge{}, zap.NewNop(), nil)

	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(consts.CtxKeyUserId, int64(77))
		c.Set(consts.CtxKeyIsAdmin, false)
		c.Next()
	})

	wrapper := httpcontext.NewHandlerFuncWrapper(zap.NewNop())
	router.POST("/mount", wrapper.Wrap(handler.MountSubscription()))

	req := httptest.NewRequestWithContext(
		stdctx.Background(),
		http.MethodPost,
		"/mount",
		strings.NewReader(`{"title":"测试资源","shareUrl":"https://cloud.189.cn/t/abc123","shareCode":"abc123","mountPath":"/热门订阅/测试资源"}`),
	)
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected unauthorized, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	if storageSvc.req != nil {
		t.Fatal("expected storage facade not to be called")
	}

	if !strings.Contains(recorder.Body.String(), "其他用户") {
		t.Fatalf("expected ownership error, got body=%s", recorder.Body.String())
	}
}

func TestMountSubscriptionFailsWhenConfigQueryErrors(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db := setupSubscriptionHandlerTestDB(t)
	storageSvc := &mockSubscriptionStorageFacade{}
	handler := NewHandler(db, nil, nil, storageSvc, &mockSubscriptionCloudBridge{}, zap.NewNop(), nil)
	closeSubscriptionHandlerTestDB(t, db)

	router := gin.New()
	wrapper := httpcontext.NewHandlerFuncWrapper(zap.NewNop())
	router.POST("/mount", wrapper.Wrap(handler.MountSubscription()))

	req := httptest.NewRequestWithContext(
		stdctx.Background(),
		http.MethodPost,
		"/mount",
		strings.NewReader(`{"title":"测试资源","shareUrl":"https://cloud.189.cn/t/abc123","shareCode":"abc123"}`),
	)
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected bad request, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	if storageSvc.req != nil {
		t.Fatal("expected storage facade not to be called when config query fails")
	}

	if !strings.Contains(recorder.Body.String(), "读取订阅配置失败") {
		t.Fatalf("expected config query error, got body=%s", recorder.Body.String())
	}
}

func TestSearchPanFailsWhenConfigQueryErrors(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db := setupSubscriptionHandlerTestDB(t)
	handler := NewHandler(db, nil, nil, nil, nil, zap.NewNop(), nil)
	closeSubscriptionHandlerTestDB(t, db)

	router := gin.New()
	wrapper := httpcontext.NewHandlerFuncWrapper(zap.NewNop())
	router.GET("/search", wrapper.Wrap(handler.SearchPan()))

	req := httptest.NewRequestWithContext(stdctx.Background(), http.MethodGet, "/search?keyword=test", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected bad request, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	if !strings.Contains(recorder.Body.String(), "读取订阅配置失败") {
		t.Fatalf("expected config query error, got body=%s", recorder.Body.String())
	}
}

func TestSearchPanTreatsWrappedContextCancellationAsTimeout(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db := setupSubscriptionHandlerTestDB(t)
	if err := db.Create(&Setting{
		Name: "subscription_config",
		Value: models.SubscriptionConfig{
			PanSearchURL: "https://search.example.com/api/search",
		},
	}).Error; err != nil {
		t.Fatalf("create setting: %v", err)
	}

	handler := NewHandler(db, nil, nil, nil, nil, zap.NewNop(), nil)
	handler.httpClient = &http.Client{
		Transport: failingRoundTripper{err: fmt.Errorf("wrapped: %w", stdctx.Canceled)},
	}

	router := gin.New()
	wrapper := httpcontext.NewHandlerFuncWrapper(zap.NewNop())
	router.GET("/search", wrapper.Wrap(handler.SearchPan()))

	req := httptest.NewRequestWithContext(stdctx.Background(), http.MethodGet, "/search?keyword=test", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected bad request, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	if !strings.Contains(recorder.Body.String(), "请求超时，请稍后重试") {
		t.Fatalf("expected timeout error, got body=%s", recorder.Body.String())
	}
}

func TestSearchPanTreatsWrappedDeadlineExceededAsTimeout(t *testing.T) {
	if !isPanSearchTimeoutError(fmt.Errorf("wrapped: %w", stdctx.DeadlineExceeded)) {
		t.Fatal("expected wrapped deadline exceeded to be treated as timeout")
	}
}

func TestSearchPanDoesNotTreatPlainNetworkErrorAsTimeout(t *testing.T) {
	if isPanSearchTimeoutError(io.ErrUnexpectedEOF) {
		t.Fatal("expected plain network error not to be treated as timeout")
	}
}

func TestSearchPanWithAIRejectsOversizedResponse(t *testing.T) {
	gin.SetMode(gin.TestMode)

	searchServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(strings.Repeat("x", maxPanSearchResponseSize+1)))
	}))
	defer searchServer.Close()

	db := setupSubscriptionHandlerTestDB(t)
	if err := db.Create(&Setting{
		Name: "subscription_config",
		Value: models.SubscriptionConfig{
			PanSearchURL: searchServer.URL,
		},
	}).Error; err != nil {
		t.Fatalf("create setting: %v", err)
	}

	handler := NewHandler(db, nil, nil, nil, nil, zap.NewNop(), nil)

	router := gin.New()
	wrapper := httpcontext.NewHandlerFuncWrapper(zap.NewNop())
	router.GET("/search-ai", wrapper.Wrap(handler.SearchPanWithAI()))

	req := httptest.NewRequestWithContext(stdctx.Background(), http.MethodGet, "/search-ai?keyword=test", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected bad request, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	if !strings.Contains(recorder.Body.String(), "盘搜返回体过大") {
		t.Fatalf("expected oversized response error, got body=%s", recorder.Body.String())
	}
}
