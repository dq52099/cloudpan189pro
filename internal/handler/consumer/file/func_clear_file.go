package file

import (
	"errors"
	"fmt"

	"github.com/xxcheng123/cloudpan189-share/internal/consts"
	"github.com/xxcheng123/cloudpan189-share/internal/framework/taskcontext"
	"github.com/xxcheng123/cloudpan189-share/internal/pkgs/utils"
	"github.com/xxcheng123/cloudpan189-share/internal/services/filetasklog"
	"github.com/xxcheng123/cloudpan189-share/internal/shared"
	"github.com/xxcheng123/cloudpan189-share/internal/types/topic"
	"go.uber.org/zap"
)

func (h *handler) ClearFile() taskcontext.HandlerFunc {
	return func(ctx *taskcontext.Context) error {
		req := new(topic.FileClearFileRequest)

		if err := ctx.Unmarshal(req); err != nil {
			return err
		}

		var (
			logger = ctx.GetContext().Logger
		)

		vf, err := h.virtualFileService.Query(ctx.GetContext(), req.FileId)
		if err != nil {
			logger.Error("查询文件失败", zap.Error(err), zap.Int64("file_id", req.FileId))

			return err
		}

		logger.Debug("开始清理文件", zap.Int64("file_id", vf.ID))

		tracker, logErr := h.fileTaskLogService.Create(
			ctx.GetContext(),
			req.Topic().String(),
			fmt.Sprintf("清空目录: %s", ctx.GetContext().String(consts.CtxKeyFullPath, "unknown")),
			filetasklog.WithFile(req.FileId),
			filetasklog.WithDesc(fmt.Sprintf(
				"调用者: %s, 文件ID: %d, 目录名: %s, 上级ID: %d, 挂载点ID: %d",
				ctx.GetContext().String(consts.CtxKeyInvokeHandlerName, "unknown"),
				req.FileId,
				vf.Name,
				vf.ParentId,
				vf.TopId,
			)),
		)
		if logErr != nil {
			logger.Error("创建文件任务日志失败", zap.Int64("file_id", req.FileId), zap.Error(logErr))

			return logErr
		}

		_ = h.fileTaskLogService.Running(ctx.GetContext(), tracker)

		_ = h.fileTaskLogService.FlushCount(ctx.GetContext(), tracker, filetasklog.WithTotalCounter(1))

		if shared.MediaConfig != nil && shared.MediaConfig.Enable && shared.MediaConfig.AutoClean {
			defer func() {
				_ = h.mediaFileService.ClearEmptyDir(ctx.GetContext(), shared.MediaConfig.StoragePath)
			}()
		}

		if err = h.clearMountFiles(ctx.GetContext(), vf.ID); err != nil {
			_ = h.fileTaskLogService.FlushCount(ctx.GetContext(), tracker, filetasklog.WithFailedCounter(1))
			if statusErr := h.fileTaskLogService.Failed(ctx.GetContext(), tracker, tracker.WithCost(), utils.WithField("result", err.Error())); statusErr != nil {
				if errors.Is(statusErr, filetasklog.ErrFileTaskLogTerminalState) {
					logger.Warn("文件任务日志已处于终态，跳过清理失败状态回写", zap.Int64("file_id", req.FileId), zap.Error(statusErr))

					return err
				}

				logger.Error("更新文件任务日志失败", zap.Int64("file_id", req.FileId), zap.Error(statusErr))

				return errors.Join(err, statusErr)
			}

			return err
		} else if err := h.fileTaskLogService.Completed(ctx.GetContext(), tracker, tracker.WithCost()); err != nil {
			if errors.Is(err, filetasklog.ErrFileTaskLogTerminalState) {
				logger.Warn("文件任务日志已处于终态，跳过清理成功状态回写", zap.Int64("file_id", req.FileId), zap.Error(err))

				return nil
			}

			logger.Error("更新文件任务日志失败", zap.Int64("file_id", req.FileId), zap.Error(err))

			return err
		}

		return err
	}
}
