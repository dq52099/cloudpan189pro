package autoingest

import (
	"errors"

	"github.com/xxcheng123/cloudpan189-share/internal/consts"
	"github.com/xxcheng123/cloudpan189-share/internal/framework/httpcontext"
	autoingestplanSvi "github.com/xxcheng123/cloudpan189-share/internal/services/autoingestplan"
	"gorm.io/gorm"
)

type enablePlanRequest struct {
	ID int64 `json:"id" binding:"required,min=1" example:"1"`
}

// EnablePlan 启用自动挂载计划
// @Summary 启用自动挂载计划
// @Description 根据计划ID启用自动挂载计划
// @Tags 自动挂载管理
// @Accept json
// @Produce json
// @Param Authorization header string true "Bearer token"
// @Param request body enablePlanRequest true "计划ID"
// @Success 200 {object} httpcontext.Response "启用成功"
// @Failure 400 {object} httpcontext.Response "参数验证失败，code=99998"
// @Failure 400 {object} httpcontext.Response "启用自动挂载计划失败，code=xxxx"
// @Failure 401 {object} httpcontext.Response "未授权访问"
// @Failure 403 {object} httpcontext.Response "权限不足"
// @Failure 404 {object} httpcontext.Response "自动挂载计划不存在，code=xxxx"
// @Router /api/auto_ingest/plan/enable [post]
func (h *handler) EnablePlan() httpcontext.HandlerFunc {
	return func(ctx *httpcontext.Context) {
		req := new(enablePlanRequest)
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

		if err := h.planService.EnableByOwner(ctx.GetContext(), &autoingestplanSvi.UpdateRequest{
			ID:      req.ID,
			UserID:  ctx.GetInt64(consts.CtxKeyUserId),
			IsAdmin: ctx.GetBool(consts.CtxKeyIsAdmin),
		}); err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				ctx.Fail(codePlanNotFound.WithError(err))

				return
			}

			ctx.Fail(codePlanEnableFailed.WithError(err))

			return
		}

		ctx.Success()
	}
}
