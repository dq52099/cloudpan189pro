package storage

import (
	"encoding/json"
	"fmt"

	"github.com/xxcheng123/cloudpan189-share/internal/consts"
	"github.com/xxcheng123/cloudpan189-share/internal/framework/httpcontext"
	"github.com/xxcheng123/cloudpan189-share/internal/repository/models"
	filetasklogSvi "github.com/xxcheng123/cloudpan189-share/internal/services/filetasklog"
	"github.com/xxcheng123/cloudpan189-share/internal/types/topic"
	"go.uber.org/zap"
)

type batchDeleteRequest struct {
	IDs []int64 `json:"ids" binding:"required,min=1,max=1000"`
}

type batchDeleteResponse struct {
	Total   int `json:"total"`
	Success int `json:"success"`
	Failed  int `json:"failed"`
}

// BatchDelete 批量删除存储挂载
// @Summary 批量删除存储挂载
// @Description 批量删除指定的存储挂载点，为每个挂载点创建独立任务，由工作流并发处理（上限 1000 个）
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

		// 获取当前用户信息用于权限控制
		userID := ctx.GetInt64(consts.CtxKeyUserId)
		isAdmin := ctx.GetBool(consts.CtxKeyIsAdmin)

		// 查询所有挂载点并按权限过滤；缓存已查到的对象避免二次查询
		validMountPoints := make([]*models.MountPoint, 0, len(req.IDs))
		for _, id := range req.IDs {
			mountPoint, err := h.mountPointService.Query(ctx.GetContext(), id)
			if err != nil {
				ctx.GetContext().Debug("查询挂载点失败，跳过", zap.Int64("id", id), zap.Error(err))
				continue
			}
			// 检查权限：管理员或创建者可以删除
			if !(isAdmin || mountPoint.CreatorUserID == userID) {
				ctx.GetContext().Debug("无权限删除该挂载点，跳过", zap.Int64("id", id), zap.Int64("creator_user_id", mountPoint.CreatorUserID))
				continue
			}
			validMountPoints = append(validMountPoints, mountPoint)
		}

		// 创建任务日志
		tracker, logErr := h.fileTaskLogService.Create(
			ctx.GetContext(),
			"批量删除",
			fmt.Sprintf("批量删除 %d 个挂载点", len(validMountPoints)),
			filetasklogSvi.WithDesc(fmt.Sprintf("请求ID数量: %d, 授权通过: %d", len(req.IDs), len(validMountPoints))),
		)
		if logErr != nil {
			ctx.GetContext().Warn("创建批量删除任务日志失败", zap.Error(logErr))
		} else if tracker != nil {
			_ = h.fileTaskLogService.Running(ctx.GetContext(), tracker)
			_ = h.fileTaskLogService.FlushCount(
				ctx.GetContext(), tracker,
				filetasklogSvi.WithTotalCounter(len(validMountPoints)),
			)
		}

		// 为每个挂载点推送独立删除任务
		successCount := 0
		for _, mountPoint := range validMountPoints {
			taskReq := &topic.FileBatchDeleteRequest{
				IDs: []int64{mountPoint.FileId},
			}
			body, err := json.Marshal(taskReq)
			if err != nil {
				ctx.GetContext().Warn("序列化删除任务失败", zap.Int64("file_id", mountPoint.FileId), zap.Error(err))
				continue
			}

			if err := h.taskEngine.PushMessage(
				ctx.GetContext().
					WithValue(consts.CtxKeyFullPath, mountPoint.FullPath).
					WithValue(consts.CtxKeyInvokeHandlerName, "删除文件").
					WithValue(consts.CtxKeyTaskTracker, tracker),
				taskReq.Topic(),
				body,
			); err != nil {
				ctx.GetContext().Warn("推送删除任务失败，跳过", zap.Int64("file_id", mountPoint.FileId), zap.Error(err))
				continue
			}
			successCount++
		}

		ctx.GetContext().Info(
			"批量删除请求已加入队列",
			zap.Int("total", len(req.IDs)),
			zap.Int("valid", len(validMountPoints)),
			zap.Int("success", successCount),
		)

		ctx.Success(batchDeleteResponse{
			Total:   len(req.IDs),
			Success: successCount,
			Failed:  len(req.IDs) - successCount,
		})
	}
}
