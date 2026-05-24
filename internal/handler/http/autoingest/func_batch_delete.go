package autoingest

import (
	"sync"

	"github.com/xxcheng123/cloudpan189-share/internal/consts"
	"github.com/xxcheng123/cloudpan189-share/internal/framework/httpcontext"
	autoingestplanSvi "github.com/xxcheng123/cloudpan189-share/internal/services/autoingestplan"
)

// BatchDeleteRequest 批量删除请求
type BatchDeleteRequest struct {
	IDs []int64 `json:"ids" binding:"required,min=1"`
}

// BatchDelete 批量删除计划
// @Summary 批量删除计划
// @Description 批量删除自动入库计划（上限 500 个 id，内部限流 16 并发）
// @Tags 自动挂载管理
// @Accept json
// @Produce json
// @Param Authorization header string true "Bearer token"
// @Param request body BatchDeleteRequest true "批量删除请求"
// @Success 200 {object} httpcontext.Response "操作成功"
// @Failure 400 {object} httpcontext.Response "参数验证失败，code=99998"
// @Failure 404 {object} httpcontext.Response "自动挂载计划不存在，code=xxxx"
// @Router /api/auto_ingest/plan/batch_delete [post]
func (h *handler) BatchDelete() httpcontext.HandlerFunc {
	return func(ctx *httpcontext.Context) {
		req := new(BatchDeleteRequest)
		if err := ctx.ShouldBindJSON(req); err != nil {
			ctx.AbortWithInvalidParams(err)

			return
		}

		// 获取当前用户信息用于权限控制
		userID := ctx.GetInt64(consts.CtxKeyUserId)
		isAdmin := ctx.GetBool(consts.CtxKeyIsAdmin)

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
			wg.Add(1)

			sem <- struct{}{}

			go func(planId int64) {
				defer wg.Done()
				defer func() { <-sem }()

				deleteReq := &autoingestplanSvi.DeleteRequest{
					ID:      planId,
					UserID:  userID,
					IsAdmin: isAdmin,
				}

				if err := h.planService.Delete(ctx.GetContext(), deleteReq); err != nil {
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
