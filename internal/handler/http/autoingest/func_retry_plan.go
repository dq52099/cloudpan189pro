package autoingest

import (
	"encoding/json"
	"errors"

	"github.com/xxcheng123/cloudpan189-share/internal/framework/httpcontext"
	"github.com/xxcheng123/cloudpan189-share/internal/types/topic"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// RetryPlanRequest 重试计划请求
type RetryPlanRequest struct {
	ID int64 `json:"id" binding:"required,min=1" example:"1"`
}

// RetryPlan 重新获取历史记录
// @Summary 重试计划（重新获取历史记录）
// @Description 将计划的 offset 重置为 1，然后触发刷新任务，用于重新获取所有历史记录
// @Tags 自动挂载管理
// @Accept json
// @Produce json
// @Param Authorization header string true "Bearer token"
// @Param request body RetryPlanRequest true "计划ID"
// @Success 200 {object} httpcontext.Response "操作成功"
// @Failure 400 {object} httpcontext.Response "参数验证失败，code=99998"
// @Failure 400 {object} httpcontext.Response "查询计划失败"
// @Router /api/auto_ingest/plan/retry [post]
func (h *handler) RetryPlan() httpcontext.HandlerFunc {
	return func(ctx *httpcontext.Context) {
		req := new(RetryPlanRequest)
		if err := ctx.ShouldBindJSON(req); err != nil {
			ctx.AbortWithInvalidParams(err)

			return
		}

		// 查询计划是否存在
		plan, err := h.planService.Query(ctx.GetContext(), req.ID)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				ctx.Fail(codePlanNotFound.WithError(err))

				return
			}

			ctx.Fail(codePlanQueryFailed.WithError(err))

			return
		}

		if plan == nil || plan.ID == 0 {
			ctx.Fail(codePlanNotFound)

			return
		}

		if !ensurePlanAccess(ctx, plan) {
			return
		}

		// 重置 offset 为 1
		if err := h.planService.UpdateOffset(ctx.GetContext(), req.ID, 1); err != nil {
			ctx.Fail(codePlanUpdateFailed.WithError(err))

			return
		}

		// 重置计数
		if err := h.planService.ResetCounters(ctx.GetContext(), req.ID); err != nil {
			ctx.Fail(codePlanUpdateFailed.WithError(err))

			return
		}

		// 触发刷新任务
		taskReq := &topic.AutoIngestRefreshSubscribeRequest{
			PlanId:  req.ID,
			IsRetry: true,
		}

		taskBody, jerr := json.Marshal(taskReq)
		if jerr != nil {
			ctx.GetContext().Warn("序列化重试任务失败", zap.Int64("plan_id", req.ID), zap.Error(jerr))
			ctx.Fail(codePlanRefreshFailed.WithError(jerr))

			return
		}

		if perr := h.taskEngine.PushMessage(ctx.GetContext(), taskReq.Topic(), taskBody); perr != nil {
			ctx.GetContext().Warn("下发重试任务失败", zap.Int64("plan_id", req.ID), zap.Error(perr))
			ctx.Fail(codePlanRefreshFailed.WithError(perr))

			return
		}

		ctx.Success()
	}
}
