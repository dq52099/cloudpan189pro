package telegram

import (
	stdctx "context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/xxcheng123/cloudpan189-share/internal/framework/httpcontext"
	"github.com/xxcheng123/cloudpan189-share/internal/pkgs/utils"
	"github.com/xxcheng123/cloudpan189-share/internal/repository/models"
	telegramSvi "github.com/xxcheng123/cloudpan189-share/internal/services/telegram"
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
	return newTelegramTestRouterWithService(db, nil)
}

func newTelegramTestRouterWithService(db *gorm.DB, tgService telegramSvi.Service) *gin.Engine {
	router := gin.New()
	wrapper := httpcontext.NewHandlerFuncWrapper(zap.NewNop())
	handler := NewHandler(db, tgService, zap.NewNop())

	router.GET("/settings", wrapper.Wrap(handler.GetSetting()))
	router.POST("/users/update", wrapper.Wrap(handler.UpdateUser()))
	router.POST("/settings/update", wrapper.Wrap(handler.UpdateSetting()))
	router.POST("/settings/test", wrapper.Wrap(handler.TestConnection()))
	router.POST("/messages/send", wrapper.Wrap(handler.SendMessage()))
	router.POST("/shares/process", wrapper.Wrap(handler.ProcessShareLink()))

	return router
}

type mockTelegramRuntimeService struct {
	enabled            bool
	updateErr          error
	parseResult        *telegramSvi.MountResult
	parseErr           error
	parseContextValues []string
}

func (m *mockTelegramRuntimeService) SendMessage(msg string) error {
	return nil
}

func (m *mockTelegramRuntimeService) SendNotification(title, content string) error {
	return nil
}

func (m *mockTelegramRuntimeService) IsEnabled() bool {
	return m.enabled
}

func (m *mockTelegramRuntimeService) GetProxyURL() string {
	return ""
}

func (m *mockTelegramRuntimeService) GetBotToken() string {
	return ""
}

func (m *mockTelegramRuntimeService) StartBot() error {
	return nil
}

func (m *mockTelegramRuntimeService) StopBot() {
}

func (m *mockTelegramRuntimeService) UpdateConfig(config telegramSvi.Config) error {
	return m.updateErr
}

func (m *mockTelegramRuntimeService) TestConnection() error {
	return nil
}

func (m *mockTelegramRuntimeService) ParseAndMountShareLink(shareURL, mountPath string, autoMount bool) (*telegramSvi.MountResult, error) {
	return m.parseResult, m.parseErr
}

func (m *mockTelegramRuntimeService) ParseAndMountShareLinkWithContext(ctx stdctx.Context, shareURL, mountPath string, autoMount bool) (*telegramSvi.MountResult, error) {
	value, _ := ctx.Value(telegramHandlerContextMarkerKey{}).(string)
	m.parseContextValues = append(m.parseContextValues, value)

	return m.parseResult, m.parseErr
}

func (m *mockTelegramRuntimeService) SetMountDependencies(fetcher telegramSvi.ShareInfoFetcher, mounter telegramSvi.StorageMounter) {
}

type telegramHandlerContextMarkerKey struct{}

func TestInvalidParamsKeepsOriginalErrorForLogging(t *testing.T) {
	err := errors.New("database accessToken=secret-access failed")

	busErr := invalidParams(err)

	if !errors.Is(busErr.GetError(), err) {
		t.Fatalf("expected original error to be preserved, got %v", busErr.GetError())
	}

	if busErr.GetHTTPCode() != http.StatusBadRequest || busErr.GetCode() != 400 {
		t.Fatalf("unexpected business error codes: http=%d business=%d", busErr.GetHTTPCode(), busErr.GetCode())
	}
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

func TestProcessShareLinkRedactsRuntimeServiceErrorMessage(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db := setupTelegramHandlerTestDB(t)
	router := newTelegramTestRouterWithService(db, &mockTelegramRuntimeService{
		enabled: true,
		parseErr: errors.New(
			`telegram failed https://api-user:api-pass@api.telegram.org/bot123456:ABC-def/sendMessage?accessToken=secret-access#refreshToken=secret-fragment 提取码：wxyz Authorization: Bearer bearer-secret botToken=telegram-secret`,
		),
	})
	req := httptest.NewRequestWithContext(
		stdctx.Background(),
		http.MethodPost,
		"/shares/process",
		strings.NewReader(`{"shareUrl":"https://cloud.189.cn/t/abcDEF","autoMount":true}`),
	)
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	assertTelegramBadRequestRedactsSensitiveMessage(t, recorder)
}

func TestProcessShareLinkRedactsRuntimeServiceFailureResultMessage(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db := setupTelegramHandlerTestDB(t)
	router := newTelegramTestRouterWithService(db, &mockTelegramRuntimeService{
		enabled: true,
		parseResult: &telegramSvi.MountResult{
			Success: false,
			Message: `mount failed https://api-user:api-pass@proxy.example.test/api?accessCode=abcd#token=secret-fragment botToken=telegram-secret`,
		},
	})
	req := httptest.NewRequestWithContext(
		stdctx.Background(),
		http.MethodPost,
		"/shares/process",
		strings.NewReader(`{"shareUrl":"https://cloud.189.cn/t/abcDEF","autoMount":true}`),
	)
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	assertTelegramBadRequestRedactsSensitiveMessage(t, recorder)
}

func TestProcessShareLinkUsesRequestContextForRuntimeService(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db := setupTelegramHandlerTestDB(t)
	runtimeSvc := &mockTelegramRuntimeService{
		enabled: true,
		parseResult: &telegramSvi.MountResult{
			Success:   true,
			Message:   "挂载成功",
			MountPath: "/Telegram/Shared",
			ShareID:   123,
			FileID:    "file-id",
		},
	}
	router := newTelegramTestRouterWithService(db, runtimeSvc)
	ctx := stdctx.WithValue(stdctx.Background(), telegramHandlerContextMarkerKey{}, "handler-marker")
	req := httptest.NewRequestWithContext(
		ctx,
		http.MethodPost,
		"/shares/process",
		strings.NewReader(`{"shareUrl":"https://cloud.189.cn/t/abcDEF","autoMount":true}`),
	)
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected ok, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	if got := runtimeSvc.parseContextValues; len(got) != 1 || got[0] != "handler-marker" {
		t.Fatalf("expected handler request context marker, got %#v", got)
	}
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

func assertTelegramBadRequestRedactsSensitiveMessage(t *testing.T, recorder *httptest.ResponseRecorder) {
	t.Helper()

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected bad request, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	body := recorder.Body.String()
	for _, leaked := range []string{
		"api-user",
		"api-pass",
		"ABC-def",
		"secret-access",
		"secret-fragment",
		"wxyz",
		"bearer-secret",
		"telegram-secret",
		"abcd",
	} {
		if strings.Contains(body, leaked) {
			t.Fatalf("expected %q to be redacted, got body=%s", leaked, body)
		}
	}

	if !strings.Contains(body, utils.RedactedSecret) {
		t.Fatalf("expected redacted marker in body=%s", body)
	}
}

func TestGetSettingMasksBotToken(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db := setupTelegramHandlerTestDB(t)

	rawProxyURL := "http://proxy-user:proxy-pass@example.test:8080/proxy?token=secret-token#access_token=secret-fragment"
	rawAPIURL := "https://api-user:api-pass@telegram.example.test/bot?client_secret=secret-client#token=secret-api-fragment"

	setting := &models.TelegramSetting{
		ID:                1,
		BotTokenEncrypted: "stored-token",
		ProxyURL:          rawProxyURL,
		APIURL:            rawAPIURL,
		DefaultMountPath:  "/转存",
		EnableNotify:      true,
		Enable:            true,
	}
	if err := db.Create(setting).Error; err != nil {
		t.Fatalf("create telegram setting: %v", err)
	}

	router := newTelegramTestRouter(db)
	req := httptest.NewRequestWithContext(stdctx.Background(), http.MethodGet, "/settings", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected ok, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	if strings.Contains(recorder.Body.String(), "stored-token") {
		t.Fatalf("expected bot token to be masked, got body=%s", recorder.Body.String())
	}

	for _, leaked := range []string{
		"proxy-user",
		"proxy-pass",
		"secret-token",
		"secret-fragment",
		"api-user",
		"api-pass",
		"secret-client",
		"secret-api-fragment",
	} {
		if strings.Contains(recorder.Body.String(), leaked) {
			t.Fatalf("expected %q to be redacted, got body=%s", leaked, recorder.Body.String())
		}
	}

	var response struct {
		Code int                    `json:"code"`
		Data models.TelegramSetting `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if response.Data.BotToken != utils.RedactedSecret {
		t.Fatalf("expected masked bot token, got %q", response.Data.BotToken)
	}

	if response.Data.BotTokenEncrypted != "" {
		t.Fatalf("expected encrypted token omitted, got %q", response.Data.BotTokenEncrypted)
	}

	if response.Data.ProxyURL != utils.RedactURLForLog(rawProxyURL) {
		t.Fatalf("expected redacted proxy URL, got %q", response.Data.ProxyURL)
	}

	if response.Data.APIURL != utils.RedactURLForLog(rawAPIURL) {
		t.Fatalf("expected redacted API URL, got %q", response.Data.APIURL)
	}

	var stored models.TelegramSetting
	if err := db.First(&stored, setting.ID).Error; err != nil {
		t.Fatalf("query telegram setting: %v", err)
	}

	if stored.BotTokenEncrypted != "stored-token" {
		t.Fatalf("expected stored token unchanged, got %q", stored.BotTokenEncrypted)
	}

	if stored.ProxyURL != rawProxyURL || stored.APIURL != rawAPIURL {
		t.Fatalf("expected stored URLs unchanged, got proxy=%q api=%q", stored.ProxyURL, stored.APIURL)
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

	if strings.Contains(recorder.Body.String(), "new-token") {
		t.Fatalf("expected updated token to be masked in response, got body=%s", recorder.Body.String())
	}

	var response struct {
		Code int                    `json:"code"`
		Data models.TelegramSetting `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if response.Data.BotToken != utils.RedactedSecret {
		t.Fatalf("expected masked bot token in update response, got %q", response.Data.BotToken)
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

func TestUpdateSettingPreservesBotTokenWhenMaskedPlaceholderSubmitted(t *testing.T) {
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
	body := `{"botToken":"` + utils.RedactedSecret + `","defaultMountPath":"/new"}`
	req := httptest.NewRequestWithContext(stdctx.Background(), http.MethodPost, "/settings/update", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected ok, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	if strings.Contains(recorder.Body.String(), "old-token") {
		t.Fatalf("expected existing token to be masked in response, got body=%s", recorder.Body.String())
	}

	var response struct {
		Code int                    `json:"code"`
		Data models.TelegramSetting `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if response.Data.BotToken != utils.RedactedSecret {
		t.Fatalf("expected masked bot token in update response, got %q", response.Data.BotToken)
	}

	var updated models.TelegramSetting
	if err := db.First(&updated, setting.ID).Error; err != nil {
		t.Fatalf("query telegram setting: %v", err)
	}

	if updated.BotTokenEncrypted != "old-token" {
		t.Fatalf("expected masked placeholder to preserve token, got %q", updated.BotTokenEncrypted)
	}

	if updated.DefaultMountPath != "/new" {
		t.Fatalf("expected default mount path updated, got %q", updated.DefaultMountPath)
	}
}

func TestUpdateSettingPreservesURLWhenRedactedPlaceholderSubmitted(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db := setupTelegramHandlerTestDB(t)

	rawProxyURL := "http://proxy-user:proxy-pass@example.test:8080/proxy?token=secret-token#access_token=secret-fragment"
	rawAPIURL := "https://api-user:api-pass@telegram.example.test/bot?client_secret=secret-client#token=secret-api-fragment"

	setting := &models.TelegramSetting{
		ID:                1,
		BotTokenEncrypted: "old-token",
		ProxyURL:          rawProxyURL,
		ProxyType:         "http",
		APIURL:            rawAPIURL,
		ChatID:            "old-chat",
		DefaultMountPath:  "/old",
		EnableNotify:      true,
		Enable:            true,
	}
	if err := db.Create(setting).Error; err != nil {
		t.Fatalf("create telegram setting: %v", err)
	}

	body, err := json.Marshal(map[string]interface{}{
		"proxyURL":         utils.RedactURLForLog(rawProxyURL),
		"apiURL":           utils.RedactURLForLog(rawAPIURL),
		"defaultMountPath": "/new",
	})
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}

	router := newTelegramTestRouter(db)
	req := httptest.NewRequestWithContext(stdctx.Background(), http.MethodPost, "/settings/update", strings.NewReader(string(body)))
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected ok, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	for _, leaked := range []string{
		"proxy-user",
		"proxy-pass",
		"secret-token",
		"secret-fragment",
		"api-user",
		"api-pass",
		"secret-client",
		"secret-api-fragment",
	} {
		if strings.Contains(recorder.Body.String(), leaked) {
			t.Fatalf("expected %q to be redacted, got body=%s", leaked, recorder.Body.String())
		}
	}

	var updated models.TelegramSetting
	if err := db.First(&updated, setting.ID).Error; err != nil {
		t.Fatalf("query telegram setting: %v", err)
	}

	if updated.ProxyURL != rawProxyURL {
		t.Fatalf("expected proxy URL preserved, got %q", updated.ProxyURL)
	}

	if updated.APIURL != rawAPIURL {
		t.Fatalf("expected API URL preserved, got %q", updated.APIURL)
	}

	if updated.DefaultMountPath != "/new" {
		t.Fatalf("expected default mount path updated, got %q", updated.DefaultMountPath)
	}
}

func TestUpdateSettingMergesWhenSettingIsCreatedConcurrently(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db := setupTelegramHandlerTestDB(t)

	const callbackName = "telegram_test_create_setting_before_upsert"

	seeded := false

	if err := db.Callback().Create().Before("gorm:create").Register(callbackName, func(tx *gorm.DB) {
		if seeded || tx.Statement.Table != "telegram_settings" {
			return
		}

		seeded = true

		competing := &models.TelegramSetting{
			ID:                1,
			BotTokenEncrypted: "raced-token",
			ChatID:            "raced-chat",
			DefaultMountPath:  "/raced",
			APIURL:            "https://raced.example.com",
			EnableNotify:      true,
			Enable:            true,
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

	router := newTelegramTestRouter(db)
	req := httptest.NewRequestWithContext(
		stdctx.Background(),
		http.MethodPost,
		"/settings/update",
		strings.NewReader(`{"defaultMountPath":"/new","enable":false}`),
	)
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected ok, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	if !seeded {
		t.Fatal("expected callback to simulate concurrent telegram setting creation")
	}

	var count int64
	if err := db.Model(&models.TelegramSetting{}).Count(&count).Error; err != nil {
		t.Fatalf("count telegram settings: %v", err)
	}

	if count != 1 {
		t.Fatalf("expected one telegram setting, got %d", count)
	}

	var updated models.TelegramSetting
	if err := db.First(&updated, int64(1)).Error; err != nil {
		t.Fatalf("query telegram setting: %v", err)
	}

	if updated.DefaultMountPath != "/new" {
		t.Fatalf("expected request default mount path to win, got %q", updated.DefaultMountPath)
	}

	if updated.Enable {
		t.Fatal("expected request enable=false to win conflict")
	}

	if updated.BotTokenEncrypted != "raced-token" || updated.ChatID != "raced-chat" {
		t.Fatalf("expected concurrent existing fields to be preserved, got %+v", updated)
	}
}

func TestUpdateSettingSynchronizesRuntimeService(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db := setupTelegramHandlerTestDB(t)

	runtimeService := telegramSvi.NewService("old-token", "old-chat", "", "", "", zap.NewNop())
	router := newTelegramTestRouterWithService(db, runtimeService)
	req := httptest.NewRequestWithContext(
		stdctx.Background(),
		http.MethodPost,
		"/settings/update",
		strings.NewReader(`{"botToken":"new-token","proxyURL":"http://127.0.0.1:7890","proxyType":"http","apiURL":"https://api.telegram.org","chatID":"new-chat","defaultMountPath":"/new","enableNotify":true,"enable":false}`),
	)
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected ok, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	if runtimeService.GetBotToken() != "new-token" {
		t.Fatalf("expected runtime token synchronized, got %q", runtimeService.GetBotToken())
	}

	if runtimeService.GetProxyURL() != "http://127.0.0.1:7890" {
		t.Fatalf("expected runtime proxy synchronized, got %q", runtimeService.GetProxyURL())
	}

	if runtimeService.IsEnabled() {
		t.Fatal("expected runtime service disabled after enable=false update")
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

func TestUpdateUserTargetsCurrentRowByTelegramUserID(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db := setupTelegramHandlerTestDB(t)

	user := &models.TelegramUser{
		UserID:     100,
		Username:   "stale",
		MountPath:  "/old",
		IsAdmin:    true,
		LastSeenAt: time.Now(),
	}
	if err := db.Create(user).Error; err != nil {
		t.Fatalf("create telegram user: %v", err)
	}

	const callbackName = "telegram_test_replace_user_before_update"

	replaced := false

	if err := db.Callback().Update().Before("gorm:update").Register(callbackName, func(tx *gorm.DB) {
		if replaced || tx.Statement.Table != "telegram_users" {
			return
		}

		replaced = true

		if err := tx.Session(&gorm.Session{NewDB: true}).
			Exec("DELETE FROM telegram_users WHERE id = ?", user.ID).Error; err != nil {
			_ = tx.AddError(err)

			return
		}

		replacement := &models.TelegramUser{
			UserID:     user.UserID,
			Username:   "replacement",
			MountPath:  "/replacement",
			IsAdmin:    true,
			LastSeenAt: time.Now(),
		}
		if err := tx.Session(&gorm.Session{NewDB: true}).Create(replacement).Error; err != nil {
			_ = tx.AddError(err)
		}
	}); err != nil {
		t.Fatalf("register update callback: %v", err)
	}

	t.Cleanup(func() {
		_ = db.Callback().Update().Remove(callbackName)
	})

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

	if !replaced {
		t.Fatal("expected callback to simulate telegram user replacement")
	}

	var users []models.TelegramUser
	if err := db.Order("id").Find(&users).Error; err != nil {
		t.Fatalf("query telegram users: %v", err)
	}

	if len(users) != 1 {
		t.Fatalf("expected one telegram user, got %d: %#v", len(users), users)
	}

	updated := users[0]
	if updated.ID == user.ID {
		t.Fatalf("expected stale row to be replaced, got original id %d", updated.ID)
	}

	if updated.MountPath != "/new" || updated.IsAdmin {
		t.Fatalf("expected replacement row to be updated by user_id, got %+v", updated)
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
