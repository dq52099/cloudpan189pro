package subscription

import (
	stdctx "context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/xxcheng123/cloudpan189-share/internal/consts"
	appContext "github.com/xxcheng123/cloudpan189-share/internal/framework/context"
	"github.com/xxcheng123/cloudpan189-share/internal/framework/httpcontext"
	"github.com/xxcheng123/cloudpan189-share/internal/pkgs/utils"
	"github.com/xxcheng123/cloudpan189-share/internal/repository/models"
	"github.com/xxcheng123/cloudpan189-share/internal/services/cloudbridge"
	"github.com/xxcheng123/cloudpan189-share/internal/services/douban"
	"github.com/xxcheng123/cloudpan189-share/internal/services/storagefacade"
	subscriptionSvc "github.com/xxcheng123/cloudpan189-share/internal/services/subscription"
	"github.com/xxcheng123/cloudpan189-share/internal/services/tmdb"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
	"gorm.io/gorm"
)

type mockSubscriptionStorageFacade struct {
	storagefacade.Service
	req *storagefacade.CreateStorageRequest
	err error
}

func (m *mockSubscriptionStorageFacade) CreateStorage(ctx appContext.Context, req *storagefacade.CreateStorageRequest) (int64, error) {
	m.req = req
	if m.err != nil {
		return 0, m.err
	}

	return 12345, nil
}

type mockSubscriptionCloudBridge struct {
	cloudbridge.Service
	shareCodes  []string
	accessCodes []string
	err         error
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

type tmdbHandlerContextMarkerKey struct{}

type contextAwareMockSubscriptionTMDB struct {
	tmdb.Service
	contextValue  string
	contextCalled bool
	legacyCalled  bool
}

func (m *contextAwareMockSubscriptionTMDB) GetPopularMoviesWithContext(ctx stdctx.Context, _ int) ([]tmdb.Movie, error) {
	m.contextCalled = true
	if value, ok := ctx.Value(tmdbHandlerContextMarkerKey{}).(string); ok {
		m.contextValue = value
	}

	return []tmdb.Movie{{ID: 1, Title: "测试电影", VoteAverage: 8.5, ReleaseDate: "2024-01-01"}}, nil
}

func (m *contextAwareMockSubscriptionTMDB) GetPopularTVsWithContext(ctx stdctx.Context, _ int) ([]tmdb.TV, error) {
	m.contextCalled = true
	if value, ok := ctx.Value(tmdbHandlerContextMarkerKey{}).(string); ok {
		m.contextValue = value
	}

	return []tmdb.TV{{ID: 2, Name: "测试剧集", VoteAverage: 9.0, FirstAirDate: "2024-01-01"}}, nil
}

func (m *contextAwareMockSubscriptionTMDB) GetMoviesByGenreWithContext(ctx stdctx.Context, _ int, _ int) ([]tmdb.Movie, error) {
	m.contextCalled = true
	if value, ok := ctx.Value(tmdbHandlerContextMarkerKey{}).(string); ok {
		m.contextValue = value
	}

	return []tmdb.Movie{{ID: 3, Title: "测试类型电影", VoteAverage: 8.0, ReleaseDate: "2024-01-01"}}, nil
}

func (m *contextAwareMockSubscriptionTMDB) GetPopularMovies(_ int) ([]tmdb.Movie, error) {
	m.legacyCalled = true

	return []tmdb.Movie{{ID: 1, Title: "测试电影", VoteAverage: 8.5, ReleaseDate: "2024-01-01"}}, nil
}

func (m *contextAwareMockSubscriptionTMDB) GetPopularTVs(_ int) ([]tmdb.TV, error) {
	m.legacyCalled = true

	return []tmdb.TV{{ID: 2, Name: "测试剧集", VoteAverage: 9.0, FirstAirDate: "2024-01-01"}}, nil
}

func (m *contextAwareMockSubscriptionTMDB) GetMoviesByGenre(_ int, _ int) ([]tmdb.Movie, error) {
	m.legacyCalled = true

	return []tmdb.Movie{{ID: 3, Title: "测试类型电影", VoteAverage: 8.0, ReleaseDate: "2024-01-01"}}, nil
}

type mockSubscriptionDouban struct {
	douban.Service
	calls int32
}

func (m *mockSubscriptionDouban) GetMoviesByTag(tag string) ([]douban.Subject, error) {
	atomic.AddInt32(&m.calls, 1)

	return nil, nil
}

type doubanHandlerContextMarkerKey struct{}

type contextAwareMockSubscriptionDouban struct {
	douban.Service
	contextValue  string
	contextCalled bool
	legacyCalled  bool
}

func (m *contextAwareMockSubscriptionDouban) GetMoviesByTagWithContext(ctx stdctx.Context, _ string) ([]douban.Subject, error) {
	m.contextCalled = true
	if value, ok := ctx.Value(doubanHandlerContextMarkerKey{}).(string); ok {
		m.contextValue = value
	}

	return []douban.Subject{{ID: "1234567", Title: "测试电影", Year: "2024", Rating: 8.5}}, nil
}

func (m *contextAwareMockSubscriptionDouban) GetMoviesByTag(_ string) ([]douban.Subject, error) {
	m.legacyCalled = true

	return []douban.Subject{{ID: "1234567", Title: "测试电影", Year: "2024", Rating: 8.5}}, nil
}

type mockSubscriptionRuntimeConfigUpdater struct {
	configs []subscriptionSvc.SubscriptionConfig
}

func (m *mockSubscriptionRuntimeConfigUpdater) UpdateConfig(config subscriptionSvc.SubscriptionConfig) {
	m.configs = append(m.configs, config)
}

type failingRoundTripper struct {
	err error
}

func (t failingRoundTripper) RoundTrip(*http.Request) (*http.Response, error) {
	return nil, t.err
}

type requestURLFailingRoundTripper struct{}

func (requestURLFailingRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	return nil, fmt.Errorf("Get %q: accessCode=abcd", req.URL.String())
}

type panSearchReadErrorBody struct {
	err error
}

func (b panSearchReadErrorBody) Read([]byte) (int, error) {
	return 0, b.err
}

func (b panSearchReadErrorBody) Close() error {
	return nil
}

type panSearchReadErrorRoundTripper struct{}

func (panSearchReadErrorRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{},
		Body: panSearchReadErrorBody{
			err: fmt.Errorf(`read failed from %q Authorization: Bearer bearer-secret accessCode=abcd`, req.URL.String()),
		},
		Request: req,
	}, nil
}

type mockSubscriptionOpenAI struct {
	keyword string
	err     error
}

func (m mockSubscriptionOpenAI) GenerateUpgradeKeyword(string, string) (string, error) {
	if m.err != nil {
		return "", m.err
	}

	return m.keyword, nil
}

type subscriptionOpenAIContextMarkerKey struct{}

type contextAwareMockSubscriptionOpenAI struct {
	keyword       string
	contextValue  string
	contextCalled bool
	legacyCalled  bool
}

func (m *contextAwareMockSubscriptionOpenAI) GenerateUpgradeKeywordWithContext(ctx stdctx.Context, _ string, _ string) (string, error) {
	m.contextCalled = true
	if value, ok := ctx.Value(subscriptionOpenAIContextMarkerKey{}).(string); ok {
		m.contextValue = value
	}

	return m.keyword, nil
}

func (m *contextAwareMockSubscriptionOpenAI) GenerateUpgradeKeyword(_ string, _ string) (string, error) {
	m.legacyCalled = true

	return m.keyword, nil
}

func TestGenerateUpgradeKeywordUsesContextAwareOpenAIService(t *testing.T) {
	openaiSvc := &contextAwareMockSubscriptionOpenAI{keyword: "测试电影 2024 4K"}
	handler := &Handler{openaiSvc: openaiSvc}
	ctx := stdctx.WithValue(stdctx.Background(), subscriptionOpenAIContextMarkerKey{}, "request-marker")

	keyword, err := handler.generateUpgradeKeyword(ctx, "测试电影", "movie")
	if err != nil {
		t.Fatalf("generate keyword: %v", err)
	}

	if keyword != "测试电影 2024 4K" {
		t.Fatalf("expected keyword from context-aware service, got %q", keyword)
	}

	if !openaiSvc.contextCalled {
		t.Fatal("expected context-aware OpenAI method to be called")
	}

	if openaiSvc.legacyCalled {
		t.Fatal("expected legacy OpenAI method not to be called when context-aware method exists")
	}

	if openaiSvc.contextValue != "request-marker" {
		t.Fatalf("expected request context marker to propagate, got %q", openaiSvc.contextValue)
	}
}

func (m *mockSubscriptionCloudBridge) GetShareInfo(ctx appContext.Context, shareCode string, accessCode string) (*cloudbridge.ShareInfo, error) {
	m.shareCodes = append(m.shareCodes, shareCode)

	m.accessCodes = append(m.accessCodes, accessCode)
	if m.err != nil {
		return nil, m.err
	}

	return &cloudbridge.ShareInfo{
		Name:       "share-name",
		IsFolder:   true,
		AccessCode: "share-access-code",
		ShareId:    67890,
		ID:         "share-file-id",
	}, nil
}

func assertSubscriptionAdditionValue(t *testing.T, addition map[string]interface{}, key string, want interface{}) {
	t.Helper()

	got, ok := addition[key]
	if !ok {
		t.Fatalf("expected addition %q to exist, got %#v", key, addition)
	}

	if got != want {
		t.Fatalf("expected addition %q = %#v (%T), got %#v (%T)", key, want, want, got, got)
	}
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

func setupSubscriptionHandlerFileTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "subscription-handler.db")), &gorm.Config{})
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get sql db: %v", err)
	}

	sqlDB.SetMaxOpenConns(5)

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

func TestNewHandlerTreatsTypedNilTMDBServiceAsMissing(t *testing.T) {
	db := setupSubscriptionHandlerTestDB(t)
	if err := db.Create(&Setting{
		Name: subscriptionConfigName,
		Value: models.SubscriptionConfig{
			TMDBAPIKey: "stored-tmdb-key",
		},
	}).Error; err != nil {
		t.Fatalf("create setting: %v", err)
	}

	var tmdbSvc *mockSubscriptionTMDB

	handler := NewHandler(db, tmdbSvc, nil, nil, nil, zap.NewNop(), nil)
	if handler.tmdbAPIKey != "stored-tmdb-key" {
		t.Fatalf("expected stored TMDB API key to be preserved, got %q", handler.tmdbAPIKey)
	}
}

func TestTMDBHandlersTreatTypedNilServiceAsMissing(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name        string
		path        string
		register    func(*gin.Engine, *Handler, *httpcontext.HandlerFuncWrapper)
		bodySnippet string
	}{
		{
			name: "movies",
			path: "/movies",
			register: func(router *gin.Engine, handler *Handler, wrapper *httpcontext.HandlerFuncWrapper) {
				router.GET("/movies", wrapper.Wrap(handler.GetTMDbMovies()))
			},
			bodySnippet: "TMDB 服务未初始化",
		},
		{
			name: "tvs",
			path: "/tvs",
			register: func(router *gin.Engine, handler *Handler, wrapper *httpcontext.HandlerFuncWrapper) {
				router.GET("/tvs", wrapper.Wrap(handler.GetTMDbTVs()))
			},
			bodySnippet: "TMDB 服务未初始化",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := setupSubscriptionHandlerTestDB(t)
			handler := NewHandler(db, nil, nil, nil, nil, zap.NewNop(), nil)

			var tmdbSvc *mockSubscriptionTMDB

			handler.tmdb = tmdbSvc

			router := gin.New()
			wrapper := httpcontext.NewHandlerFuncWrapper(zap.NewNop())
			tt.register(router, handler, wrapper)

			req := httptest.NewRequestWithContext(stdctx.Background(), http.MethodGet, tt.path, nil)
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, req)

			if recorder.Code == http.StatusOK {
				t.Fatalf("expected failure response, got ok body=%s", recorder.Body.String())
			}

			if !strings.Contains(recorder.Body.String(), tt.bodySnippet) {
				t.Fatalf("expected body to contain %q, got %s", tt.bodySnippet, recorder.Body.String())
			}
		})
	}
}

func TestGetTMDbMoviesUsesRequestContextAwareService(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tmdbSvc := &contextAwareMockSubscriptionTMDB{}
	handler := &Handler{
		tmdb:   tmdbSvc,
		logger: zap.NewNop(),
	}

	router := gin.New()
	wrapper := httpcontext.NewHandlerFuncWrapper(zap.NewNop())
	router.GET("/movies", wrapper.Wrap(handler.GetTMDbMovies()))

	baseCtx := stdctx.WithValue(stdctx.Background(), tmdbHandlerContextMarkerKey{}, "request-marker")
	req := httptest.NewRequestWithContext(baseCtx, http.MethodGet, "/movies?category=movie_popular", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected ok, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	if !tmdbSvc.contextCalled {
		t.Fatal("expected context-aware TMDB method to be called")
	}

	if tmdbSvc.legacyCalled {
		t.Fatal("expected legacy TMDB method not to be called when context-aware method exists")
	}

	if tmdbSvc.contextValue != "request-marker" {
		t.Fatalf("expected request context marker to propagate, got %q", tmdbSvc.contextValue)
	}
}

func TestDoubanHandlerTreatsTypedNilServiceAsMissing(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db := setupSubscriptionHandlerTestDB(t)
	handler := NewHandler(db, nil, nil, nil, nil, zap.NewNop(), nil)

	var doubanSvc *mockSubscriptionDouban

	handler.douban = doubanSvc

	router := gin.New()
	wrapper := httpcontext.NewHandlerFuncWrapper(zap.NewNop())
	router.GET("/douban", wrapper.Wrap(handler.GetDoubanMovies()))

	req := httptest.NewRequestWithContext(stdctx.Background(), http.MethodGet, "/douban", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code == http.StatusOK {
		t.Fatalf("expected failure response, got ok body=%s", recorder.Body.String())
	}

	if !strings.Contains(recorder.Body.String(), "豆瓣服务未初始化") {
		t.Fatalf("expected missing Douban service response, got %s", recorder.Body.String())
	}
}

func TestGetDoubanMoviesUsesRequestContextAwareService(t *testing.T) {
	gin.SetMode(gin.TestMode)

	doubanSvc := &contextAwareMockSubscriptionDouban{}
	handler := &Handler{
		douban: doubanSvc,
		logger: zap.NewNop(),
	}

	router := gin.New()
	wrapper := httpcontext.NewHandlerFuncWrapper(zap.NewNop())
	router.GET("/douban", wrapper.Wrap(handler.GetDoubanMovies()))

	baseCtx := stdctx.WithValue(stdctx.Background(), doubanHandlerContextMarkerKey{}, "request-marker")
	req := httptest.NewRequestWithContext(baseCtx, http.MethodGet, "/douban?category=热门", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected ok, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	if !doubanSvc.contextCalled {
		t.Fatal("expected context-aware Douban method to be called")
	}

	if doubanSvc.legacyCalled {
		t.Fatal("expected legacy Douban method not to be called when context-aware method exists")
	}

	if doubanSvc.contextValue != "request-marker" {
		t.Fatalf("expected request context marker to propagate, got %q", doubanSvc.contextValue)
	}
}

func TestSyncRuntimeSubscriptionConfigTreatsTypedNilRuntimeAsMissing(t *testing.T) {
	var runtimeSvc *mockSubscriptionRuntimeConfigUpdater

	syncRuntimeSubscriptionConfig(runtimeSvc, models.SubscriptionConfig{
		Enabled:      true,
		PanSearchURL: "https://example.com/api/search",
	})
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

	if response.Data.Enabled {
		t.Fatal("expected default subscription scheduler to be disabled")
	}

	var count int64
	if err := db.Model(&Setting{}).Where("name = ?", "subscription_config").Count(&count).Error; err != nil {
		t.Fatalf("count settings: %v", err)
	}

	if count != 0 {
		t.Fatalf("expected GET config not to create setting, got count %d", count)
	}
}

func TestGetConfigMasksStoredAPIKeys(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db := setupSubscriptionHandlerTestDB(t)

	rawPanSearchURL := "https://search-user:search-pass@search.example.com/api/search?token=secret-token&source=custom#access_token=secret-fragment"
	rawOpenAIBaseURL := "https://openai-user:openai-pass@openai.example.com/v1?client_secret=secret-client#token=secret-openai-fragment"

	existing := &Setting{
		Name: subscriptionConfigName,
		Value: models.SubscriptionConfig{
			EnableTMDB:       true,
			EnableDouban:     true,
			PanSearchURL:     rawPanSearchURL,
			CronExpression:   "0 1 * * *",
			DefaultMountPath: "/old",
			TMDBAPIKey:       "stored-tmdb-key",
			OpenAIAPIKey:     "stored-openai-key",
			OpenAIBaseURL:    rawOpenAIBaseURL,
			OpenAIModel:      "model",
		},
	}
	if err := db.Create(existing).Error; err != nil {
		t.Fatalf("create setting: %v", err)
	}

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

	if strings.Contains(recorder.Body.String(), "stored-tmdb-key") ||
		strings.Contains(recorder.Body.String(), "stored-openai-key") {
		t.Fatalf("expected API keys to be masked in response, got %s", recorder.Body.String())
	}

	for _, leaked := range []string{
		"search-user",
		"search-pass",
		"secret-token",
		"secret-fragment",
		"openai-user",
		"openai-pass",
		"secret-client",
		"secret-openai-fragment",
	} {
		if strings.Contains(recorder.Body.String(), leaked) {
			t.Fatalf("expected %q to be redacted, got body=%s", leaked, recorder.Body.String())
		}
	}

	var response struct {
		Code int                `json:"code"`
		Data SubscriptionConfig `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}

	if response.Data.TMDBAPIKey != utils.RedactedSecret {
		t.Fatalf("expected masked TMDB API key, got %q", response.Data.TMDBAPIKey)
	}

	if response.Data.OpenAIAPIKey != utils.RedactedSecret {
		t.Fatalf("expected masked OpenAI API key, got %q", response.Data.OpenAIAPIKey)
	}

	if response.Data.PanSearchURL != utils.RedactURLForLog(rawPanSearchURL) {
		t.Fatalf("expected redacted pan search URL, got %q", response.Data.PanSearchURL)
	}

	if response.Data.OpenAIBaseURL != utils.RedactURLForLog(rawOpenAIBaseURL) {
		t.Fatalf("expected redacted OpenAI base URL, got %q", response.Data.OpenAIBaseURL)
	}

	var stored Setting
	if err := db.First(&stored, existing.ID).Error; err != nil {
		t.Fatalf("query stored setting: %v", err)
	}

	if stored.Value.TMDBAPIKey != "stored-tmdb-key" || stored.Value.OpenAIAPIKey != "stored-openai-key" {
		t.Fatalf("expected stored API keys unchanged, got %+v", stored.Value)
	}

	if stored.Value.PanSearchURL != rawPanSearchURL || stored.Value.OpenAIBaseURL != rawOpenAIBaseURL {
		t.Fatalf("expected stored URLs unchanged, got %+v", stored.Value)
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

func TestUpdateConfigSynchronizesRuntimeSubscription(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db := setupSubscriptionHandlerTestDB(t)
	runtimeSvc := &mockSubscriptionRuntimeConfigUpdater{}
	handler := NewHandler(db, nil, nil, nil, nil, zap.NewNop(), nil, runtimeSvc)

	router := gin.New()
	wrapper := httpcontext.NewHandlerFuncWrapper(zap.NewNop())
	router.POST("/config", wrapper.Wrap(handler.UpdateConfig()))

	req := httptest.NewRequestWithContext(
		stdctx.Background(),
		http.MethodPost,
		"/config",
		strings.NewReader(`{"enabled":true,"enableTMDB":false,"enableDouban":false,"panSearchURL":"https://runtime.example.com/api/search","cronExpression":"0 4 * * *"}`),
	)
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected ok, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	if len(runtimeSvc.configs) != 1 {
		t.Fatalf("expected one runtime config sync, got %d", len(runtimeSvc.configs))
	}

	got := runtimeSvc.configs[0]
	if !got.Enabled {
		t.Fatal("expected runtime enabled config to be true")
	}

	if got.EnableTMDB {
		t.Fatal("expected runtime TMDB config to be false")
	}

	if got.EnableDouban {
		t.Fatal("expected runtime Douban config to be false")
	}

	if got.PanSearchURL != "https://runtime.example.com/api/search" {
		t.Fatalf("expected runtime pan search URL updated, got %q", got.PanSearchURL)
	}

	if got.CronExpression != "0 4 * * *" {
		t.Fatalf("expected runtime cron expression updated, got %q", got.CronExpression)
	}
}

func TestUpdateConfigRejectsInvalidConfigFieldsBeforeWriting(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name string
		body string
	}{
		{
			name: "invalid pan search url",
			body: `{"panSearchURL":"ftp://search.example.com/api/search"}`,
		},
		{
			name: "relative default mount path",
			body: `{"defaultMountPath":"热门订阅"}`,
		},
		{
			name: "invalid cron expression",
			body: `{"cronExpression":"not a cron"}`,
		},
		{
			name: "invalid openai base url",
			body: `{"openaiBaseURL":"mailto:api@example.com"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := setupSubscriptionHandlerTestDB(t)
			handler := NewHandler(db, nil, nil, nil, nil, zap.NewNop(), nil)

			router := gin.New()
			wrapper := httpcontext.NewHandlerFuncWrapper(zap.NewNop())
			router.POST("/config", wrapper.Wrap(handler.UpdateConfig()))

			req := httptest.NewRequestWithContext(stdctx.Background(), http.MethodPost, "/config", strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/json")

			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, req)

			if recorder.Code != http.StatusBadRequest {
				t.Fatalf("expected bad request, got %d body=%s", recorder.Code, recorder.Body.String())
			}

			var count int64
			if err := db.Model(&Setting{}).Where("name = ?", subscriptionConfigName).Count(&count).Error; err != nil {
				t.Fatalf("count settings: %v", err)
			}

			if count != 0 {
				t.Fatalf("expected invalid config not to create setting, got count %d", count)
			}
		})
	}
}

func TestUpdateConfigNormalizesOptionalConfigFields(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db := setupSubscriptionHandlerTestDB(t)
	handler := NewHandler(db, nil, nil, nil, nil, zap.NewNop(), nil)

	router := gin.New()
	wrapper := httpcontext.NewHandlerFuncWrapper(zap.NewNop())
	router.POST("/config", wrapper.Wrap(handler.UpdateConfig()))

	req := httptest.NewRequestWithContext(
		stdctx.Background(),
		http.MethodPost,
		"/config",
		strings.NewReader(`{"panSearchURL":" https://new.example.com/api/search ","defaultMountPath":" /new ","cronExpression":" 0 3 * * * ","openaiBaseURL":" https://api.openai.com/v1 "}`),
	)
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected ok, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	var updated Setting
	if err := db.Where("name = ?", subscriptionConfigName).First(&updated).Error; err != nil {
		t.Fatalf("query updated setting: %v", err)
	}

	if updated.Value.PanSearchURL != "https://new.example.com/api/search" {
		t.Fatalf("expected trimmed pan search URL, got %q", updated.Value.PanSearchURL)
	}

	if updated.Value.DefaultMountPath != "/new" {
		t.Fatalf("expected trimmed default mount path, got %q", updated.Value.DefaultMountPath)
	}

	if updated.Value.CronExpression != "0 3 * * *" {
		t.Fatalf("expected trimmed cron expression, got %q", updated.Value.CronExpression)
	}

	if updated.Value.OpenAIBaseURL != "https://api.openai.com/v1" {
		t.Fatalf("expected trimmed OpenAI base URL, got %q", updated.Value.OpenAIBaseURL)
	}
}

func TestUpdateConfigAllowsClearingOptionalConfigFields(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db := setupSubscriptionHandlerTestDB(t)
	handler := NewHandler(db, nil, nil, nil, nil, zap.NewNop(), nil)

	existing := &Setting{
		Name: subscriptionConfigName,
		Value: models.SubscriptionConfig{
			PanSearchURL:     "https://old.example.com/api/search",
			CronExpression:   "0 1 * * *",
			DefaultMountPath: "/old",
			OpenAIBaseURL:    "https://old-openai.example.com",
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
		strings.NewReader(`{"panSearchURL":"","defaultMountPath":"","cronExpression":"","openaiBaseURL":""}`),
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

	if updated.Value.PanSearchURL != "" ||
		updated.Value.DefaultMountPath != "" ||
		updated.Value.CronExpression != "" ||
		updated.Value.OpenAIBaseURL != "" {
		t.Fatalf("expected optional config fields cleared, got %+v", updated.Value)
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

func TestUpdateConfigPreservesAPIKeysWhenMaskedPlaceholderSubmitted(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db := setupSubscriptionHandlerTestDB(t)

	existing := &Setting{
		Name: subscriptionConfigName,
		Value: models.SubscriptionConfig{
			EnableTMDB:       true,
			EnableDouban:     true,
			PanSearchURL:     "https://old.example.com/api/search",
			CronExpression:   "0 1 * * *",
			DefaultMountPath: "/old",
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

	body := fmt.Sprintf(
		`{"panSearchURL":"https://new.example.com/api/search","tmdbAPIKey":%q,"openaiAPIKey":%q}`,
		utils.RedactedSecret,
		utils.RedactedSecret,
	)
	req := httptest.NewRequestWithContext(stdctx.Background(), http.MethodPost, "/config", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected ok, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	if strings.Contains(recorder.Body.String(), "old-db-key") ||
		strings.Contains(recorder.Body.String(), "old-openai-key") {
		t.Fatalf("expected API keys to be masked in update response, got %s", recorder.Body.String())
	}

	var response struct {
		Code int                `json:"code"`
		Data SubscriptionConfig `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}

	if response.Data.TMDBAPIKey != utils.RedactedSecret || response.Data.OpenAIAPIKey != utils.RedactedSecret {
		t.Fatalf("expected masked API keys in response, got %+v", response.Data)
	}

	var updated Setting
	if err := db.First(&updated, existing.ID).Error; err != nil {
		t.Fatalf("query updated setting: %v", err)
	}

	if updated.Value.PanSearchURL != "https://new.example.com/api/search" {
		t.Fatalf("expected pan search URL updated, got %q", updated.Value.PanSearchURL)
	}

	if updated.Value.TMDBAPIKey != "old-db-key" || updated.Value.OpenAIAPIKey != "old-openai-key" {
		t.Fatalf("expected masked placeholders to preserve API keys, got %+v", updated.Value)
	}

	if len(tmdbSvc.keys) != setAPIKeyCalls {
		t.Fatalf("expected masked TMDB API key not to resync runtime key, got calls %v", tmdbSvc.keys)
	}
}

func TestUpdateConfigPreservesURLWhenRedactedPlaceholderSubmitted(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db := setupSubscriptionHandlerTestDB(t)

	rawPanSearchURL := "https://search-user:search-pass@search.example.com/api/search?token=secret-token&source=custom#access_token=secret-fragment"
	rawOpenAIBaseURL := "https://openai-user:openai-pass@openai.example.com/v1?client_secret=secret-client#token=secret-openai-fragment"

	existing := &Setting{
		Name: subscriptionConfigName,
		Value: models.SubscriptionConfig{
			EnableTMDB:       true,
			EnableDouban:     true,
			PanSearchURL:     rawPanSearchURL,
			CronExpression:   "0 1 * * *",
			DefaultMountPath: "/old",
			OpenAIBaseURL:    rawOpenAIBaseURL,
			OpenAIModel:      "old-model",
		},
	}
	if err := db.Create(existing).Error; err != nil {
		t.Fatalf("create setting: %v", err)
	}

	handler := NewHandler(db, nil, nil, nil, nil, zap.NewNop(), nil)
	router := gin.New()
	wrapper := httpcontext.NewHandlerFuncWrapper(zap.NewNop())
	router.POST("/config", wrapper.Wrap(handler.UpdateConfig()))

	body, err := json.Marshal(map[string]interface{}{
		"panSearchURL":     utils.RedactURLForLog(rawPanSearchURL),
		"openaiBaseURL":    utils.RedactURLForLog(rawOpenAIBaseURL),
		"defaultMountPath": "/new",
	})
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}

	req := httptest.NewRequestWithContext(stdctx.Background(), http.MethodPost, "/config", strings.NewReader(string(body)))
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected ok, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	for _, leaked := range []string{
		"search-user",
		"search-pass",
		"secret-token",
		"secret-fragment",
		"openai-user",
		"openai-pass",
		"secret-client",
		"secret-openai-fragment",
	} {
		if strings.Contains(recorder.Body.String(), leaked) {
			t.Fatalf("expected %q to be redacted, got body=%s", leaked, recorder.Body.String())
		}
	}

	var updated Setting
	if err := db.First(&updated, existing.ID).Error; err != nil {
		t.Fatalf("query updated setting: %v", err)
	}

	if updated.Value.PanSearchURL != rawPanSearchURL {
		t.Fatalf("expected pan search URL preserved, got %q", updated.Value.PanSearchURL)
	}

	if updated.Value.OpenAIBaseURL != rawOpenAIBaseURL {
		t.Fatalf("expected OpenAI base URL preserved, got %q", updated.Value.OpenAIBaseURL)
	}

	if updated.Value.DefaultMountPath != "/new" {
		t.Fatalf("expected default mount path updated, got %q", updated.Value.DefaultMountPath)
	}
}

func TestUpdateConfigResponseKeepsRuntimeTMDBKeyConfiguredWhenStoredKeyEmpty(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db := setupSubscriptionHandlerTestDB(t)

	existing := &Setting{
		Name: subscriptionConfigName,
		Value: models.SubscriptionConfig{
			EnableTMDB:       true,
			EnableDouban:     true,
			PanSearchURL:     "https://old.example.com/api/search",
			CronExpression:   "0 1 * * *",
			DefaultMountPath: "/old",
		},
	}
	if err := db.Create(existing).Error; err != nil {
		t.Fatalf("create setting: %v", err)
	}

	tmdbSvc := &mockSubscriptionTMDB{config: &tmdb.Config{APIKey: "runtime-tmdb-key"}}
	handler := NewHandler(db, tmdbSvc, nil, nil, nil, zap.NewNop(), nil)

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

	if strings.Contains(recorder.Body.String(), "runtime-tmdb-key") {
		t.Fatalf("expected runtime TMDB key to be masked in update response, got %s", recorder.Body.String())
	}

	var response struct {
		Code int                `json:"code"`
		Data SubscriptionConfig `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}

	if response.Data.TMDBAPIKey != utils.RedactedSecret {
		t.Fatalf("expected masked runtime TMDB key in response, got %q", response.Data.TMDBAPIKey)
	}

	var updated Setting
	if err := db.First(&updated, existing.ID).Error; err != nil {
		t.Fatalf("query updated setting: %v", err)
	}

	if updated.Value.TMDBAPIKey != "" {
		t.Fatalf("expected stored TMDB API key to remain empty, got %q", updated.Value.TMDBAPIKey)
	}

	if handler.tmdbAPIKey != "runtime-tmdb-key" {
		t.Fatalf("expected runtime handler TMDB key preserved, got %q", handler.tmdbAPIKey)
	}
}

func TestUpdateConfigFirstSaveWithMaskedPlaceholderKeepsRuntimeTMDBKey(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db := setupSubscriptionHandlerTestDB(t)
	tmdbSvc := &mockSubscriptionTMDB{config: &tmdb.Config{APIKey: "runtime-tmdb-key"}}
	handler := NewHandler(db, tmdbSvc, nil, nil, nil, zap.NewNop(), nil)

	router := gin.New()
	wrapper := httpcontext.NewHandlerFuncWrapper(zap.NewNop())
	router.POST("/config", wrapper.Wrap(handler.UpdateConfig()))

	body := fmt.Sprintf(
		`{"panSearchURL":"https://new.example.com/api/search","tmdbAPIKey":%q}`,
		utils.RedactedSecret,
	)
	req := httptest.NewRequestWithContext(stdctx.Background(), http.MethodPost, "/config", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected ok, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	if strings.Contains(recorder.Body.String(), "runtime-tmdb-key") {
		t.Fatalf("expected runtime TMDB key to be masked in update response, got %s", recorder.Body.String())
	}

	var response struct {
		Code int                `json:"code"`
		Data SubscriptionConfig `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}

	if response.Data.TMDBAPIKey != utils.RedactedSecret {
		t.Fatalf("expected masked runtime TMDB key in response, got %q", response.Data.TMDBAPIKey)
	}

	var created Setting
	if err := db.Where("name = ?", subscriptionConfigName).First(&created).Error; err != nil {
		t.Fatalf("query created setting: %v", err)
	}

	if created.Value.TMDBAPIKey != "runtime-tmdb-key" {
		t.Fatalf("expected first save to keep runtime TMDB API key, got %q", created.Value.TMDBAPIKey)
	}

	if created.Value.PanSearchURL != "https://new.example.com/api/search" {
		t.Fatalf("expected pan search URL updated, got %q", created.Value.PanSearchURL)
	}
}

func TestUpdateConfigConcurrentPartialUpdatesPreserveFields(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db := setupSubscriptionHandlerFileTestDB(t)

	existing := &Setting{
		Name: subscriptionConfigName,
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

	handler := NewHandler(db, nil, nil, nil, nil, zap.NewNop(), nil)

	const updateCallbackName = "subscription_test_pause_first_config_update"

	const queryCallbackName = "subscription_test_detect_unlocked_config_read"

	firstUpdateReached := make(chan struct{})
	releaseFirstUpdate := make(chan struct{})
	staleReadDuringUpdate := make(chan struct{})

	var firstUpdatePaused atomic.Bool

	var firstUpdateReleased atomic.Bool

	var staleReadDetected atomic.Bool

	defer func() {
		if firstUpdateReleased.CompareAndSwap(false, true) {
			close(releaseFirstUpdate)
		}
	}()

	if err := db.Callback().Update().Before("gorm:update").Register(updateCallbackName, func(tx *gorm.DB) {
		if !isSubscriptionSettingTestStatement(tx) {
			return
		}

		if !firstUpdatePaused.CompareAndSwap(false, true) {
			return
		}

		close(firstUpdateReached)
		<-releaseFirstUpdate
	}); err != nil {
		t.Fatalf("register update callback: %v", err)
	}

	t.Cleanup(func() {
		_ = db.Callback().Update().Remove(updateCallbackName)
	})

	if err := db.Callback().Query().Before("gorm:query").Register(queryCallbackName, func(tx *gorm.DB) {
		if !isSubscriptionSettingTestStatement(tx) {
			return
		}

		if firstUpdatePaused.Load() && !firstUpdateReleased.Load() && staleReadDetected.CompareAndSwap(false, true) {
			close(staleReadDuringUpdate)
		}
	}); err != nil {
		t.Fatalf("register query callback: %v", err)
	}

	t.Cleanup(func() {
		_ = db.Callback().Query().Remove(queryCallbackName)
	})

	router := gin.New()
	wrapper := httpcontext.NewHandlerFuncWrapper(zap.NewNop())
	router.POST("/config", wrapper.Wrap(handler.UpdateConfig()))

	postConfig := func(body string) subscriptionConfigTestResponse {
		req := httptest.NewRequestWithContext(stdctx.Background(), http.MethodPost, "/config", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")

		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, req)

		return subscriptionConfigTestResponse{code: recorder.Code, body: recorder.Body.String()}
	}

	firstDone := make(chan subscriptionConfigTestResponse, 1)
	go func() {
		firstDone <- postConfig(`{"panSearchURL":"https://new.example.com/api/search"}`)
	}()

	select {
	case <-firstUpdateReached:
	case <-time.After(2 * time.Second):
		t.Fatal("expected first update to reach update callback")
	}

	secondDone := make(chan subscriptionConfigTestResponse, 1)
	go func() {
		secondDone <- postConfig(`{"cronExpression":"0 3 * * *"}`)
	}()

	select {
	case <-staleReadDuringUpdate:
		t.Fatal("second partial update read subscription config while the first update was not committed")
	case result := <-secondDone:
		t.Fatalf("expected second partial update to wait for the first one, got early response %d body=%s", result.code, result.body)
	case <-time.After(100 * time.Millisecond):
	}

	if firstUpdateReleased.CompareAndSwap(false, true) {
		close(releaseFirstUpdate)
	}

	firstResult := waitSubscriptionConfigTestResponse(t, firstDone, "first update")

	secondResult := waitSubscriptionConfigTestResponse(t, secondDone, "second update")
	if firstResult.code != http.StatusOK {
		t.Fatalf("expected first update ok, got %d body=%s", firstResult.code, firstResult.body)
	}

	if secondResult.code != http.StatusOK {
		t.Fatalf("expected second update ok, got %d body=%s", secondResult.code, secondResult.body)
	}

	var updated Setting
	if err := db.Where("name = ?", subscriptionConfigName).First(&updated).Error; err != nil {
		t.Fatalf("query updated setting: %v", err)
	}

	if updated.Value.PanSearchURL != "https://new.example.com/api/search" {
		t.Fatalf("expected first partial update to persist, got %q", updated.Value.PanSearchURL)
	}

	if updated.Value.CronExpression != "0 3 * * *" {
		t.Fatalf("expected second partial update to persist, got %q", updated.Value.CronExpression)
	}

	if !updated.Value.EnableTMDB || !updated.Value.EnableDouban || !updated.Value.AutoMount || updated.Value.DefaultMountPath != "/old" {
		t.Fatalf("expected untouched fields preserved, got %+v", updated.Value)
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

type subscriptionConfigTestResponse struct {
	code int
	body string
}

func waitSubscriptionConfigTestResponse(
	t *testing.T,
	ch <-chan subscriptionConfigTestResponse,
	name string,
) subscriptionConfigTestResponse {
	t.Helper()

	select {
	case result := <-ch:
		return result
	case <-time.After(2 * time.Second):
		t.Fatalf("timed out waiting for %s", name)
	}

	return subscriptionConfigTestResponse{}
}

func isSubscriptionSettingTestStatement(tx *gorm.DB) bool {
	if tx == nil || tx.Statement == nil {
		return false
	}

	if tx.Statement.Table == "system_settings" {
		return true
	}

	return tx.Statement.Schema != nil && tx.Statement.Schema.Table == "system_settings"
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

	assertSubscriptionAdditionValue(t, storageSvc.req.Addition, consts.FileAdditionKeyShareId, int64(67890))
	assertSubscriptionAdditionValue(t, storageSvc.req.Addition, consts.FileAdditionKeyIsFolder, true)
	assertSubscriptionAdditionValue(t, storageSvc.req.Addition, consts.FileAdditionKeyAccessCode, "share-access-code")
	assertSubscriptionAdditionValue(t, storageSvc.req.Addition, consts.FileAdditionKeyShareMode, 1)
}

func TestMountSubscriptionNormalizesExplicitMountPath(t *testing.T) {
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
		strings.NewReader(`{"title":"测试资源","shareUrl":"https://cloud.189.cn/t/abc123","shareCode":"abc123","mountPath":"/热门订阅//%E4%B8%AD%E6%96%87%20/a%3ab"}`),
	)
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected ok, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	wantPath := "/热门订阅/中文/a_b"
	if storageSvc.req == nil || storageSvc.req.LocalPath != wantPath {
		t.Fatalf("expected storage path %q, got %+v", wantPath, storageSvc.req)
	}

	var response struct {
		Data struct {
			MountPath string `json:"mountPath"`
		} `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}

	if response.Data.MountPath != wantPath {
		t.Fatalf("expected response mount path %q, got %q", wantPath, response.Data.MountPath)
	}
}

func TestMountSubscriptionEscapesLiteralPercentInDefaultMountPath(t *testing.T) {
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
		strings.NewReader(`{"title":"a%2Fb","shareUrl":"https://cloud.189.cn/t/abc123","shareCode":"abc123"}`),
	)
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected ok, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	wantPath := "/热门订阅/a%252Fb"
	if storageSvc.req == nil || storageSvc.req.LocalPath != wantPath {
		t.Fatalf("expected storage path %q, got %+v", wantPath, storageSvc.req)
	}

	var response struct {
		Data struct {
			MountPath string `json:"mountPath"`
		} `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}

	if response.Data.MountPath != wantPath {
		t.Fatalf("expected response mount path %q, got %q", wantPath, response.Data.MountPath)
	}
}

func TestMountSubscriptionExtractsShareCodeAndAccessCodeFromWWWUppercaseLink(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db := setupSubscriptionHandlerTestDB(t)
	storageSvc := &mockSubscriptionStorageFacade{}
	cloudBridge := &mockSubscriptionCloudBridge{}
	handler := NewHandler(db, nil, nil, storageSvc, cloudBridge, zap.NewNop(), nil)

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
		strings.NewReader(`{"title":"测试资源","shareUrl":"HTTPS://WWW.CLOUD.189.CN/t/AbC123?shareId=1 提取码：wxyz","mountPath":"/热门订阅/测试资源"}`),
	)
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected ok, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	if len(cloudBridge.shareCodes) != 1 || cloudBridge.shareCodes[0] != "AbC123" {
		t.Fatalf("expected share code AbC123, got %v", cloudBridge.shareCodes)
	}

	if len(cloudBridge.accessCodes) != 1 || cloudBridge.accessCodes[0] != "wxyz" {
		t.Fatalf("expected access code wxyz, got %v", cloudBridge.accessCodes)
	}

	if storageSvc.req == nil {
		t.Fatal("expected storage facade to be called")
	}
}

func TestMountSubscriptionFallsBackToShareURLWhenShareCodeInvalid(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db := setupSubscriptionHandlerTestDB(t)
	storageSvc := &mockSubscriptionStorageFacade{}
	cloudBridge := &mockSubscriptionCloudBridge{}
	handler := NewHandler(db, nil, nil, storageSvc, cloudBridge, zap.NewNop(), nil)

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
		strings.NewReader(`{"title":"测试资源","shareUrl":"https://cloud.189.cn/t/abc123 提取码：wxyz","shareCode":"https://example.com/share?token=secret-token","mountPath":"/热门订阅/测试资源"}`),
	)
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected ok, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	if len(cloudBridge.shareCodes) != 1 || cloudBridge.shareCodes[0] != "abc123" {
		t.Fatalf("expected share code from share URL, got %v", cloudBridge.shareCodes)
	}

	if len(cloudBridge.accessCodes) != 1 || cloudBridge.accessCodes[0] != "wxyz" {
		t.Fatalf("expected access code from share URL, got %v", cloudBridge.accessCodes)
	}

	if storageSvc.req == nil {
		t.Fatal("expected storage facade to be called")
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

func TestMountSubscriptionRejectsTypedNilStorageFacade(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db := setupSubscriptionHandlerTestDB(t)

	var storageSvc *mockSubscriptionStorageFacade

	cloudBridge := &mockSubscriptionCloudBridge{}
	handler := NewHandler(db, nil, nil, storageSvc, cloudBridge, zap.NewNop(), nil)

	router := gin.New()
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

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected bad request, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	if !strings.Contains(recorder.Body.String(), "storage service is nil") {
		t.Fatalf("expected storage service error, got body=%s", recorder.Body.String())
	}

	if len(cloudBridge.shareCodes) != 0 || len(cloudBridge.accessCodes) != 0 {
		t.Fatalf("expected cloud bridge not to be called, got share=%v access=%v", cloudBridge.shareCodes, cloudBridge.accessCodes)
	}
}

func TestMountSubscriptionRejectsTypedNilCloudBridge(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db := setupSubscriptionHandlerTestDB(t)
	storageSvc := &mockSubscriptionStorageFacade{}

	var cloudBridge *mockSubscriptionCloudBridge

	handler := NewHandler(db, nil, nil, storageSvc, cloudBridge, zap.NewNop(), nil)

	router := gin.New()
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

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected bad request, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	if !strings.Contains(recorder.Body.String(), "cloud bridge service is nil") {
		t.Fatalf("expected cloud bridge service error, got body=%s", recorder.Body.String())
	}

	if storageSvc.req != nil {
		t.Fatal("expected storage facade not to be called")
	}
}

func TestMountSubscriptionRejectsInvalidShareCodeBeforeCloudBridge(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db := setupSubscriptionHandlerTestDB(t)
	storageSvc := &mockSubscriptionStorageFacade{}
	cloudBridge := &mockSubscriptionCloudBridge{}
	handler := NewHandler(db, nil, nil, storageSvc, cloudBridge, zap.NewNop(), nil)

	router := gin.New()
	wrapper := httpcontext.NewHandlerFuncWrapper(zap.NewNop())
	router.POST("/mount", wrapper.Wrap(handler.MountSubscription()))

	req := httptest.NewRequestWithContext(
		stdctx.Background(),
		http.MethodPost,
		"/mount",
		strings.NewReader(`{"title":"测试资源","shareUrl":"https://example.com/share?token=secret-token","mountPath":"/热门订阅/测试资源"}`),
	)
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected bad request, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	if len(cloudBridge.shareCodes) != 0 || len(cloudBridge.accessCodes) != 0 {
		t.Fatalf("expected cloud bridge not to be called, got share=%v access=%v", cloudBridge.shareCodes, cloudBridge.accessCodes)
	}

	if storageSvc.req != nil {
		t.Fatal("expected storage facade not to be called")
	}

	text := recorder.Body.String()
	if !strings.Contains(text, "分享码") {
		t.Fatalf("expected share code error, got body=%s", text)
	}

	if strings.Contains(text, "secret-token") || strings.Contains(text, "example.com") {
		t.Fatalf("expected invalid input not to be echoed, got body=%s", text)
	}
}

func TestMountSubscriptionRejectsInvalidAccessCodeBeforeCloudBridge(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db := setupSubscriptionHandlerTestDB(t)
	storageSvc := &mockSubscriptionStorageFacade{}
	cloudBridge := &mockSubscriptionCloudBridge{}
	handler := NewHandler(db, nil, nil, storageSvc, cloudBridge, zap.NewNop(), nil)

	router := gin.New()
	wrapper := httpcontext.NewHandlerFuncWrapper(zap.NewNop())
	router.POST("/mount", wrapper.Wrap(handler.MountSubscription()))

	req := httptest.NewRequestWithContext(
		stdctx.Background(),
		http.MethodPost,
		"/mount",
		strings.NewReader(`{"title":"测试资源","shareUrl":"https://cloud.189.cn/t/abc123","shareAccessCode":"https://example.com/share?token=secret-token","mountPath":"/热门订阅/测试资源"}`),
	)
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected bad request, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	if len(cloudBridge.shareCodes) != 0 || len(cloudBridge.accessCodes) != 0 {
		t.Fatalf("expected cloud bridge not to be called, got share=%v access=%v", cloudBridge.shareCodes, cloudBridge.accessCodes)
	}

	if storageSvc.req != nil {
		t.Fatal("expected storage facade not to be called")
	}

	text := recorder.Body.String()
	if !strings.Contains(text, "访问码格式无效") {
		t.Fatalf("expected access code error, got body=%s", text)
	}

	if strings.Contains(text, "secret-token") || strings.Contains(text, "example.com") {
		t.Fatalf("expected invalid input not to be echoed, got body=%s", text)
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

func TestMountSubscriptionMapsFacadeExistingPathForbiddenToForbidden(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db := setupSubscriptionHandlerTestDB(t)
	storageSvc := &mockSubscriptionStorageFacade{err: storagefacade.ErrExistingPathForbidden}
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

	if storageSvc.req == nil {
		t.Fatal("expected storage facade to be called")
	}

	if !strings.Contains(recorder.Body.String(), "其他用户") {
		t.Fatalf("expected ownership error, got body=%s", recorder.Body.String())
	}
}

func TestMountSubscriptionRedactsShareInfoFailureInResponse(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db := setupSubscriptionHandlerTestDB(t)
	storageSvc := &mockSubscriptionStorageFacade{}
	cloudBridge := &mockSubscriptionCloudBridge{
		err: fmt.Errorf("upstream failed https://proxy-user:proxy-pass@api.example.test/share?accessToken=secret-access&filename=private-name.mkv#refreshToken=fragment-secret accessCode=abcd Authorization: Bearer bearer-secret"),
	}
	handler := NewHandler(db, nil, nil, storageSvc, cloudBridge, zap.NewNop(), nil)

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

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected bad request, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	if storageSvc.req != nil {
		t.Fatal("expected storage facade not to be called")
	}

	text := recorder.Body.String()
	if !strings.Contains(text, "获取分享信息失败") {
		t.Fatalf("expected share info error, got body=%s", text)
	}

	for _, leaked := range []string{"proxy-user", "proxy-pass", "secret-access", "private-name.mkv", "fragment-secret", "abcd", "bearer-secret"} {
		if strings.Contains(text, leaked) {
			t.Fatalf("expected %q to be redacted from response %s", leaked, text)
		}
	}

	if !strings.Contains(text, utils.RedactedSecret) {
		t.Fatalf("expected redacted marker in response %s", text)
	}
}

func TestMountSubscriptionRedactsStorageFailureInLogsAndResponse(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db := setupSubscriptionHandlerTestDB(t)
	storageSvc := &mockSubscriptionStorageFacade{
		err: fmt.Errorf("create storage failed https://proxy-user:proxy-pass@api.example.test/storage?accessToken=secret-access&filename=private-name.mkv#refreshToken=fragment-secret accessCode=abcd Authorization: Bearer bearer-secret"),
	}
	core, logs := observer.New(zap.ErrorLevel)
	logger := zap.New(core)
	handler := NewHandler(db, nil, nil, storageSvc, &mockSubscriptionCloudBridge{}, logger, nil)

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

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected bad request, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	if storageSvc.req == nil {
		t.Fatal("expected storage facade to be called")
	}

	entries := logs.FilterMessage("Failed to create storage").All()
	if len(entries) != 1 {
		t.Fatalf("expected one storage error log, got %d", len(entries))
	}

	bodyText := recorder.Body.String()

	logText := entries[0].Message + fmt.Sprint(entries[0].Context)
	for _, text := range []string{bodyText, logText} {
		for _, leaked := range []string{"proxy-user", "proxy-pass", "secret-access", "private-name.mkv", "fragment-secret", "abcd", "bearer-secret"} {
			if strings.Contains(text, leaked) {
				t.Fatalf("expected %q to be redacted from %s", leaked, text)
			}
		}

		if !strings.Contains(text, utils.RedactedSecret) {
			t.Fatalf("expected redacted marker in %s", text)
		}
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

func TestPanSearchRequestURLPreservesExistingQuery(t *testing.T) {
	db := setupSubscriptionHandlerTestDB(t)
	if err := db.Create(&Setting{
		Name: subscriptionConfigName,
		Value: models.SubscriptionConfig{
			PanSearchURL: "https://search.example.com/proxy?token=secret-token&source=custom",
		},
	}).Error; err != nil {
		t.Fatalf("create setting: %v", err)
	}

	handler := NewHandler(db, nil, nil, nil, nil, zap.NewNop(), nil)

	got, err := handler.panSearchRequestURL("测试 关键词")
	if err != nil {
		t.Fatalf("build pan search URL: %v", err)
	}

	parsedURL, err := url.Parse(got)
	if err != nil {
		t.Fatalf("parse pan search URL: %v", err)
	}

	if parsedURL.Path != "/proxy/api/search" {
		t.Fatalf("expected /proxy/api/search path, got %q", parsedURL.Path)
	}

	query := parsedURL.Query()
	if query.Get("token") != "secret-token" || query.Get("source") != "custom" {
		t.Fatalf("expected existing query values preserved, got %s", parsedURL.RawQuery)
	}

	if query.Get("kw") != "测试 关键词" || query.Get("cloud_types") != "tianyi" {
		t.Fatalf("expected search query values appended, got %s", parsedURL.RawQuery)
	}
}

func TestPanSearchRequestURLAppendsAPISearchForPartialPathMatch(t *testing.T) {
	db := setupSubscriptionHandlerTestDB(t)
	if err := db.Create(&Setting{
		Name: subscriptionConfigName,
		Value: models.SubscriptionConfig{
			PanSearchURL: "https://search.example.com/proxy/api/search-bak?token=secret-token",
		},
	}).Error; err != nil {
		t.Fatalf("create setting: %v", err)
	}

	handler := NewHandler(db, nil, nil, nil, nil, zap.NewNop(), nil)

	got, err := handler.panSearchRequestURL("测试 关键词")
	if err != nil {
		t.Fatalf("build pan search URL: %v", err)
	}

	parsedURL, err := url.Parse(got)
	if err != nil {
		t.Fatalf("parse pan search URL: %v", err)
	}

	if parsedURL.Path != "/proxy/api/search-bak/api/search" {
		t.Fatalf("expected /api/search appended for partial path match, got %q", parsedURL.Path)
	}

	query := parsedURL.Query()
	if query.Get("token") != "secret-token" || query.Get("kw") != "测试 关键词" || query.Get("cloud_types") != "tianyi" {
		t.Fatalf("expected query values preserved and appended, got %s", parsedURL.RawQuery)
	}
}

func TestPanSearchRequestURLKeepsAPISearchWithTrailingSlash(t *testing.T) {
	db := setupSubscriptionHandlerTestDB(t)
	if err := db.Create(&Setting{
		Name: subscriptionConfigName,
		Value: models.SubscriptionConfig{
			PanSearchURL: "https://search.example.com/proxy/api/search/?token=secret-token",
		},
	}).Error; err != nil {
		t.Fatalf("create setting: %v", err)
	}

	handler := NewHandler(db, nil, nil, nil, nil, zap.NewNop(), nil)

	got, err := handler.panSearchRequestURL("测试 关键词")
	if err != nil {
		t.Fatalf("build pan search URL: %v", err)
	}

	parsedURL, err := url.Parse(got)
	if err != nil {
		t.Fatalf("parse pan search URL: %v", err)
	}

	if parsedURL.Path != "/proxy/api/search/" {
		t.Fatalf("expected configured API path preserved, got %q", parsedURL.Path)
	}

	query := parsedURL.Query()
	if query.Get("token") != "secret-token" || query.Get("kw") != "测试 关键词" || query.Get("cloud_types") != "tianyi" {
		t.Fatalf("expected query values preserved and appended, got %s", parsedURL.RawQuery)
	}
}

func TestPanSearchHTTPClientRedactsInvalidProxyParseError(t *testing.T) {
	t.Setenv("TG_PROXY", "http://proxy-user:proxy-pass@[::1")

	core, logs := observer.New(zap.WarnLevel)
	logger := zap.New(core)
	client := &http.Client{Timeout: time.Second}
	handler := &Handler{logger: logger, httpClient: client}

	if got := handler.panSearchHTTPClient(); got != client {
		t.Fatal("expected invalid proxy to fall back to base client")
	}

	entries := logs.FilterMessage("Invalid TG_PROXY for pan search").All()
	if len(entries) != 1 {
		t.Fatalf("expected one invalid proxy log, got %d", len(entries))
	}

	logText := entries[0].Message + fmt.Sprint(entries[0].Context)
	for _, leaked := range []string{"proxy-user", "proxy-pass"} {
		if strings.Contains(logText, leaked) {
			t.Fatalf("expected %q to be redacted from log %s", leaked, logText)
		}
	}

	if !strings.Contains(logText, utils.RedactedSecret) {
		t.Fatalf("expected redacted marker in log %s", logText)
	}
}

func TestPanSearchHTTPClientProxyClonesBaseTransport(t *testing.T) {
	t.Setenv("TG_PROXY", "http://proxy-user:proxy-pass@127.0.0.1:7890")

	baseTransport := &http.Transport{
		DisableKeepAlives:     false,
		MaxIdleConns:          123,
		MaxIdleConnsPerHost:   45,
		MaxConnsPerHost:       67,
		IdleConnTimeout:       89 * time.Second,
		TLSHandshakeTimeout:   7 * time.Second,
		ExpectContinueTimeout: 3 * time.Second,
		DisableCompression:    true,
		ForceAttemptHTTP2:     false,
	}
	baseClient := &http.Client{
		Timeout:   11 * time.Second,
		Transport: baseTransport,
	}
	handler := &Handler{logger: zap.NewNop(), httpClient: baseClient}

	got := handler.panSearchHTTPClient()
	if got == baseClient {
		t.Fatal("expected proxy client to be a copy of the base client")
	}

	if got.Timeout != baseClient.Timeout {
		t.Fatalf("expected proxy client timeout %s, got %s", baseClient.Timeout, got.Timeout)
	}

	transport, ok := got.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("expected proxy transport, got %#v", got.Transport)
	}

	if transport == baseTransport {
		t.Fatal("expected proxy transport to clone base transport")
	}

	if baseTransport.Proxy != nil {
		t.Fatal("expected base transport proxy to remain unchanged")
	}

	proxyURL, err := transport.Proxy(httptest.NewRequest(http.MethodGet, "https://search.example.test/api/search", nil))
	if err != nil {
		t.Fatalf("resolve proxy URL: %v", err)
	}

	wantProxyURL, err := url.Parse("http://proxy-user:proxy-pass@127.0.0.1:7890")
	if err != nil {
		t.Fatalf("parse expected proxy URL: %v", err)
	}

	if proxyURL.String() != wantProxyURL.String() {
		t.Fatalf("expected proxy URL %q, got %q", wantProxyURL, proxyURL)
	}

	if transport.MaxIdleConns != baseTransport.MaxIdleConns ||
		transport.MaxIdleConnsPerHost != baseTransport.MaxIdleConnsPerHost ||
		transport.MaxConnsPerHost != baseTransport.MaxConnsPerHost ||
		transport.IdleConnTimeout != baseTransport.IdleConnTimeout ||
		transport.TLSHandshakeTimeout != baseTransport.TLSHandshakeTimeout ||
		transport.ExpectContinueTimeout != baseTransport.ExpectContinueTimeout ||
		transport.DisableCompression != baseTransport.DisableCompression ||
		transport.ForceAttemptHTTP2 != baseTransport.ForceAttemptHTTP2 {
		t.Fatal("expected proxy transport to preserve base transport settings")
	}
}

func TestPanSearchHTTPClientProxyReusesConfiguredClient(t *testing.T) {
	t.Setenv("TG_PROXY", "http://127.0.0.1:7890")

	baseTransport := &http.Transport{
		MaxIdleConns:        32,
		MaxIdleConnsPerHost: 16,
	}
	baseClient := &http.Client{
		Timeout:   11 * time.Second,
		Transport: baseTransport,
	}
	handler := &Handler{logger: zap.NewNop(), httpClient: baseClient}

	first := handler.panSearchHTTPClient()
	second := handler.panSearchHTTPClient()

	if first != second {
		t.Fatal("expected proxied pan search client to be cached")
	}

	if first.Transport == nil || first.Transport != second.Transport {
		t.Fatal("expected cached proxied client to reuse its transport")
	}

	t.Setenv("TG_PROXY", "http://127.0.0.1:7891")

	third := handler.panSearchHTTPClient()
	if third == first {
		t.Fatal("expected proxy client to refresh when TG_PROXY changes")
	}

	if third.Transport == baseTransport {
		t.Fatal("expected refreshed proxy transport to clone base transport")
	}
}

func TestSearchPanRedactsSearchURLInLogs(t *testing.T) {
	gin.SetMode(gin.TestMode)

	searchServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("token") != "secret-token" {
			t.Fatalf("expected token query to reach search service, got %s", r.URL.RawQuery)
		}

		if r.URL.Query().Get("kw") != "secret keyword" {
			t.Fatalf("expected keyword query to reach search service, got %s", r.URL.RawQuery)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"code":0,"message":"ok","data":{"total":0,"merged_by_type":{}}}`))
	}))
	defer searchServer.Close()

	db := setupSubscriptionHandlerTestDB(t)
	if err := db.Create(&Setting{
		Name: subscriptionConfigName,
		Value: models.SubscriptionConfig{
			PanSearchURL: searchServer.URL + "/api/search?token=secret-token",
		},
	}).Error; err != nil {
		t.Fatalf("create setting: %v", err)
	}

	core, logs := observer.New(zap.InfoLevel)
	logger := zap.New(core)
	handler := NewHandler(db, nil, nil, nil, nil, logger, nil)

	router := gin.New()
	wrapper := httpcontext.NewHandlerFuncWrapper(zap.NewNop())
	router.GET("/search", wrapper.Wrap(handler.SearchPan()))

	req := httptest.NewRequestWithContext(stdctx.Background(), http.MethodGet, "/search?keyword="+url.QueryEscape("secret keyword"), nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected ok, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	entries := logs.FilterMessage("Searching pan").All()
	if len(entries) != 1 {
		t.Fatalf("expected one search log, got %d", len(entries))
	}

	logText := entries[0].Message + fmt.Sprint(entries[0].Context)
	for _, leaked := range []string{"secret-token", "secret keyword"} {
		if strings.Contains(logText, leaked) {
			t.Fatalf("expected %q to be redacted from log %s", leaked, logText)
		}
	}

	if !strings.Contains(logText, utils.RedactedSecret) {
		t.Fatalf("expected redacted marker in log %s", logText)
	}
}

func TestSearchPanRedactsRequestErrorsInLogsAndResponse(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db := setupSubscriptionHandlerTestDB(t)
	if err := db.Create(&Setting{
		Name: subscriptionConfigName,
		Value: models.SubscriptionConfig{
			PanSearchURL: "https://search.example.com/api/search?token=secret-token",
		},
	}).Error; err != nil {
		t.Fatalf("create setting: %v", err)
	}

	core, logs := observer.New(zap.ErrorLevel)
	logger := zap.New(core)
	handler := NewHandler(db, nil, nil, nil, nil, logger, nil)
	handler.httpClient = &http.Client{Transport: requestURLFailingRoundTripper{}}

	router := gin.New()
	wrapper := httpcontext.NewHandlerFuncWrapper(zap.NewNop())
	router.GET("/search", wrapper.Wrap(handler.SearchPan()))

	req := httptest.NewRequestWithContext(stdctx.Background(), http.MethodGet, "/search?keyword="+url.QueryEscape("secret keyword"), nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected bad request, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	entries := logs.FilterMessage("Pan search request failed").All()
	if len(entries) != 1 {
		t.Fatalf("expected one request error log, got %d", len(entries))
	}

	bodyText := recorder.Body.String()

	logText := entries[0].Message + fmt.Sprint(entries[0].Context)
	for _, text := range []string{bodyText, logText} {
		for _, leaked := range []string{"secret-token", "secret+keyword", "secret keyword", "abcd"} {
			if strings.Contains(text, leaked) {
				t.Fatalf("expected %q to be redacted from %s", leaked, text)
			}
		}

		if !strings.Contains(text, utils.RedactedSecret) {
			t.Fatalf("expected redacted marker in %s", text)
		}
	}
}

func TestSearchPanRedactsBodyReadErrorsInLogsAndResponse(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db := setupSubscriptionHandlerTestDB(t)
	if err := db.Create(&Setting{
		Name: subscriptionConfigName,
		Value: models.SubscriptionConfig{
			PanSearchURL: "https://search.example.com/api/search?token=secret-token",
		},
	}).Error; err != nil {
		t.Fatalf("create setting: %v", err)
	}

	core, logs := observer.New(zap.ErrorLevel)
	logger := zap.New(core)
	handler := NewHandler(db, nil, nil, nil, nil, logger, nil)
	handler.httpClient = &http.Client{Transport: panSearchReadErrorRoundTripper{}}

	router := gin.New()
	wrapper := httpcontext.NewHandlerFuncWrapper(zap.NewNop())
	router.GET("/search", wrapper.Wrap(handler.SearchPan()))

	req := httptest.NewRequestWithContext(stdctx.Background(), http.MethodGet, "/search?keyword="+url.QueryEscape("secret keyword"), nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected bad request, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	entries := logs.FilterMessage("Failed to read response body").All()
	if len(entries) != 1 {
		t.Fatalf("expected one body read error log, got %d", len(entries))
	}

	bodyText := recorder.Body.String()
	logText := entries[0].Message + fmt.Sprint(entries[0].Context)

	for _, text := range []string{bodyText, logText} {
		for _, leaked := range []string{"secret-token", "secret+keyword", "secret keyword", "bearer-secret", "abcd"} {
			if strings.Contains(text, leaked) {
				t.Fatalf("expected %q to be redacted from %s", leaked, text)
			}
		}

		if !strings.Contains(text, utils.RedactedSecret) {
			t.Fatalf("expected redacted marker in %s", text)
		}
	}
}

func TestSearchPanRedactsBusinessErrorMessageInResponse(t *testing.T) {
	gin.SetMode(gin.TestMode)

	searchServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"code": 1,
			"message": "upstream failed https://proxy-user:proxy-pass@api.example.test/search?accessToken=secret-access&kw=secret+keyword#refreshToken=fragment-secret accessCode=abcd Authorization: Bearer bearer-secret",
			"data": {"total": 0, "merged_by_type": {}}
		}`))
	}))
	defer searchServer.Close()

	db := setupSubscriptionHandlerTestDB(t)
	if err := db.Create(&Setting{
		Name: subscriptionConfigName,
		Value: models.SubscriptionConfig{
			PanSearchURL: searchServer.URL,
		},
	}).Error; err != nil {
		t.Fatalf("create setting: %v", err)
	}

	handler := NewHandler(db, nil, nil, nil, nil, zap.NewNop(), nil)

	router := gin.New()
	wrapper := httpcontext.NewHandlerFuncWrapper(zap.NewNop())
	router.GET("/search", wrapper.Wrap(handler.SearchPan()))

	req := httptest.NewRequestWithContext(stdctx.Background(), http.MethodGet, "/search?keyword="+url.QueryEscape("secret keyword"), nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected bad request, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	text := recorder.Body.String()
	for _, leaked := range []string{"proxy-user", "proxy-pass", "secret-access", "secret+keyword", "secret keyword", "fragment-secret", "abcd", "bearer-secret"} {
		if strings.Contains(text, leaked) {
			t.Fatalf("expected %q to be redacted from %s", leaked, text)
		}
	}

	if !strings.Contains(text, utils.RedactedSecret) {
		t.Fatalf("expected redacted marker in response %s", text)
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
	if got.ShareCode != "abc123" || got.ShareAccessCode != "p123" || got.Name != "测试资源 4K" || got.Cover == "" {
		t.Fatalf("unexpected fallback result: %+v", got)
	}
}

func TestSearchPanWithAITreatsTypedNilOpenAIServiceAsMissing(t *testing.T) {
	gin.SetMode(gin.TestMode)

	searchServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
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
		Name: subscriptionConfigName,
		Value: models.SubscriptionConfig{
			PanSearchURL: searchServer.URL,
		},
	}).Error; err != nil {
		t.Fatalf("create setting: %v", err)
	}

	var openaiSvc *mockSubscriptionOpenAI

	handler := NewHandler(db, nil, nil, nil, nil, zap.NewNop(), openaiSvc)

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
			Message    string         `json:"message"`
			Result     *SearchResult  `json:"result"`
			AllResults []SearchResult `json:"allResults"`
		} `json:"data"`
	}

	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if response.Code != http.StatusOK {
		t.Fatalf("expected business code 200, got %d", response.Code)
	}

	if response.Data.Message != "AI服务未配置，返回全部搜索结果" {
		t.Fatalf("unexpected message: %q", response.Data.Message)
	}

	if response.Data.Result != nil {
		t.Fatalf("expected no AI-picked result, got %+v", response.Data.Result)
	}

	if len(response.Data.AllResults) != 1 {
		t.Fatalf("expected one fallback result, got %+v", response.Data.AllResults)
	}
}

func TestPanSearchResultsFromResponseParsesWWWUppercaseShareLinks(t *testing.T) {
	results := panSearchResultsFromResponse(panSearchResponseV2{
		Data: struct {
			Total        int                        `json:"total"`
			MergedByType map[string][]panSearchItem `json:"merged_by_type"`
		}{
			MergedByType: map[string][]panSearchItem{
				"tianyi": {
					{URL: "HTTPS://WWW.CLOUD.189.CN/t/AbC123?shareId=1", Password: "wxyz", Note: "测试资源"},
				},
			},
		},
	})

	if len(results) != 1 {
		t.Fatalf("expected one result, got %+v", results)
	}

	if results[0].ShareCode != "AbC123" {
		t.Fatalf("expected share code AbC123, got %+v", results[0])
	}

	if results[0].ShareAccessCode != "wxyz" {
		t.Fatalf("expected access code wxyz, got %+v", results[0])
	}
}

func TestSearchPanWithAIRedactsRecommendedKeywordInLogs(t *testing.T) {
	gin.SetMode(gin.TestMode)

	searchServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
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

	sensitiveKeyword := "https://proxy-user:proxy-pass@example.test/search?accessToken=secret-access&filename=private-name.mkv#refreshToken=fragment-secret accessCode=abcd Authorization: Bearer bearer-secret"
	core, logs := observer.New(zap.InfoLevel)
	logger := zap.New(core)
	handler := NewHandler(db, nil, nil, nil, nil, logger, mockSubscriptionOpenAI{keyword: sensitiveKeyword})

	router := gin.New()
	wrapper := httpcontext.NewHandlerFuncWrapper(zap.NewNop())
	router.GET("/search-ai", wrapper.Wrap(handler.SearchPanWithAI()))

	req := httptest.NewRequestWithContext(stdctx.Background(), http.MethodGet, "/search-ai?keyword=test", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected ok, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	entries := logs.FilterMessage("AI recommended keyword").All()
	if len(entries) != 1 {
		t.Fatalf("expected one AI keyword log, got %d", len(entries))
	}

	logText := entries[0].Message + fmt.Sprint(entries[0].Context)
	for _, leaked := range []string{"proxy-user", "proxy-pass", "secret-access", "private-name.mkv", "fragment-secret", "abcd", "bearer-secret"} {
		if strings.Contains(logText, leaked) {
			t.Fatalf("expected %q to be redacted from log %s", leaked, logText)
		}
	}

	if !strings.Contains(logText, utils.RedactedSecret) {
		t.Fatalf("expected redacted marker in log %s", logText)
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
