package autoingest

import (
	"github.com/xxcheng123/cloudpan189-share/internal/consts"
	"github.com/xxcheng123/cloudpan189-share/internal/framework/httpcontext"
	"github.com/xxcheng123/cloudpan189-share/internal/repository/models"
	autoingestplanSvi "github.com/xxcheng123/cloudpan189-share/internal/services/autoingestplan"
)

type (
	// 复用 service 层的查询请求结构
	planListRequest = autoingestplanSvi.ListRequest

	planListResponse struct {
		Total       int64                    `json:"total" example:"100"`     // 总记录数
		CurrentPage int                      `json:"currentPage" example:"1"` // 当前页码
		PageSize    int                      `json:"pageSize" example:"10"`   // 每页大小
		Data        []*models.AutoIngestPlan `json:"data"`                    // 计划列表数据
	}
)

// PlanList 获取自动挂载计划列表
// @Summary 获取自动挂载计划列表
// @Description 分页获取自动挂载计划列表，支持按名称模糊搜索
// @Tags 自动挂载管理
// @Accept json
// @Produce json
// @Param Authorization header string true "Bearer token"
// @Param currentPage query int false "当前页码，默认为1" default(1)
// @Param pageSize query int false "每页大小，默认为10" default(10)
// @Param name query string false "按名称模糊搜索"
// @Success 200 {object} httpcontext.Response{data=planListResponse} "获取计划列表成功"
// @Failure 400 {object} httpcontext.Response "参数验证失败，code=99998"
// @Failure 400 {object} httpcontext.Response "获取自动挂载计划列表失败，code=xxxx"
// @Failure 401 {object} httpcontext.Response "未授权访问"
// @Failure 403 {object} httpcontext.Response "权限不足"
// @Router /api/auto_ingest/plan/list [get]
func (h *handler) PlanList() httpcontext.HandlerFunc {
	return func(ctx *httpcontext.Context) {
		req := new(planListRequest)
		if err := ctx.ShouldBindQuery(req); err != nil {
			ctx.AbortWithInvalidParams(err)

			return
		}

		// 获取当前用户信息用于权限控制
		req.UserID = ctx.GetInt64(consts.CtxKeyUserId)
		req.IsAdmin = ctx.GetBool(consts.CtxKeyIsAdmin)

		list, err := h.planService.List(ctx.GetContext(), req)
		if err != nil {
			ctx.Fail(codePlanListFailed.WithError(err))

			return
		}

		total, err := h.planService.Count(ctx.GetContext(), req)
		if err != nil {
			ctx.Fail(codePlanListFailed.WithError(err))

			return
		}

		ctx.Success(&planListResponse{
			Total:       total,
			Data:        list,
			PageSize:    req.PageSize,
			CurrentPage: req.CurrentPage,
		})
	}
}
