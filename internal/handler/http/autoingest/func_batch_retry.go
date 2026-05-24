package autoingest

import (
	"sync"

	"github.com/xxcheng123/cloudpan189-share/internal/framework/httpcontext"
	"github.com/xxcheng123/cloudpan189-share/internal/repository/models"
	"github.com/xxcheng123/cloudpan189-share/internal/types/autoingest"
)

// BatchRetryRequest 批量重试请求
type BatchRetryRequest struct {
	IDs []int64 `json:"ids" binding:"required,min=1"`
}

// BatchRetry 批量重试计划
// @Summary 批量重试计划
// @Description 批量将计划的偏移量重置为 1 并重新扫描（上限 500 个，内部限流 16 并发）
// @Tags 自动挂载管理
// @Accept json
// @Produce json
// @Param Authorization header string true "Bearer token"
// @Param request body BatchRetryRequest true "批量重试请求"
// @Success 200 {object} httpcontext.Response "操作成功"
// @Failure 400 {object} httpcontext.Response "参数验证失败，code=99998"
// @Failure 404 {object} httpcontext.Response "自动挂载计划不存在，code=xxxx"
// @Router /api/auto_ingest/plan/batch_retry [post]
func (h *handler) BatchRetry() httpcontext.HandlerFunc {
	return func(ctx *httpcontext.Context) {
		req := new(BatchRetryRequest)
		if err := ctx.ShouldBindJSON(req); err != nil {
			ctx.AbortWithInvalidParams(err)

			return
		}

		requestIDs, err := normalizeBatchIDs(req.IDs)
		if err != nil {
			ctx.AbortWithInvalidParams(err)

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
			if plan.SourceType != autoingest.SourceTypeSubscribe {
				mu.Lock()
				failCnt++
				mu.Unlock()

				continue
			}

			wg.Add(1)

			sem <- struct{}{}

			go func(plan *models.AutoIngestPlan) {
				defer wg.Done()
				defer func() { <-sem }()

				if err := h.dispatchRetryWithRollback(ctx, plan, 1, true, true); err != nil {
					mu.Lock()
					failCnt++
					mu.Unlock()

					return
				}

				mu.Lock()
				successCnt++
				mu.Unlock()
			}(plan)
		}

		wg.Wait()

		ctx.Success(map[string]int{
			"success": successCnt,
			"failed":  failCnt,
		})
	}
}
