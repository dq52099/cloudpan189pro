package file

import (
	"errors"
	"fmt"
	"time"

	"github.com/xxcheng123/cloudpan189-share/internal/consts"
	appContext "github.com/xxcheng123/cloudpan189-share/internal/framework/context"
	"github.com/xxcheng123/cloudpan189-share/internal/framework/taskcontext"
	"github.com/xxcheng123/cloudpan189-share/internal/pkgs/utils"
	"github.com/xxcheng123/cloudpan189-share/internal/repository/models"
	"github.com/xxcheng123/cloudpan189-share/internal/services/filetasklog"
	mountpointSvi "github.com/xxcheng123/cloudpan189-share/internal/services/mountpoint"
	"github.com/xxcheng123/cloudpan189-share/internal/shared"
	"github.com/xxcheng123/cloudpan189-share/internal/types/topic"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

var errDeleteTaskOwnerChanged = errors.New("删除任务归属已变化")

type deleteTaskAccess struct {
	userID           int64
	triggeredByAdmin bool
	enforced         bool
}

func newDeleteTaskAccess(userID int64, triggeredByAdmin bool) deleteTaskAccess {
	return deleteTaskAccess{
		userID:           userID,
		triggeredByAdmin: triggeredByAdmin,
		enforced:         userID > 0 || triggeredByAdmin,
	}
}

// HandleBatchDelete 后台排队删除处理逻辑
func (h *handler) HandleBatchDelete() taskcontext.HandlerFunc {
	return func(ctx *taskcontext.Context) (retErr error) {
		req := new(topic.FileBatchDeleteRequest)

		if err := ctx.Unmarshal(req); err != nil {
			h.logger.Error("解析删除任务失败", zap.Error(err))

			return err
		}

		requestIDs, err := normalizeFileTaskIDs(req.IDs)
		if err != nil {
			h.logger.Warn("批量删除任务 ID 非法", zap.Int64s("ids", req.IDs), zap.Error(err))

			return err
		}

		h.logger.Info("消费者开始处理批量删除", zap.Int("count", len(requestIDs)))

		access := newDeleteTaskAccess(req.ExpectedUserID, req.TriggeredByAdmin)

		// 获取父任务tracker（用于存储批量删除任务汇总进度）
		var parentTracker *filetasklog.Tracker

		if v := ctx.GetContext().Value(consts.CtxKeyTaskTracker); v != nil {
			if tracker, ok := v.(*filetasklog.Tracker); ok {
				parentTracker = tracker
			}
		}

		// 创建任务日志
		tracker, logErr := h.fileTaskLogService.Create(
			ctx.GetContext(),
			req.Topic().String(),
			fmt.Sprintf("批量删除 %d 个文件", len(requestIDs)),
			filetasklog.WithFile(requestIDs[0]),
			filetasklog.WithDesc(fmt.Sprintf("批量删除任务, 共 %d 个文件", len(requestIDs))),
		)
		if logErr != nil {
			h.logger.Error("创建任务日志失败", zap.Error(logErr))
		} else {
			_ = h.fileTaskLogService.Running(ctx.GetContext(), tracker)
			_ = h.fileTaskLogService.FlushCount(ctx.GetContext(), tracker, filetasklog.WithTotalCounter(len(requestIDs)))
		}

		var completedCount, failedCount int

		recordCompleted := func() {
			completedCount++

			if tracker != nil {
				_ = h.fileTaskLogService.FlushCount(ctx.GetContext(), tracker, filetasklog.WithCompletedOneCounter())
			}

			if parentTracker != nil {
				_ = h.fileTaskLogService.FlushCount(ctx.GetContext(), parentTracker, filetasklog.WithCompletedOneCounter())
				_ = h.fileTaskLogService.CompleteIfProgressDone(ctx.GetContext(), parentTracker, parentTracker.WithCost())
			}
		}

		recordFailed := func() {
			failedCount++

			if tracker != nil {
				_ = h.fileTaskLogService.FlushCount(ctx.GetContext(), tracker, filetasklog.WithFailedCounter(1))
			}

			if parentTracker != nil {
				_ = h.fileTaskLogService.FlushCount(ctx.GetContext(), parentTracker, filetasklog.WithFailedCounter(1))
				_ = h.fileTaskLogService.CompleteIfProgressDone(ctx.GetContext(), parentTracker, parentTracker.WithCost())
			}
		}

		defer func() {
			if tracker == nil {
				return
			}

			var err error
			if failedCount > 0 {
				err = h.fileTaskLogService.Failed(
					ctx.GetContext(),
					tracker,
					tracker.WithCost(),
					utils.WithField("result", fmt.Sprintf("批量删除部分失败: 成功 %d，失败 %d", completedCount, failedCount)),
				)
			} else {
				err = h.fileTaskLogService.Completed(ctx.GetContext(), tracker, tracker.WithCost())
			}

			if err != nil {
				if errors.Is(err, filetasklog.ErrFileTaskLogTerminalState) {
					h.logger.Warn("文件任务日志已处于终态，跳过批量删除状态回写", zap.Error(err))

					return
				}

				h.logger.Error("更新任务日志失败", zap.Error(err))
				retErr = errors.Join(retErr, err)
			}
		}()

		for _, id := range requestIDs {
			targetFileID := id
			fileInfo, fileErr := h.virtualFileService.Query(ctx.GetContext(), targetFileID)

			// 如果查不到文件信息，说明可能已经被删了，但仍尝试清理残留的子文件
			if fileErr != nil || fileInfo == nil {
				if fileErr != nil && !errors.Is(fileErr, gorm.ErrRecordNotFound) {
					h.logger.Error("查询待删除文件失败", zap.Int64("id", id), zap.Error(fileErr))
					recordFailed()

					continue
				}

				if err := h.ensureDeleteTaskMountPointOwner(ctx.GetContext(), id, access); err != nil {
					h.logger.Warn("删除任务归属校验失败，跳过", zap.Int64("id", id), zap.Error(err))
					recordFailed()

					continue
				}

				if err := h.clearMountFiles(ctx.GetContext(), targetFileID); err != nil {
					h.logger.Error("清理残留虚拟文件失败", zap.Int64("fid", targetFileID), zap.Error(err))
					recordFailed()

					continue
				}

				deleteReq := access.mountPointBatchDeleteRequest(id)
				if err := h.mountPointService.BatchDelete(ctx.GetContext(), deleteReq); err != nil {
					if !errors.Is(err, gorm.ErrRecordNotFound) {
						h.logger.Error("后台删除挂载点记录失败", zap.Int64("id", id), zap.Error(err))
						recordFailed()

						continue
					}

					h.logger.Debug("后台删除挂载点记录已不存在", zap.Int64("id", id), zap.Error(err))
				}

				recordCompleted()

				continue
			}

			// 记录父ID，用于稍后递归清理
			parentId := fileInfo.ParentId

			topID := fileInfo.TopId
			if topID <= 0 {
				topID = fileInfo.ID
			}

			if err := h.ensureDeleteTaskMountPointOwner(ctx.GetContext(), topID, access); err != nil {
				h.logger.Warn("删除任务归属校验失败，跳过", zap.Int64("id", id), zap.Int64("top_id", topID), zap.Error(err))
				recordFailed()

				continue
			}

			if err := h.deleteVirtualFileTree(ctx.GetContext(), fileInfo, access); err != nil {
				h.logger.Error("删除虚拟文件失败", zap.Int64("fid", targetFileID), zap.Error(err))
				recordFailed()
			} else {
				h.logger.Info("后台删除虚拟文件完成", zap.Int64("fid", targetFileID))
				// 4. 本地空目录清理 (确保 strm 删完后执行)
				mediaConfig := shared.GetMediaConfig()
				if mediaConfig != nil && mediaConfig.Enable {
					h.clearMediaEmptyDir(ctx.GetContext(), mediaConfig.StoragePath)
				}

				// 5. 手动清理数据库中的空祖先目录。
				h.cleanupEmptyAncestorFolders(ctx.GetContext(), parentId)
				recordCompleted()
			}

			time.Sleep(20 * time.Millisecond)
		}

		return nil
	}
}

// HandleDelete 单个文件删除处理逻辑
func (h *handler) HandleDelete() taskcontext.HandlerFunc {
	return func(ctx *taskcontext.Context) (handleErr error) {
		req := new(topic.FileDeleteRequest)

		if err := ctx.Unmarshal(req); err != nil {
			h.logger.Error("解析删除任务失败", zap.Error(err))

			return err
		}

		targetFileID := req.FileId
		if targetFileID <= 0 {
			h.logger.Warn("单文件删除任务 ID 非法", zap.Int64("file_id", targetFileID), zap.Error(errInvalidFileTaskID))

			return errInvalidFileTaskID
		}

		h.logger.Info("消费者开始处理单个文件删除", zap.Int64("file_id", targetFileID))

		access := newDeleteTaskAccess(req.ExpectedUserID, req.TriggeredByAdmin)

		// 获取父任务tracker（用于批量任务汇总进度）
		var parentTracker *filetasklog.Tracker

		if v := ctx.GetContext().Value(consts.CtxKeyTaskTracker); v != nil {
			if tracker, ok := v.(*filetasklog.Tracker); ok {
				parentTracker = tracker
			}
		}

		tracker, logErr := h.fileTaskLogService.Create(
			ctx.GetContext(),
			req.Topic().String(),
			fmt.Sprintf("删除文件: %d", targetFileID),
			filetasklog.WithFile(targetFileID),
			filetasklog.WithDesc(fmt.Sprintf("删除文件ID: %d", targetFileID)),
		)
		if logErr != nil {
			h.logger.Error("创建任务日志失败", zap.Error(logErr))
		} else {
			_ = h.fileTaskLogService.Running(ctx.GetContext(), tracker)
			_ = h.fileTaskLogService.FlushCount(ctx.GetContext(), tracker, filetasklog.WithTotalCounter(1))
		}

		defer func() {
			businessErr := handleErr

			if tracker != nil {
				var err error
				if handleErr != nil {
					err = h.fileTaskLogService.Failed(ctx.GetContext(), tracker, tracker.WithCost(), utils.WithField("result", handleErr.Error()))
				} else {
					err = h.fileTaskLogService.Completed(ctx.GetContext(), tracker, tracker.WithCost(), utils.WithField("completed", 1))
				}

				if err != nil {
					if errors.Is(err, filetasklog.ErrFileTaskLogTerminalState) {
						h.logger.Warn("文件任务日志已处于终态，跳过删除状态回写", zap.Error(err))

						err = nil
					}
				}

				if err != nil {
					h.logger.Error("更新任务日志失败", zap.Error(err))
					handleErr = errors.Join(handleErr, err)
				}
			}
			// 更新父任务进度
			if parentTracker != nil {
				if businessErr != nil {
					_ = h.fileTaskLogService.FlushCount(ctx.GetContext(), parentTracker, filetasklog.WithFailedCounter(1))
				} else {
					_ = h.fileTaskLogService.FlushCount(ctx.GetContext(), parentTracker, filetasklog.WithCompletedOneCounter())
				}

				_ = h.fileTaskLogService.CompleteIfProgressDone(ctx.GetContext(), parentTracker, parentTracker.WithCost())
			}
		}()

		fileInfo, fileErr := h.virtualFileService.Query(ctx.GetContext(), targetFileID)

		// 如果查不到文件信息，说明可能已经被删了，但仍尝试清理残留的子文件
		if fileErr != nil || fileInfo == nil {
			if fileErr != nil && !errors.Is(fileErr, gorm.ErrRecordNotFound) {
				h.logger.Error("查询待删除文件失败", zap.Int64("id", targetFileID), zap.Error(fileErr))

				return fileErr
			}

			if err := h.ensureDeleteTaskMountPointOwner(ctx.GetContext(), targetFileID, access); err != nil {
				h.logger.Warn("删除任务归属校验失败，跳过", zap.Int64("id", targetFileID), zap.Error(err))

				return err
			}

			if err := h.clearMountFiles(ctx.GetContext(), targetFileID); err != nil {
				h.logger.Error("清理残留虚拟文件失败", zap.Int64("fid", targetFileID), zap.Error(err))

				return err
			}

			deleteReq := access.mountPointBatchDeleteRequest(targetFileID)
			if err := h.mountPointService.BatchDelete(ctx.GetContext(), deleteReq); err != nil {
				if !errors.Is(err, gorm.ErrRecordNotFound) {
					h.logger.Error("后台删除挂载点记录失败", zap.Int64("id", targetFileID), zap.Error(err))

					return err
				}

				h.logger.Debug("后台删除挂载点记录已不存在", zap.Int64("id", targetFileID), zap.Error(err))
			}

			if tracker != nil {
				_ = h.fileTaskLogService.FlushCount(ctx.GetContext(), tracker, filetasklog.WithCompletedOneCounter())
			}

			return nil
		}

		// 记录父ID，用于稍后递归清理
		parentId := fileInfo.ParentId

		topID := fileInfo.TopId
		if topID <= 0 {
			topID = fileInfo.ID
		}

		if err := h.ensureDeleteTaskMountPointOwner(ctx.GetContext(), topID, access); err != nil {
			h.logger.Warn("删除任务归属校验失败，跳过", zap.Int64("id", targetFileID), zap.Int64("top_id", topID), zap.Error(err))

			return err
		}

		if err := h.deleteVirtualFileTree(ctx.GetContext(), fileInfo, access); err != nil {
			h.logger.Error("删除虚拟文件失败", zap.Int64("fid", targetFileID), zap.Error(err))

			return err
		}

		{
			if tracker != nil {
				_ = h.fileTaskLogService.FlushCount(ctx.GetContext(), tracker, filetasklog.WithCompletedOneCounter())
			}

			h.logger.Info("后台删除虚拟文件完成", zap.Int64("fid", targetFileID))
			// 4. 本地空目录清理 (确保 strm 删完后执行)
			mediaConfig := shared.GetMediaConfig()
			if mediaConfig != nil && mediaConfig.Enable {
				h.clearMediaEmptyDir(ctx.GetContext(), mediaConfig.StoragePath)
			}

			// 5. 手动清理数据库中的空祖先目录。
			h.cleanupEmptyAncestorFolders(ctx.GetContext(), parentId)
		}

		return nil
	}
}

func (h *handler) deleteVirtualFileTree(ctx appContext.Context, fileInfo *models.VirtualFile, access deleteTaskAccess) error {
	targetFileID := fileInfo.ID

	if fileInfo.IsTop || fileInfo.TopId == fileInfo.ID {
		// 清理挂载点下的子文件 (触发 deleteStrmIterator)
		if err := h.clearMountFiles(ctx, targetFileID); err != nil {
			h.logger.Error("清理挂载点子文件失败", zap.Int64("fid", targetFileID), zap.Error(err))

			return err
		}

		if err := h.virtualFileService.Delete(ctx, targetFileID); err != nil {
			h.logger.Error("删除根虚拟文件失败", zap.Int64("fid", targetFileID), zap.Error(err))

			return err
		}

		deleteReq := access.mountPointBatchDeleteRequest(targetFileID)
		if err := h.mountPointService.BatchDelete(ctx, deleteReq); err != nil {
			if !errors.Is(err, gorm.ErrRecordNotFound) {
				h.logger.Error("后台删除挂载点记录失败", zap.Int64("id", targetFileID), zap.Error(err))

				return err
			}

			h.logger.Debug("后台删除挂载点记录已不存在", zap.Int64("id", targetFileID), zap.Error(err))
		}

		return nil
	}

	if err := h.batchDeleteFiles(ctx, []*models.VirtualFile{fileInfo}); err != nil {
		h.logger.Error("递归删除虚拟文件失败", zap.Int64("fid", targetFileID), zap.Error(err))

		return err
	}

	return nil
}

func (a deleteTaskAccess) requiresOwnerCheck() bool {
	return a.enforced && !a.triggeredByAdmin
}

func (a deleteTaskAccess) mountPointBatchDeleteRequest(fileID int64) *mountpointSvi.BatchDeleteRequest {
	req := &mountpointSvi.BatchDeleteRequest{
		FileIds: []int64{fileID},
		IsAdmin: true,
	}

	if a.requiresOwnerCheck() {
		req.CreatorUserID = a.userID
		req.IsAdmin = false
	}

	return req
}

func (h *handler) ensureDeleteTaskMountPointOwner(ctx appContext.Context, fileID int64, access deleteTaskAccess) error {
	if !access.requiresOwnerCheck() {
		return nil
	}

	mountPoint, err := h.mountPointService.Query(ctx, fileID)
	if err != nil {
		return err
	}

	if mountPoint == nil {
		return gorm.ErrRecordNotFound
	}

	if mountPoint.CreatorUserID != access.userID {
		return errDeleteTaskOwnerChanged
	}

	return nil
}
