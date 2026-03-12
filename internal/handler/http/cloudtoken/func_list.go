package cloudtoken

import (
	"github.com/xxcheng123/cloudpan189-share/internal/consts"
	"github.com/xxcheng123/cloudpan189-share/internal/framework/httpcontext"
	"github.com/xxcheng123/cloudpan189-share/internal/repository/models"
	"github.com/xxcheng123/cloudpan189-share/internal/services/cloudtoken"
)

type (
	listRequest = cloudtoken.ListRequest

	listResponse struct {
		Total       int64                `json:"total" example:"100"`     // 总记录数
		CurrentPage int                  `json:"currentPage" example:"1"` // 当前页码
		PageSize    int                  `json:"pageSize" example:"10"`   // 每页大小
		Data        []*models.CloudToken `json:"data"`                    // 云盘令牌列表数据
	}
)

// List 获取云盘令牌列表
// @Summary 获取云盘令牌列表
// @Description 分页获取云盘令牌列表，支持按名称模糊搜索
// @Tags 云盘令牌管理
// @Accept json
// @Produce json
// @Param Authorization header string true "Bearer token"
// @Param currentPage query int false "当前页码，默认为1" default(1)
// @Param pageSize query int false "每页大小，默认为10" default(10)
// @Param noPaginate query bool false "是否不分页，默认false" default(false)
// @Param name query string false "名称模糊搜索"
// @Success 200 {object} httpcontext.Response{data=listResponse} "获取云盘令牌列表成功"
// @Failure 400 {object} httpcontext.Response "参数验证失败，code=99998"
// @Failure 400 {object} httpcontext.Response "获取云盘令牌列表失败，code=5005"
// @Failure 401 {object} httpcontext.Response "未授权访问"
// @Failure 403 {object} httpcontext.Response "权限不足"
// @Router /api/cloud_token/list [get]
func (h *handler) List() httpcontext.HandlerFunc {
	return func(ctx *httpcontext.Context) {
		req := new(listRequest)
		if err := ctx.ShouldBindQuery(req); err != nil {
			ctx.AbortWithInvalidParams(err)

			return
		}

		// 获取当前用户信息用于权限控制
		req.UserID = ctx.GetInt64(consts.CtxKeyUserId)
		req.IsAdmin = ctx.GetBool(consts.CtxKeyIsAdmin)

		cloudTokenList, err := h.cloudTokenService.List(ctx.GetContext(), req)
		if err != nil {
			ctx.Fail(codeListFailed.WithError(err))

			return
		}

		var total int64
		if !req.NoPaginate {
			total, err = h.cloudTokenService.Count(ctx.GetContext(), req)
			if err != nil {
				ctx.Fail(codeListFailed.WithError(err))

				return
			}
		} else {
			total = int64(len(cloudTokenList))
		}

		ctx.Success(&listResponse{
			Total:       total,
			Data:        cloudTokenList,
			PageSize:    req.PageSize,
			CurrentPage: req.CurrentPage,
		})
	}
}
