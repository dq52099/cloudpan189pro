package media

import (
	"fmt"
	"path"
	"slices"
	"strings"
	"sync/atomic"

	"github.com/xxcheng123/cloudpan189-share/internal/framework/context"
	"github.com/xxcheng123/cloudpan189-share/internal/framework/taskcontext"
	"github.com/xxcheng123/cloudpan189-share/internal/pkgs/ptr"
	filetasklogSvi "github.com/xxcheng123/cloudpan189-share/internal/services/filetasklog"
	"github.com/xxcheng123/cloudpan189-share/internal/services/mountpoint"
	"github.com/xxcheng123/cloudpan189-share/internal/services/virtualfile"
	"github.com/xxcheng123/cloudpan189-share/internal/shared"
	"github.com/xxcheng123/cloudpan189-share/internal/types/media"
	"github.com/xxcheng123/cloudpan189-share/internal/types/topic"
	"go.uber.org/zap"

	verifySvi "github.com/xxcheng123/cloudpan189-share/internal/services/verify"
)

const maxRecursionDepth = 100

// rebuildProgress STRM 重建进度统计。
// 字段必须用 atomic.AddInt32/LoadInt32 访问，避免 data race。
type rebuildProgress struct {
	totalFolders     int32
	processedFolders int32
	totalFiles       int32
	successFiles     int32
	failedFiles      int32
}

// safeRate 计算百分比，分母为 0 时返回 100%（避免 NaN/+Inf）。
func safeRate(success, total int32) float64 {
	if total == 0 {
		return 100
	}

	return float64(success) / float64(total) * 100
}

func (h *handler) RebuildStrmFile() taskcontext.HandlerFunc {
	return func(ctx *taskcontext.Context) error {
		logger := ctx.GetContext().Logger

		if shared.MediaConfig == nil || !shared.MediaConfig.Enable {
			logger.Warn("媒体功能未启用，跳过strm重建")
			return nil
		}

		logger.Info("开始重建strm文件", zap.String("storage_path", shared.MediaConfig.StoragePath))

		mountpoints, err := h.mountpointService.List(ctx.GetContext(), &mountpoint.ListRequest{
			NoPaginate: true,
		})
		if err != nil {
			logger.Error("查询挂载点失败", zap.Error(err))

			return err
		}

		if len(mountpoints) == 0 {
			logger.Warn("没有挂载点，跳过strm重建")
			return nil
		}

		logger.Info(fmt.Sprintf("[strm生成] 开始重建 strm，共 %d 个挂载文件夹（根路径: %s）", len(mountpoints), shared.MediaConfig.StoragePath))

		tracker, logErr := h.fileTaskLogService.Create(
			ctx.GetContext(),
			topic.KeyMediaRebuildStrmFile,
			fmt.Sprintf("重建STRM文件，共 %d 个挂载点", len(mountpoints)),
			filetasklogSvi.WithDesc(fmt.Sprintf("存储路径: %s", shared.MediaConfig.StoragePath)),
		)
		if logErr != nil {
			logger.Error("创建任务日志失败", zap.Error(logErr))
		} else if tracker != nil {
			if err := h.fileTaskLogService.Running(ctx.GetContext(), tracker); err != nil {
				logger.Warn("标记任务运行中失败", zap.Error(err))
			}
			if err := h.fileTaskLogService.FlushCount(ctx.GetContext(), tracker, filetasklogSvi.WithTotalCounter(len(mountpoints))); err != nil {
				logger.Warn("刷新任务总数失败", zap.Error(err))
			}
		}

		progress := &rebuildProgress{
			totalFolders: int32(len(mountpoints)),
		}

		car := media.NewWriterCar(shared.MediaConfig.StoragePath, shared.MediaConfig.ConflictPolicy, shared.BaseURL)

		for idx, mp := range mountpoints {
			atomic.StoreInt32(&progress.processedFolders, int32(idx+1))

			successFiles := atomic.LoadInt32(&progress.successFiles)
			totalFiles := atomic.LoadInt32(&progress.totalFiles)
			logger.Info(fmt.Sprintf(
				"[strm生成] 处理文件夹: %s (进度 %d/%d，成功率 %.1f%%)",
				mp.FullPath, idx+1, len(mountpoints), safeRate(successFiles, totalFiles),
			))

			h.walkBuildStrm(ctx.GetContext(), mp.FileId, car.NewSubCar(mp.FullPath), 0, progress)

			// 每处理完一个挂载点，Completed 计数 +1
			if tracker != nil {
				if err := h.fileTaskLogService.FlushCount(ctx.GetContext(), tracker, filetasklogSvi.WithCompletedOneCounter()); err != nil {
					logger.Warn("刷新完成计数失败", zap.Error(err))
				}
			}
		}

		finalSuccess := atomic.LoadInt32(&progress.successFiles)
		finalFailed := atomic.LoadInt32(&progress.failedFiles)
		finalTotal := atomic.LoadInt32(&progress.totalFiles)
		logger.Info(fmt.Sprintf(
			"[strm生成] strm重建完成：共处理 %d 个文件夹，成功 %d 个文件，失败 %d 个文件，成功率 %.1f%%",
			progress.totalFolders, finalSuccess, finalFailed, safeRate(finalSuccess, finalTotal),
		))

		if tracker != nil {
			if err := h.fileTaskLogService.Completed(ctx.GetContext(), tracker); err != nil {
				logger.Warn("标记任务完成失败", zap.Error(err))
			}
		}

		return nil
	}
}

func (h *handler) RebuildStrmFileByMountPoint() taskcontext.HandlerFunc {
	return func(ctx *taskcontext.Context) error {
		logger := ctx.GetContext().Logger

		if shared.MediaConfig == nil || !shared.MediaConfig.Enable {
			logger.Warn("媒体功能未启用，跳过strm重建")
			return nil
		}

		req := new(topic.MediaRebuildStrmFileByMountPointRequest)
		if err := ctx.Unmarshal(req); err != nil {
			logger.Error("解析STRM重建任务失败", zap.Error(err))
			return err
		}

		logger.Info("开始重建strm文件",
			zap.Int64("mount_point_file_id", req.MountPointFileId),
			zap.String("path", req.MountPointPath),
		)

		tracker, logErr := h.fileTaskLogService.Create(
			ctx.GetContext(),
			topic.KeyMediaRebuildStrmFileByMountPoint,
			fmt.Sprintf("STRM重建: %s", req.MountPointPath),
			filetasklogSvi.WithFile(req.MountPointFileId),
			filetasklogSvi.WithDesc(fmt.Sprintf("挂载点路径: %s", req.MountPointPath)),
		)
		if logErr != nil {
			logger.Error("创建任务日志失败", zap.Error(logErr))
		} else if tracker != nil {
			if err := h.fileTaskLogService.Running(ctx.GetContext(), tracker); err != nil {
				logger.Warn("标记任务运行中失败", zap.Error(err))
			}
			if err := h.fileTaskLogService.FlushCount(ctx.GetContext(), tracker, filetasklogSvi.WithTotalCounter(1)); err != nil {
				logger.Warn("刷新任务总数失败", zap.Error(err))
			}
		}

		progress := &rebuildProgress{
			totalFolders: 1,
		}

		car := media.NewWriterCar(shared.MediaConfig.StoragePath, shared.MediaConfig.ConflictPolicy, shared.BaseURL)
		h.walkBuildStrm(ctx.GetContext(), req.MountPointFileId, car.NewSubCar(req.MountPointPath), 0, progress)

		finalSuccess := atomic.LoadInt32(&progress.successFiles)
		finalFailed := atomic.LoadInt32(&progress.failedFiles)
		finalTotal := atomic.LoadInt32(&progress.totalFiles)
		logger.Info(fmt.Sprintf(
			"[strm生成] strm重建完成：成功 %d 个文件，失败 %d 个文件，成功率 %.1f%%",
			finalSuccess, finalFailed, safeRate(finalSuccess, finalTotal),
		))

		if tracker != nil {
			if err := h.fileTaskLogService.FlushCount(ctx.GetContext(), tracker, filetasklogSvi.WithCompletedOneCounter()); err != nil {
				logger.Warn("刷新完成计数失败", zap.Error(err))
			}
			if err := h.fileTaskLogService.Completed(ctx.GetContext(), tracker); err != nil {
				logger.Warn("标记任务完成失败", zap.Error(err))
			}
		}

		return nil
	}
}

func (h *handler) walkBuildStrm(ctx context.Context, fid int64, car media.WriterCar, depth int, progress *rebuildProgress) {
	if depth >= maxRecursionDepth {
		ctx.Warn("达到最大递归深度，停止处理", zap.Int("depth", depth), zap.Int64("parent_id", fid))

		return
	}

	files, err := h.virtualfileService.List(ctx, &virtualfile.ListRequest{
		ParentId: ptr.Of(fid),
	})
	if err != nil {
		ctx.Error("查询子文件失败", zap.Error(err), zap.Int64("parent_id", fid))

		return
	}

	for _, file := range files {
		if file.IsDir {
			h.walkBuildStrm(ctx, file.ID, car.NewSubCar(file.Name), depth+1, progress)
			continue
		}

		atomic.AddInt32(&progress.totalFiles, 1)

		extName := path.Ext(file.Name)
		if len(shared.MediaConfig.IncludedSuffixes) > 0 && !slices.Contains(shared.MediaConfig.IncludedSuffixes, extName) {
			ctx.Debug("重建strm - 跳过文件（非媒体格式）", zap.String("file_name", file.Name))

			continue
		}

		filename := strings.TrimSuffix(file.Name, extName) + ".strm"

		values, err := h.verifyService.SignV1(ctx, file.ID, verifySvi.WithV1NoExpire())
		if err != nil {
			ctx.Error("重建strm - 获取文件签名失败", zap.Int64("file_id", file.ID), zap.Error(err))
			atomic.AddInt32(&progress.failedFiles, 1)
			continue
		}

		if shared.MediaConfig.ConflictPolicy == media.FileConflictPolicyReplace {
			if err := h.mediaFileService.DeleteStrm(ctx, file.ID, shared.MediaConfig.StoragePath); err != nil {
				ctx.Warn("重建strm - 删除旧 strm 文件失败", zap.Int64("file_id", file.ID), zap.Error(err))
			}
		}

		carWithFilename := car.NewSubCar(filename)
		if id, err := h.mediaFileService.WriteStrm(ctx, carWithFilename, file.ID, shared.JoinDownloadURL(file.ID, values)); err != nil {
			ctx.Error("重建strm - 创建 strm 文件失败", zap.String("file", file.Name), zap.Int64("file_id", file.ID), zap.Error(err))
			atomic.AddInt32(&progress.failedFiles, 1)
		} else {
			ctx.Debug("重建strm - 创建 strm 文件成功", zap.String("file", file.Name), zap.Int64("file_id", file.ID), zap.Int64("media_file_id", id))
			atomic.AddInt32(&progress.successFiles, 1)
		}
	}
}
