package autoingest

import (
	"github.com/samber/lo"
	"github.com/xxcheng123/cloudpan189-share/internal/consts"
	"github.com/xxcheng123/cloudpan189-share/internal/framework/httpcontext"
	"github.com/xxcheng123/cloudpan189-share/internal/repository/models"
	autoingestlogSvi "github.com/xxcheng123/cloudpan189-share/internal/services/autoingestlog"
	autoingestplanSvi "github.com/xxcheng123/cloudpan189-share/internal/services/autoingestplan"
)

type (
	logListRequest = autoingestlogSvi.ListRequest

	logDTO struct {
		*models.AutoIngestLog
		PlanName string `json:"planName"`
	}

	logListResponse struct {
		Total       int64     `json:"total" example:"100"`     // 总记录数
		CurrentPage int       `json:"currentPage" example:"1"` // 当前页码
		PageSize    int       `json:"pageSize" example:"10"`   // 每页大小
		Data        []*logDTO `json:"data"`                    // 日志列表数据
	}
)

// LogList 日志查询
// @Summary 获取自动挂载日志列表
// @Description 分页获取自动挂载日志，支持按计划ID与级别筛选
// @Tags 自动挂载管理
// @Accept json
// @Produce json
// @Param Authorization header string true "Bearer token"
// @Param currentPage query int false "当前页码，默认为1" default(1)
// @Param pageSize query int false "每页大小，默认为10" default(10)
// @Param planId query int false "计划ID"
// @Param level query string false "日志级别，如 info/error"
// @Success 200 {object} httpcontext.Response{data=logListResponse} "获取日志列表成功"
// @Failure 400 {object} httpcontext.Response "参数验证失败，code=99998"
// @Failure 400 {object} httpcontext.Response "获取自动挂载日志列表失败，code=xxxx"
// @Failure 401 {object} httpcontext.Response "未授权访问"
// @Failure 403 {object} httpcontext.Response "权限不足"
// @Router /api/auto_ingest/log/list [get]
func (h *handler) LogList() httpcontext.HandlerFunc {
	return func(ctx *httpcontext.Context) {
		req := new(logListRequest)
		if err := ctx.ShouldBindQuery(req); err != nil {
			ctx.AbortWithInvalidParams(err)

			return
		}

		userID := ctx.GetInt64(consts.CtxKeyUserId)
		isAdmin := ctx.GetBool(consts.CtxKeyIsAdmin)

		// 仅列出当前用户可见的计划作为 planName 映射，避免越权暴露其他用户的计划名
		planList, err := h.planService.List(ctx.GetContext(), &autoingestplanSvi.ListRequest{
			NoPaginate: true,
			UserID:     userID,
			IsAdmin:    isAdmin,
		})
		if err != nil {
			ctx.Fail(codeLogListFailed.WithError(err))

			return
		}

		planNameMap := lo.SliceToMap(planList, func(item *models.AutoIngestPlan) (int64, string) { return item.ID, item.Name })
		visiblePlanIDs := lo.Keys(planNameMap)

		if !isAdmin {
			if req.PlanId > 0 {
				if !lo.Contains(visiblePlanIDs, req.PlanId) {
					ctx.Success(&logListResponse{
						Total:       0,
						Data:        []*logDTO{},
						PageSize:    req.PageSize,
						CurrentPage: req.CurrentPage,
					})

					return
				}
			} else {
				if len(visiblePlanIDs) == 0 {
					ctx.Success(&logListResponse{
						Total:       0,
						Data:        []*logDTO{},
						PageSize:    req.PageSize,
						CurrentPage: req.CurrentPage,
					})

					return
				}

				req.PlanIdList = visiblePlanIDs
			}
		}

		list, err := h.logService.List(ctx.GetContext(), req)
		if err != nil {
			ctx.Fail(codeLogListFailed.WithError(err))

			return
		}

		total, err := h.logService.Count(ctx.GetContext(), req)
		if err != nil {
			ctx.Fail(codeLogListFailed.WithError(err))

			return
		}

		var dtoList = make([]*logDTO, 0, len(list))
		for _, item := range list {
			planName := "计划不存在"

			if _planName, ok := planNameMap[item.PlanId]; ok {
				planName = _planName
			}

			dtoList = append(dtoList, &logDTO{
				AutoIngestLog: item,
				PlanName:      planName,
			})
		}

		ctx.Success(&logListResponse{
			Total:       total,
			Data:        dtoList,
			PageSize:    req.PageSize,
			CurrentPage: req.CurrentPage,
		})
	}
}
