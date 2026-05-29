package subscription

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	appContext "github.com/xxcheng123/cloudpan189-share/internal/framework/context"
	"github.com/xxcheng123/cloudpan189-share/internal/repository/models"
	"github.com/xxcheng123/cloudpan189-share/internal/services/douban"
	"github.com/xxcheng123/cloudpan189-share/internal/services/tmdb"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

func setupTestDB() (*gorm.DB, error) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		return nil, err
	}

	// 自动迁移表结构
	err = db.AutoMigrate(
		&models.Subscription{},
		&models.MatchHistory{},
		&models.DailyHotHistory{},
	)
	if err != nil {
		return nil, err
	}

	return db, nil
}

func TestNewService(t *testing.T) {
	db, err := setupTestDB()
	if err != nil {
		t.Fatalf("Failed to setup test DB: %v", err)
	}

	logger := zap.NewNop()
	config := &SubscriptionConfig{
		PanSearchURL: "https://test.com",
		EnableTMDB:   true,
		EnableDouban: true,
	}

	svc := NewService(db, logger, config)
	if svc == nil {
		t.Fatal("Expected service to be created")
	}
}

func TestSubscriptionCRUD(t *testing.T) {
	db, err := setupTestDB()
	if err != nil {
		t.Fatalf("Failed to setup test DB: %v", err)
	}

	logger := zap.NewNop()
	config := &SubscriptionConfig{}
	svc := NewService(db, logger, config)

	// 测试创建订阅
	sub := &models.Subscription{
		Name:     "测试订阅",
		Source:   "tmdb",
		Category: "movie",
		Enable:   true,
	}

	err = svc.CreateSubscription(sub)
	if err != nil {
		t.Fatalf("Failed to create subscription: %v", err)
	}

	if sub.ID == 0 {
		t.Fatal("Expected subscription ID to be set")
	}

	// 测试获取订阅列表
	subs, err := svc.GetSubscriptions()
	if err != nil {
		t.Fatalf("Failed to get subscriptions: %v", err)
	}

	if len(subs) != 1 {
		t.Fatalf("Expected 1 subscription, got %d", len(subs))
	}

	// 测试更新订阅
	sub.Name = "更新后的订阅"

	err = svc.UpdateSubscription(sub)
	if err != nil {
		t.Fatalf("Failed to update subscription: %v", err)
	}

	// 测试删除订阅
	err = svc.DeleteSubscription(sub.ID)
	if err != nil {
		t.Fatalf("Failed to delete subscription: %v", err)
	}

	subs, err = svc.GetSubscriptions()
	if err != nil {
		t.Fatalf("Failed to get subscriptions: %v", err)
	}

	if len(subs) != 0 {
		t.Fatalf("Expected 0 subscriptions, got %d", len(subs))
	}
}

func TestDeleteSubscriptionReturnsNotFoundWhenMissing(t *testing.T) {
	db, err := setupTestDB()
	if err != nil {
		t.Fatalf("Failed to setup test DB: %v", err)
	}

	logger := zap.NewNop()
	config := &SubscriptionConfig{}
	svc := NewService(db, logger, config)

	err = svc.DeleteSubscription(99999)
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("expected record not found, got %v", err)
	}
}

func TestUpdateSubscriptionReturnsNotFoundWhenMissing(t *testing.T) {
	db, err := setupTestDB()
	if err != nil {
		t.Fatalf("Failed to setup test DB: %v", err)
	}

	logger := zap.NewNop()
	config := &SubscriptionConfig{}
	svc := NewService(db, logger, config)

	err = svc.UpdateSubscription(&models.Subscription{
		ID:       99999,
		Name:     "不存在的订阅",
		Source:   "tmdb",
		Category: "movie",
		Enable:   true,
	})
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("expected record not found, got %v", err)
	}

	subs, err := svc.GetSubscriptions()
	if err != nil {
		t.Fatalf("Failed to get subscriptions: %v", err)
	}

	if len(subs) != 0 {
		t.Fatalf("expected missing update not to create subscription, got %d", len(subs))
	}
}

func TestUpdateSubscriptionPersistsZeroValues(t *testing.T) {
	db, err := setupTestDB()
	if err != nil {
		t.Fatalf("Failed to setup test DB: %v", err)
	}

	logger := zap.NewNop()
	config := &SubscriptionConfig{}
	svc := NewService(db, logger, config)

	sub := &models.Subscription{
		Name:              "测试订阅",
		Source:            "tmdb",
		Category:          "movie",
		Keywords:          "old-keyword",
		ListType:          "popular",
		MountPath:         "/old",
		Enable:            true,
		EnableAutoUpgrade: true,
		MatchCount:        3,
		SuccessCount:      2,
	}

	err = svc.CreateSubscription(sub)
	if err != nil {
		t.Fatalf("Failed to create subscription: %v", err)
	}

	sub.Keywords = ""
	sub.ListType = ""
	sub.MountPath = ""
	sub.Enable = false
	sub.EnableAutoUpgrade = false
	sub.MatchCount = 0
	sub.SuccessCount = 0

	err = svc.UpdateSubscription(sub)
	if err != nil {
		t.Fatalf("Failed to update subscription: %v", err)
	}

	var updated models.Subscription
	if err := db.First(&updated, sub.ID).Error; err != nil {
		t.Fatalf("Failed to query updated subscription: %v", err)
	}

	if updated.Keywords != "" {
		t.Fatalf("expected keywords cleared, got %q", updated.Keywords)
	}

	if updated.ListType != "" {
		t.Fatalf("expected list type cleared, got %q", updated.ListType)
	}

	if updated.MountPath != "" {
		t.Fatalf("expected mount path cleared, got %q", updated.MountPath)
	}

	if updated.Enable {
		t.Fatal("expected subscription disabled")
	}

	if updated.EnableAutoUpgrade {
		t.Fatal("expected auto upgrade disabled")
	}

	if updated.MatchCount != 0 || updated.SuccessCount != 0 {
		t.Fatalf("expected counters reset, got match=%d success=%d", updated.MatchCount, updated.SuccessCount)
	}
}

func TestCreateSubscriptionPersistsDisabledState(t *testing.T) {
	db, err := setupTestDB()
	if err != nil {
		t.Fatalf("Failed to setup test DB: %v", err)
	}

	svc := NewService(db, zap.NewNop(), &SubscriptionConfig{})
	sub := &models.Subscription{
		Name:              "关闭的订阅",
		Source:            "tmdb",
		Category:          "movie",
		Keywords:          "",
		ListType:          "",
		MountPath:         "",
		Enable:            false,
		EnableAutoUpgrade: false,
	}

	if err := svc.CreateSubscription(sub); err != nil {
		t.Fatalf("Failed to create subscription: %v", err)
	}

	if sub.Enable {
		t.Fatal("expected returned subscription to stay disabled")
	}

	if sub.EnableAutoUpgrade {
		t.Fatal("expected returned subscription auto upgrade to stay disabled")
	}

	var created models.Subscription
	if err := db.First(&created, sub.ID).Error; err != nil {
		t.Fatalf("Failed to query created subscription: %v", err)
	}

	if created.Enable {
		t.Fatal("expected created subscription to stay disabled")
	}

	if created.EnableAutoUpgrade {
		t.Fatal("expected auto upgrade to stay disabled")
	}
}

func TestUpdateSubscriptionRunProgressReturnsNotFoundWhenMissing(t *testing.T) {
	db, err := setupTestDB()
	if err != nil {
		t.Fatalf("Failed to setup test DB: %v", err)
	}

	svc := NewService(db, zap.NewNop(), &SubscriptionConfig{}).(*service)
	now := time.Now()

	err = svc.updateSubscriptionRunProgress(&models.Subscription{
		ID:        99999,
		LastRunAt: &now,
	})
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("expected record not found, got %v", err)
	}

	subs, err := svc.GetSubscriptions()
	if err != nil {
		t.Fatalf("Failed to get subscriptions: %v", err)
	}

	if len(subs) != 0 {
		t.Fatalf("expected missing progress update not to create subscription, got %d", len(subs))
	}
}

func TestUpdateSubscriptionMatchProgressOnlyUpdatesProgressFields(t *testing.T) {
	db, err := setupTestDB()
	if err != nil {
		t.Fatalf("Failed to setup test DB: %v", err)
	}

	svc := NewService(db, zap.NewNop(), &SubscriptionConfig{}).(*service)
	sub := &models.Subscription{
		Name:         "原名称",
		Source:       "tmdb",
		Category:     "movie",
		Keywords:     "keyword",
		SuccessCount: 1,
	}

	if err := svc.CreateSubscription(sub); err != nil {
		t.Fatalf("Failed to create subscription: %v", err)
	}

	matchAt := time.Now()
	sub.Name = "不应写回"
	sub.SuccessCount = 5
	sub.LastMatchAt = &matchAt

	if err := svc.updateSubscriptionMatchProgress(sub); err != nil {
		t.Fatalf("update match progress: %v", err)
	}

	var updated models.Subscription
	if err := db.First(&updated, sub.ID).Error; err != nil {
		t.Fatalf("Failed to query subscription: %v", err)
	}

	if updated.Name != "原名称" {
		t.Fatalf("expected name unchanged, got %q", updated.Name)
	}

	if updated.SuccessCount != 5 {
		t.Fatalf("expected success count 5, got %d", updated.SuccessCount)
	}

	if updated.LastMatchAt == nil {
		t.Fatal("expected last match time to be set")
	}
}

func TestCheckSubscriptionUpdateResultAllowsExistingNoop(t *testing.T) {
	db, err := setupTestDB()
	if err != nil {
		t.Fatalf("Failed to setup test DB: %v", err)
	}

	svc := NewService(db, zap.NewNop(), &SubscriptionConfig{}).(*service)
	sub := &models.Subscription{
		Name:     "测试订阅",
		Source:   "tmdb",
		Category: "movie",
		Enable:   true,
	}

	if err := svc.CreateSubscription(sub); err != nil {
		t.Fatalf("Failed to create subscription: %v", err)
	}

	if err := svc.checkSubscriptionUpdateResult(&gorm.DB{RowsAffected: 0}, sub.ID); err != nil {
		t.Fatalf("expected existing no-op update to succeed, got %v", err)
	}
}

func TestCheckSubscriptionUpdateResultReturnsNotFoundWhenMissing(t *testing.T) {
	db, err := setupTestDB()
	if err != nil {
		t.Fatalf("Failed to setup test DB: %v", err)
	}

	svc := NewService(db, zap.NewNop(), &SubscriptionConfig{}).(*service)

	err = svc.checkSubscriptionUpdateResult(&gorm.DB{RowsAffected: 0}, 99999)
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("expected record not found, got %v", err)
	}
}

func TestSetServices(t *testing.T) {
	db, err := setupTestDB()
	if err != nil {
		t.Fatalf("Failed to setup test DB: %v", err)
	}

	logger := zap.NewNop()
	config := &SubscriptionConfig{}
	svc := NewService(db, logger, config)

	// 测试设置 Telegram 服务
	mockTelegram := &mockTelegramService{}
	svc.SetTelegramService(mockTelegram)

	// 测试设置 TMDB 服务
	mockTMDB := &mockTMDBService{}
	svc.SetTMDBService(mockTMDB)

	// 测试设置 Douban 服务
	mockDouban := &mockDoubanService{}
	svc.SetDoubanService(mockDouban)

	// 测试设置 OpenAI 服务
	mockOpenAI := &mockOpenAIService{}
	svc.SetOpenAIService(mockOpenAI)
}

func TestMatchResult(t *testing.T) {
	db, err := setupTestDB()
	if err != nil {
		t.Fatalf("Failed to setup test DB: %v", err)
	}

	logger := zap.NewNop()
	config := &SubscriptionConfig{}
	svc := NewService(db, logger, config)

	sub := &models.Subscription{
		Name:      "测试订阅",
		Source:    "tmdb",
		Category:  "movie",
		MountPath: "/test",
		Enable:    true,
	}

	result, err := svc.MatchAndMount(sub, SearchResult{}, "测试电影", "2024", "movie")
	if err != nil {
		t.Fatalf("MatchAndMount should record a failed result without returning an error: %v", err)
	}

	if result == nil {
		t.Fatal("Expected match result to be non-nil")
	}

	if result.Success {
		t.Fatal("Expected match result to be unsuccessful for empty SearchResult")
	}
}

func TestGetMatchHistory(t *testing.T) {
	db, err := setupTestDB()
	if err != nil {
		t.Fatalf("Failed to setup test DB: %v", err)
	}

	logger := zap.NewNop()
	config := &SubscriptionConfig{}
	svc := NewService(db, logger, config)

	// 创建订阅
	sub := &models.Subscription{
		Name:     "测试订阅",
		Source:   "tmdb",
		Category: "movie",
		Enable:   true,
	}

	err = svc.CreateSubscription(sub)
	if err != nil {
		t.Fatalf("Failed to create subscription: %v", err)
	}

	// 创建匹配历史（即使失败也会写入失败记录）
	_, _ = svc.MatchAndMount(sub, SearchResult{}, "测试电影", "2024", "movie")

	// 获取匹配历史
	history, err := svc.GetMatchHistory(sub.ID)
	if err != nil {
		t.Fatalf("Failed to get match history: %v", err)
	}

	if len(history) < 1 {
		t.Fatalf("Expected at least 1 history record, got %d", len(history))
	}
}

func TestMatchAndMountSucceedsWhenMatchHistoryCreateFails(t *testing.T) {
	db, err := setupTestDB()
	if err != nil {
		t.Fatalf("Failed to setup test DB: %v", err)
	}

	svc := NewService(db, zap.NewNop(), &SubscriptionConfig{}).(*service)
	mountService := &mockMountService{id: 42}
	svc.SetMountService(mountService)
	svc.SetShareInfoFetcher(&mockShareInfoFetcher{
		info: &ShareInfo{ID: "share-file-id", Name: "分享目录", IsFolder: true},
	})

	sub := &models.Subscription{
		Name:      "热门订阅",
		Source:    "tmdb",
		Category:  "movie",
		MountPath: "/订阅",
		Enable:    true,
	}
	if err := svc.CreateSubscription(sub); err != nil {
		t.Fatalf("Failed to create subscription: %v", err)
	}

	callbackName := "subscription:test_match_history_create_error"
	if err := db.Callback().Create().Before("gorm:create").Register(callbackName, func(tx *gorm.DB) {
		if tx.Statement.Schema != nil && tx.Statement.Schema.Table == (&models.MatchHistory{}).TableName() {
			_ = tx.AddError(errors.New("injected match history create failure"))
		}
	}); err != nil {
		t.Fatalf("register create callback: %v", err)
	}
	defer func() {
		if removeErr := db.Callback().Create().Remove(callbackName); removeErr != nil {
			t.Fatalf("remove create callback: %v", removeErr)
		}
	}()

	result, err := svc.MatchAndMount(
		sub,
		SearchResult{Title: "测试电影", ShareURL: "https://cloud.189.cn/t/abcdef"},
		"测试电影",
		"2024",
		"movie",
	)
	if err != nil {
		t.Fatalf("MatchAndMount should not fail when only history create fails: %v", err)
	}

	if result == nil || !result.Success {
		t.Fatalf("expected successful match result, got %+v", result)
	}

	if result.STrmPath != "/订阅/测试电影 (2024)" {
		t.Fatalf("unexpected strm path: %q", result.STrmPath)
	}

	if len(mountService.requests) != 1 {
		t.Fatalf("expected one mount request, got %d", len(mountService.requests))
	}

	if mountService.requests[0].FileId != "share-file-id" {
		t.Fatalf("expected share file id passed to mount request, got %q", mountService.requests[0].FileId)
	}

	if !mountService.requests[0].IsAdmin || mountService.requests[0].CreatorUserID != 1 {
		t.Fatalf("expected subscription background mount to use system admin owner, got %+v", mountService.requests[0])
	}

	var historyCount int64
	if err := db.Model(&models.MatchHistory{}).Where("subscription_id = ?", sub.ID).Count(&historyCount).Error; err != nil {
		t.Fatalf("count match history: %v", err)
	}

	if historyCount != 0 {
		t.Fatalf("expected no match history rows after injected create failure, got %d", historyCount)
	}
}

// Mock services for testing
type mockTelegramService struct{}

func (m *mockTelegramService) SendNotification(title, content string) error {
	return nil
}

type mockTMDBService struct {
	movies []tmdb.Movie
	tvs    []tmdb.TV
}

func (m *mockTMDBService) GetPopularMovies(page int) ([]tmdb.Movie, error) {
	if m.movies != nil {
		return m.movies, nil
	}

	return []tmdb.Movie{}, nil
}

func (m *mockTMDBService) GetPopularTVs(page int) ([]tmdb.TV, error) {
	if m.tvs != nil {
		return m.tvs, nil
	}

	return []tmdb.TV{}, nil
}

type mockDoubanService struct{}

func (m *mockDoubanService) GetPopularMovies() ([]douban.Subject, error) {
	return []douban.Subject{}, nil
}

type mockOpenAIService struct{}

func (m *mockOpenAIService) GenerateUpgradeKeyword(title, category string) (string, error) {
	return "", nil
}

type mockMountService struct {
	id       int64
	err      error
	requests []*MountStorageRequest
}

func (m *mockMountService) CreateStorage(_ appContext.Context, req *MountStorageRequest) (int64, error) {
	m.requests = append(m.requests, req)
	if m.err != nil {
		return 0, m.err
	}

	if m.id == 0 {
		return 1, nil
	}

	return m.id, nil
}

type mockShareInfoFetcher struct {
	info       *ShareInfo
	err        error
	shareCodes []string
}

func (m *mockShareInfoFetcher) GetShareInfo(_ appContext.Context, shareCode string, _ string) (*ShareInfo, error) {
	m.shareCodes = append(m.shareCodes, shareCode)
	if m.err != nil {
		return nil, m.err
	}

	if m.info != nil {
		return m.info, nil
	}

	return &ShareInfo{ID: "share-file-id", Name: "分享目录", IsFolder: true}, nil
}

func TestHotResourceStruct(t *testing.T) {
	hr := HotResource{
		Title:    "测试",
		Year:     "2024",
		Category: "movie",
		Rating:   8.5,
		Source:   "tmdb",
	}

	if hr.Title != "测试" {
		t.Errorf("Expected title '测试', got '%s'", hr.Title)
	}
}

func TestSearchResultStruct(t *testing.T) {
	sr := SearchResult{
		Title:    "测试",
		ShareURL: "https://test.com",
		FileID:   123,
		Size:     "1GB",
	}

	if sr.FileID != 123 {
		t.Errorf("Expected file ID 123, got %d", sr.FileID)
	}
}

func TestMatchResultStruct(t *testing.T) {
	mr := MatchResult{
		Success:  true,
		ShareURL: "https://test.com",
		STrmPath: "/test/path.strm",
		Message:  "Success",
	}

	if !mr.Success {
		t.Error("Expected success to be true")
	}
}

func TestSubscriptionConfig(t *testing.T) {
	config := &SubscriptionConfig{
		PanSearchURL: "https://test.com",
		EnableTMDB:   true,
		EnableDouban: false,
	}

	if config.PanSearchURL != "https://test.com" {
		t.Errorf("Expected PanSearchURL 'https://test.com', got '%s'", config.PanSearchURL)
	}

	if !config.EnableTMDB {
		t.Error("Expected EnableTMDB to be true")
	}

	if config.EnableDouban {
		t.Error("Expected EnableDouban to be false")
	}
}

func TestSearchPan(t *testing.T) {
	db, err := setupTestDB()
	if err != nil {
		t.Fatalf("Failed to setup test DB: %v", err)
	}

	logger := zap.NewNop()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/search" {
			t.Fatalf("Expected /api/search path, got %s", r.URL.Path)
		}

		if r.URL.Query().Get("kw") != "test" {
			t.Fatalf("Expected kw=test, got %s", r.URL.Query().Get("kw"))
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"code": 0,
			"message": "ok",
			"data": {
				"total": 1,
				"merged_by_type": {
					"tianyi": [
						{
							"url": "https://cloud.189.cn/t/abcdef",
							"password": "",
							"note": "测试资源",
							"datetime": "2026-05-19",
							"source": "test",
							"images": []
						}
					]
				}
			}
		}`))
	}))
	defer server.Close()

	config := &SubscriptionConfig{
		PanSearchURL: server.URL,
	}
	svc := NewService(db, logger, config)

	results, err := svc.SearchPan("test")
	if err != nil {
		t.Fatalf("SearchPan failed: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("Expected 1 result, got %d", len(results))
	}

	if results[0].Title != "测试资源" || results[0].ShareURL != "https://cloud.189.cn/t/abcdef" {
		t.Fatalf("Unexpected search result: %+v", results[0])
	}
}

func TestSearchPanRejectsOversizedResponse(t *testing.T) {
	db, err := setupTestDB()
	if err != nil {
		t.Fatalf("Failed to setup test DB: %v", err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(strings.Repeat("x", maxPanSearchResponseSize+1)))
	}))
	defer server.Close()

	logger := zap.NewNop()
	config := &SubscriptionConfig{
		PanSearchURL: server.URL,
	}
	svc := NewService(db, logger, config)

	_, err = svc.SearchPan("test")
	if err == nil {
		t.Fatal("expected oversized response error")
	}

	if !strings.Contains(err.Error(), "盘搜返回体过大") {
		t.Fatalf("expected oversized response error, got %v", err)
	}
}

func TestProcessSubscriptionSkipsSearchWhenDailyHotHistoryQueryFails(t *testing.T) {
	db, err := setupTestDB()
	if err != nil {
		t.Fatalf("Failed to setup test DB: %v", err)
	}

	var searchRequests int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&searchRequests, 1)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"code":0,"message":"ok","data":{"total":0,"merged_by_type":{}}}`))
	}))
	defer server.Close()

	svc := NewService(db, zap.NewNop(), &SubscriptionConfig{
		PanSearchURL: server.URL,
		EnableTMDB:   true,
	}).(*service)
	svc.SetTMDBService(&mockTMDBService{
		movies: []tmdb.Movie{{Title: "查询失败电影", ReleaseDate: "2024-01-02"}},
	})

	sub := &models.Subscription{
		Name:     "热门订阅",
		Source:   "tmdb",
		Category: "movie",
		Enable:   true,
	}
	if err := svc.CreateSubscription(sub); err != nil {
		t.Fatalf("Failed to create subscription: %v", err)
	}

	callbackName := "subscription:test_daily_hot_history_query_error"
	if err := db.Callback().Query().Before("gorm:query").Register(callbackName, func(tx *gorm.DB) {
		if tx.Statement.Schema != nil && tx.Statement.Schema.Table == (&models.DailyHotHistory{}).TableName() {
			_ = tx.AddError(errors.New("injected daily hot history query failure"))
		}
	}); err != nil {
		t.Fatalf("register query callback: %v", err)
	}
	defer func() {
		if removeErr := db.Callback().Query().Remove(callbackName); removeErr != nil {
			t.Fatalf("remove query callback: %v", removeErr)
		}
	}()

	svc.processSubscription(sub)

	if got := atomic.LoadInt32(&searchRequests); got != 0 {
		t.Fatalf("expected search not to run after history query failure, got %d requests", got)
	}
}

func TestProcessSubscriptionSkipsSearchWhenDailyHotHistoryCreateFails(t *testing.T) {
	db, err := setupTestDB()
	if err != nil {
		t.Fatalf("Failed to setup test DB: %v", err)
	}

	var searchRequests int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&searchRequests, 1)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"code":0,"message":"ok","data":{"total":0,"merged_by_type":{}}}`))
	}))
	defer server.Close()

	svc := NewService(db, zap.NewNop(), &SubscriptionConfig{
		PanSearchURL: server.URL,
		EnableTMDB:   true,
	}).(*service)
	svc.SetTMDBService(&mockTMDBService{
		movies: []tmdb.Movie{{Title: "写入失败电影", ReleaseDate: "2024-01-02"}},
	})

	sub := &models.Subscription{
		Name:     "热门订阅",
		Source:   "tmdb",
		Category: "movie",
		Enable:   true,
	}
	if err := svc.CreateSubscription(sub); err != nil {
		t.Fatalf("Failed to create subscription: %v", err)
	}

	callbackName := "subscription:test_daily_hot_history_create_error"
	if err := db.Callback().Create().Before("gorm:create").Register(callbackName, func(tx *gorm.DB) {
		if tx.Statement.Schema != nil && tx.Statement.Schema.Table == (&models.DailyHotHistory{}).TableName() {
			_ = tx.AddError(errors.New("injected daily hot history create failure"))
		}
	}); err != nil {
		t.Fatalf("register create callback: %v", err)
	}
	defer func() {
		if removeErr := db.Callback().Create().Remove(callbackName); removeErr != nil {
			t.Fatalf("remove create callback: %v", removeErr)
		}
	}()

	svc.processSubscription(sub)

	if got := atomic.LoadInt32(&searchRequests); got != 0 {
		t.Fatalf("expected search not to run after history create failure, got %d requests", got)
	}

	var historyCount int64
	if err := db.Model(&models.DailyHotHistory{}).Count(&historyCount).Error; err != nil {
		t.Fatalf("count daily hot history: %v", err)
	}

	if historyCount != 0 {
		t.Fatalf("expected no history rows after injected create failure, got %d", historyCount)
	}
}

func TestGetDailyHotMovies(t *testing.T) {
	db, err := setupTestDB()
	if err != nil {
		t.Fatalf("Failed to setup test DB: %v", err)
	}

	logger := zap.NewNop()
	config := &SubscriptionConfig{
		EnableTMDB:   false, // 禁用 TMDB 以避免实际调用
		EnableDouban: false,
	}
	svc := NewService(db, logger, config)

	movies, err := svc.GetDailyHotMovies()
	if err != nil {
		t.Fatalf("GetDailyHotMovies failed: %v", err)
	}

	// 由于 TMDB 和 Douban 都禁用了，应该返回空列表
	if len(movies) != 0 {
		t.Logf("Expected 0 movies when services are disabled, got %d", len(movies))
	}
}

func TestGetDailyHotTVs(t *testing.T) {
	db, err := setupTestDB()
	if err != nil {
		t.Fatalf("Failed to setup test DB: %v", err)
	}

	logger := zap.NewNop()
	config := &SubscriptionConfig{
		EnableTMDB: false,
	}
	svc := NewService(db, logger, config)

	tvs, err := svc.GetDailyHotTVs()
	if err != nil {
		t.Fatalf("GetDailyHotTVs failed: %v", err)
	}

	// 由于 TMDB 禁用了，应该返回空列表
	if len(tvs) != 0 {
		t.Logf("Expected 0 TVs when service is disabled, got %d", len(tvs))
	}
}

func TestRunSubscriptionJob(t *testing.T) {
	db, err := setupTestDB()
	if err != nil {
		t.Fatalf("Failed to setup test DB: %v", err)
	}

	logger := zap.NewNop()
	config := &SubscriptionConfig{}
	svc := NewService(db, logger, config)

	// 创建测试订阅
	sub := &models.Subscription{
		Name:     "测试订阅",
		Source:   "tmdb",
		Category: "movie",
		Enable:   true,
		Keywords: "test",
	}

	err = svc.CreateSubscription(sub)
	if err != nil {
		t.Fatalf("Failed to create subscription: %v", err)
	}

	// 运行订阅任务（可能不会实际执行，因为服务未配置）
	err = svc.RunSubscriptionJob()
	if err != nil {
		t.Logf("RunSubscriptionJob returned error (may be expected): %v", err)
	}
}

func TestUpdateSubscriptionTimestamp(t *testing.T) {
	db, err := setupTestDB()
	if err != nil {
		t.Fatalf("Failed to setup test DB: %v", err)
	}

	logger := zap.NewNop()
	config := &SubscriptionConfig{}
	svc := NewService(db, logger, config)

	sub := &models.Subscription{
		Name:     "测试",
		Source:   "tmdb",
		Category: "movie",
		Enable:   true,
	}

	err = svc.CreateSubscription(sub)
	if err != nil {
		t.Fatalf("Failed to create subscription: %v", err)
	}

	initialUpdated := sub.UpdatedAt

	// 等待一小段时间
	time.Sleep(10 * time.Millisecond)

	sub.Name = "更新测试"

	err = svc.UpdateSubscription(sub)
	if err != nil {
		t.Fatalf("Failed to update subscription: %v", err)
	}

	if sub.UpdatedAt.Before(initialUpdated) {
		t.Error("Expected UpdatedAt to be updated")
	}
}
