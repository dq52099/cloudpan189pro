package user

import (
	"github.com/xxcheng123/cloudpan189-share/internal/framework/context"
	"go.uber.org/zap"
)

type BindGroupRequest struct {
	UserID  int64 `json:"userId" binding:"required,min=1" example:"1001"` // 用户ID，必须大于0
	GroupID int64 `json:"groupId" binding:"min=0" example:"2"`            // 用户组ID，0表示默认用户组
}

// BindGroup 绑定用户到用户组
func (s *service) BindGroup(ctx context.Context, req *BindGroupRequest) error {
	if req == nil || req.UserID <= 0 {
		return errInvalidUserID
	}

	if req.GroupID < 0 {
		return errInvalidUserGroupID
	}

	result := s.getDB(ctx).Where("id = ?", req.UserID).Update("group_id", req.GroupID)
	if err := s.checkUserUpdateResult(ctx, result, req.UserID); err != nil {
		ctx.Error("绑定用户到用户组失败",
			zap.Error(err),
			zap.Int64("user_id", req.UserID),
			zap.Int64("group_id", req.GroupID))

		return err
	}

	return nil
}
