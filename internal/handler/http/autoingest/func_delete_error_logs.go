package autoingest

import (
	"errors"
	"io"

	"github.com/samber/lo"
	"github.com/xxcheng123/cloudpan189-share/internal/consts"
	"github.com/xxcheng123/cloudpan189-share/internal/framework/httpcontext"
	"github.com/xxcheng123/cloudpan189-share/internal/repository/models"
	autoingestplanSvi "github.com/xxcheng123/cloudpan189-share/internal/services/autoingestplan"
	"gorm.io/gorm"
)

type deleteErrorLogsRequest struct {
	PlanId *int64 `json:"planId" example:"1"`
}

// DeleteErrorLogs 删除错误日志
// @Summary 删除错误日志
// @Description 删除指定计划的错误日志，如果不指定计划则删除所有计划的错误日志
// @Tags 自动挂载管理
// @Accept json
// @Produce json
// @Param Authorization header string true "Bearer token"
// @Param request body deleteErrorLogsRequest false "删除请求参数"
// @Success 200 {object} httpcontext.Response "删除成功，data 为受影响记录数"
// @Failure 400 {object} httpcontext.Response "参数验证失败"
// @Failure 401 {object} httpcontext.Response "未授权访问"
// @Router /api/auto_ingest/log/delete_error [post]
func (h *handler) DeleteErrorLogs() httpcontext.HandlerFunc {
	return func(ctx *httpcontext.Context) {
		req := new(deleteErrorLogsRequest)
		if err := ctx.ShouldBindJSON(req); err != nil && !errors.Is(err, io.EOF) {
			ctx.AbortWithInvalidParams(err)

			return
		}

		var (
			count int64
			err   error
		)

		if req.PlanId != nil {
			if *req.PlanId <= 0 {
				ctx.AbortWithInvalidParams(errors.New("自动入库计划 ID 必须大于 0"))

				return
			}

			plan, queryErr := h.planService.Query(ctx.GetContext(), *req.PlanId)
			if queryErr != nil {
				if errors.Is(queryErr, gorm.ErrRecordNotFound) {
					ctx.Fail(codePlanNotFound.WithError(queryErr))

					return
				}

				ctx.Fail(codePlanQueryFailed.WithError(queryErr))

				return
			}

			if !ensurePlanAccess(ctx, plan) {
				return
			}

			count, err = h.logService.DeleteErrorLogsByPlanId(ctx.GetContext(), *req.PlanId)
		} else {
			count, err = h.deleteVisibleErrorLogs(ctx)
		}

		if err != nil {
			ctx.Fail(codeLogDeleteFailed.WithError(err))

			return
		}

		ctx.Success(count)
	}
}

func (h *handler) deleteVisibleErrorLogs(ctx *httpcontext.Context) (int64, error) {
	if ctx.GetBool(consts.CtxKeyIsAdmin) {
		return h.logService.DeleteAllErrorLogs(ctx.GetContext())
	}

	plans, err := h.planService.List(ctx.GetContext(), &autoingestplanSvi.ListRequest{
		NoPaginate: true,
		UserID:     ctx.GetInt64(consts.CtxKeyUserId),
		IsAdmin:    false,
	})
	if err != nil {
		return 0, err
	}

	return h.logService.DeleteErrorLogsByPlanIds(ctx.GetContext(), lo.Map(plans, func(plan *models.AutoIngestPlan, _ int) int64 {
		return plan.ID
	}))
}
