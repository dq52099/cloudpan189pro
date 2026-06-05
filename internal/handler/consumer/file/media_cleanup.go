package file

import (
	appContext "github.com/xxcheng123/cloudpan189-share/internal/framework/context"
	"go.uber.org/zap"
)

func (h *handler) clearMediaEmptyDir(ctx appContext.Context, storagePath string) {
	if !h.hasMediaFileService() {
		ctx.Warn("媒体文件服务未初始化，跳过本地空目录清理")

		return
	}

	if err := h.mediaFileService.ClearEmptyDir(ctx, storagePath); err != nil {
		ctx.Warn("清理本地空目录失败", zap.Error(err))
	}
}
