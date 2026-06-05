package file

import (
	"errors"
	"reflect"

	"github.com/xxcheng123/cloudpan189-share/internal/framework/httpcontext"
	"go.uber.org/zap"
)

func (h *handler) ensureVirtualFileService(ctx *httpcontext.Context, busErr httpcontext.BusinessError) bool {
	if isNilDependency(h.virtualFileService) {
		ctx.Fail(busErr.WithError(errors.New("虚拟文件服务未初始化")))

		return false
	}

	return true
}

func (h *handler) ensureVerifyService(ctx *httpcontext.Context, busErr httpcontext.BusinessError) bool {
	if isNilDependency(h.verifyService) {
		ctx.Fail(busErr.WithError(errors.New("文件签名服务未初始化")))

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

func (h *handler) ensureCloudTokenService(ctx *httpcontext.Context, busErr httpcontext.BusinessError) bool {
	if isNilDependency(h.cloudTokenService) {
		ctx.Fail(busErr.WithError(errors.New("云盘令牌服务未初始化")))

		return false
	}

	return true
}

func (h *handler) ensureCloudBridgeService(ctx *httpcontext.Context, busErr httpcontext.BusinessError) bool {
	if isNilDependency(h.cloudBridgeService) {
		ctx.Fail(busErr.WithError(errors.New("云盘接口服务未初始化")))

		return false
	}

	return true
}

func (h *handler) ensureTaskEngine(ctx *httpcontext.Context, busErr httpcontext.BusinessError) bool {
	if isNilDependency(h.taskEngine) {
		err := errors.New("任务引擎未初始化")
		ctx.GetContext().Error("推送文件批量删除任务失败", zap.Error(err))
		ctx.Fail(busErr.WithError(err))

		return false
	}

	return true
}

func (h *handler) hasGroup2FileService() bool {
	return !isNilDependency(h.group2FileService)
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
