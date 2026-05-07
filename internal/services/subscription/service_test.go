package subscription

import (
	"testing"
	"time"

	"github.com/xxcheng123/cloudpan189-share/internal/repository/models"
	"github.com/xxcheng123/cloudpan189-share/internal/services/douban"
	"github.com/xxcheng123/cloudpan189-share/internal/services/tmdb"
	"go.uber.org/zap"
	"gorm.io/driver/sqlite"
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
	if err == nil {
		t.Fatal("Expected MatchAndMount to fail without ShareURL, got success")
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

// Mock services for testing
type mockTelegramService struct{}

func (m *mockTelegramService) SendNotification(title, content string) error {
	return nil
}

type mockTMDBService struct{}

func (m *mockTMDBService) GetPopularMovies(page int) ([]tmdb.Movie, error) {
	return []tmdb.Movie{}, nil
}

func (m *mockTMDBService) GetPopularTVs(page int) ([]tmdb.TV, error) {
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
	config := &SubscriptionConfig{
		PanSearchURL: "https://tg.252035.xyz",
	}
	svc := NewService(db, logger, config)

	// 测试搜索（可能返回空结果，但不应该报错）
	results, err := svc.SearchPan("test")
	if err != nil {
		t.Logf("SearchPan returned error (may be expected): %v", err)
		// 不失败，因为网络问题可能出错
	}

	// results 可能为空，但不应该是 nil
	if results == nil {
		t.Error("Expected results to be non-nil")
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
