package autoingestplan

import (
	"github.com/xxcheng123/cloudpan189-share/internal/framework/context"
	"github.com/xxcheng123/cloudpan189-share/internal/pkgs/utils"
	"go.uber.org/zap"
)

// Update 更新自动挂载计划字段
func (s *service) Update(ctx context.Context, id int64, fields ...utils.Field) error {
	if id <= 0 {
		return errInvalidAutoIngestPlanID
	}

	mp := make(map[string]interface{})
	for _, field := range fields {
		mp[field.Key] = field.Value
	}

	if len(mp) == 0 {
		return errEmptyAutoIngestPlanUpdateFields
	}

	result := s.getDB(ctx).Where("id = ?", id).Updates(mp)
	if result.Error != nil {
		ctx.Error("更新自动挂载计划失败", zap.Error(result.Error), zap.Int64("id", id))

		return result.Error
	}

	if result.RowsAffected == 0 {
		if err := s.ensurePlanExists(ctx, id); err != nil {
			ctx.Error("更新自动挂载计划失败，记录不存在", zap.Int64("id", id))

			return err
		}
	}

	return nil
}
