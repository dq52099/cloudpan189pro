package bootstrap

import (
	"errors"
	"strings"
	"testing"

	"github.com/xxcheng123/cloudpan189-share/internal/configs"
)

func TestMigrateSQLiteDataIfNeededSkipsWhenDisabled(t *testing.T) {
	called := false

	err := migrateSQLiteDataIfNeeded(
		new(configs.Config),
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
