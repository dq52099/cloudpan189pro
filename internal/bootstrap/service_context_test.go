package bootstrap

import (
	"errors"
	"strings"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/xxcheng123/cloudpan189-share/internal/configs"
	"github.com/xxcheng123/cloudpan189-share/internal/pkgs/taskengine"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

func TestServiceContextCloseStopsTaskEngineAndClosesDB(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get sql db: %v", err)
	}

	engine := taskengine.NewTaskEngine(taskengine.WithLogger(zap.NewNop()))
	if err := engine.Start(); err != nil {
		t.Fatalf("start task engine: %v", err)
	}

	ctx := &serviceContext{
		db:         db,
		logger:     zap.NewNop(),
		taskEngine: engine,
	}

	ctx.Close()

	if engine.IsRunning() {
		t.Fatal("expected task engine to be stopped")
	}

	if err := sqlDB.Ping(); err == nil {
		t.Fatal("expected database connection to be closed")
	}
}

func TestServiceContextCloseAllowsNilResources(t *testing.T) {
	ctx := &serviceContext{}

	ctx.Close()
}

func TestMigrateSQLiteDataIfNeededSkipsWhenDisabled(t *testing.T) {
	called := false

	err := migrateSQLiteDataIfNeeded(
		new(configs.Config),
		zap.NewNop(),
		func(*configs.Config) bool { return false },
		func(*configs.Config) error {
			called = true

			return nil
		},
	)
	if err != nil {
		t.Fatalf("expected no error when migration is disabled, got %v", err)
	}

	if called {
		t.Fatal("expected migration function not to be called")
	}
}

func TestMigrateSQLiteDataIfNeededReturnsMigrationError(t *testing.T) {
	migrateErr := errors.New("copy failed")

	err := migrateSQLiteDataIfNeeded(
		new(configs.Config),
		zap.NewNop(),
		func(*configs.Config) bool { return true },
		func(*configs.Config) error { return migrateErr },
	)
	if !errors.Is(err, migrateErr) {
		t.Fatalf("expected wrapped migration error, got %v", err)
	}

	if !strings.Contains(err.Error(), "数据迁移失败") {
		t.Fatalf("expected migration failure context, got %v", err)
	}
}

func TestMigrateSQLiteDataIfNeededAllowsSuccessfulMigration(t *testing.T) {
	called := false

	err := migrateSQLiteDataIfNeeded(
		new(configs.Config),
		zap.NewNop(),
		func(*configs.Config) bool { return true },
		func(*configs.Config) error {
			called = true

			return nil
		},
	)
	if err != nil {
		t.Fatalf("expected no error after successful migration, got %v", err)
	}

	if !called {
		t.Fatal("expected migration function to be called")
	}
}
