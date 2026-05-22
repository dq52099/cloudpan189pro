package setting

import (
	"github.com/xxcheng123/cloudpan189-share/internal/framework/context"
	"github.com/xxcheng123/cloudpan189-share/internal/pkgs/utils"
	"github.com/xxcheng123/cloudpan189-share/internal/repository/models"
	"github.com/xxcheng123/cloudpan189-share/internal/shared"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

func (s *service) Update(ctx context.Context, fields ...utils.Field) error {
	mp := make(map[string]interface{})

	for _, field := range fields {
		mp[field.Key] = field.Value
	}

	if len(mp) == 0 {
		return errEmptySettingUpdateFields
	}

	updatedSetting := new(models.Setting)

	err := s.getDB(ctx).Transaction(func(tx *gorm.DB) error {
		setting := new(models.Setting)
		if err := tx.First(setting).Error; err != nil {
			ctx.Error("设置查询失败", zap.Error(err))

			return err
		}

		result := tx.Where("id = ?", setting.ID).Updates(mp)
		if result.Error != nil {
			ctx.Error("更新信息失败", zap.Error(result.Error), zap.Int64("id", setting.ID))

			return result.Error
		}

		if err := tx.First(updatedSetting, "id = ?", setting.ID).Error; err != nil {
			ctx.Error("设置回查写入失败", zap.Error(err), zap.Int64("id", setting.ID))

			return err
		}

		return nil
	})
	if err != nil {
		return err
	}

	syncSharedSetting(updatedSetting)

	return nil
}

func syncSharedSetting(setting *models.Setting) {
	shared.SaltKey = setting.SaltKey
	shared.BaseURL = setting.BaseURL
	shared.EnableAuth = setting.EnableAuth

	addition := setting.Addition
	addition.WebDAVAllowedSuffixes = append([]string(nil), addition.WebDAVAllowedSuffixes...)
	shared.SettingAddition = addition
}
