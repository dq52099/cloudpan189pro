package setting

import (
	"github.com/xxcheng123/cloudpan189-share/internal/bootstrap"
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
		if field.Key == "base_url" {
			value, ok := field.Value.(string)
			if !ok {
				return errInvalidSettingBaseURL
			}

			baseURL, err := normalizeSettingBaseURL(value)
			if err != nil {
				return err
			}

			field.Value = baseURL
		}

		if field.Key == "addition" {
			addition, err := normalizeSettingAddition(field.Value)
			if err != nil {
				return err
			}

			field.Value = addition
		}

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

		if err := checkSettingUpdateResult(ctx, tx, result, setting.ID); err != nil {
			return err
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

	if bootstrap.AddAfterCommitHook(ctx, func() {
		syncSharedSetting(updatedSetting)
	}) {
		return nil
	}

	syncSharedSetting(updatedSetting)

	return nil
}

func checkSettingUpdateResult(ctx context.Context, db *gorm.DB, result *gorm.DB, id int64) error {
	if result.Error != nil {
		return result.Error
	}

	if result.RowsAffected != 0 {
		return nil
	}

	var count int64
	if err := db.Model(new(models.Setting)).Where("id = ?", id).Count(&count).Error; err != nil {
		ctx.Error("确认设置是否存在失败", zap.Error(err), zap.Int64("id", id))

		return err
	}

	if count == 0 {
		ctx.Error("设置更新失败，记录不存在", zap.Int64("id", id))

		return gorm.ErrRecordNotFound
	}

	return nil
}

func syncSharedSetting(setting *models.Setting) {
	addition := setting.Addition
	addition.WebDAVAllowedSuffixes = append([]string(nil), addition.WebDAVAllowedSuffixes...)
	shared.SetSetting(setting.SaltKey, setting.BaseURL, setting.EnableAuth, addition)
}
