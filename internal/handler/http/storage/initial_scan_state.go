package storage

import (
	"fmt"

	"github.com/xxcheng123/cloudpan189-share/internal/framework/context"
	"go.uber.org/zap"
)

func (h *handler) markInitialScanFailed(ctx context.Context, fileID int64, reason string, err error) {
	if !h.hasMountPointService() {
		return
	}

	state := fmt.Sprintf("失败: 初始化扫描任务%s", reason)
	if err != nil {
		state = fmt.Sprintf("%s: %s", state, err.Error())
	}

	state = sanitizeStorageText(state)
	if updateErr := h.mountPointService.UpdateLastState(ctx, fileID, state); updateErr != nil {
		ctx.Warn("写入初始化扫描失败状态失败",
			zap.Int64("file_id", fileID),
			zap.String("state", state),
			zap.Error(updateErr))
	}
}
