package user

import (
	"github.com/xxcheng123/cloudpan189-share/internal/framework/context"
	"github.com/xxcheng123/cloudpan189-share/internal/pkgs/utils"
	"github.com/xxcheng123/cloudpan189-share/internal/repository/models"
	"go.uber.org/zap"
)

type AddRequest struct {
	Username string `json:"username" binding:"required,min=3,max=20" example:"tom123"`   // 用户名，长度3-20位
	Password string `json:"password" binding:"required,min=6,max=20" example:"12345678"` // 密码，长度6-20
}

type AddOptionFunc = func(u *models.User)

func WithAdmin() AddOptionFunc {
	return func(u *models.User) {
		u.IsAdmin = true
	}
}

type AddResponse struct {
	ID int64 `json:"id" example:"1001"` // 新创建用户的ID
}

func (s *service) Add(ctx context.Context, req *AddRequest, opts ...AddOptionFunc) (resp *AddResponse, err error) {
	if req == nil || req.Username == "" {
		return nil, errInvalidUsername
	}

	if req.Password == "" {
		return nil, errInvalidUserPassword
	}

	u := &models.User{
		Username: req.Username,
		Password: utils.MD5(req.Password),
	}

	for _, opt := range opts {
		opt(u)
	}

	if err = s.getDB(ctx).Create(u).Error; err != nil {
		ctx.Error("新建用户失败", zap.Error(err))

		return nil, err
	}

	return &AddResponse{
		ID: u.ID,
	}, nil
}
