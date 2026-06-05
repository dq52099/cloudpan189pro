package storage

import (
	"errors"
	"reflect"

	"github.com/xxcheng123/cloudpan189-share/internal/framework/httpcontext"
)

func (h *handler) ensureTaskEngine(ctx *httpcontext.Context, busErr httpcontext.BusinessError) bool {
	if isNilDependency(h.taskEngine) {
		ctx.Fail(busErr.WithError(errors.New("任务引擎未初始化")))

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

func (h *handler) ensureVirtualFileService(ctx *httpcontext.Context, busErr httpcontext.BusinessError) bool {
	if isNilDependency(h.virtualFileService) {
		ctx.Fail(busErr.WithError(errors.New("虚拟文件服务未初始化")))

		return false
	}

	return true
}

func (h *handler) ensureMediaFileService(ctx *httpcontext.Context, busErr httpcontext.BusinessError) bool {
	if isNilDependency(h.mediaFileService) {
		ctx.Fail(busErr.WithError(errors.New("媒体文件服务未初始化")))

		return false
	}

	return true
}

func (h *handler) ensureStorageFacadeService(ctx *httpcontext.Context, busErr httpcontext.BusinessError) bool {
	if isNilDependency(h.storageFacadeService) {
		ctx.Fail(busErr.WithError(errors.New("存储组合服务未初始化")))

		return false
	}

	return true
}

func (h *handler) ensureGroup2FileService(ctx *httpcontext.Context, busErr httpcontext.BusinessError) bool {
	if isNilDependency(h.group2FileService) {
		ctx.Fail(busErr.WithError(errors.New("用户组绑定服务未初始化")))

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

func (h *handler) ensureUserMountPointTokenService(ctx *httpcontext.Context, busErr httpcontext.BusinessError) bool {
	if isNilDependency(h.userMountPointTokenService) {
		ctx.Fail(busErr.WithError(errors.New("用户挂载点令牌服务未初始化")))

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

func (h *handler) missingCloudBridgeBusinessError(busErr httpcontext.BusinessError) httpcontext.BusinessError {
	if isNilDependency(h.cloudBridgeService) {
		return busErr.WithError(errors.New("云盘接口服务未初始化"))
	}

	return nil
}

func (h *handler) missingCloudTokenBusinessError(busErr httpcontext.BusinessError) httpcontext.BusinessError {
	if isNilDependency(h.cloudTokenService) {
		return busErr.WithError(errors.New("云盘令牌服务未初始化"))
	}

	return nil
}

func (h *handler) hasTaskEngine() bool {
	return !isNilDependency(h.taskEngine)
}

func (h *handler) hasMountPointService() bool {
	return !isNilDependency(h.mountPointService)
}

func (h *handler) hasFileTaskLogService() bool {
	return !isNilDependency(h.fileTaskLogService)
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
