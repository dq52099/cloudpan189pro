package media

import (
	"fmt"
	"strings"

	"github.com/xxcheng123/cloudpan189-share/internal/framework/taskcontext"
	"github.com/xxcheng123/cloudpan189-share/internal/shared"
	"go.uber.org/zap"
)

func (h *handler) Clear() taskcontext.HandlerFunc {
	return func(ctx *taskcontext.Context) error {
		logger := ctx.GetContext().Logger

		mediaConfig := shared.GetMediaConfig()
		if mediaConfig == nil {
			logger.Warn("媒体功能未配置，跳过清理")

			return nil
		}

		storagePath := mediaConfig.StoragePath

		// 基本验证
		if strings.TrimSpace(storagePath) == "" {
			logger.Error("媒体存储路径为空，无法执行清理操作")

			return fmt.Errorf("媒体存储路径为空")
		}

		if err := h.ensureMediaFileService(); err != nil {
			return err
		}

		logger.Info("开始清理媒体文件", zap.String("storage_path", storagePath))

		err := h.mediaFileService.Clear(ctx.GetContext(), storagePath)
		if err != nil {
			logger.Error("清理媒体文件失败", zap.Error(err))
		} else {
			logger.Info("清理媒体文件成功")
		}

		return err
	}
}
