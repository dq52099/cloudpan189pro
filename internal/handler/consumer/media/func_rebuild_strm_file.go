package media

import (
	"fmt"
	"path"
	"slices"
	"strings"
	"sync"
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

type rebuildProgress struct {
	totalFolders     int32
	processedFolders int32
	totalFiles       int32
	successFiles     int32
	failedFiles      int32
	currentFolder    string
	mu               sync.Mutex
}

func (h *handler) RebuildStrmFile() taskcontext.HandlerFunc {
	return func(ctx *taskcontext.Context) error {
		var (
			logger = ctx.GetContext().Logger
		)

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
		} else {
			_ = h.fileTaskLogService.Running(ctx.GetContext(), tracker)
			_ = h.fileTaskLogService.FlushCount(ctx.GetContext(), tracker, filetasklogSvi.WithTotalCounter(len(mountpoints)))
		}

		progress := &rebuildProgress{
			totalFolders: int32(len(mountpoints)),
		}

		car := media.NewWriterCar(shared.MediaConfig.StoragePath, shared.MediaConfig.ConflictPolicy, shared.BaseURL)

		for idx, mountpoint := range mountpoints {
			progress.mu.Lock()
			progress.processedFolders = int32(idx + 1)
			progress.currentFolder = mountpoint.FullPath
			progress.mu.Unlock()

			rate := float64(progress.successFiles) / float64(progress.totalFiles) * 100
			if progress.totalFiles == 0 {
				rate = 100
			}
			logger.Info(fmt.Sprintf("[strm生成] 处理文件夹: %s (进度 %d/%d，成功率 %.1f%%)", mountpoint.FullPath, idx+1, len(mountpoints), rate))

			h.walkBuildStrm(ctx.GetContext(), mountpoint.FileId, car.NewSubCar(mountpoint.FullPath), 0, progress)

			if tracker != nil {
				_ = h.fileTaskLogService.FlushCount(ctx.GetContext(), tracker, filetasklogSvi.WithCompletedOneCounter())
			}
		}

		finalRate := float64(progress.successFiles) / float64(progress.totalFiles) * 100
		if progress.totalFiles == 0 {
			finalRate = 100
		}
		logger.Info(fmt.Sprintf("[strm生成] strm重建完成：共处理 %d 个文件夹，成功 %d 个文件，失败 %d 个文件，成功率 %.1f%%", progress.totalFolders, progress.successFiles, progress.failedFiles, finalRate))

		if tracker != nil {
			_ = h.fileTaskLogService.FlushCount(ctx.GetContext(), tracker, filetasklogSvi.WithCompletedOneCounter())
			_ = h.fileTaskLogService.Completed(ctx.GetContext(), tracker)
		}

		return nil
	}
}

func (h *handler) RebuildStrmFileByMountPoint() taskcontext.HandlerFunc {
	return func(ctx *taskcontext.Context) error {
		var (
			logger = ctx.GetContext().Logger
		)

		if shared.MediaConfig == nil || !shared.MediaConfig.Enable {
			logger.Warn("媒体功能未启用，跳过strm重建")
			return nil
		}

		req := new(topic.MediaRebuildStrmFileByMountPointRequest)
		if err := ctx.Unmarshal(req); err != nil {
			logger.Error("解析STRM重建任务失败", zap.Error(err))
			return nil
		}

		logger.Info("开始重建strm文件", zap.Int64("mount_point_file_id", req.MountPointFileId), zap.String("path", req.MountPointPath))

		tracker, logErr := h.fileTaskLogService.Create(
			ctx.GetContext(),
			topic.KeyMediaRebuildStrmFileByMountPoint,
			fmt.Sprintf("STRM重建: %s", req.MountPointPath),
			filetasklogSvi.WithFile(req.MountPointFileId),
			filetasklogSvi.WithDesc(fmt.Sprintf("挂载点路径: %s", req.MountPointPath)),
		)
		if logErr != nil {
			logger.Error("创建任务日志失败", zap.Error(logErr))
		} else {
			_ = h.fileTaskLogService.Running(ctx.GetContext(), tracker)
			_ = h.fileTaskLogService.FlushCount(ctx.GetContext(), tracker, filetasklogSvi.WithTotalCounter(1))
		}

		progress := &rebuildProgress{
			totalFolders: 1,
		}

		car := media.NewWriterCar(shared.MediaConfig.StoragePath, shared.MediaConfig.ConflictPolicy, shared.BaseURL)
		h.walkBuildStrm(ctx.GetContext(), req.MountPointFileId, car.NewSubCar(req.MountPointPath), 0, progress)

		finalRate := float64(progress.successFiles) / float64(progress.totalFiles) * 100
		if progress.totalFiles == 0 {
			finalRate = 100
		}
		logger.Info(fmt.Sprintf("[strm生成] strm重建完成：成功 %d 个文件，失败 %d 个文件，成功率 %.1f%%", progress.successFiles, progress.failedFiles, finalRate))

		if tracker != nil {
			_ = h.fileTaskLogService.FlushCount(ctx.GetContext(), tracker, filetasklogSvi.WithCompletedOneCounter())
			_ = h.fileTaskLogService.Completed(ctx.GetContext(), tracker)
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

		if shared.MediaConfig.ConflictPolicy == "replace" {
			_ = h.mediaFileService.DeleteStrm(ctx, file.ID, shared.MediaConfig.StoragePath)
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

func (h *handler) ForceRebuildStrmFile() taskcontext.HandlerFunc {
	return func(ctx *taskcontext.Context) error {
		var (
			logger = ctx.GetContext().Logger
		)

		if shared.MediaConfig == nil || !shared.MediaConfig.Enable {
			logger.Warn("媒体功能未启用，跳过强制strm重建")
			return nil
		}

		logger.Info("开始强制重建strm文件（覆盖模式）", zap.String("storage_path", shared.MediaConfig.StoragePath))

		mountpoints, err := h.mountpointService.List(ctx.GetContext(), &mountpoint.ListRequest{
			NoPaginate: true,
		})
		if err != nil {
			logger.Error("查询挂载点失败", zap.Error(err))
			return err
		}

		if len(mountpoints) == 0 {
			logger.Warn("没有挂载点，跳过强制strm重建")
			return nil
		}

		logger.Info(fmt.Sprintf("[strm生成] 开始强制重建 strm，共 %d 个挂载文件夹（根路径: %s）", len(mountpoints), shared.MediaConfig.StoragePath))

		progress := &rebuildProgress{
			totalFolders: int32(len(mountpoints)),
		}

		originalPolicy := shared.MediaConfig.ConflictPolicy
		shared.MediaConfig.ConflictPolicy = "replace"

		car := media.NewWriterCar(shared.MediaConfig.StoragePath, "replace", shared.BaseURL)

		for idx, mountpoint := range mountpoints {
			progress.mu.Lock()
			progress.processedFolders = int32(idx + 1)
			progress.currentFolder = mountpoint.FullPath
			progress.mu.Unlock()

			rate := float64(progress.successFiles) / float64(progress.totalFiles) * 100
			if progress.totalFiles == 0 {
				rate = 100
			}
			logger.Info(fmt.Sprintf("[strm生成] 强制重建 - 处理文件夹: %s (进度 %d/%d，成功率 %.1f%%)", mountpoint.FullPath, idx+1, len(mountpoints), rate))

			h.walkBuildStrm(ctx.GetContext(), mountpoint.FileId, car.NewSubCar(mountpoint.FullPath), 0, progress)
		}

		shared.MediaConfig.ConflictPolicy = originalPolicy

		finalRate := float64(progress.successFiles) / float64(progress.totalFiles) * 100
		if progress.totalFiles == 0 {
			finalRate = 100
		}
		logger.Info(fmt.Sprintf("[strm生成] 强制strm重建完成：共处理 %d 个文件夹，成功 %d 个文件，失败 %d 个文件，成功率 %.1f%%", progress.totalFolders, progress.successFiles, progress.failedFiles, finalRate))

		return nil
	}
}

func (h *handler) rebuildStrmByFileIds(ctx context.Context, fileIds []int64, car media.WriterCar, progress *rebuildProgress) {
	for _, fid := range fileIds {
		file, err := h.virtualfileService.Query(ctx, fid)
		if err != nil {
			ctx.Error("重建strm - 查询文件失败", zap.Int64("file_id", fid), zap.Error(err))
			atomic.AddInt32(&progress.failedFiles, 1)
			continue
		}

		if file.IsDir {
			continue
		}

		atomic.AddInt32(&progress.totalFiles, 1)

		extName := path.Ext(file.Name)
		if len(shared.MediaConfig.IncludedSuffixes) > 0 && !slices.Contains(shared.MediaConfig.IncludedSuffixes, extName) {
			continue
		}

		filename := strings.TrimSuffix(file.Name, extName) + ".strm"

		values, err := h.verifyService.SignV1(ctx, file.ID, verifySvi.WithV1NoExpire())
		if err != nil {
			ctx.Error("重建strm - 获取文件签名失败", zap.Int64("file_id", file.ID), zap.Error(err))
			atomic.AddInt32(&progress.failedFiles, 1)
			continue
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
