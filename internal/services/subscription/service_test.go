package subscription

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/xxcheng123/cloudpan189-share/internal/consts"
	appContext "github.com/xxcheng123/cloudpan189-share/internal/framework/context"
	"github.com/xxcheng123/cloudpan189-share/internal/pkgs/utils"
	"github.com/xxcheng123/cloudpan189-share/internal/repository/models"
	"github.com/xxcheng123/cloudpan189-share/internal/services/douban"
	"github.com/xxcheng123/cloudpan189-share/internal/services/tmdb"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
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

func TestNewServiceWithNilLoggerDoesNotPanic(t *testing.T) {
	db, err := setupTestDB()
	if err != nil {
		t.Fatalf("Failed to setup test DB: %v", err)
	}

	svc := NewService(db, nil, &SubscriptionConfig{})
	if svc == nil {
		t.Fatal("Expected service to be created")
	}

	if err := svc.RunSubscriptionJob(); err != nil {
		t.Fatalf("RunSubscriptionJob failed: %v", err)
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
		info: &ShareInfo{
			ID:         "share-file-id",
			Name:       "分享目录",
			IsFolder:   true,
			ShareId:    67890,
			ShareMode:  2,
			AccessCode: "share-access-code",
		},
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

	assertSubscriptionAdditionValue(t, mountService.requests[0].Addition, consts.FileAdditionKeyShareId, int64(67890))
	assertSubscriptionAdditionValue(t, mountService.requests[0].Addition, consts.FileAdditionKeyIsFolder, true)
	assertSubscriptionAdditionValue(t, mountService.requests[0].Addition, consts.FileAdditionKeyAccessCode, "share-access-code")
	assertSubscriptionAdditionValue(t, mountService.requests[0].Addition, consts.FileAdditionKeyShareMode, 2)

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

func TestMatchAndMountEscapesLiteralPercentInGeneratedMountPath(t *testing.T) {
	db, err := setupTestDB()
	if err != nil {
		t.Fatalf("Failed to setup test DB: %v", err)
	}

	svc := NewService(db, zap.NewNop(), &SubscriptionConfig{}).(*service)
	mountService := &mockMountService{id: 42}
	svc.SetMountService(mountService)
	svc.SetShareInfoFetcher(&mockShareInfoFetcher{
		info: &ShareInfo{
			ID:       "share-file-id",
			Name:     "分享目录",
			IsFolder: true,
			ShareId:  67890,
		},
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

	result, err := svc.MatchAndMount(
		sub,
		SearchResult{Title: "a%2Fb", ShareURL: "https://cloud.189.cn/t/abcdef"},
		"a%2Fb",
		"2024",
		"movie",
	)
	if err != nil {
		t.Fatalf("MatchAndMount should succeed: %v", err)
	}

	wantPath := "/订阅/a%252Fb (2024)"
	if result == nil || !result.Success || result.STrmPath != wantPath {
		t.Fatalf("expected successful result path %q, got %+v", wantPath, result)
	}

	if len(mountService.requests) != 1 {
		t.Fatalf("expected one mount request, got %d", len(mountService.requests))
	}

	if mountService.requests[0].LocalPath != wantPath {
		t.Fatalf("expected mount request path %q, got %q", wantPath, mountService.requests[0].LocalPath)
	}
}

func TestRecordMatchHistoryRedactsCreateFailureLog(t *testing.T) {
	db, err := setupTestDB()
	if err != nil {
		t.Fatalf("Failed to setup test DB: %v", err)
	}

	core, logs := observer.New(zap.ErrorLevel)
	svc := NewService(db, zap.New(core), &SubscriptionConfig{}).(*service)

	callbackName := "subscription:test_record_match_history_redact_create_error"
	rawErr := errors.New(`GET "https://proxy-user:proxy-pass@api.example.test/file?accessToken=secret-access&filename=private-name.mkv&shareCode=abcDEF&shareAccessCode=wxyz#token=fragment-secret": Authorization: Bearer bearer-secret accessCode=wxyz`)

	if err := db.Callback().Create().Before("gorm:create").Register(callbackName, func(tx *gorm.DB) {
		if tx.Statement.Schema != nil && tx.Statement.Schema.Table == (&models.MatchHistory{}).TableName() {
			_ = tx.AddError(rawErr)
		}
	}); err != nil {
		t.Fatalf("register create callback: %v", err)
	}
	defer func() {
		if removeErr := db.Callback().Create().Remove(callbackName); removeErr != nil {
			t.Fatalf("remove create callback: %v", removeErr)
		}
	}()

	svc.recordMatchHistory(&models.MatchHistory{
		SubscriptionID: 1,
		Title:          "敏感失败记录",
		Category:       models.SubscriptionCategoryMovie,
		Status:         models.MatchStatusFailed,
		ShareURL:       "https://cloud.189.cn/t/abcDEF?shareAccessCode=wxyz#token=fragment-secret",
		STrmPath:       "/订阅/敏感失败记录",
		ErrorMessage:   rawErr.Error(),
	}, "记录匹配历史失败")

	entries := logs.FilterMessage("记录匹配历史失败").All()
	if len(entries) != 1 {
		t.Fatalf("expected one create failure log, got %d", len(entries))
	}

	fields := entries[0].ContextMap()
	logText := entries[0].Message

	for _, key := range []string{"error", "share_url", "error_message"} {
		if value, ok := fields[key].(string); ok {
			logText += " " + value
		}
	}

	for _, leaked := range []string{"proxy-user", "proxy-pass", "secret-access", "private-name.mkv", "abcDEF", "wxyz", "fragment-secret", "bearer-secret"} {
		if strings.Contains(logText, leaked) {
			t.Fatalf("expected %q to be redacted from log %q", leaked, logText)
		}
	}

	if !strings.Contains(logText, utils.RedactedSecret) {
		t.Fatalf("expected redacted placeholder in log %q", logText)
	}
}

func TestMatchAndMountDefaultsShareModeInAddition(t *testing.T) {
	db, err := setupTestDB()
	if err != nil {
		t.Fatalf("Failed to setup test DB: %v", err)
	}

	svc := NewService(db, zap.NewNop(), &SubscriptionConfig{}).(*service)
	mountService := &mockMountService{id: 42}
	svc.SetMountService(mountService)
	svc.SetShareInfoFetcher(&mockShareInfoFetcher{
		info: &ShareInfo{
			ID:         "default-share-file-id",
			Name:       "默认分享模式目录",
			IsFolder:   false,
			ShareId:    13579,
			AccessCode: "default-access-code",
		},
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

	result, err := svc.MatchAndMount(
		sub,
		SearchResult{Title: "默认分享模式电影", ShareURL: "https://cloud.189.cn/t/defaultmode"},
		"默认分享模式电影",
		"",
		"movie",
	)
	if err != nil {
		t.Fatalf("MatchAndMount should succeed: %v", err)
	}

	if result == nil || !result.Success {
		t.Fatalf("expected successful match result, got %+v", result)
	}

	if len(mountService.requests) != 1 {
		t.Fatalf("expected one mount request, got %d", len(mountService.requests))
	}

	req := mountService.requests[0]
	if req.FileId != "default-share-file-id" {
		t.Fatalf("expected share file id passed to mount request, got %q", req.FileId)
	}

	assertSubscriptionAdditionValue(t, req.Addition, consts.FileAdditionKeyShareId, int64(13579))
	assertSubscriptionAdditionValue(t, req.Addition, consts.FileAdditionKeyIsFolder, false)
	assertSubscriptionAdditionValue(t, req.Addition, consts.FileAdditionKeyAccessCode, "default-access-code")
	assertSubscriptionAdditionValue(t, req.Addition, consts.FileAdditionKeyShareMode, 1)
}

func TestMatchAndMountParsesWWWUppercaseShareLinkWithAccessCode(t *testing.T) {
	db, err := setupTestDB()
	if err != nil {
		t.Fatalf("Failed to setup test DB: %v", err)
	}

	svc := NewService(db, zap.NewNop(), &SubscriptionConfig{}).(*service)
	mountService := &mockMountService{id: 42}
	svc.SetMountService(mountService)

	shareInfoFetcher := &mockShareInfoFetcher{
		info: &ShareInfo{
			ID:        "share-file-id",
			Name:      "分享目录",
			IsFolder:  true,
			ShareId:   67890,
			ShareMode: 1,
		},
	}
	svc.SetShareInfoFetcher(shareInfoFetcher)

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

	result, err := svc.MatchAndMount(
		sub,
		SearchResult{Title: "大小写链接电影", ShareURL: "HTTPS://WWW.CLOUD.189.CN/t/AbC123?shareId=1 提取码：wxyz"},
		"大小写链接电影",
		"2024",
		"movie",
	)
	if err != nil {
		t.Fatalf("MatchAndMount should succeed: %v", err)
	}

	if result == nil || !result.Success {
		t.Fatalf("expected successful match result, got %+v", result)
	}

	if len(shareInfoFetcher.shareCodes) != 1 || shareInfoFetcher.shareCodes[0] != "AbC123" {
		t.Fatalf("expected share lookup AbC123, got %v", shareInfoFetcher.shareCodes)
	}

	if len(shareInfoFetcher.accessCodes) != 1 || shareInfoFetcher.accessCodes[0] != "wxyz" {
		t.Fatalf("expected access code lookup wxyz, got %v", shareInfoFetcher.accessCodes)
	}

	if len(mountService.requests) != 1 {
		t.Fatalf("expected one mount request, got %d", len(mountService.requests))
	}

	assertSubscriptionAdditionValue(t, mountService.requests[0].Addition, consts.FileAdditionKeyAccessCode, "wxyz")
}

func TestMatchAndMountRejectsInvalidShareURLBeforeShareInfoLookup(t *testing.T) {
	db, err := setupTestDB()
	if err != nil {
		t.Fatalf("Failed to setup test DB: %v", err)
	}

	svc := NewService(db, zap.NewNop(), &SubscriptionConfig{}).(*service)
	mountService := &mockMountService{id: 42}
	shareInfoFetcher := &mockShareInfoFetcher{}

	svc.SetMountService(mountService)
	svc.SetShareInfoFetcher(shareInfoFetcher)

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

	result, err := svc.MatchAndMount(
		sub,
		SearchResult{Title: "无效链接电影", ShareURL: "https://example.com/share?token=secret-token"},
		"无效链接电影",
		"2024",
		"movie",
	)
	if err != nil {
		t.Fatalf("invalid share URL should record a failed result without returning an error: %v", err)
	}

	if result == nil || result.Success {
		t.Fatalf("expected failed match result, got %+v", result)
	}

	if !strings.Contains(result.Message, "无法从分享链接中提取分享码") {
		t.Fatalf("expected invalid share code message, got %q", result.Message)
	}

	if strings.Contains(result.Message, "secret-token") || strings.Contains(result.Message, "example.com") {
		t.Fatalf("expected invalid input not to be echoed, got %q", result.Message)
	}

	if len(shareInfoFetcher.shareCodes) != 0 || len(shareInfoFetcher.accessCodes) != 0 {
		t.Fatalf("expected share info lookup not to be called, got share=%v access=%v", shareInfoFetcher.shareCodes, shareInfoFetcher.accessCodes)
	}

	if len(mountService.requests) != 0 {
		t.Fatalf("expected mount service not to be called, got %+v", mountService.requests)
	}

	var history models.MatchHistory
	if err := db.Where("subscription_id = ?", sub.ID).First(&history).Error; err != nil {
		t.Fatalf("query match history: %v", err)
	}

	if history.Status != models.MatchStatusFailed {
		t.Fatalf("expected failed history, got %s", history.Status)
	}

	if !strings.Contains(history.ErrorMessage, "无法从分享链接中提取分享码") {
		t.Fatalf("expected invalid share code history, got %q", history.ErrorMessage)
	}

	if strings.Contains(history.ErrorMessage, "secret-token") || strings.Contains(history.ErrorMessage, "example.com") {
		t.Fatalf("expected invalid input not to be echoed in history, got %q", history.ErrorMessage)
	}
}

func TestMatchAndMountRejectsInvalidAccessCodeBeforeShareInfoLookup(t *testing.T) {
	db, err := setupTestDB()
	if err != nil {
		t.Fatalf("Failed to setup test DB: %v", err)
	}

	svc := NewService(db, zap.NewNop(), &SubscriptionConfig{}).(*service)
	mountService := &mockMountService{id: 42}
	shareInfoFetcher := &mockShareInfoFetcher{}

	svc.SetMountService(mountService)
	svc.SetShareInfoFetcher(shareInfoFetcher)

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

	result, err := svc.MatchAndMount(
		sub,
		SearchResult{
			Title:           "无效访问码电影",
			ShareURL:        "https://cloud.189.cn/t/abcDEF",
			ShareAccessCode: "https://example.com/share?token=secret-token",
		},
		"无效访问码电影",
		"2024",
		"movie",
	)
	if err != nil {
		t.Fatalf("invalid access code should record a failed result without returning an error: %v", err)
	}

	if result == nil || result.Success {
		t.Fatalf("expected failed match result, got %+v", result)
	}

	if !strings.Contains(result.Message, "访问码格式无效") {
		t.Fatalf("expected invalid access code message, got %q", result.Message)
	}

	if strings.Contains(result.Message, "secret-token") || strings.Contains(result.Message, "example.com") {
		t.Fatalf("expected invalid input not to be echoed, got %q", result.Message)
	}

	if len(shareInfoFetcher.shareCodes) != 0 || len(shareInfoFetcher.accessCodes) != 0 {
		t.Fatalf("expected share info lookup not to be called, got share=%v access=%v", shareInfoFetcher.shareCodes, shareInfoFetcher.accessCodes)
	}

	if len(mountService.requests) != 0 {
		t.Fatalf("expected mount service not to be called, got %+v", mountService.requests)
	}

	var history models.MatchHistory
	if err := db.Where("subscription_id = ?", sub.ID).First(&history).Error; err != nil {
		t.Fatalf("query match history: %v", err)
	}

	if history.Status != models.MatchStatusFailed {
		t.Fatalf("expected failed history, got %s", history.Status)
	}

	if !strings.Contains(history.ErrorMessage, "访问码格式无效") {
		t.Fatalf("expected invalid access code history, got %q", history.ErrorMessage)
	}

	if strings.Contains(history.ErrorMessage, "secret-token") || strings.Contains(history.ErrorMessage, "example.com") {
		t.Fatalf("expected invalid input not to be echoed in history, got %q", history.ErrorMessage)
	}
}

func TestMatchAndMountRedactsSensitiveFailureMessages(t *testing.T) {
	db, err := setupTestDB()
	if err != nil {
		t.Fatalf("Failed to setup test DB: %v", err)
	}

	svc := NewService(db, zap.NewNop(), &SubscriptionConfig{}).(*service)
	svc.SetMountService(&mockMountService{})
	svc.SetShareInfoFetcher(&mockShareInfoFetcher{
		err: errors.New("GET https://proxy-user:proxy-pass@api.example.test/file?accessToken=secret-access&filename=private-name.mkv&shareCode=abcDEF&shareAccessCode=wxyz#token=fragment-secret Authorization: Bearer bearer-secret"),
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

	result, err := svc.MatchAndMount(
		sub,
		SearchResult{Title: "敏感错误电影", ShareURL: "https://cloud.189.cn/t/abcdef"},
		"敏感错误电影",
		"2024",
		"movie",
	)
	if err == nil {
		t.Fatal("expected share info failure to return an error")
	}

	if result == nil || result.Success {
		t.Fatalf("expected failed match result, got %+v", result)
	}

	var history models.MatchHistory
	if err := db.Where("subscription_id = ?", sub.ID).First(&history).Error; err != nil {
		t.Fatalf("query match history: %v", err)
	}

	for _, text := range []string{err.Error(), result.Message, history.ErrorMessage} {
		for _, leaked := range []string{"proxy-user", "proxy-pass", "secret-access", "private-name.mkv", "abcDEF", "wxyz", "fragment-secret", "bearer-secret"} {
			if strings.Contains(text, leaked) {
				t.Fatalf("expected sensitive value %q to be redacted from %q", leaked, text)
			}
		}

		if !strings.Contains(text, utils.RedactedSecret) {
			t.Fatalf("expected redacted placeholder in %q", text)
		}
	}
}

func TestMatchAndMountRejectsTypedNilMountDependencies(t *testing.T) {
	var (
		typedNilMountService     *mockMountService
		typedNilShareInfoFetcher *mockShareInfoFetcher
	)

	tests := []struct {
		name             string
		mountService     MountService
		shareInfoFetcher ShareInfoFetcher
	}{
		{
			name:             "typed nil mount service",
			mountService:     typedNilMountService,
			shareInfoFetcher: &mockShareInfoFetcher{},
		},
		{
			name:             "typed nil share info fetcher",
			mountService:     &mockMountService{},
			shareInfoFetcher: typedNilShareInfoFetcher,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, err := setupTestDB()
			if err != nil {
				t.Fatalf("Failed to setup test DB: %v", err)
			}

			svc := NewService(db, zap.NewNop(), &SubscriptionConfig{}).(*service)
			svc.SetMountService(tt.mountService)
			svc.SetShareInfoFetcher(tt.shareInfoFetcher)

			result, err := svc.MatchAndMount(
				&models.Subscription{
					Name:      "依赖测试",
					Category:  "movie",
					MountPath: "/订阅",
				},
				SearchResult{Title: "依赖测试", ShareURL: "https://cloud.189.cn/t/abcdef"},
				"依赖测试",
				"2024",
				"movie",
			)
			if err != nil {
				t.Fatalf("expected dependency failure to be recorded without returned error, got %v", err)
			}

			if result == nil || result.Success || !strings.Contains(result.Message, "挂载或分享信息服务未初始化") {
				t.Fatalf("expected dependency failure result, got %+v", result)
			}
		})
	}
}

func TestProcessSearchResultSkipsTypedNilTelegramService(t *testing.T) {
	db, err := setupTestDB()
	if err != nil {
		t.Fatalf("Failed to setup test DB: %v", err)
	}

	svc := NewService(db, zap.NewNop(), &SubscriptionConfig{}).(*service)
	svc.SetMountService(&mockMountService{id: 42})
	svc.SetShareInfoFetcher(&mockShareInfoFetcher{})

	var telegramService *mockTelegramService
	svc.SetTelegramService(telegramService)

	sub := &models.Subscription{
		Name:      "通知测试",
		Source:    "tmdb",
		Category:  "movie",
		MountPath: "/订阅",
		Enable:    true,
	}
	if err := svc.CreateSubscription(sub); err != nil {
		t.Fatalf("create subscription: %v", err)
	}

	if ok := svc.processSearchResult(sub, SearchResult{Title: "通知测试", ShareURL: "https://cloud.189.cn/t/abcdef"}); !ok {
		t.Fatal("expected successful match without telegram notification")
	}

	if sub.SuccessCount != 1 || sub.LastMatchAt == nil {
		t.Fatalf("expected match progress to update, got success=%d last=%v", sub.SuccessCount, sub.LastMatchAt)
	}
}

func TestProcessAutoUpgradeSkipsTypedNilOpenAIService(t *testing.T) {
	db, err := setupTestDB()
	if err != nil {
		t.Fatalf("Failed to setup test DB: %v", err)
	}

	svc := NewService(db, zap.NewNop(), &SubscriptionConfig{}).(*service)

	var openaiService *mockOpenAIService
	svc.SetOpenAIService(openaiService)

	svc.processAutoUpgrade(
		&models.Subscription{ID: 1, Name: "升级测试", Category: "movie"},
		HotResource{Title: "升级测试", Category: "movie", Year: "2024"},
	)

	var historyCount int64
	if err := db.Model(&models.MatchHistory{}).Count(&historyCount).Error; err != nil {
		t.Fatalf("count match history: %v", err)
	}

	if historyCount != 0 {
		t.Fatalf("expected typed nil openai service to skip upgrade history, got %d", historyCount)
	}
}

func TestSubscriptionTreatsTypedNilContentProvidersAsMissing(t *testing.T) {
	db, err := setupTestDB()
	if err != nil {
		t.Fatalf("Failed to setup test DB: %v", err)
	}

	svc := NewService(db, zap.NewNop(), &SubscriptionConfig{EnableTMDB: true, EnableDouban: true}).(*service)

	var (
		tmdbService   *mockTMDBService
		doubanService *mockDoubanService
	)

	svc.SetTMDBService(tmdbService)
	svc.SetDoubanService(doubanService)

	if got := svc.getTMDbMovies(tmdbService); len(got) != 0 {
		t.Fatalf("expected no TMDB movies, got %+v", got)
	}

	if got := svc.getTMDbTVs(tmdbService); len(got) != 0 {
		t.Fatalf("expected no TMDB TVs, got %+v", got)
	}

	if got := svc.getDoubanMovies(doubanService); len(got) != 0 {
		t.Fatalf("expected no Douban movies, got %+v", got)
	}

	movies, err := svc.GetDailyHotMovies()
	if err != nil {
		t.Fatalf("GetDailyHotMovies: %v", err)
	}

	if len(movies) != 0 {
		t.Fatalf("expected no daily hot movies, got %+v", movies)
	}

	tvs, err := svc.GetDailyHotTVs()
	if err != nil {
		t.Fatalf("GetDailyHotTVs: %v", err)
	}

	if len(tvs) != 0 {
		t.Fatalf("expected no daily hot TVs, got %+v", tvs)
	}
}

func TestSubscriptionDependencySnapshotsAvoidConcurrentSetterRace(t *testing.T) {
	db, err := setupTestDB()
	if err != nil {
		t.Fatalf("Failed to setup test DB: %v", err)
	}

	svc := NewService(db, zap.NewNop(), &SubscriptionConfig{}).(*service)
	svc.SetMountService(&mockMountService{})
	svc.SetShareInfoFetcher(&mockShareInfoFetcher{})
	svc.SetTelegramService(&mockTelegramService{})
	svc.SetOpenAIService(&mockOpenAIService{err: errors.New("skip upgrade")})

	sub := &models.Subscription{
		Name:      "并发订阅",
		Source:    "tmdb",
		Category:  "movie",
		MountPath: "/订阅",
		Enable:    true,
	}
	if err := svc.CreateSubscription(sub); err != nil {
		t.Fatalf("create subscription: %v", err)
	}

	stop := make(chan struct{})

	var wg sync.WaitGroup
	wg.Add(1)

	go func() {
		defer wg.Done()

		for {
			select {
			case <-stop:
				return
			default:
				svc.SetTelegramService(&mockTelegramService{})
				svc.SetOpenAIService(&mockOpenAIService{err: errors.New("skip upgrade")})
				svc.SetMountService(&mockMountService{})
				svc.SetShareInfoFetcher(&mockShareInfoFetcher{})
			}
		}
	}()

	for i := 0; i < 100; i++ {
		_, _ = svc.MatchAndMount(
			sub,
			SearchResult{Title: "并发资源", ShareURL: "https://cloud.189.cn/t/abcdef"},
			"并发资源",
			"2024",
			"movie",
		)
		svc.processAutoUpgrade(sub, HotResource{Title: "并发资源", Category: "movie", Year: "2024"})
		_ = svc.processSearchResult(sub, SearchResult{Title: "并发资源", ShareURL: "https://cloud.189.cn/t/abcdef"})
	}

	close(stop)
	wg.Wait()
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

// Mock services for testing
type mockTelegramService struct {
	calls int32
	err   error
}

func (m *mockTelegramService) SendNotification(title, content string) error {
	atomic.AddInt32(&m.calls, 1)

	if m.err != nil {
		return m.err
	}

	return nil
}

type mockTMDBService struct {
	movies     []tmdb.Movie
	tvs        []tmdb.TV
	movieCalls int32
	tvCalls    int32
}

func (m *mockTMDBService) GetPopularMovies(page int) ([]tmdb.Movie, error) {
	atomic.AddInt32(&m.movieCalls, 1)

	if m.movies != nil {
		return m.movies, nil
	}

	return []tmdb.Movie{}, nil
}

func (m *mockTMDBService) GetPopularTVs(page int) ([]tmdb.TV, error) {
	atomic.AddInt32(&m.tvCalls, 1)

	if m.tvs != nil {
		return m.tvs, nil
	}

	return []tmdb.TV{}, nil
}

type mockDoubanService struct {
	calls int32
}

func (m *mockDoubanService) GetPopularMovies() ([]douban.Subject, error) {
	atomic.AddInt32(&m.calls, 1)

	return []douban.Subject{}, nil
}

type mockOpenAIService struct {
	calls   int32
	keyword string
	err     error
}

func (m *mockOpenAIService) GenerateUpgradeKeyword(title, category string) (string, error) {
	atomic.AddInt32(&m.calls, 1)

	if m.err != nil {
		return "", m.err
	}

	return m.keyword, nil
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
	info        *ShareInfo
	err         error
	shareCodes  []string
	accessCodes []string
}

func (m *mockShareInfoFetcher) GetShareInfo(_ appContext.Context, shareCode string, accessCode string) (*ShareInfo, error) {
	m.shareCodes = append(m.shareCodes, shareCode)
	m.accessCodes = append(m.accessCodes, accessCode)

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

func TestUpdateConfigSwitchesSearchPanRuntimeURL(t *testing.T) {
	db, err := setupTestDB()
	if err != nil {
		t.Fatalf("Failed to setup test DB: %v", err)
	}

	var oldRequests int32

	oldServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&oldRequests, 1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer oldServer.Close()

	var newRequests int32

	newServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&newRequests, 1)

		if r.URL.Query().Get("kw") != "runtime" {
			t.Fatalf("Expected kw=runtime, got %s", r.URL.Query().Get("kw"))
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
							"url": "https://cloud.189.cn/t/runtime",
								"password": "p123",
							"note": "热更新资源",
							"datetime": "2026-05-19",
							"source": "test",
							"images": []
						}
					]
				}
			}
		}`))
	}))
	defer newServer.Close()

	svc := NewService(db, zap.NewNop(), &SubscriptionConfig{PanSearchURL: oldServer.URL})
	svc.UpdateConfig(SubscriptionConfig{PanSearchURL: newServer.URL})

	results, err := svc.SearchPan("runtime")
	if err != nil {
		t.Fatalf("SearchPan failed after runtime config update: %v", err)
	}

	if got := atomic.LoadInt32(&oldRequests); got != 0 {
		t.Fatalf("expected old search server not to be called, got %d requests", got)
	}

	if got := atomic.LoadInt32(&newRequests); got != 1 {
		t.Fatalf("expected new search server to be called once, got %d requests", got)
	}

	if len(results) != 1 || results[0].Title != "热更新资源" {
		t.Fatalf("unexpected search results: %+v", results)
	}
}

func TestUpdateConfigControlsDailyHotTMDB(t *testing.T) {
	db, err := setupTestDB()
	if err != nil {
		t.Fatalf("Failed to setup test DB: %v", err)
	}

	tmdbSvc := &mockTMDBService{
		movies: []tmdb.Movie{{Title: "Runtime Movie", ReleaseDate: "2024-01-02"}},
	}
	svc := NewService(db, zap.NewNop(), &SubscriptionConfig{EnableTMDB: true, EnableDouban: false})
	svc.SetTMDBService(tmdbSvc)

	movies, err := svc.GetDailyHotMovies()
	if err != nil {
		t.Fatalf("GetDailyHotMovies failed: %v", err)
	}

	if len(movies) != 1 {
		t.Fatalf("expected one TMDB movie before disabling TMDB, got %d", len(movies))
	}

	if got := atomic.LoadInt32(&tmdbSvc.movieCalls); got != 1 {
		t.Fatalf("expected one TMDB call before runtime update, got %d", got)
	}

	svc.UpdateConfig(SubscriptionConfig{EnableTMDB: false, EnableDouban: false})

	movies, err = svc.GetDailyHotMovies()
	if err != nil {
		t.Fatalf("GetDailyHotMovies failed after runtime config update: %v", err)
	}

	if len(movies) != 0 {
		t.Fatalf("expected no movies after disabling TMDB, got %d", len(movies))
	}

	if got := atomic.LoadInt32(&tmdbSvc.movieCalls); got != 1 {
		t.Fatalf("expected TMDB not to be called after disabling, got %d calls", got)
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
								"password": "p123",
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

	if results[0].ShareAccessCode != "p123" {
		t.Fatalf("expected pan search password to be preserved as share access code, got %+v", results[0])
	}
}

func TestSearchPanPreservesExistingQuery(t *testing.T) {
	db, err := setupTestDB()
	if err != nil {
		t.Fatalf("Failed to setup test DB: %v", err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/proxy/api/search" {
			t.Fatalf("Expected /proxy/api/search path, got %s", r.URL.Path)
		}

		query := r.URL.Query()
		if query.Get("token") != "secret-token" || query.Get("source") != "custom" {
			t.Fatalf("expected existing query values preserved, got %s", r.URL.RawQuery)
		}

		if query.Get("kw") != "测试 关键词" || query.Get("cloud_types") != "tianyi" {
			t.Fatalf("expected search query values appended, got %s", r.URL.RawQuery)
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
							"url": "https://cloud.189.cn/t/query",
								"password": "p123",
							"note": "带查询参数资源",
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

	svc := NewService(db, zap.NewNop(), &SubscriptionConfig{
		PanSearchURL: server.URL + "/proxy?token=secret-token&source=custom",
	})

	results, err := svc.SearchPan("测试 关键词")
	if err != nil {
		t.Fatalf("SearchPan failed: %v", err)
	}

	if len(results) != 1 || results[0].Title != "带查询参数资源" {
		t.Fatalf("unexpected search results: %+v", results)
	}
}

func TestBuildPanSearchRequestURLAppendsAPISearchForPartialPathMatch(t *testing.T) {
	got, err := BuildPanSearchRequestURL(
		"https://search.example.com/proxy/api/search-bak?token=secret-token",
		"测试 关键词",
	)
	if err != nil {
		t.Fatalf("build pan search URL: %v", err)
	}

	if !strings.Contains(got, "/proxy/api/search-bak/api/search") {
		t.Fatalf("expected /api/search to be appended for partial path match, got %q", got)
	}

	if !strings.Contains(got, "token=secret-token") {
		t.Fatalf("expected existing query preserved, got %q", got)
	}
}

func TestBuildPanSearchRequestURLKeepsAPISearchWithTrailingSlash(t *testing.T) {
	got, err := BuildPanSearchRequestURL(
		"https://search.example.com/proxy/api/search/?token=secret-token",
		"测试 关键词",
	)
	if err != nil {
		t.Fatalf("build pan search URL: %v", err)
	}

	if strings.Contains(got, "/api/search/api/search") {
		t.Fatalf("expected trailing slash API path not to duplicate /api/search, got %q", got)
	}

	if !strings.Contains(got, "/proxy/api/search/") {
		t.Fatalf("expected configured API path preserved, got %q", got)
	}
}

func TestRedactPanSearchErrorHidesURLQueryAndSensitiveText(t *testing.T) {
	searchURL, err := BuildPanSearchRequestURL(
		"https://search.example.com/api/search?token=secret-token&source=custom",
		"secret keyword",
	)
	if err != nil {
		t.Fatalf("build pan search URL: %v", err)
	}

	got := RedactPanSearchError(searchURL, errors.New("Get \""+searchURL+"\": accessCode=abcd"))
	for _, leaked := range []string{"secret-token", "secret+keyword", "secret keyword", "abcd"} {
		if strings.Contains(got, leaked) {
			t.Fatalf("expected %q to be redacted from %q", leaked, got)
		}
	}

	if !strings.Contains(got, utils.RedactedSecret) {
		t.Fatalf("expected redacted placeholder in %q", got)
	}
}

func TestSearchPanRedactsBodyReadError(t *testing.T) {
	db, err := setupTestDB()
	if err != nil {
		t.Fatalf("Failed to setup test DB: %v", err)
	}

	oldTransport := panSearchClient.Transport

	panSearchClient.Transport = panSearchReadErrorRoundTripper{}
	defer func() {
		panSearchClient.Transport = oldTransport
	}()

	svc := NewService(db, zap.NewNop(), &SubscriptionConfig{
		PanSearchURL: "https://search.example.com/api/search?token=secret-token",
	})

	_, err = svc.SearchPan("secret keyword")
	if err == nil {
		t.Fatal("expected body read error")
	}

	text := err.Error()
	for _, leaked := range []string{"secret-token", "secret+keyword", "secret keyword", "bearer-secret", "abcd"} {
		if strings.Contains(text, leaked) {
			t.Fatalf("expected %q to be redacted from %q", leaked, text)
		}
	}

	if !strings.Contains(text, utils.RedactedSecret) {
		t.Fatalf("expected redacted placeholder in %q", text)
	}
}

func TestSearchPanRedactsBusinessErrorMessage(t *testing.T) {
	db, err := setupTestDB()
	if err != nil {
		t.Fatalf("Failed to setup test DB: %v", err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"code": 1,
			"message": "upstream failed https://proxy-user:proxy-pass@api.example.test/search?accessToken=secret-access&kw=secret+keyword#refreshToken=fragment-secret accessCode=abcd Authorization: Bearer bearer-secret",
			"data": {"total": 0, "merged_by_type": {}}
		}`))
	}))
	defer server.Close()

	svc := NewService(db, zap.NewNop(), &SubscriptionConfig{PanSearchURL: server.URL})

	_, err = svc.SearchPan("secret keyword")
	if err == nil {
		t.Fatal("expected pan search business error")
	}

	text := err.Error()
	for _, leaked := range []string{"proxy-user", "proxy-pass", "secret-access", "secret+keyword", "secret keyword", "fragment-secret", "abcd", "bearer-secret"} {
		if strings.Contains(text, leaked) {
			t.Fatalf("expected %q to be redacted from %q", leaked, text)
		}
	}

	if !strings.Contains(text, utils.RedactedSecret) {
		t.Fatalf("expected redacted placeholder in %q", text)
	}
}

func TestProcessSubscriptionRedactsKeywordInSearchFailureLog(t *testing.T) {
	db, err := setupTestDB()
	if err != nil {
		t.Fatalf("Failed to setup test DB: %v", err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`search unavailable`))
	}))
	defer server.Close()

	core, logs := observer.New(zap.ErrorLevel)
	svc := NewService(db, zap.New(core), &SubscriptionConfig{
		PanSearchURL: server.URL,
	}).(*service)

	sensitiveKeyword := "https://proxy-user:proxy-pass@example.test/share?accessToken=query-secret&filename=private-name.mkv#refreshToken=fragment-secret accessCode=abcd Authorization: Bearer bearer-secret"

	sub := &models.Subscription{
		Name:     "关键词订阅",
		Source:   "custom",
		Category: "movie",
		Keywords: sensitiveKeyword,
		Enable:   true,
	}
	if err := svc.CreateSubscription(sub); err != nil {
		t.Fatalf("Failed to create subscription: %v", err)
	}

	svc.processSubscription(sub)

	entries := logs.FilterMessage("Search failed").All()
	if len(entries) != 1 {
		t.Fatalf("expected one search failure log, got %d", len(entries))
	}

	fields := entries[0].ContextMap()

	keyword, ok := fields["keyword"].(string)
	if !ok {
		t.Fatalf("expected keyword field in log, got %#v", fields)
	}

	errorText, ok := fields["error"].(string)
	if !ok {
		t.Fatalf("expected error field in log, got %#v", fields)
	}

	logText := keyword + " " + errorText
	for _, leaked := range []string{"proxy-user", "proxy-pass", "query-secret", "private-name.mkv", "fragment-secret", "abcd", "bearer-secret"} {
		if strings.Contains(logText, leaked) {
			t.Fatalf("expected %q to be redacted from log %s", leaked, logText)
		}
	}

	if !strings.Contains(keyword, utils.RedactedSecret) {
		t.Fatalf("expected redacted marker in keyword field %q", keyword)
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
		_, _ = w.Write([]byte(`{
			"code": 0,
			"message": "ok",
			"data": {
				"total": 1,
				"merged_by_type": {
					"tianyi": [
						{
							"url": "https://cloud.189.cn/t/abcdef",
								"password": "p123",
							"note": "写入失败电影",
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

	svc := NewService(db, zap.NewNop(), &SubscriptionConfig{
		PanSearchURL: server.URL,
		EnableTMDB:   true,
	}).(*service)
	svc.SetTMDBService(&mockTMDBService{
		movies: []tmdb.Movie{{Title: "查询失败电影", ReleaseDate: "2024-01-02"}},
	})

	sub := &models.Subscription{
		Name:      "热门订阅",
		Source:    "tmdb",
		Category:  "movie",
		MountPath: "/热门",
		Enable:    true,
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

func TestProcessSubscriptionRetriesWhenDailyHotSearchFails(t *testing.T) {
	db, err := setupTestDB()
	if err != nil {
		t.Fatalf("Failed to setup test DB: %v", err)
	}

	var searchRequests int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&searchRequests, 1)
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`search unavailable`))
	}))
	defer server.Close()

	svc := NewService(db, zap.NewNop(), &SubscriptionConfig{
		PanSearchURL: server.URL,
		EnableTMDB:   true,
	}).(*service)
	svc.SetTMDBService(&mockTMDBService{
		movies: []tmdb.Movie{{Title: "搜索失败电影", ReleaseDate: "2024-01-02"}},
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

	svc.processSubscription(sub)
	svc.processSubscription(sub)

	if got := atomic.LoadInt32(&searchRequests); got != 2 {
		t.Fatalf("expected failed search to be retried, got %d requests", got)
	}

	var historyCount int64
	if err := db.Model(&models.DailyHotHistory{}).Count(&historyCount).Error; err != nil {
		t.Fatalf("count daily hot history: %v", err)
	}

	if historyCount != 0 {
		t.Fatalf("expected no history rows after failed searches, got %d", historyCount)
	}
}

func TestProcessSubscriptionRecordsDailyHotHistoryAfterSuccessfulMount(t *testing.T) {
	db, err := setupTestDB()
	if err != nil {
		t.Fatalf("Failed to setup test DB: %v", err)
	}

	var searchRequests int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&searchRequests, 1)
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
								"password": "p123",
							"note": "成功电影",
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

	mountService := &mockMountService{}
	svc := NewService(db, zap.NewNop(), &SubscriptionConfig{
		PanSearchURL: server.URL,
		EnableTMDB:   true,
	}).(*service)
	svc.SetTMDBService(&mockTMDBService{
		movies: []tmdb.Movie{{Title: "成功电影", ReleaseDate: "2024-01-02"}},
	})

	shareInfoFetcher := &mockShareInfoFetcher{}

	svc.SetMountService(mountService)
	svc.SetShareInfoFetcher(shareInfoFetcher)

	sub := &models.Subscription{
		Name:      "热门订阅",
		Source:    "tmdb",
		Category:  "movie",
		MountPath: "/热门",
		Enable:    true,
	}
	if err := svc.CreateSubscription(sub); err != nil {
		t.Fatalf("Failed to create subscription: %v", err)
	}

	svc.processSubscription(sub)
	svc.processSubscription(sub)

	if got := atomic.LoadInt32(&searchRequests); got != 1 {
		t.Fatalf("expected successful daily hot item to be skipped on second run, got %d search requests", got)
	}

	if len(mountService.requests) != 1 {
		t.Fatalf("expected one mount request, got %d", len(mountService.requests))
	}

	if len(shareInfoFetcher.accessCodes) != 1 || shareInfoFetcher.accessCodes[0] != "p123" {
		t.Fatalf("expected pan search password to be used for share lookup, got %v", shareInfoFetcher.accessCodes)
	}

	var historyCount int64
	if err := db.Model(&models.DailyHotHistory{}).Count(&historyCount).Error; err != nil {
		t.Fatalf("count daily hot history: %v", err)
	}

	if historyCount != 1 {
		t.Fatalf("expected one history row after successful mount, got %d", historyCount)
	}
}

func TestProcessSubscriptionStillSearchesWhenDailyHotHistoryCreateFails(t *testing.T) {
	db, err := setupTestDB()
	if err != nil {
		t.Fatalf("Failed to setup test DB: %v", err)
	}

	var searchRequests int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&searchRequests, 1)
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
							"note": "写入失败电影",
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

	mountService := &mockMountService{}
	svc := NewService(db, zap.NewNop(), &SubscriptionConfig{
		PanSearchURL: server.URL,
		EnableTMDB:   true,
	}).(*service)
	svc.SetTMDBService(&mockTMDBService{
		movies: []tmdb.Movie{{Title: "写入失败电影", ReleaseDate: "2024-01-02"}},
	})
	svc.SetMountService(mountService)
	svc.SetShareInfoFetcher(&mockShareInfoFetcher{})

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

	if got := atomic.LoadInt32(&searchRequests); got != 1 {
		t.Fatalf("expected search to run before history create failure, got %d requests", got)
	}

	if len(mountService.requests) != 1 {
		t.Fatalf("expected mount before history create failure, got %d requests", len(mountService.requests))
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
