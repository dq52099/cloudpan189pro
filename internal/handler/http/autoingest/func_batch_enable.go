package autoingest

import (
	"sync"

	"github.com/xxcheng123/cloudpan189-share/internal/framework/httpcontext"
)

// BatchEnableRequest 批量启用请求
type BatchEnableRequest struct {
	IDs []int64 `json:"ids" binding:"required,min=1,max=500"`
}

// BatchEnable 批量启用计划
// @Summary 批量启用计划
// @Description 批量启用自动入库计划（上限 500 个，内部限流 16 并发）
// @Tags 自动挂载管理
// @Accept json
// @Produce json
// @Param Authorization header string true "Bearer token"
// @Param request body BatchEnableRequest true "批量启用请求"
// @Success 200 {object} httpcontext.Response "操作成功"
func (h *handler) BatchEnable() httpcontext.HandlerFunc {
	return func(ctx *httpcontext.Context) {
		req := new(BatchEnableRequest)
		if err := ctx.ShouldBindJSON(req); err != nil {
			ctx.AbortWithInvalidParams(err)
			return
		}

		var (
			wg         sync.WaitGroup
			successCnt int
			failCnt    int
			mu         sync.Mutex
			sem        = make(chan struct{}, maxBatchConcurrency)
		)

		for _, id := range req.IDs {
			wg.Add(1)
			sem <- struct{}{}

			go func(planId int64) {
				defer wg.Done()
				defer func() { <-sem }()

				if err := h.planService.Enable(ctx.GetContext(), planId); err != nil {
					mu.Lock()
					failCnt++
					mu.Unlock()
					return
				}

				mu.Lock()
				successCnt++
				mu.Unlock()
			}(id)
		}

		wg.Wait()

		ctx.Success(map[string]int{
			"success": successCnt,
			"failed":  failCnt,
		})
	}
}
