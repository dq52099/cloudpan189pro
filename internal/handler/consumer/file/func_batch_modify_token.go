package file

import (
	"fmt"

	"github.com/pkg/errors"
	"github.com/xxcheng123/cloudpan189-share/internal/framework/taskcontext"
	"github.com/xxcheng123/cloudpan189-share/internal/pkgs/utils"
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

		taskName := "批量修改令牌"
		taskDesc := fmt.Sprintf("批量修改 %d 个挂载点的令牌，新令牌ID: %d", len(req.IDs), req.TokenID)

		tracker, logErr := h.fileTaskLogService.Create(
			ctx.GetContext(),
			taskName,
			taskDesc,
			filetasklogSvi.WithFile(req.IDs[0]),
			filetasklogSvi.WithDesc(fmt.Sprintf("用户ID: %d, 管理员: %t", req.UserID, req.IsAdmin)),
		)
		if logErr != nil {
			return logErr
		}

		_ = h.fileTaskLogService.Running(ctx.GetContext(), tracker)

		successCount := 0
		failCount := 0

		var groupFileIDs []int64
		if !req.IsAdmin && req.UserGroupID > 0 {
			groupFileIDs, _ = h.group2FileService.GetBindFiles(ctx.GetContext(), req.UserGroupID)
		}

		for _, id := range req.IDs {
			_ = h.fileTaskLogService.FlushCount(ctx.GetContext(), tracker, filetasklogSvi.WithTotalCounter(1))

			mp, err := h.mountPointService.Query(ctx.GetContext(), id)
			if err != nil {
				failCount++
				_ = h.fileTaskLogService.FlushCount(ctx.GetContext(), tracker, filetasklogSvi.WithCompletedOneCounter())
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
				if !hasAccess {
					failCount++
					_ = h.fileTaskLogService.FlushCount(ctx.GetContext(), tracker, filetasklogSvi.WithCompletedOneCounter())
					ctx.GetContext().Warn("挂载点无权限修改，跳过", zap.Int64("id", id), zap.Int64("user_id", req.UserID))
					continue
				}
			}

			if req.TokenID == 0 {
				err = h.userMountPointTokenService.UnbindToken(ctx.GetContext(), req.UserID, mp.ID)
			} else {
				err = h.userMountPointTokenService.BindToken(ctx.GetContext(), req.UserID, mp.ID, req.TokenID)
			}
			if err != nil {
				failCount++
				_ = h.fileTaskLogService.FlushCount(ctx.GetContext(), tracker, filetasklogSvi.WithCompletedOneCounter())
				ctx.GetContext().Warn("修改挂载点令牌失败，跳过", zap.Int64("id", id), zap.Error(err))
				continue
			}

			successCount++
			_ = h.fileTaskLogService.FlushCount(ctx.GetContext(), tracker, filetasklogSvi.WithCompletedOneCounter())
		}

		if failCount > 0 {
			err := errors.Errorf("批量修改令牌完成，成功 %d 个，失败 %d 个", successCount, failCount)
			_ = h.fileTaskLogService.Failed(ctx.GetContext(), tracker, tracker.WithCost(), utils.WithField("result", err.Error()))
			return err
		}

		return h.fileTaskLogService.Completed(
			ctx.GetContext(),
			tracker,
			tracker.WithCost(),
			utils.WithField("result", fmt.Sprintf("批量修改令牌完成，成功 %d 个", successCount)),
		)
	}
}
