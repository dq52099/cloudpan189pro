package file

import (
	"encoding/json"
	"errors"

	"github.com/xxcheng123/cloudpan189-share/internal/consts"
	"github.com/xxcheng123/cloudpan189-share/internal/framework/httpcontext"
	"github.com/xxcheng123/cloudpan189-share/internal/repository/models"
	"github.com/xxcheng123/cloudpan189-share/internal/types/topic"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type batchDeleteRequest struct {
	IDs []int64 `json:"ids" binding:"required,min=1"`
}

// BatchDelete 批量删除文件
// @Router /api/file/batch_delete [post]
func (h *handler) BatchDelete() httpcontext.HandlerFunc {
	return func(ctx *httpcontext.Context) {
		req := new(batchDeleteRequest)
		if err := ctx.ShouldBindJSON(req); err != nil {
			ctx.AbortWithInvalidParams(err)

			return
		}

		// 获取当前用户信息用于权限控制
		userID := ctx.GetInt64(consts.CtxKeyUserId)
		isAdmin := ctx.GetBool(consts.CtxKeyIsAdmin)

		requestIDs, err := normalizeBatchIDs(req.IDs)
		if err != nil {
			ctx.AbortWithInvalidParams(err)

			return
		}

		topMountPoints := make(map[int64]*models.MountPoint)

		for _, id := range requestIDs {
			file, err := h.virtualFileService.Query(ctx.GetContext(), id)
			if err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					ctx.Fail(busCodeFileNotFound.WithError(err))
				} else {
					ctx.Fail(busCodeFileQueryError.WithError(err))
				}

				return
			}

			if isAdmin {
				continue
			}

			if userID <= 0 || file.TopId <= 0 {
				ctx.Unauthorized("无权限删除")

				return
			}

			mountPoint, ok := topMountPoints[file.TopId]
			if !ok {
				mountPoint, err = h.mountPointService.Query(ctx.GetContext(), file.TopId)
				if err != nil {
					if errors.Is(err, gorm.ErrRecordNotFound) {
						ctx.Unauthorized("无权限删除")
					} else {
						ctx.Fail(busCodeQueryTopIdError.WithError(err))
					}

					return
				}

				topMountPoints[file.TopId] = mountPoint
			}

			if mountPoint.CreatorUserID != userID {
				ctx.Unauthorized("无权限删除")

				return
			}
		}

		// 构造消息队列请求
		task := &topic.FileBatchDeleteRequest{IDs: requestIDs}
		body, _ := json.Marshal(task)

		fullPath := ctx.GetContext().String(consts.CtxKeyFullPath, "unknown")

		// 推送消息到队列
		err = h.taskEngine.PushMessage(
			ctx.GetContext().
				WithValue(consts.CtxKeyFullPath, fullPath).
				WithValue(consts.CtxKeyInvokeHandlerName, "API批量删除文件"),
			task.Topic(),
			body,
		)
		if err != nil {
			ctx.GetContext().Error("推送文件批量删除任务失败", zap.Error(err))
			// [修正] 使用 busCodeBatchDeleteError 替代 NewError
			ctx.Fail(busCodeBatchDeleteError.WithError(err))

			return
		}

		ctx.GetContext().Info("批量删除文件请求已加入队列", zap.Int("count", len(requestIDs)), zap.Int("original_count", len(req.IDs)))
		ctx.Success("删除任务已提交，后台处理中")
	}
}

func uniqueInt64s(ids []int64) []int64 {
	seen := make(map[int64]struct{}, len(ids))
	result := make([]int64, 0, len(ids))

	for _, id := range ids {
		if _, ok := seen[id]; ok {
			continue
		}

		seen[id] = struct{}{}
		result = append(result, id)
	}

	return result
}

func normalizeBatchIDs(ids []int64) ([]int64, error) {
	result := uniqueInt64s(ids)

	for _, id := range result {
		if id <= 0 {
			return nil, errors.New("ids 必须全部大于 0")
		}
	}

	return result, nil
}
