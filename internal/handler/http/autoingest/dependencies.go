package autoingest

import (
	"errors"
	"reflect"

	"github.com/xxcheng123/cloudpan189-share/internal/framework/httpcontext"
	"go.uber.org/zap"
)

func (h *handler) ensurePlanService(ctx *httpcontext.Context, busErr httpcontext.BusinessError) bool {
	if isNilDependency(h.planService) {
		ctx.Fail(busErr.WithError(errors.New("自动入库计划服务未初始化")))

		return false
	}

	return true
}

func (h *handler) ensureLogService(ctx *httpcontext.Context, busErr httpcontext.BusinessError) bool {
	if isNilDependency(h.logService) {
		ctx.Fail(busErr.WithError(errors.New("自动入库日志服务未初始化")))

		return false
	}

	return true
}

func (h *handler) ensureCloudBridgeService(ctx *httpcontext.Context, busErr httpcontext.BusinessError) bool {
	if isNilDependency(h.cloudBridgeService) {
		ctx.Fail(busErr.WithError(errors.New("云盘桥接服务未初始化")))

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

func (h *handler) hasLogService(ctx *httpcontext.Context) bool {
	if isNilDependency(h.logService) {
		ctx.GetContext().Warn("自动入库日志服务未初始化", zap.String("handler", "autoingest"))

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
