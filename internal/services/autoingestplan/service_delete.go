package autoingestplan

import (
	"errors"

	"github.com/xxcheng123/cloudpan189-share/internal/framework/context"
	"github.com/xxcheng123/cloudpan189-share/internal/repository/models"
	"go.uber.org/zap"
)

type DeleteRequest struct {
	ID      int64 `json:"id"`
	UserID  int64 // 当前用户ID
	IsAdmin bool  // 是否管理员
}

// Delete 删除自动挂载计划
func (s *service) Delete(ctx context.Context, req *DeleteRequest) error {
	// 非管理员只能删除自己的计划
	if !req.IsAdmin && req.UserID > 0 {
		result := s.getDB(ctx).Where("id = ? AND user_id = ?", req.ID, req.UserID).Delete(new(models.AutoIngestPlan))
		if result.Error != nil {
			ctx.Error("删除自动挂载计划失败", zap.Error(result.Error), zap.Int64("id", req.ID), zap.Int64("user_id", req.UserID))
			return result.Error
		}
		if result.RowsAffected == 0 {
			ctx.Error("删除自动挂载计划失败，无权限或不存在", zap.Int64("id", req.ID), zap.Int64("user_id", req.UserID))
			return errors.New("计划不存在或无权限删除")
		}
		return nil
	}

	// 管理员可以删除任何计划
	if err := s.getDB(ctx).Where("id = ?", req.ID).Delete(new(models.AutoIngestPlan)).Error; err != nil {
		ctx.Error("删除自动挂载计划失败", zap.Error(err), zap.Int64("id", req.ID))

		return err
	}

	return nil
}
