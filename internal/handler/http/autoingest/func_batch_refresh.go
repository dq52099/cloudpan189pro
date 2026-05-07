package autoingest

import (
	"encoding/json"
	"sync"

	"github.com/xxcheng123/cloudpan189-share/internal/framework/httpcontext"
	"github.com/xxcheng123/cloudpan189-share/internal/types/autoingest"
	"github.com/xxcheng123/cloudpan189-share/internal/types/topic"
)

// BatchRefreshRequest 批量刷新请求
type BatchRefreshRequest struct {
	IDs []int64 `json:"ids" binding:"required,min=1,max=500"`
}

// BatchRefresh 批量刷新计划
// @Summary 批量刷新计划
// @Description 批量触发计划的刷新任务（上限 500 个，内部限流 16 并发）
// @Tags 自动挂载管理
// @Accept json
// @Produce json
// @Param Authorization header string true "Bearer token"
// @Param request body BatchRefreshRequest true "批量刷新请求"
// @Success 200 {object} httpcontext.Response "操作成功"
func (h *handler) BatchRefresh() httpcontext.HandlerFunc {
	return func(ctx *httpcontext.Context) {
		req := new(BatchRefreshRequest)
		if err := ctx.ShouldBindJSON(req); err != nil {
			ctx.AbortWithInvalidParams(err)
			return
		}

		plans, err := h.planService.ListByIDs(ctx.GetContext(), req.IDs)
		if err != nil {
			ctx.Fail(codePlanListFailed.WithError(err))
			return
		}

		var (
			wg         sync.WaitGroup
			successCnt int
			failCnt    int
			mu         sync.Mutex
			sem        = make(chan struct{}, maxBatchConcurrency)
		)

		for _, plan := range plans {
			if plan.SourceType != autoingest.SourceTypeSubscribe {
				mu.Lock()
				failCnt++
				mu.Unlock()
				continue
			}

			wg.Add(1)
			sem <- struct{}{}

			go func(planId int64) {
				defer wg.Done()
				defer func() { <-sem }()

				taskReq := &topic.AutoIngestRefreshSubscribeRequest{
					PlanId:  planId,
					IsRetry: false,
				}
				body, err := json.Marshal(taskReq)
				if err != nil {
					mu.Lock()
					failCnt++
					mu.Unlock()
					return
				}

				if err := h.taskEngine.PushMessage(ctx.GetContext(), taskReq.Topic(), body); err != nil {
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
