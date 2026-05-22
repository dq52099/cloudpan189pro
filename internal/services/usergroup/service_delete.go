package usergroup

import (
	"github.com/xxcheng123/cloudpan189-share/internal/framework/context"
	"github.com/xxcheng123/cloudpan189-share/internal/repository/models"

	"go.uber.org/zap"
	"gorm.io/gorm"
)

type DeleteRequest struct {
	ID int64 `json:"id" binding:"required,min=1" example:"1001"` // 用户组ID，必须大于1
}

func (s *service) Delete(ctx context.Context, req *DeleteRequest) error {
	if req == nil || req.ID <= 0 {
		return errInvalidUserGroupID
	}

	err := s.svc.GetDB(ctx).Transaction(func(tx *gorm.DB) error {
		result := tx.Model(new(models.UserGroup)).Where("id = ?", req.ID).Delete(&models.UserGroup{})
		if result.Error != nil {
			ctx.Error("用户组删除失败", zap.Int64("id", req.ID), zap.Error(result.Error))

			return result.Error
		}

		if result.RowsAffected == 0 {
			ctx.Error("用户组删除失败，记录不存在", zap.Int64("id", req.ID))

			return gorm.ErrRecordNotFound
		}

		if err := tx.Model(new(models.User)).
			Where("group_id = ?", req.ID).
			Update("group_id", 0).Error; err != nil {
			ctx.Error("重置用户组绑定失败", zap.Int64("id", req.ID), zap.Error(err))

			return err
		}

		if err := tx.Where("group_id = ?", req.ID).Delete(new(models.Group2File)).Error; err != nil {
			ctx.Error("清理用户组文件权限失败", zap.Int64("id", req.ID), zap.Error(err))

			return err
		}

		return nil
	})
	if err != nil {
		return err
	}

	return nil
}
