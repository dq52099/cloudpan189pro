package storage

import (
	"encoding/json"
	"fmt"

	"github.com/xxcheng123/cloudpan189-share/internal/consts"
	"github.com/xxcheng123/cloudpan189-share/internal/framework/httpcontext"
	"github.com/xxcheng123/cloudpan189-share/internal/pkgs/utils"
	filetasklogSvi "github.com/xxcheng123/cloudpan189-share/internal/services/filetasklog"
	"github.com/xxcheng123/cloudpan189-share/internal/types/topic"
	"go.uber.org/zap"
)

type batchRefreshRequest struct {
	IDs  []int64 `json:"ids" binding:"required,min=1,max=1000"`
	Deep bool    `json:"deep"` // 是否深度刷新
}

// BatchRefresh 批量刷新存储挂载
// @Summary 批量刷新存储挂载
// @Description 批量刷新指定的存储挂载点，触发文件扫描任务重新同步文件信息（上限 1000 个）
// @Tags 存储管理
// @Accept json
// @Produce json
// @Param Authorization header string true "Bearer token"
// @Param request body batchRefreshRequest true "批量刷新请求参数"
// @Success 200 {object} httpcontext.Response "刷新任务已提交，后台处理中"
// @Failure 400 {object} httpcontext.Response "参数验证失败，code=99998"
// @Failure 400 {object} httpcontext.Response "发送刷新任务失败，code=4024"
// @Failure 401 {object} httpcontext.Response "未授权访问"
// @Failure 403 {object} httpcontext.Response "权限不足"
// @Router /api/storage/batch_refresh [post]
func (h *handler) BatchRefresh() httpcontext.HandlerFunc {
	return func(ctx *httpcontext.Context) {
		req := new(batchRefreshRequest)
		if err := ctx.ShouldBindJSON(req); err != nil {
			ctx.AbortWithInvalidParams(err)
			return
		}

		refreshType := "普通刷新"
		if req.Deep {
			refreshType = "深度刷新"
		}

		// 预查询所有挂载点，减少循环中的重复查询
		type mpInfo struct {
			fileId   int64
			fullPath string
		}
		mountPoints := make(map[int64]mpInfo, len(req.IDs))

		for _, id := range req.IDs {
			mp, err := h.mountPointService.Query(ctx.GetContext(), id)
			if err != nil {
				ctx.GetContext().Warn("查询挂载点失败，跳过", zap.Int64("id", id), zap.Error(err))
				continue
			}
			mountPoints[id] = mpInfo{
				fileId:   mp.FileId,
				fullPath: mp.FullPath,
			}
		}

		// 创建任务日志（记录真实有效 id 数量）
		tracker, logErr := h.fileTaskLogService.Create(
			ctx.GetContext(),
			fmt.Sprintf("批量%s", refreshType),
			fmt.Sprintf("批量%s %d 个挂载点", refreshType, len(mountPoints)),
			filetasklogSvi.WithDesc(fmt.Sprintf("ID列表: %v, 类型: %s", req.IDs, refreshType)),
		)
		if logErr != nil {
			ctx.GetContext().Warn("创建批量刷新任务日志失败", zap.Error(logErr))
		} else if tracker != nil {
			_ = h.fileTaskLogService.Running(ctx.GetContext(), tracker)
			_ = h.fileTaskLogService.FlushCount(
				ctx.GetContext(), tracker,
				filetasklogSvi.WithTotalCounter(len(mountPoints)),
			)
		}

		successCount := 0
		for id, mp := range mountPoints {
			taskReq := &topic.FileScanFileRequest{
				FileId: mp.fileId,
				Deep:   req.Deep,
			}
			body, err := json.Marshal(taskReq)
			if err != nil {
				ctx.GetContext().Warn("序列化刷新任务失败", zap.Int64("id", id), zap.Error(err))
				continue
			}

			if err := h.taskEngine.PushMessage(
				ctx.GetContext().
					WithValue(consts.CtxKeyFullPath, mp.fullPath).
					WithValue(consts.CtxKeyInvokeHandlerName, fmt.Sprintf("批量%s", refreshType)),
				taskReq.Topic(),
				body,
			); err != nil {
				ctx.GetContext().Warn("推送刷新任务失败，跳过", zap.Int64("id", id), zap.Error(err))
				continue
			}
			successCount++
		}

		// 批量刷新本身只负责派发任务；真正的扫描进度由各 scan 子任务独立记录日志。
		// 这里把派发任务的 tracker 立刻标记完成，避免其长时间 Running 被 stale checker 置为 Failed。
		if tracker != nil {
			_ = h.fileTaskLogService.FlushCount(
				ctx.GetContext(), tracker,
				filetasklogSvi.WithCompletedCounter(successCount),
			)
			if err := h.fileTaskLogService.Completed(
				ctx.GetContext(), tracker,
				tracker.WithCost(),
				utils.WithField("result", fmt.Sprintf("派发成功 %d 个，失败 %d 个，具体扫描进度请查看对应子任务", successCount, len(req.IDs)-successCount)),
			); err != nil {
				ctx.GetContext().Warn("批量刷新任务标记完成失败", zap.Error(err))
			}
		}

		ctx.GetContext().Info("批量刷新请求已加入队列", zap.Int("total", len(req.IDs)), zap.Int("success", successCount))
		ctx.Success(fmt.Sprintf("刷新任务已提交，成功 %d 个，失败 %d 个", successCount, len(req.IDs)-successCount))
	}
}
