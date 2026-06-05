package file

import (
	"errors"
	"reflect"
)

var (
	errVirtualFileServiceNotInitialized = errors.New("虚拟文件服务未初始化")
	errCloudBridgeServiceNotInitialized = errors.New("云盘桥接服务未初始化")
	errCloudTokenServiceNotInitialized  = errors.New("云盘令牌服务未初始化")
	errMountPointServiceNotInitialized  = errors.New("挂载点服务未初始化")
	errFileTaskLogServiceNotInitialized = errors.New("文件任务日志服务未初始化")
)

func (h *handler) ensureScanFileBaseDependencies() error {
	if !h.hasVirtualFileService() {
		return errVirtualFileServiceNotInitialized
	}

	return nil
}

func (h *handler) ensureCloudBridgeService() error {
	if !h.hasCloudBridgeService() {
		return errCloudBridgeServiceNotInitialized
	}

	return nil
}

func (h *handler) hasCloudTokenService() bool {
	return !isNilDependency(h.cloudTokenService)
}

func (h *handler) ensureCloudTokenService() error {
	if !h.hasCloudTokenService() {
		return errCloudTokenServiceNotInitialized
	}

	return nil
}

func (h *handler) hasCloudBridgeService() bool {
	return !isNilDependency(h.cloudBridgeService)
}

func (h *handler) hasFileTaskLogService() bool {
	return !isNilDependency(h.fileTaskLogService)
}

func (h *handler) ensureFileTaskLogService() error {
	if !h.hasFileTaskLogService() {
		return errFileTaskLogServiceNotInitialized
	}

	return nil
}

func (h *handler) hasMountPointService() bool {
	return !isNilDependency(h.mountPointService)
}

func (h *handler) hasMediaFileService() bool {
	return !isNilDependency(h.mediaFileService)
}

func (h *handler) ensureMountPointService() error {
	if !h.hasMountPointService() {
		return errMountPointServiceNotInitialized
	}

	return nil
}

func (h *handler) hasUserMountPointTokenService() bool {
	return !isNilDependency(h.userMountPointTokenService)
}

func (h *handler) hasVirtualFileService() bool {
	return !isNilDependency(h.virtualFileService)
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
