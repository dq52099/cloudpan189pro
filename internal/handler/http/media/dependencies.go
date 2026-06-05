package media

import (
	"errors"
	"reflect"

	"github.com/xxcheng123/cloudpan189-share/internal/framework/httpcontext"
)

func (h *handler) ensureMediaConfigService(ctx *httpcontext.Context, busErr httpcontext.BusinessError) bool {
	if isNilDependency(h.mediaConfigService) {
		ctx.Fail(busErr.WithError(errors.New("媒体配置服务未初始化")))

		return false
	}

	return true
}

func (h *handler) ensureMountPointService(ctx *httpcontext.Context, busErr httpcontext.BusinessError) bool {
	if isNilDependency(h.mountpointService) {
		ctx.Fail(busErr.WithError(errors.New("挂载点服务未初始化")))

		return false
	}

	return true
}

func (h *handler) ensureTaskEngine(ctx *httpcontext.Context, busErr httpcontext.BusinessError) bool {
	if isNilDependency(h.taskEngine) {
		ctx.Fail(busErr.WithError(errors.New("任务引擎未初始化")))

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
