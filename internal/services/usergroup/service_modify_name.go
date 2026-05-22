package usergroup

import (
	"errors"
	"strings"

	"github.com/xxcheng123/cloudpan189-share/internal/framework/context"
	"github.com/xxcheng123/cloudpan189-share/internal/repository/models"

	"go.uber.org/zap"
)

// ModifyNameRequest 修改用户组名称请求
type ModifyNameRequest struct {
	ID   int64  `json:"id" binding:"required,min=1" example:"1"`              // 用户组ID
	Name string `json:"name" binding:"required,min=1,max=255" example:"管理员组"` // 新的用户组名称
}

// ModifyName 修改用户组名称
func (s *service) ModifyName(ctx context.Context, req *ModifyNameRequest) error {
	if req == nil || req.ID <= 0 {
		return errInvalidUserGroupID
	}

	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		return errInvalidUserGroupName
	}

	var existCount int64
	if err := s.getDB(ctx).Model(&models.UserGroup{}).
		Where("name = ? AND id != ?", req.Name, req.ID).
		Count(&existCount).Error; err != nil {
		ctx.Error("检查用户组名称是否存在失败", zap.Int64("id", req.ID), zap.String("name", req.Name), zap.Error(err))

		return err
	}

	if existCount > 0 {
		return errors.New("用户组名称已存在")
	}

	// 执行更新操作
	result := s.getDB(ctx).Model(&models.UserGroup{}).
		Where("id = ?", req.ID).
		Update("name", req.Name)

	if err := s.checkUserGroupUpdateResult(ctx, result, req.ID); err != nil {
		ctx.Error("修改用户组名称失败", zap.Int64("id", req.ID), zap.String("name", req.Name), zap.Error(err))

		return err
	}

	return nil
}
