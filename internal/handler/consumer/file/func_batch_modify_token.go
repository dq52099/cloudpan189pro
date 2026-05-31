package file

import (
	"errors"
	"fmt"

	pkgErrors "github.com/pkg/errors"
	appContext "github.com/xxcheng123/cloudpan189-share/internal/framework/context"
	"github.com/xxcheng123/cloudpan189-share/internal/framework/taskcontext"
	"github.com/xxcheng123/cloudpan189-share/internal/pkgs/utils"
	"github.com/xxcheng123/cloudpan189-share/internal/repository/models"
	filetasklogSvi "github.com/xxcheng123/cloudpan189-share/internal/services/filetasklog"
	"github.com/xxcheng123/cloudpan189-share/internal/types/topic"
	"go.uber.org/zap"
)

func (h *handler) HandleBatchModifyToken() taskcontext.HandlerFunc {
	return func(ctx *taskcontext.Context) error {
		req := new(topic.FileBatchModifyTokenRequest)
		if err := ctx.Unmarshal(req); err != nil {
			return err
		}

		requestIDs, useMountPointIDs, err := normalizeBatchModifyTokenIDs(req)
		if err != nil {
			ctx.GetContext().Warn(
				"批量修改令牌任务 ID 非法",
				zap.Int64s("ids", req.IDs),
				zap.Int64s("mount_point_ids", req.MountPointIDs),
				zap.Error(err),
			)

			return err
		}

		taskName := "批量修改令牌"
		taskDesc := fmt.Sprintf("批量修改 %d 个挂载点的令牌，新令牌ID: %d", len(requestIDs), req.TokenID)

		taskLogFileID := requestIDs[0]
		if useMountPointIDs && len(req.IDs) > 0 {
			taskLogFileID = req.IDs[0]
		}

		tracker, logErr := h.fileTaskLogService.Create(
			ctx.GetContext(),
			taskName,
			taskDesc,
			filetasklogSvi.WithFile(taskLogFileID),
			filetasklogSvi.WithDesc(fmt.Sprintf("用户ID: %d, 管理员: %t", req.UserID, req.IsAdmin)),
		)
		if logErr != nil {
			return logErr
		}

		_ = h.fileTaskLogService.Running(ctx.GetContext(), tracker)

		successCount := 0
		failCount := 0
		unchangedCount := 0

		var groupFileIDs []int64

		if req.TokenID > 0 {
			if h.cloudTokenService == nil {
				return errors.New("云盘令牌服务未初始化")
			}

			if _, err = h.cloudTokenService.QueryAccessible(ctx.GetContext(), req.TokenID, req.UserID, req.IsAdmin); err != nil {
				if statusErr := h.fileTaskLogService.Failed(ctx.GetContext(), tracker, tracker.WithCost(), utils.WithField("result", pkgErrors.Wrap(err, "令牌不可用").Error())); statusErr != nil {
					if errors.Is(statusErr, filetasklogSvi.ErrFileTaskLogTerminalState) {
						ctx.GetContext().Warn("文件任务日志已处于终态，跳过批量修改令牌失败状态回写", zap.Error(statusErr))

						return nil
					}

					ctx.GetContext().Error("更新批量修改令牌任务失败状态失败", zap.Error(statusErr))

					return errors.Join(err, statusErr)
				}

				return nil
			}
		}

		if !req.IsAdmin && req.UserGroupID > 0 {
			var err error

			groupFileIDs, err = h.group2FileService.GetBindFiles(ctx.GetContext(), req.UserGroupID)
			if err != nil {
				if statusErr := h.fileTaskLogService.Failed(ctx.GetContext(), tracker, tracker.WithCost(), utils.WithField("result", err.Error())); statusErr != nil {
					if errors.Is(statusErr, filetasklogSvi.ErrFileTaskLogTerminalState) {
						ctx.GetContext().Warn("文件任务日志已处于终态，跳过批量修改令牌失败状态回写", zap.Error(statusErr))

						return err
					}

					ctx.GetContext().Error("更新批量修改令牌任务失败状态失败", zap.Error(statusErr))

					return errors.Join(err, statusErr)
				}

				return err
			}
		}

		for _, id := range requestIDs {
			_ = h.fileTaskLogService.FlushCount(ctx.GetContext(), tracker, filetasklogSvi.WithTotalCounter(1))

			mp, err := h.queryBatchModifyTokenMountPoint(ctx.GetContext(), id, useMountPointIDs)
			if err != nil {
				failCount++
				_ = h.fileTaskLogService.FlushCount(
					ctx.GetContext(),
					tracker,
					filetasklogSvi.WithCompletedOneCounter(),
					filetasklogSvi.WithFailedCounter(1),
				)
				ctx.GetContext().Warn("查询挂载点失败，跳过", zap.Int64("id", id), zap.Error(err))

				continue
			}

			if !req.IsAdmin {
				hasAccess := mp.CreatorUserID == req.UserID
				if !hasAccess {
					for _, fid := range groupFileIDs {
						if fid == mp.FileId {
							hasAccess = true

							break
						}
					}
				}

				if !hasAccess && h.userMountPointTokenService != nil {
					tokenID, err := h.userMountPointTokenService.GetTokenID(ctx.GetContext(), req.UserID, mp.ID)
					if err != nil {
						failCount++
						_ = h.fileTaskLogService.FlushCount(
							ctx.GetContext(),
							tracker,
							filetasklogSvi.WithCompletedOneCounter(),
							filetasklogSvi.WithFailedCounter(1),
						)
						ctx.GetContext().Warn("查询用户挂载点令牌绑定失败，跳过", zap.Int64("id", id), zap.Int64("user_id", req.UserID), zap.Error(err))

						continue
					}

					hasAccess = tokenID > 0
				}

				if !hasAccess {
					failCount++
					_ = h.fileTaskLogService.FlushCount(
						ctx.GetContext(),
						tracker,
						filetasklogSvi.WithCompletedOneCounter(),
						filetasklogSvi.WithFailedCounter(1),
					)
					ctx.GetContext().Warn("挂载点无权限修改，跳过", zap.Int64("id", id), zap.Int64("user_id", req.UserID))

					continue
				}
			}

			if req.TokenID == 0 {
				var deleted bool

				deleted, err = h.userMountPointTokenService.UnbindTokenWithResult(ctx.GetContext(), req.UserID, mp.ID)
				if err == nil && !deleted {
					unchangedCount++
					_ = h.fileTaskLogService.FlushCount(ctx.GetContext(), tracker, filetasklogSvi.WithCompletedOneCounter())

					continue
				}
			} else {
				err = h.userMountPointTokenService.BindToken(ctx.GetContext(), req.UserID, mp.ID, req.TokenID)
			}

			if err != nil {
				failCount++
				_ = h.fileTaskLogService.FlushCount(
					ctx.GetContext(),
					tracker,
					filetasklogSvi.WithCompletedOneCounter(),
					filetasklogSvi.WithFailedCounter(1),
				)
				ctx.GetContext().Warn("修改挂载点令牌失败，跳过", zap.Int64("id", id), zap.Error(err))

				continue
			}

			successCount++
			_ = h.fileTaskLogService.FlushCount(ctx.GetContext(), tracker, filetasklogSvi.WithCompletedOneCounter())
		}

		if failCount > 0 {
			err := errors.New(formatBatchModifyTokenResult(successCount, unchangedCount, failCount))
			if statusErr := h.fileTaskLogService.Failed(ctx.GetContext(), tracker, tracker.WithCost(), utils.WithField("result", err.Error())); statusErr != nil {
				if errors.Is(statusErr, filetasklogSvi.ErrFileTaskLogTerminalState) {
					ctx.GetContext().Warn("文件任务日志已处于终态，跳过批量修改令牌失败状态回写", zap.Error(statusErr))

					return nil
				}

				ctx.GetContext().Error("更新批量修改令牌任务失败状态失败", zap.Error(statusErr))

				return statusErr
			}

			return nil
		}

		if err := h.fileTaskLogService.Completed(
			ctx.GetContext(),
			tracker,
			tracker.WithCost(),
			utils.WithField("result", formatBatchModifyTokenResult(successCount, unchangedCount, failCount)),
		); err != nil {
			if errors.Is(err, filetasklogSvi.ErrFileTaskLogTerminalState) {
				ctx.GetContext().Warn("文件任务日志已处于终态，跳过批量修改令牌成功状态回写", zap.Error(err))

				return nil
			}

			return err
		}

		return nil
	}
}

func normalizeBatchModifyTokenIDs(req *topic.FileBatchModifyTokenRequest) ([]int64, bool, error) {
	if len(req.MountPointIDs) > 0 {
		ids, err := normalizeFileTaskIDs(req.MountPointIDs)

		return ids, true, err
	}

	ids, err := normalizeFileTaskIDs(req.IDs)

	return ids, false, err
}

func (h *handler) queryBatchModifyTokenMountPoint(ctx appContext.Context, id int64, useMountPointID bool) (*models.MountPoint, error) {
	if useMountPointID {
		return h.mountPointService.QueryByID(ctx, id)
	}

	return h.mountPointService.Query(ctx, id)
}

func formatBatchModifyTokenResult(successCount, unchangedCount, failCount int) string {
	if failCount > 0 {
		if unchangedCount > 0 {
			return fmt.Sprintf("批量修改令牌完成，成功 %d 个，已无绑定 %d 个，失败 %d 个", successCount, unchangedCount, failCount)
		}

		return fmt.Sprintf("批量修改令牌完成，成功 %d 个，失败 %d 个", successCount, failCount)
	}

	if unchangedCount > 0 {
		return fmt.Sprintf("批量修改令牌完成，成功 %d 个，已无绑定 %d 个", successCount, unchangedCount)
	}

	return fmt.Sprintf("批量修改令牌完成，成功 %d 个", successCount)
}
