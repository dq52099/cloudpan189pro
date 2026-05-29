package telegram

import (
	stdctx "context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/xxcheng123/cloudpan189-share/internal/framework/httpcontext"
	"github.com/xxcheng123/cloudpan189-share/internal/repository/models"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

func setupTelegramHandlerTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}

	if err := db.AutoMigrate(&models.TelegramUser{}, &models.TelegramSetting{}); err != nil {
		t.Fatalf("migrate test db: %v", err)
	}

	return db
}

func newTelegramTestRouter(db *gorm.DB) *gin.Engine {
	router := gin.New()
	wrapper := httpcontext.NewHandlerFuncWrapper(zap.NewNop())
	handler := NewHandler(db, nil, zap.NewNop())

	router.POST("/users/update", wrapper.Wrap(handler.UpdateUser()))
	router.POST("/settings/update", wrapper.Wrap(handler.UpdateSetting()))
	router.POST("/settings/test", wrapper.Wrap(handler.TestConnection()))
	router.POST("/messages/send", wrapper.Wrap(handler.SendMessage()))
	router.POST("/shares/process", wrapper.Wrap(handler.ProcessShareLink()))

	return router
}

func TestTestConnectionReturnsNotFoundWhenSettingMissing(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db := setupTelegramHandlerTestDB(t)
	router := newTelegramTestRouter(db)
	req := httptest.NewRequestWithContext(
		stdctx.Background(),
		http.MethodPost,
		"/settings/test",
		nil,
	)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	assertTelegramSettingNotInitialized(t, recorder)
}

func TestSendMessageReturnsNotFoundWhenSettingMissing(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db := setupTelegramHandlerTestDB(t)
	router := newTelegramTestRouter(db)
	req := httptest.NewRequestWithContext(
		stdctx.Background(),
		http.MethodPost,
		"/messages/send",
		strings.NewReader(`{"message":"hello"}`),
	)
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	assertTelegramSettingNotInitialized(t, recorder)
}

func TestProcessShareLinkReturnsNotFoundWhenFallbackSettingMissing(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db := setupTelegramHandlerTestDB(t)
	router := newTelegramTestRouter(db)
	req := httptest.NewRequestWithContext(
		stdctx.Background(),
		http.MethodPost,
		"/shares/process",
		strings.NewReader(`{"shareUrl":"https://cloud.189.cn/t/abcDEF"}`),
	)
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	assertTelegramSettingNotInitialized(t, recorder)
}

func assertTelegramSettingNotInitialized(t *testing.T, recorder *httptest.ResponseRecorder) {
	t.Helper()

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("expected not found, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	if !strings.Contains(recorder.Body.String(), "Telegram 配置未初始化") {
		t.Fatalf("expected telegram setting missing message, got body=%s", recorder.Body.String())
	}
}

func TestUpdateSettingPersistsFalseBooleans(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db := setupTelegramHandlerTestDB(t)

	setting := &models.TelegramSetting{
		ID:                1,
		BotTokenEncrypted: "old-token",
		ProxyURL:          "http://old-proxy",
		ProxyType:         "http",
		APIURL:            "https://old.example.com",
		ChatID:            "old-chat",
		DefaultMountPath:  "/old",
		EnableNotify:      true,
		Enable:            true,
	}
	if err := db.Create(setting).Error; err != nil {
		t.Fatalf("create telegram setting: %v", err)
	}

	router := newTelegramTestRouter(db)
	req := httptest.NewRequestWithContext(
		stdctx.Background(),
		http.MethodPost,
		"/settings/update",
		strings.NewReader(`{"botToken":"new-token","proxyURL":"","proxyType":"","apiURL":"https://api.telegram.org","chatID":"new-chat","defaultMountPath":"/new","enableNotify":false,"enable":false}`),
	)
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected ok, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	var updated models.TelegramSetting
	if err := db.First(&updated, setting.ID).Error; err != nil {
		t.Fatalf("query telegram setting: %v", err)
	}

	if updated.BotTokenEncrypted != "new-token" {
		t.Fatalf("expected token updated, got %q", updated.BotTokenEncrypted)
	}

	if updated.EnableNotify {
		t.Fatal("expected enable_notify to be false")
	}

	if updated.Enable {
		t.Fatal("expected enable to be false")
	}

	if updated.DefaultMountPath != "/new" {
		t.Fatalf("expected default mount path /new, got %q", updated.DefaultMountPath)
	}
}

func TestUpdateSettingCreatesSingletonWithExplicitFalseValues(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db := setupTelegramHandlerTestDB(t)
	router := newTelegramTestRouter(db)
	req := httptest.NewRequestWithContext(
		stdctx.Background(),
		http.MethodPost,
		"/settings/update",
		strings.NewReader(`{"botToken":"token","proxyURL":"","proxyType":"","apiURL":"https://api.telegram.org","chatID":"","defaultMountPath":"","enableNotify":false,"enable":false}`),
	)
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected ok, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	var settings []models.TelegramSetting
	if err := db.Find(&settings).Error; err != nil {
		t.Fatalf("query telegram settings: %v", err)
	}

	if len(settings) != 1 {
		t.Fatalf("expected one telegram setting, got %d: %#v", len(settings), settings)
	}

	setting := settings[0]
	if setting.ID != 1 {
		t.Fatalf("expected singleton id 1, got %d", setting.ID)
	}

	if setting.EnableNotify {
		t.Fatal("expected enable_notify to be false on create")
	}

	if setting.Enable {
		t.Fatal("expected enable to be false on create")
	}

	if setting.DefaultMountPath != "" {
		t.Fatalf("expected empty default mount path to be persisted, got %q", setting.DefaultMountPath)
	}
}

func TestUpdateSettingPartialUpdatePreservesExistingValues(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db := setupTelegramHandlerTestDB(t)

	setting := &models.TelegramSetting{
		ID:                1,
		BotTokenEncrypted: "old-token",
		ProxyURL:          "http://old-proxy",
		ProxyType:         "http",
		APIURL:            "https://old.example.com",
		ChatID:            "old-chat",
		DefaultMountPath:  "/old",
		EnableNotify:      true,
		Enable:            true,
	}
	if err := db.Create(setting).Error; err != nil {
		t.Fatalf("create telegram setting: %v", err)
	}

	router := newTelegramTestRouter(db)
	req := httptest.NewRequestWithContext(
		stdctx.Background(),
		http.MethodPost,
		"/settings/update",
		strings.NewReader(`{"defaultMountPath":"/new"}`),
	)
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected ok, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	var updated models.TelegramSetting
	if err := db.First(&updated, setting.ID).Error; err != nil {
		t.Fatalf("query telegram setting: %v", err)
	}

	if updated.DefaultMountPath != "/new" {
		t.Fatalf("expected default mount path updated, got %q", updated.DefaultMountPath)
	}

	if updated.BotTokenEncrypted != "old-token" ||
		updated.ProxyURL != "http://old-proxy" ||
		updated.ProxyType != "http" ||
		updated.APIURL != "https://old.example.com" ||
		updated.ChatID != "old-chat" ||
		!updated.EnableNotify ||
		!updated.Enable {
		t.Fatalf("expected partial update to preserve existing setting, got %+v", updated)
	}
}

func TestUpdateUserPersistsFalseAdminValue(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db := setupTelegramHandlerTestDB(t)

	user := &models.TelegramUser{
		UserID:     100,
		Username:   "tester",
		MountPath:  "/old",
		IsAdmin:    true,
		LastSeenAt: time.Now(),
	}
	if err := db.Create(user).Error; err != nil {
		t.Fatalf("create telegram user: %v", err)
	}

	router := newTelegramTestRouter(db)
	req := httptest.NewRequestWithContext(
		stdctx.Background(),
		http.MethodPost,
		"/users/update",
		strings.NewReader(`{"userID":100,"mountPath":"/new","isAdmin":false}`),
	)
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected ok, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	var updated models.TelegramUser
	if err := db.First(&updated, user.ID).Error; err != nil {
		t.Fatalf("query telegram user: %v", err)
	}

	if updated.MountPath != "/new" {
		t.Fatalf("expected mount path /new, got %q", updated.MountPath)
	}

	if updated.IsAdmin {
		t.Fatal("expected is_admin to be false")
	}
}

func TestUpdateUserRejectsInvalidUserID(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name string
		body string
	}{
		{name: "missing", body: `{"mountPath":"/new","isAdmin":false}`},
		{name: "zero", body: `{"userID":0,"mountPath":"/new","isAdmin":false}`},
		{name: "negative", body: `{"userID":-1,"mountPath":"/new","isAdmin":false}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := setupTelegramHandlerTestDB(t)
			router := newTelegramTestRouter(db)
			req := httptest.NewRequestWithContext(
				stdctx.Background(),
				http.MethodPost,
				"/users/update",
				strings.NewReader(tt.body),
			)
			req.Header.Set("Content-Type", "application/json")

			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, req)

			if recorder.Code != http.StatusBadRequest {
				t.Fatalf("expected bad request, got %d body=%s", recorder.Code, recorder.Body.String())
			}

			var count int64
			if err := db.Model(&models.TelegramUser{}).Count(&count).Error; err != nil {
				t.Fatalf("count telegram users: %v", err)
			}

			if count != 0 {
				t.Fatalf("expected invalid update not to create user, got count %d", count)
			}
		})
	}
}

func TestUpdateUserReturnsNotFoundWhenMissing(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db := setupTelegramHandlerTestDB(t)
	router := newTelegramTestRouter(db)
	req := httptest.NewRequestWithContext(
		stdctx.Background(),
		http.MethodPost,
		"/users/update",
		strings.NewReader(`{"userID":999,"mountPath":"/new","isAdmin":false}`),
	)
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("expected not found, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	var count int64
	if err := db.Model(&models.TelegramUser{}).Count(&count).Error; err != nil {
		t.Fatalf("count telegram users: %v", err)
	}

	if count != 0 {
		t.Fatalf("expected missing update not to create user, got count %d", count)
	}
}

func TestCheckTelegramSettingUpdateResultAllowsExistingNoop(t *testing.T) {
	db := setupTelegramHandlerTestDB(t)
	handler := NewHandler(db, nil, zap.NewNop())

	setting := &models.TelegramSetting{
		ID:                1,
		BotTokenEncrypted: "token",
		DefaultMountPath:  "/转存",
		APIURL:            "https://api.telegram.org",
	}
	if err := db.Create(setting).Error; err != nil {
		t.Fatalf("create telegram setting: %v", err)
	}

	if err := handler.checkTelegramSettingUpdateResult(&gorm.DB{RowsAffected: 0}, setting.ID); err != nil {
		t.Fatalf("expected existing no-op setting update to pass, got %v", err)
	}
}

func TestCheckTelegramSettingUpdateResultReturnsNotFoundWhenMissing(t *testing.T) {
	db := setupTelegramHandlerTestDB(t)
	handler := NewHandler(db, nil, zap.NewNop())

	err := handler.checkTelegramSettingUpdateResult(&gorm.DB{RowsAffected: 0}, 999)
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("expected record not found, got %v", err)
	}
}

func TestCheckTelegramUserUpdateResultAllowsExistingNoop(t *testing.T) {
	db := setupTelegramHandlerTestDB(t)
	handler := NewHandler(db, nil, zap.NewNop())

	user := &models.TelegramUser{
		UserID:     100,
		Username:   "tester",
		MountPath:  "/转存",
		LastSeenAt: time.Now(),
	}
	if err := db.Create(user).Error; err != nil {
		t.Fatalf("create telegram user: %v", err)
	}

	if err := handler.checkTelegramUserUpdateResult(&gorm.DB{RowsAffected: 0}, user.ID); err != nil {
		t.Fatalf("expected existing no-op user update to pass, got %v", err)
	}
}

func TestCheckTelegramUserUpdateResultReturnsNotFoundWhenMissing(t *testing.T) {
	db := setupTelegramHandlerTestDB(t)
	handler := NewHandler(db, nil, zap.NewNop())

	err := handler.checkTelegramUserUpdateResult(&gorm.DB{RowsAffected: 0}, 999)
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("expected record not found, got %v", err)
	}
}
