package autoingestplan

import (
	"github.com/xxcheng123/cloudpan189-share/internal/framework/context"
	"github.com/xxcheng123/cloudpan189-share/internal/repository/models"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// Create 创建自动挂载计划
func (s *service) Create(ctx context.Context, plan *models.AutoIngestPlan) (int64, error) {
	enabled := plan.Enabled
	db := s.getDB(ctx)

	if err := db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(plan).Error; err != nil {
			return err
		}

		result := tx.Model(&models.AutoIngestPlan{}).
			Where("id = ?", plan.ID).
			Update("enabled", enabled)
		if result.Error != nil {
			return result.Error
		}

		plan.Enabled = enabled

		return nil
	}); err != nil {
		ctx.Error("创建自动挂载计划失败", zap.Error(err), zap.String("name", plan.Name))

		return 0, err
	}

	return plan.ID, nil
}
