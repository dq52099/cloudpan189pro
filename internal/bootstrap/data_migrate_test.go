package bootstrap

import (
	"errors"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/xxcheng123/cloudpan189-share/internal/repository/models"
	"gorm.io/gorm"
)

type legacyUserMountPointToken struct {
	ID           int64     `gorm:"primaryKey;autoIncrement"`
	UserID       int64     `gorm:"column:user_id;type:bigint;not null;index"`
	MountPointID int64     `gorm:"column:mount_point_id;type:bigint;not null;index"`
	TokenID      int64     `gorm:"column:token_id;type:bigint;not null"`
	CreatedAt    time.Time `gorm:"column:created_at;autoCreateTime;type:timestamp"`
	UpdatedAt    time.Time `gorm:"column:updated_at;autoUpdateTime;type:timestamp"`
}

func (legacyUserMountPointToken) TableName() string {
	return new(models.UserMountPointToken).TableName()
}

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

func TestMigrateSystemSettingsPreservesSubscriptionConfigFields(t *testing.T) {
	src := openMigrationTestDB(t)
	dst := openMigrationTestDB(t)

	if err := src.AutoMigrate(&SystemSetting{}); err != nil {
		t.Fatalf("migrate source schema: %v", err)
	}

	if err := dst.AutoMigrate(&SystemSetting{}); err != nil {
		t.Fatalf("migrate destination schema: %v", err)
	}

	setting := SystemSetting{
		ID:   9,
		Name: "subscription_config",
		Value: models.SubscriptionConfig{
			Enabled:          true,
			CronExpression:   "0 2 * * *",
			PanSearchURL:     "https://example.test/search",
			EnableTMDB:       true,
			EnableDouban:     true,
			DefaultMountPath: "/subscriptions",
			AutoMount:        true,
			TMDBAPIKey:       "tmdb-key",
			OpenAIAPIKey:     "openai-key",
			OpenAIBaseURL:    "https://api.example.test",
			OpenAIModel:      "gpt-test",
		},
	}
	if err := src.Create(&setting).Error; err != nil {
		t.Fatalf("seed source system setting: %v", err)
	}

	if err := migrateSystemSettings(src, dst); err != nil {
		t.Fatalf("migrate system settings: %v", err)
	}

	var got SystemSetting
	if err := dst.First(&got, 9).Error; err != nil {
		t.Fatalf("query migrated system setting: %v", err)
	}

	if got.Value.OpenAIAPIKey != setting.Value.OpenAIAPIKey ||
		got.Value.OpenAIBaseURL != setting.Value.OpenAIBaseURL ||
		got.Value.OpenAIModel != setting.Value.OpenAIModel {
		t.Fatalf("expected OpenAI config fields to be preserved, got %+v", got.Value)
	}
}

func TestDedupeUserMountPointTokenRowsKeepsLatestID(t *testing.T) {
	rows := []models.UserMountPointToken{
		{ID: 1, UserID: 1, MountPointID: 10, TokenID: 100},
		{ID: 3, UserID: 1, MountPointID: 10, TokenID: 300},
		{ID: 2, UserID: 2, MountPointID: 10, TokenID: 200},
	}

	got := dedupeUserMountPointTokenRows(rows)
	if len(got) != 2 {
		t.Fatalf("expected 2 rows, got %d: %#v", len(got), got)
	}

	if got[0].ID != 3 || got[0].TokenID != 300 {
		t.Fatalf("expected first key to keep latest row, got %#v", got[0])
	}

	if got[1].ID != 2 || got[1].TokenID != 200 {
		t.Fatalf("expected unrelated binding preserved, got %#v", got[1])
	}
}

func TestMigrateUserMountPointTokensDedupesLegacyRows(t *testing.T) {
	src := openMigrationTestDB(t)
	dst := openMigrationTestDB(t)

	if err := src.AutoMigrate(&legacyUserMountPointToken{}); err != nil {
		t.Fatalf("migrate source schema: %v", err)
	}

	if err := dst.AutoMigrate(&models.UserMountPointToken{}); err != nil {
		t.Fatalf("migrate destination schema: %v", err)
	}

	rows := []legacyUserMountPointToken{
		{ID: 1, UserID: 1, MountPointID: 10, TokenID: 100},
		{ID: 3, UserID: 1, MountPointID: 10, TokenID: 300},
		{ID: 2, UserID: 2, MountPointID: 10, TokenID: 200},
	}
	if err := src.Create(&rows).Error; err != nil {
		t.Fatalf("seed legacy user mount point tokens: %v", err)
	}

	if err := migrateUserMountPointTokens(src, dst); err != nil {
		t.Fatalf("migrate user mount point tokens: %v", err)
	}

	var got []models.UserMountPointToken
	if err := dst.Order("id").Find(&got).Error; err != nil {
		t.Fatalf("query migrated user mount point tokens: %v", err)
	}

	if len(got) != 2 {
		t.Fatalf("expected 2 migrated rows, got %d: %#v", len(got), got)
	}

	if got[0].ID != 2 || got[0].TokenID != 200 {
		t.Fatalf("expected first migrated row to be ID 2, got %#v", got[0])
	}

	if got[1].ID != 3 || got[1].TokenID != 300 {
		t.Fatalf("expected duplicate key to keep ID 3, got %#v", got[1])
	}
}

func TestMigrateDBDedupesUserMountPointTokensBeforeUniqueIndex(t *testing.T) {
	db := openMigrationTestDB(t)

	if err := db.AutoMigrate(&legacyUserMountPointToken{}); err != nil {
		t.Fatalf("migrate legacy binding schema: %v", err)
	}

	rows := []legacyUserMountPointToken{
		{ID: 1, UserID: 1, MountPointID: 10, TokenID: 100},
		{ID: 2, UserID: 1, MountPointID: 10, TokenID: 200},
	}
	if err := db.Create(&rows).Error; err != nil {
		t.Fatalf("seed duplicate legacy bindings: %v", err)
	}

	if err := migrateDB(db); err != nil {
		t.Fatalf("migrate db: %v", err)
	}

	var count int64
	if err := db.Model(&models.UserMountPointToken{}).Where("user_id = ? AND mount_point_id = ?", 1, 10).Count(&count).Error; err != nil {
		t.Fatalf("count migrated bindings: %v", err)
	}

	if count != 1 {
		t.Fatalf("expected duplicate bindings to be deduped, got %d", count)
	}

	err := db.Create(&models.UserMountPointToken{
		UserID:       1,
		MountPointID: 10,
		TokenID:      300,
	}).Error
	if err == nil {
		t.Fatal("expected unique index to reject duplicate binding after migrate")
	}
}

func TestFindSourceRowsReturnsMissingSourceTableSentinel(t *testing.T) {
	src := openMigrationTestDB(t)

	var users []models.User

	err := findSourceRows(src, &users, new(models.User).TableName())
	if !errors.Is(err, errMissingSourceTable) {
		t.Fatalf("expected missing table sentinel, got %v", err)
	}
}

func TestFindSourceRowsReturnsExistingTableQueryErrors(t *testing.T) {
	src := openMigrationTestDB(t)

	if err := src.Exec(`
		CREATE TABLE system_settings (
			id INTEGER PRIMARY KEY,
			name TEXT,
			value INTEGER,
			created_at DATETIME,
			updated_at DATETIME
		)
	`).Error; err != nil {
		t.Fatalf("create malformed system settings table: %v", err)
	}

	if err := src.Exec("INSERT INTO system_settings (id, name, value) VALUES (?, ?, ?)", 1, "subscription_config", 123).Error; err != nil {
		t.Fatalf("seed malformed system setting: %v", err)
	}

	var settings []SystemSetting

	err := findSourceRows(src, &settings, new(SystemSetting).TableName())
	if err == nil {
		t.Fatal("expected existing table query/scan error")
	}

	if errors.Is(err, errMissingSourceTable) {
		t.Fatalf("expected real query error, got missing table sentinel: %v", err)
	}
}

func TestResetPostgresSequenceSkipsNonPostgresDialects(t *testing.T) {
	db := openMigrationTestDB(t)

	if err := resetPostgresSequence(db, new(models.User).TableName()); err != nil {
		t.Fatalf("reset sequence on sqlite should be skipped: %v", err)
	}
}
