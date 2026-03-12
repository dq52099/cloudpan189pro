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

	// 管理员可以删除任何令牌
	result := s.getDB(ctx).Where("id = ?", req.ID).Delete(nil)
	if result.Error != nil {
		ctx.Error("删除云盘令牌失败", zap.Error(result.Error), zap.Int64("id", req.ID))
		return errors.Wrap(result.Error, "删除云盘令牌失败")
	}
	if result.RowsAffected == 0 {
		ctx.Error("删除云盘令牌失败，记录不存在", zap.Int64("id", req.ID))
		return errors.Wrap(gorm.ErrRecordNotFound, "令牌不存在")
	}

	// 删除关联的挂载点（只删除创建者为自己创建的挂载点）
	s.getDB(ctx).Where("token_id = ? AND creator_user_id = ?", req.ID, req.UserID).Delete(new(models.MountPoint))

	return nil
}
