package mediaconfig

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
	"github.com/xxcheng123/cloudpan189-share/internal/types/media"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type mediaConfigTestDB struct {
	db *gorm.DB
}

func (t *mediaConfigTestDB) GetDB(ctx context.Context) *gorm.DB {
	return t.db.WithContext(ctx)
}

func (t *mediaConfigTestDB) GetDBWithoutContext() *gorm.DB {
	return t.db
}

func (t *mediaConfigTestDB) GetLogger(name string, fields ...zap.Field) *zap.Logger {
	return zap.NewNop()
}

func (t *mediaConfigTestDB) Close() {}

func (t *mediaConfigTestDB) GetPort() int {
	return 9999
}

func (t *mediaConfigTestDB) GetTaskEngine() taskengine.TaskEngine {
	return nil
}

func (t *mediaConfigTestDB) GetHTTPEngine() *gin.Engine {
	return nil
}

var _ bootstrap.ServiceContext = (*mediaConfigTestDB)(nil)

func setupMediaConfigTestDB(t *testing.T) *mediaConfigTestDB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}

	if err := db.AutoMigrate(&models.MediaConfig{}); err != nil {
		t.Fatalf("migrate test db: %v", err)
	}

	return &mediaConfigTestDB{db: db}
}

func restoreSharedMediaConfig(t *testing.T) {
	t.Helper()

	oldConfig := shared.MediaConfig

	t.Cleanup(func() {
		shared.MediaConfig = oldConfig
	})
}

func createMediaConfig(t *testing.T, db *gorm.DB) *models.MediaConfig {
	t.Helper()

	cfg := &models.MediaConfig{
		Enable:              false,
		StoragePath:         "/tmp/media-old",
		AutoClean:           false,
		ConflictPolicy:      media.FileConflictPolicySkip,
		BaseURL:             "http://old.example.test",
		IncludedSuffixes:    []string{".mp4"},
		AutoRebuildEnable:   false,
		AutoRebuildInterval: 24,
	}
	if err := db.Create(cfg).Error; err != nil {
		t.Fatalf("create media config: %v", err)
	}

	return cfg
}

func TestInitRejectsNilRequest(t *testing.T) {
	tDB := setupMediaConfigTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())

	err := svc.Init(ctx, nil)
	if !errors.Is(err, storageDisableAllowedEmpty) {
		t.Fatalf("expected storage path empty error, got %v", err)
	}

	var count int64
	if countErr := tDB.db.Model(&models.MediaConfig{}).Count(&count).Error; countErr != nil {
		t.Fatalf("count media configs: %v", countErr)
	}

	if count != 0 {
		t.Fatalf("expected no media config to be created, got %d", count)
	}
}

func TestInitUpdatesSharedMediaConfig(t *testing.T) {
	restoreSharedMediaConfig(t)

	tDB := setupMediaConfigTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())
	shared.MediaConfig = nil

	err := svc.Init(ctx, &InitRequest{
		Enable:              true,
		StoragePath:         "/tmp/media",
		AutoClean:           true,
		ConflictPolicy:      media.FileConflictPolicyReplace,
		BaseURL:             "http://media.example.test",
		IncludedSuffixes:    []string{".mkv", ".mp4"},
		AutoRebuildEnable:   true,
		AutoRebuildInterval: 12,
	})
	if err != nil {
		t.Fatalf("init media config: %v", err)
	}

	if shared.MediaConfig == nil {
		t.Fatal("expected shared media config to be set")
	}

	if !shared.MediaConfig.Enable || shared.MediaConfig.StoragePath != "/tmp/media" {
		t.Fatalf("unexpected shared media config: %#v", shared.MediaConfig)
	}

	if shared.MediaConfig.ConflictPolicy != media.FileConflictPolicyReplace {
		t.Fatalf("expected replace conflict policy, got %q", shared.MediaConfig.ConflictPolicy)
	}
}

func TestUpdateRefreshesSharedMediaConfig(t *testing.T) {
	restoreSharedMediaConfig(t)

	tDB := setupMediaConfigTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())
	cfg := createMediaConfig(t, tDB.db)
	shared.MediaConfig = cfg

	err := svc.Update(ctx,
		utils.WithField("enable", true),
		utils.WithField("storage_path", "/tmp/media-new"),
		utils.WithField("base_url", "http://new.example.test"),
	)
	if err != nil {
		t.Fatalf("update media config: %v", err)
	}

	if shared.MediaConfig == nil {
		t.Fatal("expected shared media config to be set")
	}

	if !shared.MediaConfig.Enable {
		t.Fatal("expected shared media config enabled")
	}

	if shared.MediaConfig.StoragePath != "/tmp/media-new" {
		t.Fatalf("expected updated storage path, got %q", shared.MediaConfig.StoragePath)
	}

	if shared.MediaConfig.BaseURL != "http://new.example.test" {
		t.Fatalf("expected updated base url, got %q", shared.MediaConfig.BaseURL)
	}
}

func TestUpdateRejectsEmptyFieldsWithoutSyncingSharedMediaConfig(t *testing.T) {
	restoreSharedMediaConfig(t)

	tDB := setupMediaConfigTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())
	cfg := createMediaConfig(t, tDB.db)
	shared.MediaConfig = &models.MediaConfig{BaseURL: "http://shared-before.example.test"}

	err := svc.Update(ctx)
	if !errors.Is(err, errEmptyMediaConfigUpdateFields) {
		t.Fatalf("expected empty media config update fields, got %v", err)
	}

	if shared.MediaConfig == nil {
		t.Fatal("expected shared media config to remain set")
	}

	if shared.MediaConfig.BaseURL != "http://shared-before.example.test" {
		t.Fatalf("expected shared media config unchanged, got %#v", shared.MediaConfig)
	}

	var stored models.MediaConfig
	if err := tDB.db.First(&stored, cfg.ID).Error; err != nil {
		t.Fatalf("query media config: %v", err)
	}

	if stored.BaseURL != "http://old.example.test" {
		t.Fatalf("expected stored media config unchanged, got %q", stored.BaseURL)
	}
}

func TestUpdateReturnsNotFoundWithoutSyncingSharedMediaConfig(t *testing.T) {
	restoreSharedMediaConfig(t)

	tDB := setupMediaConfigTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())

	shared.MediaConfig = &models.MediaConfig{BaseURL: "http://shared-before.example.test"}

	err := svc.Update(ctx, utils.WithField("base_url", "http://new.example.test"))
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("expected record not found, got %v", err)
	}

	if shared.MediaConfig == nil {
		t.Fatal("expected shared media config to remain set")
	}

	if shared.MediaConfig.BaseURL != "http://shared-before.example.test" {
		t.Fatalf("expected shared media config unchanged, got %#v", shared.MediaConfig)
	}
}
