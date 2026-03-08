package autoingest

import (
	"sync"

	"github.com/xxcheng123/cloudpan189-share/internal/framework/httpcontext"
)

// BatchDisableRequest 批量停用请求
type BatchDisableRequest struct {
	IDs []int64 `json:"ids" binding:"required,min=1"`
}

// BatchDisable 批量停用计划
// @Summary 批量停用计划
// @Description 批量停用自动入库计划
// @Tags 自动挂载管理
// @Accept json
// @Produce json
// @Param Authorization header string true "Bearer token"
// @Param request body BatchDisableRequest true "批量停用请求"
// @Success 200 {object} httpcontext.Response "操作成功"
func (h *handler) BatchDisable() httpcontext.HandlerFunc {
	return func(ctx *httpcontext.Context) {
		req := new(BatchDisableRequest)
		if err := ctx.ShouldBindJSON(req); err != nil {
			ctx.AbortWithInvalidParams(err)
			return
		}

		var (
			wg         sync.WaitGroup
			successCnt int
			failCnt    int
			mu         sync.Mutex
		)

		for _, id := range req.IDs {
			wg.Add(1)
			go func(planId int64) {
				defer wg.Done()

				if err := h.planService.Disable(ctx.GetContext(), planId); err != nil {
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
