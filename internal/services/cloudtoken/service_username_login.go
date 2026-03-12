package cloudtoken

import (
	"fmt"
	"time"

	"github.com/pkg/errors"

	"github.com/tickstep/cloudpan189-api/cloudpan"
	"github.com/xxcheng123/cloudpan189-share/internal/framework/context"
	"github.com/xxcheng123/cloudpan189-share/internal/repository/models"
	"go.uber.org/zap"
)

// UsernameLoginRequest 用户名登录请求
type UsernameLoginRequest struct {
	ID       int64
	Username string
	Password string
	Name     string
	UserID   int64 // 创建令牌的用户ID
}

// UsernameLoginResponse 用户名登录响应
type UsernameLoginResponse struct {
	ID int64 `json:"id" example:"1"` // 云盘令牌ID
}

func (s *service) UsernameLogin(ctx context.Context, req *UsernameLoginRequest) (resp *UsernameLoginResponse, err error) {
	loginResult, loginErr := cloudpan.AppLogin(req.Username, req.Password)
	if loginErr != nil {
		ctx.Error("用户名密码登录失败", zap.Error(loginErr), zap.String("username", req.Username))

		return nil, errors.Wrap(loginErr, "登录失败")
	}

	if req.ID > 0 {
		// 检测信息
		var oldToken models.CloudToken
		if err = s.getDB(ctx).Where("id = ?", req.ID).First(&oldToken).Error; err != nil {
			ctx.Error("查询云盘令牌失败", zap.Error(err), zap.Int64("id", req.ID))

			return nil, errors.Wrap(err, "查询云盘令牌失败")
		} else if oldToken.LoginType != models.LoginTypePassword {
			ctx.Error("云盘令牌类型错误", zap.Int64("id", req.ID))

			return nil, errors.New("云盘令牌类型错误")
		}

		addition := oldToken.Addition
		addition[models.CloudTokenAdditionAutoLoginResultKey] = fmt.Sprintf("%s, token 刷新成功", time.Now().Format(time.DateTime))
		addition[models.CloudTokenAdditionAutoLoginTimes] = 0

		updateMap := map[string]interface{}{
			"access_token": loginResult.SskAccessToken,
			"expires_in":   loginResult.SskAccessTokenExpiresIn,
			"username":     req.Username,
			"password":     req.Password,
			"addition":     addition,
		}

		result := s.getDB(ctx).Where("id = ?", oldToken.ID).Updates(updateMap)
		if result.Error != nil {
			ctx.Error("更新云盘令牌失败", zap.Error(result.Error), zap.Int64("id", oldToken.ID))

			return nil, errors.Wrap(result.Error, "更新云盘令牌失败")
		}

		return &UsernameLoginResponse{
			ID: req.ID,
		}, nil
	}

	m := &models.CloudToken{
		Name:        req.Name,
		Status:      1,
		AccessToken: loginResult.SskAccessToken,
		ExpiresIn:   loginResult.SskAccessTokenExpiresIn,
		Username:    req.Username,
		Password:    req.Password,
		LoginType:   models.LoginTypePassword,
		Addition:    map[string]interface{}{},
		UserID:      req.UserID,
	}

	if err = s.getDB(ctx).Create(m).Error; err != nil {
		ctx.Error("创建云盘令牌失败", zap.Error(err))

		return nil, errors.Wrap(err, "创建云盘令牌失败")
	}

	return &UsernameLoginResponse{
		ID: m.ID,
	}, nil
}
