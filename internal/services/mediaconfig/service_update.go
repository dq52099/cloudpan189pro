package mediaconfig

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
		return errEmptyMediaConfigUpdateFields
	}

	updatedCfg := new(models.MediaConfig)

	err := s.getDB(ctx).Transaction(func(tx *gorm.DB) error {
		cfg := new(models.MediaConfig)
		if err := tx.First(cfg).Error; err != nil {
			ctx.Error("媒体配置查询失败", zap.Error(err))

			return err
		}

		result := tx.Where("id = ?", cfg.ID).Updates(mp)
		if result.Error != nil {
			ctx.Error("媒体配置更新失败", zap.Error(result.Error), zap.Int64("id", cfg.ID))

			return result.Error
		}

		// 回查写入，按原 ID 查询，避免多配置记录时同步到错误记录。
		if err := tx.First(updatedCfg, "id = ?", cfg.ID).Error; err != nil {
			ctx.Error("媒体配置回查写入失败", zap.Error(err), zap.Int64("id", cfg.ID))

			return err
		}

		return nil
	})
	if err != nil {
		return err
	}

	shared.MediaConfig = updatedCfg

	return nil
}
