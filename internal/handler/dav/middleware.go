package dav

import (
	"errors"

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

func newAuthMiddleware(userService user.Service) *AuthMiddleware {
	return &AuthMiddleware{userService: userService}
}

func (m *AuthMiddleware) Auth() httpcontext.HandlerFunc {
	return func(ctx *httpcontext.Context) {
		if !shared.IsAuthEnabled() {
			ctx.Set(consts.CtxKeyUserId, int64(0))
			ctx.Set(consts.CtxKeyUsername, "anonymous")
			ctx.Set(consts.CtxKeyIsAdmin, true)
			ctx.Set(consts.CtxKeyUserGroupId, int64(0))
			ctx.Next()

			return
		}

		username, password, ok := ctx.Request.BasicAuth()
		if !ok {
			ctx.Header("WWW-Authenticate", `Basic realm="Restricted"`)

			ctx.Unauthorized("请输入用户名和密码")

			ctx.Abort()

			return
		}

		u, err := m.userService.QueryByUsername(ctx.GetContext(), username)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				ctx.Unauthorized("用户不存在或已禁用")
			} else {
				ctx.Unauthorized("验证失败").WithError(err)
			}

			ctx.Abort()

			return
		}

		if u.Password != utils.MD5(password) {
			ctx.Unauthorized("密码错误")

			ctx.Abort()

			return
		}

		if !u.Valid() {
			ctx.Unauthorized("用户已禁用")

			ctx.Abort()

			return
		}

		ctx.Set(consts.CtxKeyUserId, u.ID)
		ctx.Set(consts.CtxKeyUsername, username)
		ctx.Set(consts.CtxKeyIsAdmin, u.IsAdmin)
		ctx.Set(consts.CtxKeyUserGroupId, u.GroupID)

		ctx.Next()
	}
}

type BalanceMiddleware struct{}

func newBalanceMiddleware() *BalanceMiddleware {
	return &BalanceMiddleware{}
}

func (m *BalanceMiddleware) Balance() httpcontext.HandlerFunc {
	return func(ctx *httpcontext.Context) {
		if ctx.GetHeader("Depth") == "" {
			ctx.Request.Header.Add("Depth", "1")
		} else if ctx.GetHeader("Depth") == "infinity" {
			ctx.Unauthorized("不支持infinity深度查询")

			ctx.Abort()

			return
		}

		if ctx.GetHeader("X-Litmus") == "props: 3 (propfind_invalid2)" {
			ctx.Unauthorized("无效的属性名称")

			ctx.Abort()

			return
		}

		switch ctx.Request.Method {
		case "PROPFIND":
			ctx.Next()
		case "GET", "HEAD", "POST":
			ctx.Next()
		case "OPTIONS":
			allow := "OPTIONS, HEAD, GET, POST, PROPFIND"

			ctx.Header("Allow", allow)
			// http://www.webdav.org/specs/rfc4918.html#dav.compliance.classes
			ctx.Header("DAV", "1, 2")
			// http://msdn.microsoft.com/en-au/library/cc250217.aspx
			ctx.Header("MS-Author-Via", "DAV")

			ctx.Abort()
		default:
			ctx.Unauthorized("不支持的请求方法")

			ctx.Abort()
		}
	}
}
