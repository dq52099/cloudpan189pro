package bootstrap

import (
	"errors"
	"strings"
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

type legacyGroup2File struct {
	ID        int64     `gorm:"column:id;primaryKey;autoIncrement"`
	GroupID   int64     `gorm:"column:group_id;type:bigint;not null;index"`
	FileID    int64     `gorm:"column:file_id;type:bigint;not null;index"`
	CreatedAt time.Time `gorm:"column:created_at;autoCreateTime;type:timestamp"`
	UpdatedAt time.Time `gorm:"column:updated_at;autoUpdateTime;type:timestamp"`
}

func (legacyGroup2File) TableName() string {
	return new(models.Group2File).TableName()
}

type legacySingletonSetting struct {
	ID          int64     `gorm:"column:id;primaryKey;autoIncrement"`
	Title       string    `gorm:"column:title;type:varchar(255);not null"`
	EnableAuth  bool      `gorm:"column:enable_auth;type:boolean;default:true"`
	SaltKey     string    `gorm:"column:salt_key;type:varchar(255);not null"`
	BaseURL     string    `gorm:"column:base_url;type:varchar(255);not null;default:''"`
	Initialized bool      `gorm:"column:initialized;type:boolean;default:false"`
	CreatedAt   time.Time `gorm:"column:created_at;autoCreateTime;type:timestamp"`
	UpdatedAt   time.Time `gorm:"column:updated_at;autoUpdateTime;type:timestamp"`
}

func (legacySingletonSetting) TableName() string {
	return new(models.Setting).TableName()
}

type legacySingletonMediaConfig struct {
	ID                 int64     `gorm:"column:id;primaryKey;autoIncrement"`
	Enable             bool      `gorm:"column:enable;type:boolean;not null;default:false"`
	StoragePath        string    `gorm:"column:storage_path;type:varchar(255);not null"`
	AutoClean          bool      `gorm:"column:auto_clean;type:boolean;not null;default:false"`
	ConflictPolicy     string    `gorm:"column:conflict_policy;type:varchar(20);not null;default:'skip'"`
	IncludedSuffixes   string    `gorm:"column:included_suffixes;type:json;not null"`
	BaseURL            string    `gorm:"column:base_url;type:varchar(255);not null"`
	AutoRebuildEnable  bool      `gorm:"column:auto_rebuild_enable;type:boolean;not null;default:false"`
	AutoRebuildCron    string    `gorm:"column:auto_rebuild_cron;type:varchar(50);not null;default:'0 2 * * *'"`
	AutoRebuildInteral int       `gorm:"column:auto_rebuild_interval;type:int;not null;default:24"`
	CreatedAt          time.Time `gorm:"column:created_at;autoCreateTime;type:timestamp"`
	UpdatedAt          time.Time `gorm:"column:updated_at;autoUpdateTime;type:timestamp"`
}

func (legacySingletonMediaConfig) TableName() string {
	return new(models.MediaConfig).TableName()
}

type legacySingletonTelegramSetting struct {
	ID                int64     `gorm:"column:id;primaryKey;autoIncrement"`
	BotTokenEncrypted string    `gorm:"column:bot_token_encrypted;type:varchar(512);not null"`
	ProxyURL          string    `gorm:"column:proxy_url;type:varchar(255);default:''"`
	ProxyType         string    `gorm:"column:proxy_type;type:varchar(20);default:''"`
	APIURL            string    `gorm:"column:api_url;type:varchar(255);default:'https://api.telegram.org'"`
	ChatID            string    `gorm:"column:chat_id;type:varchar(64);default:''"`
	DefaultMountPath  string    `gorm:"column:default_mount_path;type:varchar(255);default:'/转存'"`
	EnableNotify      bool      `gorm:"column:enable_notify;type:boolean;default:true"`
	Enable            bool      `gorm:"column:enable;type:boolean;default:false"`
	CreatedAt         time.Time `gorm:"column:created_at;autoCreateTime;type:timestamp"`
	UpdatedAt         time.Time `gorm:"column:updated_at;autoUpdateTime;type:timestamp"`
}

func (legacySingletonTelegramSetting) TableName() string {
	return new(models.TelegramSetting).TableName()
}

type legacyUserGroupNaturalKey struct {
	ID        int64     `gorm:"column:id;primaryKey;autoIncrement"`
	Name      string    `gorm:"column:name;type:varchar(255);not null"`
	CreatedAt time.Time `gorm:"column:created_at;autoCreateTime;type:timestamp"`
	UpdatedAt time.Time `gorm:"column:updated_at;autoUpdateTime;type:timestamp"`
}

func (legacyUserGroupNaturalKey) TableName() string {
	return new(models.UserGroup).TableName()
}

type legacyVirtualFileNaturalKey struct {
	ID        int64     `gorm:"column:id;primaryKey;autoIncrement"`
	ParentID  int64     `gorm:"column:parent_id;type:bigint;not null;default:0"`
	Name      string    `gorm:"column:name;type:varchar(1024);not null"`
	CreatedAt time.Time `gorm:"column:created_at;autoCreateTime;type:timestamp"`
	UpdatedAt time.Time `gorm:"column:updated_at;autoUpdateTime;type:timestamp"`
}

func (legacyVirtualFileNaturalKey) TableName() string {
	return new(models.VirtualFile).TableName()
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

func TestPreflightSQLiteNaturalUniqueKeysSkipsMissingTables(t *testing.T) {
	src := openMigrationTestDB(t)

	if err := preflightSQLiteNaturalUniqueKeys(src); err != nil {
		t.Fatalf("expected missing source tables to be skipped, got %v", err)
	}
}

func TestPreflightSQLiteNaturalUniqueKeysRejectsDuplicateUserGroupNames(t *testing.T) {
	src := openMigrationTestDB(t)

	if err := src.AutoMigrate(&legacyUserGroupNaturalKey{}); err != nil {
		t.Fatalf("migrate legacy user group schema: %v", err)
	}

	rows := []legacyUserGroupNaturalKey{
		{ID: 7, Name: "admins"},
		{ID: 3, Name: "admins"},
	}
	if err := src.Create(&rows).Error; err != nil {
		t.Fatalf("seed duplicate user groups: %v", err)
	}

	err := preflightSQLiteNaturalUniqueKeys(src)

	var conflictErr *migrationNaturalUniqueConflictError
	if !errors.As(err, &conflictErr) {
		t.Fatalf("expected natural unique conflict error, got %v", err)
	}

	if len(conflictErr.conflicts) != 1 {
		t.Fatalf("expected one conflict, got %#v", conflictErr.conflicts)
	}

	conflict := conflictErr.conflicts[0]
	if conflict.table != new(models.UserGroup).TableName() ||
		conflict.key != `name="admins"` ||
		len(conflict.ids) != 2 ||
		conflict.ids[0] != 3 ||
		conflict.ids[1] != 7 {
		t.Fatalf("unexpected conflict: %#v", conflict)
	}

	if !strings.Contains(err.Error(), "user_groups") || !strings.Contains(err.Error(), "ids=[3 7]") {
		t.Fatalf("expected conflict details in error message, got %v", err)
	}
}

func TestPreflightSQLiteNaturalUniqueKeysRejectsSanitizedVirtualFileNameCollisions(t *testing.T) {
	src := openMigrationTestDB(t)

	if err := src.AutoMigrate(&legacyVirtualFileNaturalKey{}); err != nil {
		t.Fatalf("migrate legacy virtual file schema: %v", err)
	}

	rows := []legacyVirtualFileNaturalKey{
		{ID: 11, ParentID: 5, Name: "movie:a"},
		{ID: 12, ParentID: 5, Name: "movie?a"},
		{ID: 13, ParentID: 6, Name: "movie:a"},
	}
	if err := src.Create(&rows).Error; err != nil {
		t.Fatalf("seed virtual files: %v", err)
	}

	err := preflightSQLiteNaturalUniqueKeys(src)

	var conflictErr *migrationNaturalUniqueConflictError
	if !errors.As(err, &conflictErr) {
		t.Fatalf("expected natural unique conflict error, got %v", err)
	}

	if len(conflictErr.conflicts) != 1 {
		t.Fatalf("expected one conflict, got %#v", conflictErr.conflicts)
	}

	conflict := conflictErr.conflicts[0]
	if conflict.table != new(models.VirtualFile).TableName() ||
		conflict.key != `parent_id=5,name="movie_a"` ||
		len(conflict.ids) != 2 ||
		conflict.ids[0] != 11 ||
		conflict.ids[1] != 12 {
		t.Fatalf("unexpected conflict: %#v", conflict)
	}
}

func TestMigrationTargetTablesEmptySkipsMissingTables(t *testing.T) {
	db := openMigrationTestDB(t)

	empty, err := migrationTargetTablesEmpty(db)
	if err != nil {
		t.Fatalf("check migration target tables: %v", err)
	}

	if !empty {
		t.Fatal("expected missing target tables to be treated as empty")
	}
}

func TestDataMigrationTableNamesAreDerivedFromSteps(t *testing.T) {
	steps := dataMigrationSteps(nil, 0)
	tableNames := dataMigrationTableNames()

	if len(tableNames) != len(steps) {
		t.Fatalf("expected %d table names, got %d", len(steps), len(tableNames))
	}

	seen := make(map[string]struct{}, len(tableNames))
	for i, tableName := range tableNames {
		if tableName == "" {
			t.Fatalf("step %d has empty table name", i)
		}

		if _, ok := seen[tableName]; ok {
			t.Fatalf("duplicate migration table name: %s", tableName)
		}

		seen[tableName] = struct{}{}
	}
}

func TestMigrationTargetTablesEmptyReturnsTrueWhenKnownTablesAreEmpty(t *testing.T) {
	db := openMigrationTestDB(t)

	if err := db.AutoMigrate(&models.User{}, &models.Setting{}, &SystemSetting{}); err != nil {
		t.Fatalf("migrate target schemas: %v", err)
	}

	empty, err := migrationTargetTablesEmpty(db)
	if err != nil {
		t.Fatalf("check migration target tables: %v", err)
	}

	if !empty {
		t.Fatal("expected empty target tables to allow migration")
	}
}

func TestMigrationTargetTablesEmptyReturnsFalseWhenSettingExists(t *testing.T) {
	db := openMigrationTestDB(t)

	if err := db.AutoMigrate(&models.Setting{}); err != nil {
		t.Fatalf("migrate setting schema: %v", err)
	}

	if err := db.Create(&models.Setting{Title: "existing", SaltKey: "salt"}).Error; err != nil {
		t.Fatalf("seed setting: %v", err)
	}

	empty, err := migrationTargetTablesEmpty(db)
	if err != nil {
		t.Fatalf("check migration target tables: %v", err)
	}

	if empty {
		t.Fatal("expected existing non-user target data to block migration")
	}
}

func TestMigrationTargetTablesEmptyReturnsFalseWhenSystemSettingExists(t *testing.T) {
	db := openMigrationTestDB(t)

	if err := db.AutoMigrate(&SystemSetting{}); err != nil {
		t.Fatalf("migrate system setting schema: %v", err)
	}

	if err := db.Create(&SystemSetting{Name: "subscription_config"}).Error; err != nil {
		t.Fatalf("seed system setting: %v", err)
	}

	empty, err := migrationTargetTablesEmpty(db)
	if err != nil {
		t.Fatalf("check migration target tables: %v", err)
	}

	if empty {
		t.Fatal("expected existing system setting data to block migration")
	}
}

func TestMigrationSourceTablesHaveDataReturnsFalseWhenMissingOrEmpty(t *testing.T) {
	db := openMigrationTestDB(t)

	hasData, err := migrationSourceTablesHaveData(db)
	if err != nil {
		t.Fatalf("check missing source tables: %v", err)
	}

	if hasData {
		t.Fatal("expected missing source tables to have no migration data")
	}

	if err := db.AutoMigrate(&models.User{}, &models.Setting{}); err != nil {
		t.Fatalf("migrate source schemas: %v", err)
	}

	hasData, err = migrationSourceTablesHaveData(db)
	if err != nil {
		t.Fatalf("check empty source tables: %v", err)
	}

	if hasData {
		t.Fatal("expected empty source tables to have no migration data")
	}
}

func TestMigrationSourceTablesHaveDataReturnsTrueWhenUserExistsWithoutSetting(t *testing.T) {
	db := openMigrationTestDB(t)

	if err := db.AutoMigrate(&models.User{}); err != nil {
		t.Fatalf("migrate user schema: %v", err)
	}

	if err := db.Create(&models.User{Username: "alice", Password: "secret"}).Error; err != nil {
		t.Fatalf("seed user: %v", err)
	}

	hasData, err := migrationSourceTablesHaveData(db)
	if err != nil {
		t.Fatalf("check source tables: %v", err)
	}

	if !hasData {
		t.Fatal("expected user-only source data to trigger migration")
	}
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
	if err := dst.First(&got, 1).Error; err != nil {
		t.Fatalf("query migrated setting: %v", err)
	}

	if got.ID != 1 || !got.Initialized {
		t.Fatalf("expected setting to be initialized after migrating existing users: %+v", got)
	}
}

func TestMigrateSettingsCreatesInitializedSettingWhenSourceTableMissingAndUsersExist(t *testing.T) {
	src := openMigrationTestDB(t)
	dst := openMigrationTestDB(t)

	if err := src.AutoMigrate(&models.User{}); err != nil {
		t.Fatalf("migrate source user schema: %v", err)
	}

	if err := dst.AutoMigrate(&models.Setting{}); err != nil {
		t.Fatalf("migrate destination setting schema: %v", err)
	}

	if err := src.Create(&models.User{Username: "alice", Password: "secret"}).Error; err != nil {
		t.Fatalf("seed source user: %v", err)
	}

	userCount := countUsers(t, src)
	if err := migrateSettings(src, dst, userCount); err != nil {
		t.Fatalf("migrate settings: %v", err)
	}

	assertInitializedDefaultSetting(t, dst)
}

func TestMigrateSettingsCreatesInitializedSettingWhenSourceTableEmptyAndUsersExist(t *testing.T) {
	src := openMigrationTestDB(t)
	dst := openMigrationTestDB(t)

	if err := src.AutoMigrate(&models.User{}, &models.Setting{}); err != nil {
		t.Fatalf("migrate source schemas: %v", err)
	}

	if err := dst.AutoMigrate(&models.Setting{}); err != nil {
		t.Fatalf("migrate destination setting schema: %v", err)
	}

	if err := src.Create(&models.User{Username: "alice", Password: "secret"}).Error; err != nil {
		t.Fatalf("seed source user: %v", err)
	}

	userCount := countUsers(t, src)
	if err := migrateSettings(src, dst, userCount); err != nil {
		t.Fatalf("migrate settings: %v", err)
	}

	assertInitializedDefaultSetting(t, dst)
}

func TestMigrateSingletonTablesKeepsFirstSourceRowAsIDOne(t *testing.T) {
	src := openMigrationTestDB(t)
	dst := openMigrationTestDB(t)

	if err := src.AutoMigrate(&legacySingletonSetting{}, &legacySingletonMediaConfig{}, &legacySingletonTelegramSetting{}); err != nil {
		t.Fatalf("migrate source singleton schemas: %v", err)
	}

	if err := dst.AutoMigrate(&models.Setting{}, &models.MediaConfig{}, &models.TelegramSetting{}); err != nil {
		t.Fatalf("migrate destination singleton schemas: %v", err)
	}

	settings := []legacySingletonSetting{
		{ID: 2, Title: "second", SaltKey: "salt-2", BaseURL: "http://second.example.test"},
		{ID: 7, Title: "seventh", SaltKey: "salt-7", BaseURL: "http://seventh.example.test"},
	}
	if err := src.Create(&settings).Error; err != nil {
		t.Fatalf("seed legacy settings: %v", err)
	}

	mediaConfigs := []legacySingletonMediaConfig{
		{ID: 3, StoragePath: "/tmp/media-first", ConflictPolicy: "skip", IncludedSuffixes: "[]", BaseURL: "http://media-first.example.test", AutoRebuildCron: "0 2 * * *"},
		{ID: 4, StoragePath: "/tmp/media-second", ConflictPolicy: "replace", IncludedSuffixes: "[]", BaseURL: "http://media-second.example.test", AutoRebuildCron: "0 4 * * *"},
	}
	if err := src.Create(&mediaConfigs).Error; err != nil {
		t.Fatalf("seed legacy media configs: %v", err)
	}

	telegramSettings := []legacySingletonTelegramSetting{
		{ID: 5, BotTokenEncrypted: "token-first", APIURL: "https://api.telegram.org", DefaultMountPath: "/first"},
		{ID: 6, BotTokenEncrypted: "token-second", APIURL: "https://api.example.test", DefaultMountPath: "/second"},
	}
	if err := src.Create(&telegramSettings).Error; err != nil {
		t.Fatalf("seed legacy telegram settings: %v", err)
	}

	if err := migrateSettings(src, dst, 0); err != nil {
		t.Fatalf("migrate settings: %v", err)
	}

	if err := migrateMediaConfig(src, dst); err != nil {
		t.Fatalf("migrate media config: %v", err)
	}

	if err := migrateTelegramSettings(src, dst); err != nil {
		t.Fatalf("migrate telegram settings: %v", err)
	}

	var setting models.Setting
	if err := dst.First(&setting, 1).Error; err != nil {
		t.Fatalf("query migrated setting: %v", err)
	}

	if setting.Title != "second" || setting.SaltKey != "salt-2" {
		t.Fatalf("expected first setting row normalized to id 1, got %+v", setting)
	}

	var settingCount int64
	if err := dst.Model(&models.Setting{}).Count(&settingCount).Error; err != nil {
		t.Fatalf("count settings: %v", err)
	}

	if settingCount != 1 {
		t.Fatalf("expected one migrated setting, got %d", settingCount)
	}

	var mediaConfig models.MediaConfig
	if err := dst.First(&mediaConfig, 1).Error; err != nil {
		t.Fatalf("query migrated media config: %v", err)
	}

	if mediaConfig.StoragePath != "/tmp/media-first" || mediaConfig.BaseURL != "http://media-first.example.test" {
		t.Fatalf("expected first media config normalized to id 1, got %+v", mediaConfig)
	}

	var telegramSetting models.TelegramSetting
	if err := dst.First(&telegramSetting, 1).Error; err != nil {
		t.Fatalf("query migrated telegram setting: %v", err)
	}

	if telegramSetting.BotTokenEncrypted != "token-first" || telegramSetting.DefaultMountPath != "/first" {
		t.Fatalf("expected first telegram setting normalized to id 1, got %+v", telegramSetting)
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

func TestDedupeGroup2FileRowsKeepsLatestID(t *testing.T) {
	rows := []models.Group2File{
		{ID: 1, GroupId: 10, FileId: 1001},
		{ID: 3, GroupId: 10, FileId: 1001},
		{ID: 2, GroupId: 20, FileId: 1001},
	}

	got := dedupeGroup2FileRows(rows)
	if len(got) != 2 {
		t.Fatalf("expected 2 rows, got %d: %#v", len(got), got)
	}

	if got[0].ID != 3 {
		t.Fatalf("expected first key to keep latest row, got %#v", got[0])
	}

	if got[1].ID != 2 {
		t.Fatalf("expected unrelated binding preserved, got %#v", got[1])
	}
}

func TestMigrateGroup2FilesDedupesLegacyRows(t *testing.T) {
	src := openMigrationTestDB(t)
	dst := openMigrationTestDB(t)

	if err := src.AutoMigrate(&legacyGroup2File{}); err != nil {
		t.Fatalf("migrate source schema: %v", err)
	}

	if err := dst.AutoMigrate(&models.Group2File{}); err != nil {
		t.Fatalf("migrate destination schema: %v", err)
	}

	rows := []legacyGroup2File{
		{ID: 1, GroupID: 10, FileID: 1001},
		{ID: 3, GroupID: 10, FileID: 1001},
		{ID: 2, GroupID: 20, FileID: 1001},
	}
	if err := src.Create(&rows).Error; err != nil {
		t.Fatalf("seed legacy group file bindings: %v", err)
	}

	if err := migrateGroup2Files(src, dst); err != nil {
		t.Fatalf("migrate group file bindings: %v", err)
	}

	var got []models.Group2File
	if err := dst.Order("id").Find(&got).Error; err != nil {
		t.Fatalf("query migrated group file bindings: %v", err)
	}

	if len(got) != 2 {
		t.Fatalf("expected 2 migrated rows, got %d: %#v", len(got), got)
	}

	if got[0].ID != 2 || got[0].GroupId != 20 {
		t.Fatalf("expected first migrated row to be ID 2, got %#v", got[0])
	}

	if got[1].ID != 3 || got[1].GroupId != 10 {
		t.Fatalf("expected duplicate key to keep ID 3, got %#v", got[1])
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

func TestUpsertGroup2FileRowsReturnsErrorWhenMatchedRowDisappears(t *testing.T) {
	dst := openMigrationTestDB(t)

	if err := dst.AutoMigrate(&models.Group2File{}); err != nil {
		t.Fatalf("migrate destination schema: %v", err)
	}

	if err := dst.Create(&models.Group2File{GroupId: 10, FileId: 1001}).Error; err != nil {
		t.Fatalf("seed existing group file binding: %v", err)
	}

	const callbackName = "data_migrate_test_delete_group2file_before_update"

	deleted := false

	if err := dst.Callback().Update().Before("gorm:update").Register(callbackName, func(tx *gorm.DB) {
		if deleted || tx.Statement.Table != new(models.Group2File).TableName() {
			return
		}

		deleted = true

		if err := tx.Session(&gorm.Session{NewDB: true}).
			Exec("DELETE FROM group2files WHERE group_id = ? AND file_id = ?", 10, 1001).Error; err != nil {
			_ = tx.AddError(err)
		}
	}); err != nil {
		t.Fatalf("register update callback: %v", err)
	}

	t.Cleanup(func() {
		_ = dst.Callback().Update().Remove(callbackName)
	})

	err := upsertGroup2FileRows(dst, []models.Group2File{{GroupId: 10, FileId: 1001, UpdatedAt: time.Now()}})
	if err == nil {
		t.Fatal("expected disappearing matched row to fail")
	}
}

func TestUpsertUserMountPointTokenRowsReturnsErrorWhenMatchedRowDisappears(t *testing.T) {
	dst := openMigrationTestDB(t)

	if err := dst.AutoMigrate(&models.UserMountPointToken{}); err != nil {
		t.Fatalf("migrate destination schema: %v", err)
	}

	if err := dst.Create(&models.UserMountPointToken{UserID: 1, MountPointID: 10, TokenID: 100}).Error; err != nil {
		t.Fatalf("seed existing user mount point token: %v", err)
	}

	const callbackName = "data_migrate_test_delete_user_mount_point_token_before_update"

	deleted := false

	if err := dst.Callback().Update().Before("gorm:update").Register(callbackName, func(tx *gorm.DB) {
		if deleted || tx.Statement.Table != new(models.UserMountPointToken).TableName() {
			return
		}

		deleted = true

		if err := tx.Session(&gorm.Session{NewDB: true}).
			Exec("DELETE FROM user_mount_point_tokens WHERE user_id = ? AND mount_point_id = ?", 1, 10).Error; err != nil {
			_ = tx.AddError(err)
		}
	}); err != nil {
		t.Fatalf("register update callback: %v", err)
	}

	t.Cleanup(func() {
		_ = dst.Callback().Update().Remove(callbackName)
	})

	err := upsertUserMountPointTokenRows(dst, []models.UserMountPointToken{{UserID: 1, MountPointID: 10, TokenID: 200, UpdatedAt: time.Now()}})
	if err == nil {
		t.Fatal("expected disappearing matched row to fail")
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

func TestMigrateDBDedupesGroup2FilesBeforeUniqueIndex(t *testing.T) {
	db := openMigrationTestDB(t)

	if err := db.AutoMigrate(&legacyGroup2File{}); err != nil {
		t.Fatalf("migrate legacy group file schema: %v", err)
	}

	rows := []legacyGroup2File{
		{ID: 1, GroupID: 10, FileID: 1001},
		{ID: 2, GroupID: 10, FileID: 1001},
	}
	if err := db.Create(&rows).Error; err != nil {
		t.Fatalf("seed duplicate legacy group file bindings: %v", err)
	}

	if err := migrateDB(db); err != nil {
		t.Fatalf("migrate db: %v", err)
	}

	var count int64
	if err := db.Model(&models.Group2File{}).Where("group_id = ? AND file_id = ?", 10, 1001).Count(&count).Error; err != nil {
		t.Fatalf("count migrated group file bindings: %v", err)
	}

	if count != 1 {
		t.Fatalf("expected duplicate group file bindings to be deduped, got %d", count)
	}

	err := db.Create(&models.Group2File{
		GroupId: 10,
		FileId:  1001,
	}).Error
	if err == nil {
		t.Fatal("expected unique index to reject duplicate group file binding after migrate")
	}
}

func TestMigrateDBNormalizesSingletonTablesBeforeAutoMigrate(t *testing.T) {
	db := openMigrationTestDB(t)

	if err := db.AutoMigrate(&legacySingletonSetting{}, &legacySingletonMediaConfig{}, &legacySingletonTelegramSetting{}); err != nil {
		t.Fatalf("migrate legacy singleton schemas: %v", err)
	}

	settings := []legacySingletonSetting{
		{ID: 2, Title: "first", SaltKey: "salt-first", BaseURL: "http://first.example.test"},
		{ID: 9, Title: "second", SaltKey: "salt-second", BaseURL: "http://second.example.test"},
	}
	if err := db.Create(&settings).Error; err != nil {
		t.Fatalf("seed legacy settings: %v", err)
	}

	mediaConfigs := []legacySingletonMediaConfig{
		{ID: 3, StoragePath: "/tmp/media-first", ConflictPolicy: "skip", IncludedSuffixes: "[]", BaseURL: "http://media-first.example.test", AutoRebuildCron: "0 2 * * *"},
		{ID: 7, StoragePath: "/tmp/media-second", ConflictPolicy: "replace", IncludedSuffixes: "[]", BaseURL: "http://media-second.example.test", AutoRebuildCron: "0 4 * * *"},
	}
	if err := db.Create(&mediaConfigs).Error; err != nil {
		t.Fatalf("seed legacy media configs: %v", err)
	}

	telegramSettings := []legacySingletonTelegramSetting{
		{ID: 4, BotTokenEncrypted: "token-first", APIURL: "https://api.telegram.org", DefaultMountPath: "/first"},
		{ID: 8, BotTokenEncrypted: "token-second", APIURL: "https://api.example.test", DefaultMountPath: "/second"},
	}
	if err := db.Create(&telegramSettings).Error; err != nil {
		t.Fatalf("seed legacy telegram settings: %v", err)
	}

	if err := migrateDB(db); err != nil {
		t.Fatalf("migrate db: %v", err)
	}

	var setting models.Setting
	if err := db.First(&setting, 1).Error; err != nil {
		t.Fatalf("query normalized setting: %v", err)
	}

	if setting.Title != "first" || setting.SaltKey != "salt-first" {
		t.Fatalf("expected first setting kept as singleton, got %+v", setting)
	}

	var settingCount int64
	if err := db.Model(&models.Setting{}).Count(&settingCount).Error; err != nil {
		t.Fatalf("count normalized settings: %v", err)
	}

	if settingCount != 1 {
		t.Fatalf("expected one normalized setting, got %d", settingCount)
	}

	var mediaConfig models.MediaConfig
	if err := db.First(&mediaConfig, 1).Error; err != nil {
		t.Fatalf("query normalized media config: %v", err)
	}

	if mediaConfig.StoragePath != "/tmp/media-first" {
		t.Fatalf("expected first media config kept as singleton, got %+v", mediaConfig)
	}

	var telegramSetting models.TelegramSetting
	if err := db.First(&telegramSetting, 1).Error; err != nil {
		t.Fatalf("query normalized telegram setting: %v", err)
	}

	if telegramSetting.BotTokenEncrypted != "token-first" || telegramSetting.DefaultMountPath != "/first" {
		t.Fatalf("expected first telegram setting kept as singleton, got %+v", telegramSetting)
	}
}

func TestNormalizeSingletonTableReturnsErrorWhenKeptRowDisappears(t *testing.T) {
	db := openMigrationTestDB(t)

	if err := db.AutoMigrate(&models.Setting{}); err != nil {
		t.Fatalf("migrate setting schema: %v", err)
	}

	if err := db.Create(&models.Setting{ID: 2, Title: "first", SaltKey: "salt-first"}).Error; err != nil {
		t.Fatalf("seed setting: %v", err)
	}

	const callbackName = "data_migrate_test_delete_singleton_before_update"

	deleted := false
	tableName := new(models.Setting).TableName()

	if err := db.Callback().Update().Before("gorm:update").Register(callbackName, func(tx *gorm.DB) {
		if deleted || tx.Statement.Table != tableName {
			return
		}

		deleted = true

		if err := tx.Session(&gorm.Session{NewDB: true}).
			Exec("DELETE FROM setting WHERE id = ?", 2).Error; err != nil {
			_ = tx.AddError(err)
		}
	}); err != nil {
		t.Fatalf("register update callback: %v", err)
	}

	t.Cleanup(func() {
		_ = db.Callback().Update().Remove(callbackName)
	})

	if err := normalizeSingletonTable(db, tableName); err == nil {
		t.Fatal("expected disappearing singleton row to fail")
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

func TestRunDataMigrationStepsRollsBackDestinationWritesOnError(t *testing.T) {
	dst := openMigrationTestDB(t)

	if err := dst.AutoMigrate(&models.UserGroup{}); err != nil {
		t.Fatalf("migrate destination schema: %v", err)
	}

	migrateErr := errors.New("forced migration failure")
	steps := []dataMigrationStep{
		{
			name: "用户组",
			run: func(tx *gorm.DB) error {
				return tx.Create(&models.UserGroup{ID: 1, Name: "admins"}).Error
			},
		},
		{
			name: "失败步骤",
			run: func(*gorm.DB) error {
				return migrateErr
			},
		},
	}

	err := runDataMigrationSteps(dst, steps)
	if !errors.Is(err, migrateErr) {
		t.Fatalf("expected wrapped migration error, got %v", err)
	}

	var count int64
	if err := dst.Model(&models.UserGroup{}).Count(&count).Error; err != nil {
		t.Fatalf("count user groups after rollback: %v", err)
	}

	if count != 0 {
		t.Fatalf("expected destination writes to be rolled back, got %d rows", count)
	}
}

func countUsers(t *testing.T, db *gorm.DB) int64 {
	t.Helper()

	var count int64
	if err := db.Model(&models.User{}).Count(&count).Error; err != nil {
		t.Fatalf("count users: %v", err)
	}

	return count
}

func assertInitializedDefaultSetting(t *testing.T, db *gorm.DB) {
	t.Helper()

	var got models.Setting
	if err := db.First(&got, 1).Error; err != nil {
		t.Fatalf("query migrated setting: %v", err)
	}

	if got.ID != 1 || !got.Initialized || !got.EnableAuth || got.Title == "" || got.SaltKey == "" {
		t.Fatalf("expected initialized default setting, got %+v", got)
	}
}

func TestResetPostgresSequenceSkipsNonPostgresDialects(t *testing.T) {
	db := openMigrationTestDB(t)

	if err := resetPostgresSequence(db, new(models.User).TableName()); err != nil {
		t.Fatalf("reset sequence on sqlite should be skipped: %v", err)
	}
}
