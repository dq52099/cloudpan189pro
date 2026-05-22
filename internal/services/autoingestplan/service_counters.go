package autoingestplan

import (
	"github.com/xxcheng123/cloudpan189-share/internal/framework/context"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// IncrAddCount 增加新增计数
func (s *service) IncrAddCount(ctx context.Context, id int64, delta int64) error {
	if id <= 0 {
		return errInvalidAutoIngestPlanID
	}

	result := s.getDB(ctx).Where("id = ?", id).Update("add_count", gorm.Expr("add_count + ?", delta))
	if result.Error != nil {
		ctx.Error("更新自动挂载计划新增计数失败", zap.Error(result.Error), zap.Int64("id", id), zap.Int64("delta", delta))

		return result.Error
	}

	if result.RowsAffected == 0 {
		if err := s.ensurePlanExists(ctx, id); err != nil {
			ctx.Error("更新自动挂载计划新增计数失败，记录不存在", zap.Int64("id", id), zap.Int64("delta", delta))

			return err
		}
	}

	return nil
}

// IncrFailedCount 增加失败计数
func (s *service) IncrFailedCount(ctx context.Context, id int64, delta int64) error {
	if id <= 0 {
		return errInvalidAutoIngestPlanID
	}

	result := s.getDB(ctx).Where("id = ?", id).Update("failed_count", gorm.Expr("failed_count + ?", delta))
	if result.Error != nil {
		ctx.Error("更新自动挂载计划失败计数失败", zap.Error(result.Error), zap.Int64("id", id), zap.Int64("delta", delta))

		return result.Error
	}

	if result.RowsAffected == 0 {
		if err := s.ensurePlanExists(ctx, id); err != nil {
			ctx.Error("更新自动挂载计划失败计数失败，记录不存在", zap.Int64("id", id), zap.Int64("delta", delta))

			return err
		}
	}

	return nil
}

// ResetCounters 重置计数器
func (s *service) ResetCounters(ctx context.Context, id int64) error {
	if id <= 0 {
		return errInvalidAutoIngestPlanID
	}

	result := s.getDB(ctx).Where("id = ?", id).Updates(map[string]interface{}{
		"add_count":    0,
		"failed_count": 0,
	})
	if result.Error != nil {
		ctx.Error("重置自动挂载计划计数器失败", zap.Error(result.Error), zap.Int64("id", id))

		return result.Error
	}

	if result.RowsAffected == 0 {
		if err := s.ensurePlanExists(ctx, id); err != nil {
			ctx.Error("重置自动挂载计划计数器失败，记录不存在", zap.Int64("id", id))

			return err
		}
	}

	return nil
}
