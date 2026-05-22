package storage

import (
	"errors"
	"io"
	"strings"

	"github.com/xxcheng123/cloudpan189-share/internal/framework/httpcontext"
	"github.com/xxcheng123/cloudpan189-share/internal/shared"
	"go.uber.org/zap"
)

type ClearAllRequest struct {
	DeleteFiles bool `json:"deleteFiles"` // 是否同时删除本地媒体文件
}

// ClearAll 清空所有挂载点
// @Summary 清空所有挂载点
// @Description 删除所有存储挂载点（仅管理员）。此操作不可逆，会顺序清理 MountPoint/VirtualFile/MediaFile 表；
// deleteFiles=true 时会同时清理媒体配置里的本地媒体目录。
// 中间步骤失败时会尽力打印日志，但不会自动回滚已删除部分。
// @Tags 存储管理
// @Accept json
// @Produce json
// @Param Authorization header string true "Bearer token"
// @Param request body ClearAllRequest false "清空请求参数"
// @Success 200 {object} httpcontext.Response "清空成功"
// @Failure 400 {object} httpcontext.Response "参数验证失败"
// @Failure 401 {object} httpcontext.Response "未授权访问"
// @Failure 403 {object} httpcontext.Response "权限不足"
// @Router /api/storage/clear_all [post]
func (h *handler) ClearAll() httpcontext.HandlerFunc {
	return func(ctx *httpcontext.Context) {
		var req ClearAllRequest

		if err := ctx.ShouldBindJSON(&req); err != nil && !errors.Is(err, io.EOF) {
			ctx.AbortWithInvalidParams(err)

			return
		}

		mediaStoragePath := ""

		if req.DeleteFiles {
			if shared.MediaConfig == nil || strings.TrimSpace(shared.MediaConfig.StoragePath) == "" {
				ctx.AbortWithInvalidParams(errors.New("媒体存储路径未配置，无法删除本地媒体文件"))

				return
			}

			mediaStoragePath = shared.MediaConfig.StoragePath
		}

		count, err := h.mountPointService.ClearAll(ctx.GetContext())
		if err != nil {
			ctx.GetContext().Error("清空挂载点失败", zap.Error(err))
			ctx.Fail(busCodeStorageMountPointDeleteFail.WithError(err))

			return
		}

		if err := h.virtualFileService.ClearAll(ctx.GetContext()); err != nil {
			// MountPoint 已删除，尽力继续，但在响应里暴露错误
			ctx.GetContext().Error("清空虚拟文件失败（挂载点已清空）", zap.Error(err))
			ctx.Fail(busCodeStorageMountPointDeleteFail.WithError(err))

			return
		}

		if req.DeleteFiles {
			err = h.mediaFileService.Clear(ctx.GetContext(), mediaStoragePath)
		} else {
			err = h.mediaFileService.ClearAll(ctx.GetContext())
		}

		if err != nil {
			ctx.GetContext().Error("清空媒体文件失败（挂载点/虚拟文件已清空）", zap.Error(err))
			ctx.Fail(busCodeStorageMountPointDeleteFail.WithError(err))

			return
		}

		ctx.GetContext().Info("已清空全部挂载点", zap.Int64("count", count))
		ctx.Success(count)
	}
}
