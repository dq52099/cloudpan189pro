package media

import (
	"errors"
	"reflect"
)

var (
	errMediaFileServiceNotInitialized   = errors.New("媒体文件服务未初始化")
	errMountPointServiceNotInitialized  = errors.New("挂载点服务未初始化")
	errVirtualFileServiceNotInitialized = errors.New("虚拟文件服务未初始化")
	errVerifyServiceNotInitialized      = errors.New("签名服务未初始化")
	errFileTaskLogServiceNotInitialized = errors.New("文件任务日志服务未初始化")
)

func (h *handler) ensureMediaFileService() error {
	if isNilDependency(h.mediaFileService) {
		return errMediaFileServiceNotInitialized
	}

	return nil
}

func (h *handler) ensureRebuildStrmDependencies(requireMountPoint bool) error {
	if isNilDependency(h.mediaFileService) {
		return errMediaFileServiceNotInitialized
	}

	if requireMountPoint && isNilDependency(h.mountpointService) {
		return errMountPointServiceNotInitialized
	}

	if isNilDependency(h.virtualfileService) {
		return errVirtualFileServiceNotInitialized
	}

	if isNilDependency(h.verifyService) {
		return errVerifyServiceNotInitialized
	}

	if isNilDependency(h.fileTaskLogService) {
		return errFileTaskLogServiceNotInitialized
	}

	return nil
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
