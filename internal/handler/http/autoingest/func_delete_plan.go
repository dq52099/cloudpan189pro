package autoingest

import (
	"github.com/xxcheng123/cloudpan189-share/internal/consts"
	"github.com/xxcheng123/cloudpan189-share/internal/framework/httpcontext"
	autoingestplanSvi "github.com/xxcheng123/cloudpan189-share/internal/services/autoingestplan"
)

type deletePlanRequest struct {
	ID int64 `json:"id" binding:"required,min=1" example:"1"`
}

// DeletePlan 删除自动挂载计划
// @Summary 删除自动挂载计划
// @Description 根据计划ID删除自动挂载计划
// @Tags 自动挂载管理
// @Accept json
// @Produce json
// @Param Authorization header string true "Bearer token"
// @Param request body deletePlanRequest true "计划ID"
// @Success 200 {object} httpcontext.Response "删除成功"
// @Failure 400 {object} httpcontext.Response "参数验证失败，code=99998"
// @Failure 400 {object} httpcontext.Response "删除自动挂载计划失败，code=xxxx"
// @Failure 401 {object} httpcontext.Response "未授权访问"
// @Failure 403 {object} httpcontext.Response "权限不足"
// @Router /api/auto_ingest/plan/delete [post]
func (h *handler) DeletePlan() httpcontext.HandlerFunc {
	return func(ctx *httpcontext.Context) {
		req := new(deletePlanRequest)
		if err := ctx.ShouldBindJSON(req); err != nil {
			ctx.AbortWithInvalidParams(err)

			return
		}

		// 获取当前用户信息用于权限控制
		userID := ctx.GetInt64(consts.CtxKeyUserId)
		isAdmin := ctx.GetBool(consts.CtxKeyIsAdmin)

		deleteReq := &autoingestplanSvi.DeleteRequest{
			ID:      req.ID,
			UserID:  userID,
			IsAdmin: isAdmin,
		}

		if err := h.planService.Delete(ctx.GetContext(), deleteReq); err != nil {
			ctx.Fail(codePlanDeleteFailed.WithError(err))

			return
		}

		ctx.Success()
	}
}
