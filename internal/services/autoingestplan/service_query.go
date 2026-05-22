package autoingestplan

import (
	"errors"

	"github.com/xxcheng123/cloudpan189-share/internal/framework/context"
	"github.com/xxcheng123/cloudpan189-share/internal/repository/models"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// Query 根据ID查询自动挂载计划
func (s *service) Query(ctx context.Context, id int64) (*models.AutoIngestPlan, error) {
	if id <= 0 {
		return nil, errInvalidAutoIngestPlanID
	}

	var plan models.AutoIngestPlan

	if err := s.getDB(ctx).Where("id = ?", id).First(&plan).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}

		ctx.Error("查询自动挂载计划失败", zap.Error(err), zap.Int64("id", id))

		return nil, err
	}

	return &plan, nil
}
