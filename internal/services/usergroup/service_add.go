package usergroup

import (
	"strings"

	"github.com/xxcheng123/cloudpan189-share/internal/framework/context"
	"github.com/xxcheng123/cloudpan189-share/internal/repository/models"
	"go.uber.org/zap"
)

type AddRequest struct {
	Name string `json:"name" binding:"required,min=1,max=255" example:"管理员组"` // 用户组名称，长度1-255位
}

type AddResponse struct {
	ID int64 `json:"id" example:"1001"` // 新创建用户组的ID
}

func (s *service) Add(ctx context.Context, req *AddRequest) (resp *AddResponse, err error) {
	if req == nil {
		return nil, errInvalidUserGroupName
	}

	name := strings.TrimSpace(req.Name)
	if name == "" {
		return nil, errInvalidUserGroupName
	}

	group := models.UserGroup{
		Name: name,
	}

	if err = s.getDB(ctx).Create(&group).Error; err != nil {
		ctx.Error("用户组创建失败", zap.String("name", name), zap.Error(err))

		return nil, err
	}

	return &AddResponse{
		ID: group.ID,
	}, nil
}
