package storage

import (
	"encoding/json"
	"fmt"

	"github.com/xxcheng123/cloudpan189-share/internal/consts"
	"github.com/xxcheng123/cloudpan189-share/internal/framework/httpcontext"
	filetasklogSvi "github.com/xxcheng123/cloudpan189-share/internal/services/filetasklog"
	"github.com/xxcheng123/cloudpan189-share/internal/types/topic"
	"go.uber.org/zap"
)

type batchDeleteRequest struct {
	IDs []int64 `json:"ids" binding:"required,min=1"`
}

// BatchDelete 批量删除存储挂载
// @Summary 批量删除存储挂载
// @Description 批量删除指定的存储挂载点，将任务推送到后台异步处理
// @Tags 存储管理
// @Accept json
// @Produce json
// @Param Authorization header string true "Bearer token"
// @Param request body batchDeleteRequest true "批量删除请求参数"
// @Success 200 {object} httpcontext.Response "删除任务已提交，后台处理中"
// @Failure 400 {object} httpcontext.Response "参数验证失败，code=99998"
// @Failure 400 {object} httpcontext.Response "发送清理任务失败，code=4024"
// @Failure 401 {object} httpcontext.Response "未授权访问"
// @Failure 403 {object} httpcontext.Response "权限不足"
// @Router /api/storage/batch_delete [post]
func (h *handler) BatchDelete() httpcontext.HandlerFunc {
	return func(ctx *httpcontext.Context) {
		req := new(batchDeleteRequest)
		if err := ctx.ShouldBindJSON(req); err != nil {
			ctx.AbortWithInvalidParams(err)
			return
		}

		// 转换为file_id
		fileIDs := make([]int64, 0, len(req.IDs))
		for _, id := range req.IDs {
			mp, err := h.mountPointService.Query(ctx.GetContext(), id)
			if err != nil {
				ctx.GetContext().Warn("查询挂载点失败，跳过", zap.Int64("id", id), zap.Error(err))
				continue
			}
			fileIDs = append(fileIDs, mp.FileId)
		}

		if len(fileIDs) == 0 {
			ctx.Fail(busCodeStorageMountPointNotFound)
			return
		}

		tracker, _ := h.fileTaskLogService.Create(
			ctx.GetContext(),
			"批量删除",
			fmt.Sprintf("批量删除 %d 个挂载点", len(req.IDs)),
			filetasklogSvi.WithFile(fileIDs[0]),
			filetasklogSvi.WithDesc(fmt.Sprintf("ID列表: %v", fileIDs)),
		)
		if tracker != nil {
			_ = h.fileTaskLogService.Completed(ctx.GetContext(), tracker)
		}

		task := &topic.FileBatchDeleteRequest{IDs: fileIDs}
		body, _ := json.Marshal(task)

		err := h.taskEngine.PushMessage(
			ctx.GetContext().WithValue(consts.CtxKeyInvokeHandlerName, "批量删除"),
			task.Topic(),
			body,
		)

		if err != nil {
			ctx.GetContext().Error("推送删除任务失败", zap.Error(err))
			ctx.Fail(busCodeStorageSendTaskFail.WithError(err))
			return
		}

		ctx.GetContext().Info("批量删除请求已加入队列", zap.Int("count", len(fileIDs)))
		ctx.Success("删除任务已提交，后台处理中")
	}
}
