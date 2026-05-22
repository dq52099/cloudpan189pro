package autoingestplan

import (
	"github.com/pkg/errors"

	"github.com/xxcheng123/cloudpan189-share/internal/framework/context"
	"github.com/xxcheng123/cloudpan189-share/internal/repository/models"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type DeleteRequest struct {
	ID      int64 `json:"id"`
	UserID  int64 // 当前用户ID
	IsAdmin bool  // 是否管理员
}

// Delete 删除自动挂载计划
func (s *service) Delete(ctx context.Context, req *DeleteRequest) error {
	if req == nil || req.ID <= 0 {
		return errInvalidAutoIngestPlanID
	}

	if !req.IsAdmin {
		if req.UserID <= 0 {
			return errInvalidAutoIngestPlanUserID
		}
	}

	if err := s.getDB(ctx).Transaction(func(tx *gorm.DB) error {
		query := tx.Model(new(models.AutoIngestPlan)).Where("id = ?", req.ID)
		if !req.IsAdmin {
			// 非管理员只能删除自己的计划
			query = query.Where("user_id = ?", req.UserID)
		}

		result := query.Delete(new(models.AutoIngestPlan))
		if result.Error != nil {
			return result.Error
		}

		if result.RowsAffected == 0 {
			if !req.IsAdmin {
				return errors.Wrap(gorm.ErrRecordNotFound, "计划不存在或无权限删除")
			}

			return errors.Wrap(gorm.ErrRecordNotFound, "计划不存在")
		}

		return tx.Model(new(models.AutoIngestLog)).
			Where("plan_id = ?", req.ID).
			Delete(new(models.AutoIngestLog)).Error
	}); err != nil {
		if !req.IsAdmin {
			ctx.Error("删除自动挂载计划失败，无权限或不存在", zap.Error(err), zap.Int64("id", req.ID), zap.Int64("user_id", req.UserID))
		} else {
			ctx.Error("删除自动挂载计划失败", zap.Error(err), zap.Int64("id", req.ID))
		}

		return err
	}

	return nil
}
