package cloudtoken

import (
	"errors"
	"reflect"

	"github.com/xxcheng123/cloudpan189-share/internal/framework/httpcontext"
)

func (h *handler) ensureCloudTokenService(ctx *httpcontext.Context, busErr httpcontext.BusinessError) bool {
	if isNilDependency(h.cloudTokenService) {
		ctx.Fail(busErr.WithError(errors.New("云盘令牌服务未初始化")))

		return false
	}

	return true
}

func (h *handler) ensureMountPointService(ctx *httpcontext.Context, busErr httpcontext.BusinessError) bool {
	if isNilDependency(h.mountPointService) {
		ctx.Fail(busErr.WithError(errors.New("挂载点服务未初始化")))

		return false
	}

	return true
}

func (h *handler) hasUserMountPointTokenService() bool {
	return !isNilDependency(h.userMountPointTokenService)
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
