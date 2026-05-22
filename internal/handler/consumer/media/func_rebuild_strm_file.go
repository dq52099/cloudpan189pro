package media

import (
	"errors"
	"fmt"
	"path"
	"slices"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/xxcheng123/cloudpan189-share/internal/framework/context"
	"github.com/xxcheng123/cloudpan189-share/internal/framework/taskcontext"
	"github.com/xxcheng123/cloudpan189-share/internal/pkgs/ptr"
	"github.com/xxcheng123/cloudpan189-share/internal/pkgs/utils"
	"github.com/xxcheng123/cloudpan189-share/internal/repository/models"
	filetasklogSvi "github.com/xxcheng123/cloudpan189-share/internal/services/filetasklog"
	"github.com/xxcheng123/cloudpan189-share/internal/services/mountpoint"
	"github.com/xxcheng123/cloudpan189-share/internal/services/virtualfile"
	"github.com/xxcheng123/cloudpan189-share/internal/shared"
	"github.com/xxcheng123/cloudpan189-share/internal/types/media"
	"github.com/xxcheng123/cloudpan189-share/internal/types/topic"
	"go.uber.org/zap"

	verifySvi "github.com/xxcheng123/cloudpan189-share/internal/services/verify"
)

// maxRecursionDepth walkBuildStrm 的最大递归深度，防止循环目录导致栈溢出
const maxRecursionDepth = 100

// rebuildConcurrency 全量重建时的挂载点级并发度。
const rebuildConcurrency = 4

var errInvalidRebuildMountPointFileID = errors.New("mountPointFileId 必须大于 0")

// rebuildProgress STRM 重建进度统计。
// 全部字段必须用 atomic.* 访问，避免 data race。
type rebuildProgress struct {
	totalFolders     int32
	processedFolders int32
	failedFolders    int32
	totalFiles       int32
	successFiles     int32
	failedFiles      int32
	skippedFiles     int32
}

// safeRate 计算百分比，分母为 0 时返回 100%（避免 NaN/+Inf）。
func safeRate(success, total int32) float64 {
	if total == 0 {
		return 100
	}

	return float64(success) / float64(total) * 100
}

func mergeRebuildProgress(total, part *rebuildProgress) {
	atomic.AddInt32(&total.failedFolders, atomic.LoadInt32(&part.failedFolders))
	atomic.AddInt32(&total.totalFiles, atomic.LoadInt32(&part.totalFiles))
	atomic.AddInt32(&total.successFiles, atomic.LoadInt32(&part.successFiles))
	atomic.AddInt32(&total.failedFiles, atomic.LoadInt32(&part.failedFiles))
	atomic.AddInt32(&total.skippedFiles, atomic.LoadInt32(&part.skippedFiles))
}

func rebuildHasFailures(progress *rebuildProgress) bool {
	return atomic.LoadInt32(&progress.failedFolders) > 0 ||
		atomic.LoadInt32(&progress.failedFiles) > 0
}

func finishRebuildTaskLog(ctx context.Context, logger *zap.Logger, service filetasklogSvi.Service, tracker *filetasklogSvi.Tracker, progress *rebuildProgress) error {
	if tracker == nil {
		return nil
	}

	summary := summariseProgress(progress)
	opts := []utils.Field{
		tracker.WithCost(),
		utils.WithField("result", summary),
	}

	if rebuildHasFailures(progress) {
		if err := service.Failed(ctx, tracker, opts...); err != nil {
			logger.Warn("标记任务失败失败", zap.Error(err))

			return err
		}

		return nil
	}

	if err := service.Completed(ctx, tracker, opts...); err != nil {
		logger.Warn("标记任务完成失败", zap.Error(err))

		return err
	}

	return nil
}

// RebuildStrmFile 全量重建 STRM：遍历所有挂载点。
// 支持并发执行，每个挂载点是一个独立 unit，最多 rebuildConcurrency 个同时跑。
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
			IsAdmin:    true,
		})
		if err != nil {
			logger.Error("查询挂载点失败", zap.Error(err))

			return err
		}

		if len(mountpoints) == 0 {
			logger.Warn("没有挂载点，跳过strm重建")

			return nil
		}

		tracker, logErr := h.fileTaskLogService.Create(
			ctx.GetContext(),
			topic.KeyMediaRebuildStrmFile,
			fmt.Sprintf("重建STRM文件，共 %d 个挂载点", len(mountpoints)),
			filetasklogSvi.WithDesc(fmt.Sprintf("存储路径: %s", shared.MediaConfig.StoragePath)),
		)
		if logErr != nil {
			logger.Error("创建任务日志失败", zap.Error(logErr))
		} else if tracker != nil {
			_ = h.fileTaskLogService.Running(ctx.GetContext(), tracker)
			_ = h.fileTaskLogService.FlushCount(ctx.GetContext(), tracker, filetasklogSvi.WithTotalCounter(len(mountpoints)))
		}

		progress := &rebuildProgress{
			totalFolders: int32(len(mountpoints)),
		}

		car := media.NewWriterCar(shared.MediaConfig.StoragePath, shared.MediaConfig.ConflictPolicy, shared.BaseURL)

		// 按挂载点并发执行
		var (
			wg  sync.WaitGroup
			sem = make(chan struct{}, rebuildConcurrency)
		)

		for _, mp := range mountpoints {
			sem <- struct{}{}

			wg.Add(1)

			go func(mp *models.MountPoint) {
				defer wg.Done()
				defer func() { <-sem }()

				atomic.AddInt32(&progress.processedFolders, 1)
				logger.Info(fmt.Sprintf(
					"[strm生成] 开始处理挂载点: %s (%d/%d)",
					mp.FullPath,
					atomic.LoadInt32(&progress.processedFolders),
					len(mountpoints),
				))

				mountProgress := new(rebuildProgress)
				h.walkBuildStrm(ctx.GetContext(), mp.FileId, car.NewSubCar(mp.FullPath), 0, mountProgress)
				mergeRebuildProgress(progress, mountProgress)

				if tracker != nil {
					if rebuildHasFailures(mountProgress) {
						_ = h.fileTaskLogService.FlushCount(ctx.GetContext(), tracker, filetasklogSvi.WithFailedCounter(1))
					} else {
						_ = h.fileTaskLogService.FlushCount(ctx.GetContext(), tracker, filetasklogSvi.WithCompletedOneCounter())
					}
				}
			}(mp)
		}

		wg.Wait()

		summary := summariseProgress(progress)
		logger.Info("[strm生成] strm重建完成: " + summary)

		return finishRebuildTaskLog(ctx.GetContext(), logger, h.fileTaskLogService, tracker, progress)
	}
}

// RebuildStrmFileByMountPoint 针对单个挂载点重建 STRM。
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

		if req.MountPointFileId <= 0 {
			logger.Error("STRM单挂载点重建任务参数无效", zap.Int64("mount_point_file_id", req.MountPointFileId))

			return errInvalidRebuildMountPointFileID
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
			_ = h.fileTaskLogService.Running(ctx.GetContext(), tracker)
			_ = h.fileTaskLogService.FlushCount(ctx.GetContext(), tracker, filetasklogSvi.WithTotalCounter(1))
		}

		progress := &rebuildProgress{totalFolders: 1}

		car := media.NewWriterCar(shared.MediaConfig.StoragePath, shared.MediaConfig.ConflictPolicy, shared.BaseURL)
		h.walkBuildStrm(ctx.GetContext(), req.MountPointFileId, car.NewSubCar(req.MountPointPath), 0, progress)

		summary := summariseProgress(progress)
		logger.Info("[strm生成] strm重建完成: " + summary)

		if tracker != nil {
			if rebuildHasFailures(progress) {
				_ = h.fileTaskLogService.FlushCount(ctx.GetContext(), tracker, filetasklogSvi.WithFailedCounter(1))
			} else {
				_ = h.fileTaskLogService.FlushCount(ctx.GetContext(), tracker, filetasklogSvi.WithCompletedOneCounter())
			}

			if err := finishRebuildTaskLog(ctx.GetContext(), logger, h.fileTaskLogService, tracker, progress); err != nil {
				return err
			}
		}

		return nil
	}
}

// walkBuildStrm 深度优先遍历目录，为每个命中 includedSuffixes 的文件生成 STRM。
//
// 返回值设计：失败不中断整个遍历（只记日志和 progress 计数），配合进度显示。
func (h *handler) walkBuildStrm(ctx context.Context, fid int64, car media.WriterCar, depth int, progress *rebuildProgress) {
	if depth >= maxRecursionDepth {
		ctx.Warn("达到最大递归深度，停止处理", zap.Int("depth", depth), zap.Int64("parent_id", fid))
		atomic.AddInt32(&progress.failedFolders, 1)

		return
	}

	files, err := h.virtualfileService.List(ctx, &virtualfile.ListRequest{
		ParentId: ptr.Of(fid),
	})
	if err != nil {
		ctx.Error("查询子文件失败", zap.Error(err), zap.Int64("parent_id", fid))
		atomic.AddInt32(&progress.failedFolders, 1)

		return
	}

	for _, file := range files {
		if file.IsDir {
			h.walkBuildStrm(ctx, file.ID, car.NewSubCar(file.Name), depth+1, progress)

			continue
		}

		atomic.AddInt32(&progress.totalFiles, 1)

		// 后缀过滤
		extName := path.Ext(file.Name)
		if len(shared.MediaConfig.IncludedSuffixes) > 0 && !slices.Contains(shared.MediaConfig.IncludedSuffixes, extName) {
			atomic.AddInt32(&progress.skippedFiles, 1)

			continue
		}

		filename := strings.TrimSuffix(file.Name, extName) + ".strm"

		values, err := h.verifyService.SignV1(ctx, file.ID, verifySvi.WithV1NoExpire())
		if err != nil {
			ctx.Error("重建strm - 获取文件签名失败", zap.Int64("file_id", file.ID), zap.Error(err))
			atomic.AddInt32(&progress.failedFiles, 1)

			continue
		}

		// Replace 策略：在外层统一删除旧记录，避免和 WriteStrm 内部重复删除
		if shared.MediaConfig.ConflictPolicy == media.FileConflictPolicyReplace {
			if err := h.mediaFileService.DeleteStrm(ctx, file.ID, shared.MediaConfig.StoragePath); err != nil {
				ctx.Warn("重建strm - 删除旧 strm 文件失败", zap.Int64("file_id", file.ID), zap.Error(err))
			}
		}

		carWithFilename := car.NewSubCar(filename)

		id, err := h.mediaFileService.WriteStrm(ctx, carWithFilename, file.ID, shared.JoinDownloadURL(file.ID, values))
		if err != nil {
			ctx.Error("重建strm - 创建 strm 文件失败", zap.String("file", file.Name), zap.Int64("file_id", file.ID), zap.Error(err))
			atomic.AddInt32(&progress.failedFiles, 1)

			continue
		}

		// Skip 策略下已存在的文件会返回 (0, nil)，记为 skipped 而非 success
		if id == 0 {
			atomic.AddInt32(&progress.skippedFiles, 1)
		} else {
			atomic.AddInt32(&progress.successFiles, 1)
		}
	}
}

// summariseProgress 组装进度摘要字符串，供任务日志 result 字段展示。
func summariseProgress(p *rebuildProgress) string {
	total := atomic.LoadInt32(&p.totalFiles)
	success := atomic.LoadInt32(&p.successFiles)
	failed := atomic.LoadInt32(&p.failedFiles)
	skipped := atomic.LoadInt32(&p.skippedFiles)
	folders := atomic.LoadInt32(&p.totalFolders)
	failedFolders := atomic.LoadInt32(&p.failedFolders)

	return fmt.Sprintf(
		"挂载点 %d 个 / 媒体文件 %d 个（成功 %d, 跳过 %d, 失败 %d, 目录失败 %d, 成功率 %.1f%%）",
		folders, total, success, skipped, failed, failedFolders, safeRate(success, total),
	)
}
