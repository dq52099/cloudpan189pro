package autoingest

import (
	"sync"

	"github.com/xxcheng123/cloudpan189-share/internal/consts"
	"github.com/xxcheng123/cloudpan189-share/internal/framework/httpcontext"
	autoingestplanSvi "github.com/xxcheng123/cloudpan189-share/internal/services/autoingestplan"
)

// BatchDisableRequest 批量停用请求
type BatchDisableRequest struct {
	IDs []int64 `json:"ids" binding:"required,min=1"`
}

// BatchDisable 批量停用计划
// @Summary 批量停用计划
// @Description 批量停用自动入库计划（上限 500 个，内部限流 16 并发）
// @Tags 自动挂载管理
// @Accept json
// @Produce json
// @Param Authorization header string true "Bearer token"
// @Param request body BatchDisableRequest true "批量停用请求"
// @Success 200 {object} httpcontext.Response "操作成功"
// @Failure 400 {object} httpcontext.Response "参数验证失败，code=99998"
// @Failure 404 {object} httpcontext.Response "自动挂载计划不存在，code=xxxx"
// @Router /api/auto_ingest/plan/batch_disable [post]
func (h *handler) BatchDisable() httpcontext.HandlerFunc {
	return func(ctx *httpcontext.Context) {
		req := new(BatchDisableRequest)
		if err := ctx.ShouldBindJSON(req); err != nil {
			ctx.AbortWithInvalidParams(err)

			return
		}

		requestIDs, err := normalizeBatchIDs(req.IDs)
		if err != nil {
			ctx.AbortWithInvalidParams(err)

			return
		}

		if !h.ensurePlanService(ctx, codePlanListFailed) {
			return
		}

		plans, err := h.planService.ListByIDs(ctx.GetContext(), requestIDs)
		if err != nil {
			ctx.Fail(codePlanListFailed.WithError(err))

			return
		}

		accessiblePlans, initialFailCount := filterAccessiblePlans(ctx, requestIDs, plans)
		if abortBatchIfNoAccessiblePlans(ctx, len(accessiblePlans)) {
			return
		}

		var (
			wg         sync.WaitGroup
			successCnt int
			failCnt    = initialFailCount
			mu         sync.Mutex
			sem        = make(chan struct{}, maxBatchConcurrency)
		)

		for _, plan := range accessiblePlans {
			wg.Add(1)

			sem <- struct{}{}

			go func(planId int64) {
				defer wg.Done()
				defer func() { <-sem }()

				if err := h.planService.DisableByOwner(ctx.GetContext(), &autoingestplanSvi.UpdateRequest{
					ID:      planId,
					UserID:  ctx.GetInt64(consts.CtxKeyUserId),
					IsAdmin: ctx.GetBool(consts.CtxKeyIsAdmin),
				}); err != nil {
					mu.Lock()
					failCnt++
					mu.Unlock()

					return
				}

				mu.Lock()
				successCnt++
				mu.Unlock()
			}(plan.ID)
		}

		wg.Wait()

		ctx.Success(map[string]int{
			"success": successCnt,
			"failed":  failCnt,
		})
	}
}
