package setting

import (
	stdctx "context"
	"errors"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/xxcheng123/cloudpan189-share/internal/bootstrap"
	"github.com/xxcheng123/cloudpan189-share/internal/framework/context"
	"github.com/xxcheng123/cloudpan189-share/internal/pkgs/taskengine"
	"github.com/xxcheng123/cloudpan189-share/internal/pkgs/utils"
	"github.com/xxcheng123/cloudpan189-share/internal/repository/models"
	"github.com/xxcheng123/cloudpan189-share/internal/shared"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type settingTestDB struct {
	db *gorm.DB
}

func (t *settingTestDB) GetDB(ctx context.Context) *gorm.DB {
	return t.db.WithContext(ctx)
}

func (t *settingTestDB) GetDBWithoutContext() *gorm.DB {
	return t.db
}

func (t *settingTestDB) GetLogger(name string, fields ...zap.Field) *zap.Logger {
	return zap.NewNop()
}

func (t *settingTestDB) Close() {}

func (t *settingTestDB) GetPort() int {
	return 9999
}

func (t *settingTestDB) GetTaskEngine() taskengine.TaskEngine {
	return nil
}

func (t *settingTestDB) GetHTTPEngine() *gin.Engine {
	return nil
}

var _ bootstrap.ServiceContext = (*settingTestDB)(nil)

func setupSettingTestDB(t *testing.T) *settingTestDB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}

	if err := db.AutoMigrate(&models.Setting{}); err != nil {
		t.Fatalf("migrate test db: %v", err)
	}

	return &settingTestDB{db: db}
}

func restoreSharedSetting(t *testing.T) {
	t.Helper()

	oldSaltKey := shared.SaltKey
	oldBaseURL := shared.BaseURL
	oldEnableAuth := shared.EnableAuth
	oldAddition := shared.SettingAddition

	t.Cleanup(func() {
		shared.SaltKey = oldSaltKey
		shared.BaseURL = oldBaseURL
		shared.EnableAuth = oldEnableAuth
		shared.SettingAddition = oldAddition
	})
}

func createSetting(t *testing.T, db *gorm.DB) *models.Setting {
	t.Helper()

	setting := &models.Setting{
		ID:          1,
		Title:       "old title",
		EnableAuth:  true,
		SaltKey:     "old-salt",
		BaseURL:     "http://old.example.test",
		Initialized: true,
		Addition: models.SettingAddition{
			LocalProxy:                false,
			LocalProxyURL:             "http://old-proxy.example.test",
			MultipleStream:            false,
			MultipleStreamThreadCount: 2,
			MultipleStreamChunkSize:   2 * 1024 * 1024,
			TaskThreadCount:           1,
			WorkerCount:               3,
			EnableStorageAutoRefresh:  false,
			WebDAVUserStrmOnly:        false,
			WebDAVAllowedSuffixes:     []string{".mp4"},
		},
	}
	if err := db.Create(setting).Error; err != nil {
		t.Fatalf("create setting: %v", err)
	}

	return setting
}

func TestUpdateRefreshesSharedSetting(t *testing.T) {
	restoreSharedSetting(t)

	tDB := setupSettingTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())

	createSetting(t, tDB.db)

	addition := models.SettingAddition{
		LocalProxy:                true,
		LocalProxyURL:             "http://proxy.example.test",
		MultipleStream:            true,
		MultipleStreamThreadCount: 8,
		MultipleStreamChunkSize:   8 * 1024 * 1024,
		TaskThreadCount:           2,
		WorkerCount:               6,
		EnableStorageAutoRefresh:  true,
		WebDAVUserStrmOnly:        true,
		WebDAVAllowedSuffixes:     []string{"mkv", ".MP4", "mkv"},
	}
	addition.ApplyDefaultsForWrite()

	err := svc.Update(ctx,
		utils.WithField("enable_auth", false),
		utils.WithField("base_url", "http://new.example.test"),
		utils.WithField("addition", addition),
	)
	if err != nil {
		t.Fatalf("update setting: %v", err)
	}

	if shared.BaseURL != "http://new.example.test" {
		t.Fatalf("expected shared base url updated, got %q", shared.BaseURL)
	}

	if shared.EnableAuth {
		t.Fatal("expected shared enable auth to be false")
	}

	if !shared.SettingAddition.LocalProxy {
		t.Fatal("expected shared local proxy to be true")
	}

	if shared.SettingAddition.LocalProxyURL != "http://proxy.example.test" {
		t.Fatalf("expected shared local proxy url preserved, got %q", shared.SettingAddition.LocalProxyURL)
	}

	expectedSuffixes := []string{".mkv", ".mp4"}
	if len(shared.SettingAddition.WebDAVAllowedSuffixes) != len(expectedSuffixes) {
		t.Fatalf("expected suffixes %v, got %v", expectedSuffixes, shared.SettingAddition.WebDAVAllowedSuffixes)
	}

	for i, expected := range expectedSuffixes {
		if shared.SettingAddition.WebDAVAllowedSuffixes[i] != expected {
			t.Fatalf("expected suffixes %v, got %v", expectedSuffixes, shared.SettingAddition.WebDAVAllowedSuffixes)
		}
	}
}

func TestUpdateRejectsEmptyFieldsWithoutSyncingSharedSetting(t *testing.T) {
	restoreSharedSetting(t)

	tDB := setupSettingTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())

	createSetting(t, tDB.db)

	shared.BaseURL = "http://shared-before.example.test"

	err := svc.Update(ctx)
	if !errors.Is(err, errEmptySettingUpdateFields) {
		t.Fatalf("expected empty setting update fields, got %v", err)
	}

	if shared.BaseURL != "http://shared-before.example.test" {
		t.Fatalf("expected shared base url unchanged, got %q", shared.BaseURL)
	}
}

func TestUpdateReturnsNotFoundWithoutSyncingSharedSetting(t *testing.T) {
	restoreSharedSetting(t)

	tDB := setupSettingTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())

	shared.BaseURL = "http://shared-before.example.test"

	err := svc.Update(ctx, utils.WithField("base_url", "http://new.example.test"))
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("expected record not found, got %v", err)
	}

	if shared.BaseURL != "http://shared-before.example.test" {
		t.Fatalf("expected shared base url unchanged, got %q", shared.BaseURL)
	}
}

func TestUpdateNormalizesBaseURL(t *testing.T) {
	restoreSharedSetting(t)

	tDB := setupSettingTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())

	createSetting(t, tDB.db)

	err := svc.Update(ctx, utils.WithField("base_url", " https://new.example.test/app "))
	if err != nil {
		t.Fatalf("update setting: %v", err)
	}

	if shared.BaseURL != "https://new.example.test/app" {
		t.Fatalf("expected shared base url normalized, got %q", shared.BaseURL)
	}

	var setting models.Setting
	if err := tDB.db.First(&setting).Error; err != nil {
		t.Fatalf("query setting: %v", err)
	}

	if setting.BaseURL != "https://new.example.test/app" {
		t.Fatalf("expected stored base url normalized, got %q", setting.BaseURL)
	}
}

func TestUpdateNormalizesAdditionLocalProxyURL(t *testing.T) {
	restoreSharedSetting(t)

	tDB := setupSettingTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())

	createSetting(t, tDB.db)

	addition := models.SettingAddition{
		LocalProxy:    true,
		LocalProxyURL: " https://proxy-user:proxy-pass@proxy.example.test:8443 ",
		WorkerCount:   5,
	}

	err := svc.Update(ctx, utils.WithField("addition", addition))
	if err != nil {
		t.Fatalf("update setting addition: %v", err)
	}

	if shared.SettingAddition.LocalProxyURL != "https://proxy-user:proxy-pass@proxy.example.test:8443" {
		t.Fatalf("expected shared local proxy URL normalized, got %q", shared.SettingAddition.LocalProxyURL)
	}

	var setting models.Setting
	if err := tDB.db.First(&setting).Error; err != nil {
		t.Fatalf("query setting: %v", err)
	}

	if setting.Addition.LocalProxyURL != "https://proxy-user:proxy-pass@proxy.example.test:8443" {
		t.Fatalf("expected stored local proxy URL normalized, got %q", setting.Addition.LocalProxyURL)
	}
}

func TestUpdateRejectsInvalidBaseURLWithoutSyncingSharedSetting(t *testing.T) {
	restoreSharedSetting(t)

	tests := []struct {
		name  string
		value any
	}{
		{name: "blank", value: "   "},
		{name: "relative", value: "/cloudpan"},
		{name: "unsupported scheme", value: "ftp://example.test"},
		{name: "missing host", value: "https:///cloudpan"},
		{name: "non string", value: 123},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tDB := setupSettingTestDB(t)
			svc := NewService(tDB)
			ctx := context.NewContext(stdctx.Background())

			createSetting(t, tDB.db)

			shared.BaseURL = "http://shared-before.example.test"

			err := svc.Update(ctx, utils.WithField("base_url", tt.value))
			if !errors.Is(err, errInvalidSettingBaseURL) {
				t.Fatalf("expected invalid setting base URL, got %v", err)
			}

			if shared.BaseURL != "http://shared-before.example.test" {
				t.Fatalf("expected shared base url unchanged, got %q", shared.BaseURL)
			}

			var setting models.Setting
			if err := tDB.db.First(&setting).Error; err != nil {
				t.Fatalf("query setting: %v", err)
			}

			if setting.BaseURL != "http://old.example.test" {
				t.Fatalf("expected stored base url unchanged, got %q", setting.BaseURL)
			}
		})
	}
}

func TestUpdateRejectsInvalidAdditionWithoutSyncingSharedSetting(t *testing.T) {
	restoreSharedSetting(t)

	tests := []struct {
		name    string
		value   any
		wantErr error
	}{
		{
			name: "unsupported proxy scheme",
			value: models.SettingAddition{
				LocalProxyURL: "socks5://127.0.0.1:1080",
			},
			wantErr: errInvalidSettingLocalProxyURL,
		},
		{
			name: "invalid proxy escape",
			value: models.SettingAddition{
				LocalProxyURL: "http://proxy-user:proxy-pass@%zz?token=secret-token",
			},
			wantErr: errInvalidSettingLocalProxyURL,
		},
		{
			name: "multiple stream chunk size too large",
			value: models.SettingAddition{
				MultipleStreamChunkSize: 64*1024*1024 + 1,
			},
			wantErr: errInvalidSettingAddition,
		},
		{name: "non addition", value: map[string]interface{}{}, wantErr: errInvalidSettingAddition},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tDB := setupSettingTestDB(t)
			svc := NewService(tDB)
			ctx := context.NewContext(stdctx.Background())

			createSetting(t, tDB.db)

			shared.SettingAddition.LocalProxyURL = "http://shared-before.example.test"

			err := svc.Update(ctx, utils.WithField("addition", tt.value))
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("expected %v, got %v", tt.wantErr, err)
			}

			if shared.SettingAddition.LocalProxyURL != "http://shared-before.example.test" {
				t.Fatalf("expected shared local proxy URL unchanged, got %q", shared.SettingAddition.LocalProxyURL)
			}

			var setting models.Setting
			if err := tDB.db.First(&setting).Error; err != nil {
				t.Fatalf("query setting: %v", err)
			}

			if setting.Addition.LocalProxyURL != "http://old-proxy.example.test" {
				t.Fatalf("expected stored local proxy URL unchanged, got %q", setting.Addition.LocalProxyURL)
			}
		})
	}
}

func TestCheckSettingUpdateResultAllowsExistingNoop(t *testing.T) {
	tDB := setupSettingTestDB(t)
	ctx := context.NewContext(stdctx.Background())

	setting := createSetting(t, tDB.db)

	if err := checkSettingUpdateResult(ctx, tDB.db, &gorm.DB{RowsAffected: 0}, setting.ID); err != nil {
		t.Fatalf("expected existing no-op update to succeed, got %v", err)
	}
}

func TestCheckSettingUpdateResultReturnsNotFoundWhenMissing(t *testing.T) {
	tDB := setupSettingTestDB(t)
	ctx := context.NewContext(stdctx.Background())

	err := checkSettingUpdateResult(ctx, tDB.db, &gorm.DB{RowsAffected: 0}, 99999)
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("expected record not found, got %v", err)
	}
}

func TestInitSystemRejectsInvalidRequestWithoutSyncingSharedSetting(t *testing.T) {
	restoreSharedSetting(t)

	tDB := setupSettingTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())

	shared.BaseURL = "http://shared-before.example.test"

	tests := []struct {
		name    string
		req     *InitSystemRequest
		wantErr error
	}{
		{name: "nil request", req: nil, wantErr: errInvalidInitSystemRequest},
		{name: "empty title", req: &InitSystemRequest{Title: "", BaseURL: "http://init.example.test"}, wantErr: errInvalidInitSystemRequest},
		{name: "blank title", req: &InitSystemRequest{Title: "   ", BaseURL: "http://init.example.test"}, wantErr: errInvalidInitSystemRequest},
		{name: "empty base url", req: &InitSystemRequest{Title: "initialized", BaseURL: ""}, wantErr: errInvalidSettingBaseURL},
		{name: "blank base url", req: &InitSystemRequest{Title: "initialized", BaseURL: "   "}, wantErr: errInvalidSettingBaseURL},
		{name: "relative base url", req: &InitSystemRequest{Title: "initialized", BaseURL: "/cloudpan"}, wantErr: errInvalidSettingBaseURL},
		{name: "unsupported scheme", req: &InitSystemRequest{Title: "initialized", BaseURL: "ftp://init.example.test"}, wantErr: errInvalidSettingBaseURL},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := svc.InitSystem(ctx, tt.req)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("expected %v, got %v", tt.wantErr, err)
			}
		})
	}

	if shared.BaseURL != "http://shared-before.example.test" {
		t.Fatalf("expected shared base url unchanged, got %q", shared.BaseURL)
	}
}

func TestInitSystemRefreshesSharedSetting(t *testing.T) {
	restoreSharedSetting(t)

	tDB := setupSettingTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())

	if err := tDB.db.Create(&models.Setting{
		ID:         1,
		Title:      "not initialized",
		EnableAuth: true,
		SaltKey:    "init-salt",
		BaseURL:    "http://old.example.test",
		Addition: models.SettingAddition{
			WorkerCount: 5,
		},
	}).Error; err != nil {
		t.Fatalf("create setting: %v", err)
	}

	err := svc.InitSystem(ctx, &InitSystemRequest{
		Title:      "initialized",
		EnableAuth: false,
		BaseURL:    " http://init.example.test ",
	})
	if err != nil {
		t.Fatalf("init system: %v", err)
	}

	if shared.BaseURL != "http://init.example.test" {
		t.Fatalf("expected shared base url initialized, got %q", shared.BaseURL)
	}

	if shared.EnableAuth {
		t.Fatal("expected shared enable auth initialized to false")
	}

	if shared.SaltKey != "init-salt" {
		t.Fatalf("expected shared salt key preserved, got %q", shared.SaltKey)
	}
}
