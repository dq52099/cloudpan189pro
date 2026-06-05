package subscription

import (
	"errors"
	"reflect"

	"github.com/xxcheng123/cloudpan189-share/internal/framework/httpcontext"
)

func (h *Handler) ensureStorageFacadeService(ctx *httpcontext.Context) bool {
	if isNilDependency(h.storageFacadeService) {
		ctx.Fail(invalidParams(errors.New("storage service is nil, please restart the application")))

		return false
	}

	return true
}

func (h *Handler) ensureCloudBridgeService(ctx *httpcontext.Context) bool {
	if isNilDependency(h.cloudBridgeService) {
		ctx.Fail(invalidParams(errors.New("cloud bridge service is nil, please restart the application")))

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
