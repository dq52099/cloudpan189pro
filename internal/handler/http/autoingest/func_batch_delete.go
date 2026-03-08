package autoingest

import (
	"sync"

	"github.com/xxcheng123/cloudpan189-share/internal/framework/httpcontext"
)

// BatchDeleteRequest 批量删除请求
type BatchDeleteRequest struct {
	IDs []int64 `json:"ids" binding:"required,min=1"`
}

// BatchDelete 批量删除计划
// @Summary 批量删除计划
// @Description 批量删除自动入库计划
// @Tags 自动挂载管理
// @Accept json
// @Produce json
// @Param Authorization header string true "Bearer token"
// @Param request body BatchDeleteRequest true "批量删除请求"
// @Success 200 {object} httpcontext.Response "操作成功"
func (h *handler) BatchDelete() httpcontext.HandlerFunc {
	return func(ctx *httpcontext.Context) {
		req := new(BatchDeleteRequest)
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

				if err := h.planService.Delete(ctx.GetContext(), planId); err != nil {
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
