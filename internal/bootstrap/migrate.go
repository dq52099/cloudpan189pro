package bootstrap

import (
	"bytes"
	"github.com/xxcheng123/cloudpan189-share/internal/pkgs/utils"
	"github.com/xxcheng123/cloudpan189-share/internal/repository/models"
	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/transform"
	"gorm.io/gorm"
	"io"
)

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
