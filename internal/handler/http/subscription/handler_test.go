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
	"github.com/xxcheng123/cloudpan189-share/internal/services/tmdb"
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

type mockSubscriptionTMDB struct {
	tmdb.Service
	config *tmdb.Config
	keys   []string
}

func (m *mockSubscriptionTMDB) GetConfig() *tmdb.Config {
	return m.config
}

func (m *mockSubscriptionTMDB) SetAPIKey(apiKey string) {
	m.keys = append(m.keys, apiKey)
	m.config.APIKey = apiKey
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

func TestUpdateConfigUpsertsWhenSettingIsCreatedConcurrently(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db := setupSubscriptionHandlerTestDB(t)
	handler := NewHandler(db, nil, nil, nil, nil, zap.NewNop(), nil)

	const callbackName = "subscription_test_create_setting_before_upsert"

	seeded := false

	if err := db.Callback().Create().Before("gorm:create").Register(callbackName, func(tx *gorm.DB) {
		if seeded || tx.Statement.Table != "system_settings" {
			return
		}

		seeded = true

		competing := &Setting{
			Name: "subscription_config",
			Value: models.SubscriptionConfig{
				PanSearchURL:   "https://raced.example.com/api/search",
				CronExpression: "0 1 * * *",
			},
		}
		if err := tx.Session(&gorm.Session{NewDB: true}).Create(competing).Error; err != nil {
			_ = tx.AddError(err)
		}
	}); err != nil {
		t.Fatalf("register create callback: %v", err)
	}

	t.Cleanup(func() {
		_ = db.Callback().Create().Remove(callbackName)
	})

	router := gin.New()
	wrapper := httpcontext.NewHandlerFuncWrapper(zap.NewNop())
	router.POST("/config", wrapper.Wrap(handler.UpdateConfig()))

	req := httptest.NewRequestWithContext(
		stdctx.Background(),
		http.MethodPost,
		"/config",
		strings.NewReader(`{"panSearchURL":"https://new.example.com/api/search","cronExpression":"0 3 * * *"}`),
	)
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected ok, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	if !seeded {
		t.Fatal("expected test callback to simulate concurrent setting creation")
	}

	var count int64
	if err := db.Model(&Setting{}).Where("name = ?", "subscription_config").Count(&count).Error; err != nil {
		t.Fatalf("count settings: %v", err)
	}

	if count != 1 {
		t.Fatalf("expected one setting after concurrent upsert, got %d", count)
	}

	var updated Setting
	if err := db.Where("name = ?", "subscription_config").First(&updated).Error; err != nil {
		t.Fatalf("query updated setting: %v", err)
	}

	if updated.Value.PanSearchURL != "https://new.example.com/api/search" {
		t.Fatalf("expected request value to win conflict, got %q", updated.Value.PanSearchURL)
	}

	if updated.Value.CronExpression != "0 3 * * *" {
		t.Fatalf("expected cron expression updated, got %q", updated.Value.CronExpression)
	}
}

func TestUpdateConfigReturnsNotFoundWhenExistingSettingDisappears(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db := setupSubscriptionHandlerTestDB(t)
	handler := NewHandler(db, nil, nil, nil, nil, zap.NewNop(), nil)

	existing := &Setting{
		Name: "subscription_config",
		Value: models.SubscriptionConfig{
			PanSearchURL: "https://old.example.com/api/search",
		},
	}
	if err := db.Create(existing).Error; err != nil {
		t.Fatalf("create setting: %v", err)
	}

	const callbackName = "subscription_test_delete_setting_before_update"

	deleted := false

	if err := db.Callback().Update().Before("gorm:update").Register(callbackName, func(tx *gorm.DB) {
		if deleted || tx.Statement.Table != "system_settings" {
			return
		}

		deleted = true

		if err := tx.Session(&gorm.Session{NewDB: true}).
			Exec("DELETE FROM system_settings WHERE id = ?", existing.ID).Error; err != nil {
			_ = tx.AddError(err)
		}
	}); err != nil {
		t.Fatalf("register update callback: %v", err)
	}

	t.Cleanup(func() {
		_ = db.Callback().Update().Remove(callbackName)
	})

	router := gin.New()
	wrapper := httpcontext.NewHandlerFuncWrapper(zap.NewNop())
	router.POST("/config", wrapper.Wrap(handler.UpdateConfig()))

	req := httptest.NewRequestWithContext(
		stdctx.Background(),
		http.MethodPost,
		"/config",
		strings.NewReader(`{"panSearchURL":"https://new.example.com/api/search"}`),
	)
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("expected not found, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	var response httpcontext.Response
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if response.Code != http.StatusNotFound {
		t.Fatalf("expected business code %d, got %d", http.StatusNotFound, response.Code)
	}

	if response.Msg != "订阅配置不存在" {
		t.Fatalf("expected not found message, got %q", response.Msg)
	}
}

func TestUpdateConfigClearsRuntimeTMDBAPIKey(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db := setupSubscriptionHandlerTestDB(t)
	tmdbSvc := &mockSubscriptionTMDB{config: &tmdb.Config{APIKey: "old-runtime-key"}}
	handler := NewHandler(db, tmdbSvc, nil, nil, nil, zap.NewNop(), nil)

	existing := &Setting{
		Name: "subscription_config",
		Value: models.SubscriptionConfig{
			EnableTMDB:       true,
			PanSearchURL:     "https://old.example.com/api/search",
			CronExpression:   "0 1 * * *",
			DefaultMountPath: "/old",
			TMDBAPIKey:       "old-db-key",
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
		strings.NewReader(`{"enableTMDB":true,"enableDouban":false,"panSearchURL":"https://new.example.com/api/search","defaultMountPath":"/new","autoMount":false,"cronExpression":"0 3 * * *","tmdbAPIKey":"","openaiAPIKey":"","openaiBaseURL":"https://api.openai.com","openaiModel":"gpt-4o-mini"}`),
	)
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected ok, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	var updated Setting
	if err := db.First(&updated, existing.ID).Error; err != nil {
		t.Fatalf("query updated setting: %v", err)
	}

	if updated.Value.TMDBAPIKey != "" {
		t.Fatalf("expected DB TMDB API key cleared, got %q", updated.Value.TMDBAPIKey)
	}

	if handler.tmdbAPIKey != "" {
		t.Fatalf("expected runtime TMDB API key cleared, got %q", handler.tmdbAPIKey)
	}

	if tmdbSvc.config.APIKey != "" {
		t.Fatalf("expected TMDB service API key cleared, got %q", tmdbSvc.config.APIKey)
	}

	if got := tmdbSvc.keys[len(tmdbSvc.keys)-1]; got != "" {
		t.Fatalf("expected last SetAPIKey call to clear key, got %q", got)
	}
}

func TestUpdateConfigPartialUpdatePreservesExistingValues(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db := setupSubscriptionHandlerTestDB(t)

	existing := &Setting{
		Name: "subscription_config",
		Value: models.SubscriptionConfig{
			EnableTMDB:       true,
			EnableDouban:     true,
			PanSearchURL:     "https://old.example.com/api/search",
			CronExpression:   "0 1 * * *",
			DefaultMountPath: "/old",
			AutoMount:        true,
			TMDBAPIKey:       "old-db-key",
			OpenAIAPIKey:     "old-openai-key",
			OpenAIBaseURL:    "https://old-openai.example.com",
			OpenAIModel:      "old-model",
		},
	}
	if err := db.Create(existing).Error; err != nil {
		t.Fatalf("create setting: %v", err)
	}

	tmdbSvc := &mockSubscriptionTMDB{config: &tmdb.Config{APIKey: "runtime-before"}}
	handler := NewHandler(db, tmdbSvc, nil, nil, nil, zap.NewNop(), nil)
	setAPIKeyCalls := len(tmdbSvc.keys)

	router := gin.New()
	wrapper := httpcontext.NewHandlerFuncWrapper(zap.NewNop())
	router.POST("/config", wrapper.Wrap(handler.UpdateConfig()))

	req := httptest.NewRequestWithContext(
		stdctx.Background(),
		http.MethodPost,
		"/config",
		strings.NewReader(`{"panSearchURL":"https://new.example.com/api/search"}`),
	)
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected ok, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	var updated Setting
	if err := db.First(&updated, existing.ID).Error; err != nil {
		t.Fatalf("query updated setting: %v", err)
	}

	if updated.Value.PanSearchURL != "https://new.example.com/api/search" {
		t.Fatalf("expected pan search URL updated, got %q", updated.Value.PanSearchURL)
	}

	if !updated.Value.EnableTMDB || !updated.Value.EnableDouban || !updated.Value.AutoMount {
		t.Fatalf("expected bool config values preserved, got %+v", updated.Value)
	}

	if updated.Value.DefaultMountPath != "/old" ||
		updated.Value.CronExpression != "0 1 * * *" ||
		updated.Value.TMDBAPIKey != "old-db-key" ||
		updated.Value.OpenAIAPIKey != "old-openai-key" ||
		updated.Value.OpenAIBaseURL != "https://old-openai.example.com" ||
		updated.Value.OpenAIModel != "old-model" {
		t.Fatalf("expected partial update to preserve existing config, got %+v", updated.Value)
	}

	if len(tmdbSvc.keys) != setAPIKeyCalls {
		t.Fatalf("expected omitted tmdbAPIKey not to resync runtime key, got calls %v", tmdbSvc.keys)
	}

	if handler.tmdbAPIKey != "old-db-key" {
		t.Fatalf("expected runtime handler TMDB key preserved, got %q", handler.tmdbAPIKey)
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

func TestMountSubscriptionRejectsRelativeMountPath(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db := setupSubscriptionHandlerTestDB(t)
	storageSvc := &mockSubscriptionStorageFacade{}
	handler := NewHandler(db, nil, nil, storageSvc, &mockSubscriptionCloudBridge{}, zap.NewNop(), nil)

	router := gin.New()
	wrapper := httpcontext.NewHandlerFuncWrapper(zap.NewNop())
	router.POST("/mount", wrapper.Wrap(handler.MountSubscription()))

	req := httptest.NewRequestWithContext(
		stdctx.Background(),
		http.MethodPost,
		"/mount",
		strings.NewReader(`{"title":"测试资源","shareUrl":"https://cloud.189.cn/t/abc123","shareCode":"abc123","mountPath":"热门订阅/测试资源"}`),
	)
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected bad request, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	if storageSvc.req != nil {
		t.Fatal("expected storage facade not to be called")
	}
}

func TestMountSubscriptionWithExplicitPathDoesNotReadConfig(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db := setupSubscriptionHandlerTestDB(t)
	storageSvc := &mockSubscriptionStorageFacade{}
	handler := NewHandler(db, nil, nil, storageSvc, &mockSubscriptionCloudBridge{}, zap.NewNop(), nil)
	closeSubscriptionHandlerTestDB(t, db)

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
		strings.NewReader(`{"title":"测试资源","shareUrl":"https://cloud.189.cn/t/abc123","shareCode":"abc123","mountPath":"/自定义/测试资源"}`),
	)
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected later DB query failure after skipping config read, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	if !strings.Contains(recorder.Body.String(), "查询挂载路径失败") {
		t.Fatalf("expected mount path query error, got body=%s", recorder.Body.String())
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

	if recorder.Code != http.StatusForbidden {
		t.Fatalf("expected forbidden, got %d body=%s", recorder.Code, recorder.Body.String())
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

func TestSearchPanWithAIReturnsAllResultsWhenAIServiceMissing(t *testing.T) {
	gin.SetMode(gin.TestMode)

	searchServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"code": 0,
			"message": "ok",
			"data": {
				"total": 1,
				"merged_by_type": {
					"tianyi": [{
						"url": "https://cloud.189.cn/t/abc123",
						"password": "p123",
						"note": "测试资源 4K",
						"datetime": "2026-05-22",
						"source": "unit-test",
						"images": ["https://example.com/cover.jpg"]
					}]
				}
			}
		}`))
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

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected ok, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	var response struct {
		Code int `json:"code"`
		Data struct {
			Message       string         `json:"message"`
			Keyword       string         `json:"keyword"`
			AIDescription string         `json:"aiDescription"`
			Result        *SearchResult  `json:"result"`
			AllResults    []SearchResult `json:"allResults"`
		} `json:"data"`
	}

	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if response.Code != http.StatusOK {
		t.Fatalf("expected business code 200, got %d", response.Code)
	}

	if response.Data.Result != nil {
		t.Fatalf("expected no AI-picked result, got %+v", response.Data.Result)
	}

	if response.Data.Message != "AI服务未配置，返回全部搜索结果" {
		t.Fatalf("unexpected message: %q", response.Data.Message)
	}

	if response.Data.Keyword != "test" {
		t.Fatalf("expected keyword test, got %q", response.Data.Keyword)
	}

	if response.Data.AIDescription != "" {
		t.Fatalf("expected empty AI description, got %q", response.Data.AIDescription)
	}

	if len(response.Data.AllResults) != 1 {
		t.Fatalf("expected one fallback result, got %+v", response.Data.AllResults)
	}

	got := response.Data.AllResults[0]
	if got.ShareCode != "abc123" || got.Name != "测试资源 4K" || got.Cover == "" {
		t.Fatalf("unexpected fallback result: %+v", got)
	}
}

func TestSearchPanWithAIReturnsStableShapeWhenNoResults(t *testing.T) {
	gin.SetMode(gin.TestMode)

	searchServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"code": 0,
			"message": "ok",
			"data": {
				"total": 0,
				"merged_by_type": {}
			}
		}`))
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

	req := httptest.NewRequestWithContext(stdctx.Background(), http.MethodGet, "/search-ai?keyword=empty", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected ok, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	var response struct {
		Code int                        `json:"code"`
		Data map[string]json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if response.Code != http.StatusOK {
		t.Fatalf("expected business code 200, got %d", response.Code)
	}

	for _, key := range []string{"message", "keyword", "aiDescription", "result", "allResults"} {
		if _, ok := response.Data[key]; !ok {
			t.Fatalf("expected response data to include %q, got keys %#v", key, response.Data)
		}
	}

	var message string
	if err := json.Unmarshal(response.Data["message"], &message); err != nil {
		t.Fatalf("decode message: %v", err)
	}

	if message != "未找到相关资源" {
		t.Fatalf("unexpected message: %q", message)
	}

	var keyword string
	if err := json.Unmarshal(response.Data["keyword"], &keyword); err != nil {
		t.Fatalf("decode keyword: %v", err)
	}

	if keyword != "empty" {
		t.Fatalf("expected keyword empty, got %q", keyword)
	}

	var aiDescription string
	if err := json.Unmarshal(response.Data["aiDescription"], &aiDescription); err != nil {
		t.Fatalf("decode ai description: %v", err)
	}

	if aiDescription != "" {
		t.Fatalf("expected empty AI description, got %q", aiDescription)
	}

	if string(response.Data["result"]) != "null" {
		t.Fatalf("expected result null, got %s", response.Data["result"])
	}

	var allResults []SearchResult
	if err := json.Unmarshal(response.Data["allResults"], &allResults); err != nil {
		t.Fatalf("decode all results: %v", err)
	}

	if allResults == nil || len(allResults) != 0 {
		t.Fatalf("expected empty allResults array, got %#v", allResults)
	}
}
