package resource

import (
	"reflect"

	"go.uber.org/zap"
)

func (h *handler) hasMediaConfigService() bool {
	if isNilDependency(h.mediaConfigService) {
		h.warn("媒体配置服务未初始化", zap.String("handler", "resource"))

		return false
	}

	return true
}

func (h *handler) hasTaskEngine() bool {
	if isNilDependency(h.taskEngine) {
		h.warn("任务引擎未初始化", zap.String("handler", "resource"))

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
