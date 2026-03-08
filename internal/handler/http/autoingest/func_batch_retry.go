package autoingest

import (
	"encoding/json"
	"sync"

	"github.com/xxcheng123/cloudpan189-share/internal/framework/httpcontext"
	"github.com/xxcheng123/cloudpan189-share/internal/types/topic"
)

// BatchRetryRequest 批量重试请求
type BatchRetryRequest struct {
	IDs []int64 `json:"ids" binding:"required,min=1"`
}

// BatchRetry 批量重试计划
// @Summary 批量重试计划
// @Description 批量将计划的偏移量重置为1并重新扫描
// @Tags 自动挂载管理
// @Accept json
// @Produce json
// @Param Authorization header string true "Bearer token"
// @Param request body BatchRetryRequest true "批量重试请求"
// @Success 200 {object} httpcontext.Response "操作成功"
// @Failure 400 {object} httpcontext.Response "参数验证失败"
func (h *handler) BatchRetry() httpcontext.HandlerFunc {
	return func(ctx *httpcontext.Context) {
		req := new(BatchRetryRequest)
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
		)

		for _, plan := range plans {
			if plan.SourceType != "subscribe" {
				mu.Lock()
				failCnt++
				mu.Unlock()
				continue
			}

			wg.Add(1)
			go func(planId int64) {
				defer wg.Done()

				if err := h.planService.UpdateOffset(ctx.GetContext(), planId, 1); err != nil {
					mu.Lock()
					failCnt++
					mu.Unlock()
					return
				}

				if err := h.planService.ResetCounters(ctx.GetContext(), planId); err != nil {
					mu.Lock()
					failCnt++
					mu.Unlock()
					return
				}

				taskReq := &topic.AutoIngestRefreshSubscribeRequest{
					PlanId:  planId,
					IsRetry: true,
				}
				body, _ := json.Marshal(taskReq)
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
