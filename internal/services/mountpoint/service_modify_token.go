package mountpoint

import (
	"errors"

	"github.com/xxcheng123/cloudpan189-share/internal/framework/context"
	"github.com/xxcheng123/cloudpan189-share/internal/repository/models"
	"go.uber.org/zap"
)

var errPermissionDenied = errors.New("无权限操作")

type ModifyTokenRequest struct {
	ID            int64 `json:"id"`
	TokenId       int64 `json:"tokenId"`
	CreatorUserID int64 // 当前用户ID
	IsAdmin       bool  // 是否管理员
}

func (s *service) ModifyToken(ctx context.Context, req *ModifyTokenRequest) error {
	if req == nil || req.ID <= 0 {
		return errInvalidMountPointFileID
	}

	if req.TokenId < 0 {
		return errInvalidMountPointTokenID
	}

	if !req.IsAdmin && req.CreatorUserID <= 0 {
		return errInvalidMountPointUserID
	}

	// 先查询挂载点信息
	var mountPoint models.MountPoint
	if err := s.getDB(ctx).Where("id = ?", req.ID).First(&mountPoint).Error; err != nil {
		ctx.Error("查询挂载点失败", zap.Error(err), zap.Int64("id", req.ID))

		return err
	}

	// 非管理员只能修改自己创建的挂载点
	if !req.IsAdmin {
		if mountPoint.CreatorUserID != req.CreatorUserID {
			ctx.Error("修改挂载点令牌失败，无权限", zap.Int64("id", req.ID), zap.Int64("creator_user_id", mountPoint.CreatorUserID), zap.Int64("current_user_id", req.CreatorUserID))

			return errPermissionDenied
		}
	}

	// 验证新令牌属于当前用户（非管理员情况下）
	if !req.IsAdmin && req.TokenId > 0 {
		var token models.CloudToken
		if err := s.getDB(ctx).Where("id = ? AND user_id = ?", req.TokenId, req.CreatorUserID).First(&token).Error; err != nil {
			ctx.Error("验证令牌所属用户失败", zap.Error(err), zap.Int64("token_id", req.TokenId), zap.Int64("user_id", req.CreatorUserID))

			return errPermissionDenied
		}
	}

	updateQuery := s.getDB(ctx).Where("id = ?", req.ID)
	if !req.IsAdmin {
		updateQuery = updateQuery.Where("creator_user_id = ?", req.CreatorUserID)
	}

	result := updateQuery.Update("token_id", req.TokenId)
	if result.Error != nil {
		ctx.Error("修改挂载点令牌失败", zap.Error(result.Error), zap.Int64("id", req.ID), zap.Int64("token_id", req.TokenId))

		return result.Error
	}

	if result.RowsAffected == 0 {
		if err := s.ensureMountPointUpdateTargetExists(ctx, req.ID, req.CreatorUserID, req.IsAdmin); err != nil {
			ctx.Error("修改挂载点令牌失败，记录不存在", zap.Int64("id", req.ID), zap.Int64("token_id", req.TokenId))

			return err
		}
	}

	return nil
}
