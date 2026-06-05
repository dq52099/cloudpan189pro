package user

import (
	"errors"
	"reflect"

	"github.com/xxcheng123/cloudpan189-share/internal/framework/httpcontext"
	"go.uber.org/zap"
)

func (h *handler) ensureUserService(ctx *httpcontext.Context, busErr httpcontext.BusinessError) bool {
	if isNilDependency(h.userService) {
		ctx.Fail(busErr.WithError(errors.New("用户服务未初始化")))

		return false
	}

	return true
}

func (h *handler) ensureUserGroupService(ctx *httpcontext.Context, busErr httpcontext.BusinessError) bool {
	if isNilDependency(h.userGroupService) {
		ctx.Fail(busErr.WithError(errors.New("用户组服务未初始化")))

		return false
	}

	return true
}

func (h *handler) hasLoginLogService(ctx *httpcontext.Context) bool {
	if isNilDependency(h.loginLogService) {
		ctx.GetContext().Warn("登录日志服务未初始化", zap.String("handler", "user.RecordLog"))

		return false
	}

	return true
}

func isNilDependency(v any) bool {
	if v == nil {
		return true
	}

	value := reflect.ValueOf(v)
	switch value.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return value.IsNil()
	default:
		return false
	}
}
