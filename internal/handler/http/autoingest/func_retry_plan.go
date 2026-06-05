package autoingest

import (
	"errors"

	"github.com/xxcheng123/cloudpan189-share/internal/framework/httpcontext"
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
// @Failure 404 {object} httpcontext.Response "自动挂载计划不存在，code=xxxx"
// @Router /api/auto_ingest/plan/retry [post]
func (h *handler) RetryPlan() httpcontext.HandlerFunc {
	return func(ctx *httpcontext.Context) {
		req := new(RetryPlanRequest)
		if err := ctx.ShouldBindJSON(req); err != nil {
			ctx.AbortWithInvalidParams(err)

			return
		}

		if !h.ensurePlanService(ctx, codePlanQueryFailed) {
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

		if !h.ensureTaskEngine(ctx, codePlanRefreshFailed) {
			return
		}

		if err := h.dispatchRetryWithRollback(ctx, plan, 1, true, true); err != nil {
			failAutoIngestRetryError(ctx, err)

			return
		}

		ctx.Success()
	}
}
