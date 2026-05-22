package autoingestplan

import (
	"github.com/xxcheng123/cloudpan189-share/internal/framework/context"
	"go.uber.org/zap"
)

// UpdateOffset 更新计划偏移量
func (s *service) UpdateOffset(ctx context.Context, id int64, offset int64) error {
	if id <= 0 {
		return errInvalidAutoIngestPlanID
	}

	result := s.getDB(ctx).Where("id = ?", id).Update("offset", offset)
	if result.Error != nil {
		ctx.Error("更新自动挂载计划偏移量失败", zap.Error(result.Error), zap.Int64("id", id), zap.Int64("offset", offset))

		return result.Error
	}

	if result.RowsAffected == 0 {
		if err := s.ensurePlanExists(ctx, id); err != nil {
			ctx.Error("更新自动挂载计划偏移量失败，记录不存在", zap.Int64("id", id), zap.Int64("offset", offset))

			return err
		}
	}

	return nil
}
