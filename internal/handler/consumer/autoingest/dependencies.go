package autoingest

import (
	"reflect"

	appContext "github.com/xxcheng123/cloudpan189-share/internal/framework/context"
	autoingestType "github.com/xxcheng123/cloudpan189-share/internal/types/autoingest"
)

func (h *handler) createAutoIngestLog(ctx appContext.Context, planID int64, level autoingestType.LogLevel, content string) error {
	if isNilDependency(h.autoIngestLogService) {
		return nil
	}

	_, err := h.autoIngestLogService.Create(ctx, planID, level, content)

	return err
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
