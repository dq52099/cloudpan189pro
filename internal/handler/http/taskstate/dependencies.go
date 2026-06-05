package taskstate

import (
	"errors"
	"reflect"

	"github.com/xxcheng123/cloudpan189-share/internal/framework/httpcontext"
)

func (h *handler) ensureTaskEngine(ctx *httpcontext.Context) bool {
	if isNilDependency(h.taskEngine) {
		ctx.Fail(codeGetTaskEngineStatsFailed.WithError(errors.New("任务引擎未初始化")))

		return false
	}

	return true
}

func (h *handler) ensureFileTaskLogService(ctx *httpcontext.Context, busErr httpcontext.BusinessError) bool {
	if isNilDependency(h.fileTaskLogService) {
		ctx.Fail(busErr.WithError(errors.New("文件任务日志服务未初始化")))

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
