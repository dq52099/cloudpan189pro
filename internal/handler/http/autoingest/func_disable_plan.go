package autoingest

import (
	"errors"

	"github.com/xxcheng123/cloudpan189-share/internal/framework/httpcontext"
	"gorm.io/gorm"
)

type disablePlanRequest struct {
	ID int64 `json:"id" binding:"required,min=1" example:"1"`
}

// DisablePlan 停用自动挂载计划
// @Summary 停用自动挂载计划
// @Description 根据计划ID停用自动挂载计划
// @Tags 自动挂载管理
// @Accept json
// @Produce json
// @Param Authorization header string true "Bearer token"
// @Param request body disablePlanRequest true "计划ID"
// @Success 200 {object} httpcontext.Response "停用成功"
// @Failure 400 {object} httpcontext.Response "参数验证失败，code=99998"
// @Failure 400 {object} httpcontext.Response "停用自动挂载计划失败，code=xxxx"
// @Failure 401 {object} httpcontext.Response "未授权访问"
// @Failure 403 {object} httpcontext.Response "权限不足"
// @Failure 404 {object} httpcontext.Response "自动挂载计划不存在，code=xxxx"
// @Router /api/auto_ingest/plan/disable [post]
func (h *handler) DisablePlan() httpcontext.HandlerFunc {
	return func(ctx *httpcontext.Context) {
		req := new(disablePlanRequest)
		if err := ctx.ShouldBindJSON(req); err != nil {
			ctx.AbortWithInvalidParams(err)

			return
		}

		plan, err := h.planService.Query(ctx.GetContext(), req.ID)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				ctx.Fail(codePlanNotFound.WithError(err))

				return
			}

			ctx.Fail(codePlanQueryFailed.WithError(err))

			return
		}

		if !ensurePlanAccess(ctx, plan) {
			return
		}

		if err := h.planService.Disable(ctx.GetContext(), req.ID); err != nil {
			ctx.Fail(codePlanDisableFailed.WithError(err))

			return
		}

		ctx.Success()
	}
}
