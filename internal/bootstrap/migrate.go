package bootstrap

import (
	"bytes"
	"database/sql/driver"
	"encoding/json"
	"time"

	"github.com/xxcheng123/cloudpan189-share/internal/pkgs/utils"
	"github.com/xxcheng123/cloudpan189-share/internal/repository/models"
	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/transform"
	"gorm.io/gorm"
	"io"
)

type SystemSetting struct {
	ID        int64     `gorm:"primaryKey" json:"id"`
	Name      string    `gorm:"column:name;type:varchar(255);uniqueIndex" json:"name"`
	Value     SubConfig `gorm:"column:value;type:json" json:"value"`
	CreatedAt time.Time `gorm:"column:created_at;autoCreateTime" json:"createdAt"`
	UpdatedAt time.Time `gorm:"column:updated_at;autoUpdateTime" json:"updatedAt"`
}

func (s SystemSetting) TableName() string {
	return "system_settings"
}

type SubConfig struct {
	Enabled          bool   `json:"enabled"`
	CronExpression   string `json:"cronExpression"`
	PanSearchURL     string `json:"panSearchURL"`
	EnableTMDB       bool   `json:"enableTMDB"`
	EnableDouban     bool   `json:"enableDouban"`
	DefaultMountPath string `json:"defaultMountPath"`
	AutoMount        bool   `json:"autoMount"`
	TMDBAPIKey       string `json:"tmdbAPIKey"`
}

func (sc SubConfig) Value() (driver.Value, error) {
	if sc == (SubConfig{}) {
		return nil, nil
	}
	return json.Marshal(sc)
}

func (sc *SubConfig) Scan(value interface{}) error {
	if value == nil {
		*sc = SubConfig{}
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		return nil
	}
	return json.Unmarshal(bytes, sc)
}

func migrateDB(db *gorm.DB) (err error) {
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

func toUTF8(src string) string {
	reader := transform.NewReader(bytes.NewReader([]byte(src)), simplifiedchinese.GBK.NewDecoder())
	result, _ := io.ReadAll(reader)
	return string(result)
}

var (
	_defaultWebTitle = []byte{0xe5, 0xa4, 0xa9, 0xe7, 0xbf, 0xbc, 0xe8, 0xae, 0xa2, 0xe9, 0x98, 0x85, 0xe5, 0xb0, 0x8f, 0xe7, 0xab, 0x99}
	defaultWebTitle  = string(_defaultWebTitle)
)

func initSetting(db *gorm.DB) error {
	var count int64

	db.Model(new(models.Setting)).Count(&count)

	if count > 0 {
		return nil
	}

	setting := &models.Setting{
		Title:      defaultWebTitle,
		EnableAuth: true,
		SaltKey:    utils.GenerateString(16),
		Addition: models.SettingAddition{
			Keep: utils.GenerateString(16), // 维持结构
		},
	}

	return db.Create(setting).Error
}
