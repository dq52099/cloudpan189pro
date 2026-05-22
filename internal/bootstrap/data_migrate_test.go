package bootstrap

import (
	"errors"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/xxcheng123/cloudpan189-share/internal/repository/models"
	"gorm.io/gorm"
)

func openMigrationTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}

	t.Cleanup(func() {
		closeGormDB(db)
	})

	return db
}

func TestMigrateUsersPreservesPrimaryKeysAndUpserts(t *testing.T) {
	src := openMigrationTestDB(t)
	dst := openMigrationTestDB(t)

	if err := src.AutoMigrate(&models.User{}); err != nil {
		t.Fatalf("migrate source schema: %v", err)
	}

	if err := dst.AutoMigrate(&models.User{}); err != nil {
		t.Fatalf("migrate destination schema: %v", err)
	}

	if err := dst.Create(&models.User{ID: 42, Username: "old", Password: "old", Status: 2}).Error; err != nil {
		t.Fatalf("seed destination user: %v", err)
	}

	if err := src.Create(&models.User{ID: 42, Username: "alice", Password: "secret", Status: 1, IsAdmin: true}).Error; err != nil {
		t.Fatalf("seed source user: %v", err)
	}

	if err := migrateUsers(src, dst); err != nil {
		t.Fatalf("migrate users: %v", err)
	}

	var got models.User
	if err := dst.First(&got, 42).Error; err != nil {
		t.Fatalf("query migrated user: %v", err)
	}

	if got.ID != 42 || got.Username != "alice" || got.Password != "secret" || got.Status != 1 || !got.IsAdmin {
		t.Fatalf("unexpected migrated user: %+v", got)
	}
}

func TestMigrateSettingsMarksInitializedWhenUsersExist(t *testing.T) {
	src := openMigrationTestDB(t)
	dst := openMigrationTestDB(t)

	if err := src.AutoMigrate(&models.Setting{}); err != nil {
		t.Fatalf("migrate source schema: %v", err)
	}

	if err := dst.AutoMigrate(&models.Setting{}); err != nil {
		t.Fatalf("migrate destination schema: %v", err)
	}

	if err := src.Create(&models.Setting{ID: 7, Title: "old", SaltKey: "salt", Initialized: false}).Error; err != nil {
		t.Fatalf("seed source setting: %v", err)
	}

	if err := migrateSettings(src, dst, 1); err != nil {
		t.Fatalf("migrate settings: %v", err)
	}

	var got models.Setting
	if err := dst.First(&got, 7).Error; err != nil {
		t.Fatalf("query migrated setting: %v", err)
	}

	if !got.Initialized {
		t.Fatalf("expected setting to be initialized after migrating existing users: %+v", got)
	}
}

func TestFindSourceRowsReturnsMissingSourceTableSentinel(t *testing.T) {
	src := openMigrationTestDB(t)

	var users []models.User

	err := findSourceRows(src, &users)
	if !errors.Is(err, errMissingSourceTable) {
		t.Fatalf("expected missing table sentinel, got %v", err)
	}
}

func TestResetPostgresSequenceSkipsNonPostgresDialects(t *testing.T) {
	db := openMigrationTestDB(t)

	if err := resetPostgresSequence(db, new(models.User).TableName()); err != nil {
		t.Fatalf("reset sequence on sqlite should be skipped: %v", err)
	}
}
