package loginlog

import (
	"github.com/xxcheng123/cloudpan189-share/internal/framework/httpcontext"
	"github.com/xxcheng123/cloudpan189-share/internal/repository/models"
	loginlogSvi "github.com/xxcheng123/cloudpan189-share/internal/services/loginlog"
)

type (
	listRequest = loginlogSvi.ListRequest

	listResponse struct {
		Total       int64              `json:"total" example:"100"`     // 总记录数
		CurrentPage int                `json:"currentPage" example:"1"` // 当前页码
		PageSize    int                `json:"pageSize" example:"10"`   // 每页大小
		Data        []*models.LoginLog `json:"data"`                    // 登录日志列表数据
	}
)

// List 获取登录日志列表
// @Summary 获取登录日志列表
// @Description 分页获取登录日志列表，支持多条件过滤（用户、地址、事件、状态、来源、时间范围）
// @Tags 登录日志
// @Accept json
// @Produce json
// @Param Authorization header string true "Bearer token"
// @Param currentPage query int false "当前页码，默认为1" default(1)
// @Param pageSize query int false "每页大小，默认为10" default(10)
// @Param userId query int false "用户ID"
// @Param username query string false "用户名"
// @Param addr query string false "客户端地址或IP"
// @Param method query string false "事件来源(web/api/app/cli)"
// @Param event query string false "事件类型(login/refresh_token)"
// @Param status query string false "状态(success/failed/blocked)"
// @Param beginAt query string false "开始时间(ISO8601)" example("2025-01-01T00:00:00Z")
// @Param endAt query string false "结束时间(ISO8601)" example("2025-01-31T23:59:59Z")
// @Param noPaginate query bool false "是否不分页，默认false" default(false)
// @Success 200 {object} httpcontext.Response{data=listResponse} "获取登录日志列表成功"
// @Failure 400 {object} httpcontext.Response "参数验证失败，code=99998"
// @Failure 400 {object} httpcontext.Response "获取登录日志列表失败，code=5011"
// @Failure 401 {object} httpcontext.Response "未授权访问"
// @Failure 403 {object} httpcontext.Response "权限不足"
// @Router /api/login_log/list [get]
func (h *handler) List() httpcontext.HandlerFunc {
	return func(ctx *httpcontext.Context) {
		req := new(listRequest)
		if err := ctx.ShouldBindQuery(req); err != nil {
			ctx.AbortWithInvalidParams(err)

			return
		}

		if !h.ensureLoginLogService(ctx, codeListFailed) {
			return
		}

		logList, err := h.loginLogService.List(ctx.GetContext(), req)
		if err != nil {
			ctx.Fail(codeListFailed.WithError(err))

			return
		}

		logList = compactLoginLogs(logList)

		var total int64
		if !req.NoPaginate {
			total, err = h.loginLogService.Count(ctx.GetContext(), req)
			if err != nil {
				ctx.Fail(codeListFailed.WithError(err))

				return
			}
		} else {
			total = int64(len(logList))
		}

		ctx.Success(&listResponse{
			Total:       total,
			Data:        logList,
			PageSize:    req.PageSize,
			CurrentPage: req.CurrentPage,
		})
	}
}

func compactLoginLogs(list []*models.LoginLog) []*models.LoginLog {
	if len(list) == 0 {
		return list
	}

	writeIndex := 0

	for _, item := range list {
		if item == nil {
			continue
		}

		list[writeIndex] = item
		writeIndex++
	}

	return list[:writeIndex]
}
