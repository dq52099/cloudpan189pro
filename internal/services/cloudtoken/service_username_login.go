package cloudtoken

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/pkg/errors"

	"github.com/tickstep/cloudpan189-api/cloudpan"
	"github.com/xxcheng123/cloudpan189-share/internal/framework/context"
	"github.com/xxcheng123/cloudpan189-share/internal/pkgs/utils"
	"github.com/xxcheng123/cloudpan189-share/internal/repository/models"
	"go.uber.org/zap"
)

var cloudTokenLogURLPattern = regexp.MustCompile(`(?i)(?:[a-z][a-z0-9+.-]*://|/)[^\s"'<>]+`)

var appLogin = func(username, password string) (*cloudpan.AppLoginToken, error) {
	token, err := cloudpan.AppLogin(username, password)
	if err != nil {
		return nil, err
	}

	return token, nil
}

func sanitizeCloudTokenLogText(text string) string {
	if text == "" {
		return ""
	}

	text = cloudTokenLogURLPattern.ReplaceAllStringFunc(text, utils.RedactURLForLog)

	return utils.RedactSensitiveText(text)
}

func sanitizeCloudTokenLogError(err error) string {
	if err == nil {
		return ""
	}

	return sanitizeCloudTokenLogText(err.Error())
}

func sanitizeCloudTokenLoginFailure(err error, username string, password string) string {
	message := sanitizeCloudTokenLogError(err)

	for _, value := range []string{strings.TrimSpace(username), strings.TrimSpace(password)} {
		if value == "" {
			continue
		}

		message = strings.ReplaceAll(message, value, utils.RedactedSecret)
	}

	return message
}

// UsernameLoginRequest 用户名登录请求
type UsernameLoginRequest struct {
	ID       int64
	Username string
	Password string
	Name     string
	UserID   int64 // 创建令牌的用户ID
	IsAdmin  bool  // 是否管理员
}

// UsernameLoginResponse 用户名登录响应
type UsernameLoginResponse struct {
	ID int64 `json:"id" example:"1"` // 云盘令牌ID
}

func (s *service) UsernameLogin(ctx context.Context, req *UsernameLoginRequest) (resp *UsernameLoginResponse, err error) {
	if req == nil || req.Username == "" || req.Password == "" {
		return nil, errInvalidUsernameLoginCredentials
	}

	if req.ID > 0 {
		// 检测信息
		oldToken, queryErr := s.QueryAccessible(ctx, req.ID, req.UserID, req.IsAdmin)
		if queryErr != nil {
			return nil, queryErr
		}

		if oldToken.LoginType != models.LoginTypePassword {
			ctx.Error("云盘令牌类型错误", zap.Int64("id", req.ID))

			return nil, errors.New("云盘令牌类型错误")
		}

		loginResult, loginErr := appLogin(req.Username, req.Password)
		if loginErr != nil {
			safeErr := sanitizeCloudTokenLoginFailure(loginErr, req.Username, req.Password)
			ctx.Error("用户名密码登录失败",
				zap.String("error", safeErr),
				zap.String("username", utils.MaskSecret(req.Username)),
			)

			return nil, fmt.Errorf("登录失败: %s", safeErr)
		}

		addition := oldToken.Addition
		if addition == nil {
			addition = make(map[string]interface{})
		}

		now := time.Now()
		addition[models.CloudTokenAdditionAutoLoginResultKey] = fmt.Sprintf("%s, token 刷新成功", now.Format(time.DateTime))
		addition[models.CloudTokenAdditionAutoLoginTimes] = 0
		addition[models.CloudTokenAdditionTokenIssuedAt] = now.UnixMilli()

		updateMap := map[string]interface{}{
			"access_token": loginResult.SskAccessToken,
			"expires_in":   loginResult.SskAccessTokenExpiresIn,
			"username":     req.Username,
			"password":     req.Password,
			"addition":     addition,
		}

		query := s.getDB(ctx).Where("id = ?", oldToken.ID)
		if !req.IsAdmin {
			query = query.Where("user_id = ?", req.UserID)
		}

		result := query.Updates(updateMap)
		if result.Error != nil {
			ctx.Error("更新云盘令牌失败", zap.Error(result.Error), zap.Int64("id", oldToken.ID))

			return nil, errors.Wrap(result.Error, "更新云盘令牌失败")
		}

		if err := s.checkCloudTokenUpdateResult(ctx, result, oldToken.ID, req.UserID, req.IsAdmin, "令牌不存在"); err != nil {
			return nil, err
		}

		return &UsernameLoginResponse{
			ID: req.ID,
		}, nil
	}

	if req.UserID <= 0 {
		return nil, errInvalidCloudTokenUserID
	}

	loginResult, loginErr := appLogin(req.Username, req.Password)
	if loginErr != nil {
		safeErr := sanitizeCloudTokenLoginFailure(loginErr, req.Username, req.Password)
		ctx.Error("用户名密码登录失败",
			zap.String("error", safeErr),
			zap.String("username", utils.MaskSecret(req.Username)),
		)

		return nil, fmt.Errorf("登录失败: %s", safeErr)
	}

	m := &models.CloudToken{
		Name:        req.Name,
		Status:      1,
		AccessToken: loginResult.SskAccessToken,
		ExpiresIn:   loginResult.SskAccessTokenExpiresIn,
		Username:    req.Username,
		Password:    req.Password,
		LoginType:   models.LoginTypePassword,
		Addition: map[string]interface{}{
			models.CloudTokenAdditionTokenIssuedAt: time.Now().UnixMilli(),
		},
		UserID: req.UserID,
	}

	if err = s.getDB(ctx).Create(m).Error; err != nil {
		ctx.Error("创建云盘令牌失败", zap.Error(err))

		return nil, errors.Wrap(err, "创建云盘令牌失败")
	}

	return &UsernameLoginResponse{
		ID: m.ID,
	}, nil
}
