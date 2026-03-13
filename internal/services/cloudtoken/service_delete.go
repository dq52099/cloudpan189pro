package cloudtoken

import (
	"github.com/pkg/errors"
	"github.com/xxcheng123/cloudpan189-share/internal/framework/context"
	"github.com/xxcheng123/cloudpan189-share/internal/repository/models"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// DeleteRequest 删除云盘令牌请求
type DeleteRequest struct {
	ID      int64 `json:"id" binding:"required" example:"1"` // 云盘令牌ID
	UserID  int64 `json:"-"`                                 // 当前用户ID
	IsAdmin bool  `json:"-"`                                 // 是否管理员
}

func (s *service) Delete(ctx context.Context, req *DeleteRequest) (err error) {
	// 非管理员只能删除自己的令牌
	if !req.IsAdmin && req.UserID > 0 {
		result := s.getDB(ctx).Where("id = ? AND user_id = ?", req.ID, req.UserID).Delete(nil)
		if result.Error != nil {
			ctx.Error("删除云盘令牌失败", zap.Error(result.Error), zap.Int64("id", req.ID), zap.Int64("user_id", req.UserID))
			return errors.Wrap(result.Error, "删除云盘令牌失败")
		}
		if result.RowsAffected == 0 {
			ctx.Error("删除云盘令牌失败，无权限或不存在", zap.Int64("id", req.ID), zap.Int64("user_id", req.UserID))
			return errors.Wrap(gorm.ErrRecordNotFound, "令牌不存在或无权限删除")
		}

		return nil
	}

	// 管理员删除前先清理所有挂载点与用户挂载令牌的关联，避免旧数据阻塞删除。
	if err := s.svc.GetDB(ctx).
		Model(new(models.MountPoint)).
		Where("token_id = ?", req.ID).
		Update("token_id", 0).Error; err != nil {
		ctx.Error("清理挂载点令牌引用失败", zap.Error(err), zap.Int64("id", req.ID))
		return errors.Wrap(err, "清理挂载点令牌引用失败")
	}

	if err := s.svc.GetDB(ctx).
		Where("token_id = ?", req.ID).
		Delete(new(models.UserMountPointToken)).Error; err != nil {
		ctx.Error("清理用户挂载点令牌绑定失败", zap.Error(err), zap.Int64("id", req.ID))
		return errors.Wrap(err, "清理用户挂载点令牌绑定失败")
	}

	result := s.getDB(ctx).Where("id = ?", req.ID).Delete(nil)
	if result.Error != nil {
		ctx.Error("删除云盘令牌失败", zap.Error(result.Error), zap.Int64("id", req.ID))
		return errors.Wrap(result.Error, "删除云盘令牌失败")
	}
	if result.RowsAffected == 0 {
		ctx.Error("删除云盘令牌失败，记录不存在", zap.Int64("id", req.ID))
		return errors.Wrap(gorm.ErrRecordNotFound, "令牌不存在")
	}

	return nil
}
