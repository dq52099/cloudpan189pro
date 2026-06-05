package http

import (
	"strings"

	"go.uber.org/zap"

	"github.com/xxcheng123/cloudpan189-share/internal/consts"
	"github.com/xxcheng123/cloudpan189-share/internal/framework/httpcontext"
	"github.com/xxcheng123/cloudpan189-share/internal/pkgs/utils"
	"github.com/xxcheng123/cloudpan189-share/internal/services/user"
	"github.com/xxcheng123/cloudpan189-share/internal/shared"
	"gorm.io/gorm"
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

func setAnonymousAdminContext(ctx *httpcontext.Context) {
	ctx.Set(consts.CtxKeyUserId, int64(0))
	ctx.Set(consts.CtxKeyUsername, "anonymous")
	ctx.Set(consts.CtxKeyIsAdmin, true)
	ctx.Set(consts.CtxKeyUserGroupId, int64(0))
}

func parseBearerToken(authHeader string) (string, bool) {
	fields := strings.Fields(authHeader)
	if len(fields) != 2 || !strings.EqualFold(fields[0], "Bearer") {
		return "", false
	}

	return fields[1], true
}

func (m *AuthMiddleware) Auth(requireAdmins ...bool) httpcontext.HandlerFunc {
	requireAdmin := utils.UseSimplify(false, requireAdmins...)

	return func(ctx *httpcontext.Context) {
		if !shared.IsAuthEnabled() {
			setAnonymousAdminContext(ctx)
			ctx.Next()

			return
		}

		var (
			logger = ctx.GetContext().Logger
		)

		authHeader := ctx.GetHeader("Authorization")
		if authHeader == "" {
			ctx.Unauthorized(errMessageMissTokenHeader)

			return
		}

		token, ok := parseBearerToken(authHeader)
		if !ok {
			ctx.Unauthorized(errMessageTokenHeaderFormatError)

			return
		}

		uid, username, version, err := m.userService.ParseAccessToken(token)
		if err != nil {
			ctx.Unauthorized(errMessageTokenParseErr).WithError(err)

			return
		}

		u, err := m.userService.Query(ctx.GetContext(), uid)
		if err != nil {
			ctx.Unauthorized(errMessageUserInfoQueryErr).WithError(err)

			return
		}

		if u == nil {
			ctx.Unauthorized(errMessageUserInfoQueryErr).WithError(gorm.ErrRecordNotFound)

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
