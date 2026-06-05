package bootstrap

import (
	"time"

	"github.com/xxcheng123/cloudpan189-share/internal/pkgs/utils"
	"github.com/xxcheng123/cloudpan189-share/internal/repository/models"
	"gorm.io/gorm"
)

type SystemSetting struct {
	ID        int64                     `gorm:"primaryKey" json:"id"`
	Name      string                    `gorm:"column:name;type:varchar(255);uniqueIndex" json:"name"`
	Value     models.SubscriptionConfig `gorm:"column:value;type:json" json:"value"`
	CreatedAt time.Time                 `gorm:"column:created_at;autoCreateTime" json:"createdAt"`
	UpdatedAt time.Time                 `gorm:"column:updated_at;autoUpdateTime" json:"updatedAt"`
}

func (s SystemSetting) TableName() string {
	return "system_settings"
}

func migrateDB(db *gorm.DB) (err error) {
	if err := normalizeSingletonTables(db); err != nil {
		return err
	}

	if err := dedupeGroup2Files(db); err != nil {
		return err
	}

	if err := dedupeUserMountPointTokens(db); err != nil {
		return err
	}

	return db.AutoMigrate(
		new(models.Setting),
		new(models.User),
		new(models.UserGroup),
		new(models.Group2File),
		new(models.VirtualFile),
		new(models.MediaFile),
		new(models.FileTaskLog),
		new(models.CloudToken),
		new(models.MountPoint),
		new(models.UserMountPointToken),
		new(models.AutoIngestLog),
		new(models.AutoIngestPlan),
		new(models.LoginLog),
		new(models.MediaConfig),
		new(models.TelegramSetting),
		new(models.TelegramUser),
		new(models.Subscription),
		new(models.MatchHistory),
		new(models.DailyHotHistory),
		new(SystemSetting),
	)
}

func normalizeSingletonTables(db *gorm.DB) error {
	normalizers := []func(*gorm.DB) error{
		normalizeSettingSingleton,
		normalizeMediaConfigSingleton,
		normalizeTelegramSettingSingleton,
	}

	for _, normalize := range normalizers {
		if err := normalize(db); err != nil {
			return err
		}
	}

	return nil
}

func normalizeSettingSingleton(db *gorm.DB) error {
	return normalizeSingletonTable(db, new(models.Setting).TableName())
}

func normalizeMediaConfigSingleton(db *gorm.DB) error {
	return normalizeSingletonTable(db, new(models.MediaConfig).TableName())
}

func normalizeTelegramSettingSingleton(db *gorm.DB) error {
	return normalizeSingletonTable(db, new(models.TelegramSetting).TableName())
}

func normalizeSingletonTable(db *gorm.DB, tableName string) error {
	if !db.Migrator().HasTable(tableName) {
		return nil
	}

	var ids []int64
	if err := db.Table(tableName).Order("id ASC").Pluck("id", &ids).Error; err != nil {
		return err
	}

	if len(ids) == 0 {
		return nil
	}

	keepID := ids[0]
	if keepID != 1 {
		result := db.Table(tableName).Where("id = ?", keepID).Update("id", int64(1))
		if err := ensureSingleRowAffected(result, "normalize "+tableName+" singleton id"); err != nil {
			return err
		}
	}

	return db.Exec("DELETE FROM "+tableName+" WHERE id <> ?", int64(1)).Error
}

type duplicatedGroup2FileKey struct {
	GroupID int64
	FileID  int64
	KeepID  int64
	Count   int64
}

func dedupeGroup2Files(db *gorm.DB) error {
	if !db.Migrator().HasTable(new(models.Group2File)) {
		return nil
	}

	var duplicates []duplicatedGroup2FileKey
	if err := db.Model(new(models.Group2File)).
		Select("group_id, file_id, MAX(id) AS keep_id, COUNT(*) AS count").
		Group("group_id, file_id").
		Having("COUNT(*) > 1").
		Scan(&duplicates).Error; err != nil {
		return err
	}

	for _, duplicate := range duplicates {
		if err := db.
			Where("group_id = ? AND file_id = ? AND id <> ?", duplicate.GroupID, duplicate.FileID, duplicate.KeepID).
			Delete(new(models.Group2File)).Error; err != nil {
			return err
		}
	}

	return nil
}

type duplicatedUserMountPointTokenKey struct {
	UserID       int64
	MountPointID int64
	KeepID       int64
	Count        int64
}

func dedupeUserMountPointTokens(db *gorm.DB) error {
	if !db.Migrator().HasTable(new(models.UserMountPointToken)) {
		return nil
	}

	var duplicates []duplicatedUserMountPointTokenKey
	if err := db.Model(new(models.UserMountPointToken)).
		Select("user_id, mount_point_id, MAX(id) AS keep_id, COUNT(*) AS count").
		Group("user_id, mount_point_id").
		Having("COUNT(*) > 1").
		Scan(&duplicates).Error; err != nil {
		return err
	}

	for _, duplicate := range duplicates {
		if err := db.
			Where("user_id = ? AND mount_point_id = ? AND id <> ?", duplicate.UserID, duplicate.MountPointID, duplicate.KeepID).
			Delete(new(models.UserMountPointToken)).Error; err != nil {
			return err
		}
	}

	return nil
}

var (
	_defaultWebTitle = []byte{0xe5, 0xa4, 0xa9, 0xe7, 0xbf, 0xbc, 0xe8, 0xae, 0xa2, 0xe9, 0x98, 0x85, 0xe5, 0xb0, 0x8f, 0xe7, 0xab, 0x99}
	defaultWebTitle  = string(_defaultWebTitle)
)

func initSetting(db *gorm.DB) error {
	setting := &models.Setting{
		ID:         1,
		Title:      defaultWebTitle,
		EnableAuth: true,
		SaltKey:    utils.GenerateString(16),
		Addition: models.SettingAddition{
			Keep: utils.GenerateString(16), // 维持结构
		},
	}

	return db.FirstOrCreate(setting, "id = ?", int64(1)).Error
}
