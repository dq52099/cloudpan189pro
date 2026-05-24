package http

import (
	"strings"

	"github.com/xxcheng123/cloudpan189-share/internal/framework/httpcontext"
	"github.com/xxcheng123/cloudpan189-share/internal/pkgs/utils"

	"github.com/xxcheng123/cloudpan189-share/internal/consts"
	"github.com/xxcheng123/cloudpan189-share/internal/services/user"
	"go.uber.org/zap"
)

type AuthMiddleware struct {
	userService user.Service
}

var (
	errMessageMissTokenHeader        = "缺少Authorization头"
	errMessageTokenHeaderFormatError = "Authorization 头格式错误"
	errMessageTokenParseErr          = "Authorization 解析失败"
	errMessageUserDisabled           = "用户被禁用"
	errMessageUserInfoFlush          = "用户信息已更新，请重新登录"
	errMessageUserInfoQueryErr       = "用户信息查询失败"
	errMessageRequireAdmin           = "这个接口需要管理员才能访问"
)

func newAuthMiddleware(userService user.Service) *AuthMiddleware {
	return &AuthMiddleware{userService: userService}
}

func (m *AuthMiddleware) Auth(requireAdmins ...bool) httpcontext.HandlerFunc {
	requireAdmin := utils.UseSimplify(false, requireAdmins...)

	return func(ctx *httpcontext.Context) {
		var (
			logger = ctx.GetContext().Logger
		)

		authHeader := ctx.GetHeader("Authorization")
		if authHeader == "" {
			ctx.Unauthorized(errMessageMissTokenHeader)

			return
		}

		tokenParts := strings.SplitN(authHeader, " ", 2)
		if len(tokenParts) != 2 || tokenParts[0] != "Bearer" {
			ctx.Unauthorized(errMessageTokenHeaderFormatError)

			return
		}

		uid, username, version, err := m.userService.ParseAccessToken(tokenParts[1])
		if err != nil {
			ctx.Unauthorized(errMessageTokenParseErr).WithError(err)

			return
		}

		u, err := m.userService.Query(ctx.GetContext(), uid)
		if err != nil {
			ctx.Unauthorized(errMessageUserInfoQueryErr).WithError(err)

			return
		}

		if u.Version > version {
			logger.Warn("用户版本不匹配",
				zap.Int64("user_id", uid),
				zap.Int("user_version", u.Version),
				zap.Int("token_version", version))

			ctx.Unauthorized(errMessageUserInfoFlush)

			return
		}

		if !u.Valid() {
			ctx.Unauthorized(errMessageUserDisabled)

			return
		}

		if requireAdmin && !u.IsAdmin {
			ctx.Forbidden(errMessageRequireAdmin)

			return
		}

		ctx.Set(consts.CtxKeyUserId, uid)
		ctx.Set(consts.CtxKeyUsername, username)
		ctx.Set(consts.CtxKeyIsAdmin, u.IsAdmin)
		ctx.Set(consts.CtxKeyUserGroupId, u.GroupID)

		ctx.Next()
	}
}
