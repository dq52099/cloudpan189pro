package setting

import (
	"github.com/xxcheng123/cloudpan189-share/internal/framework/httpcontext"
	"github.com/xxcheng123/cloudpan189-share/internal/pkgs/utils"
)

type modifyBaseURLRequest struct {
	BaseURL string `json:"baseURL" binding:"required,url,max=255" example:"https://example.com"` // 系统基础URL
}

// ModifyBaseURL 修改系统基础URL
// @Summary 修改系统基础URL
// @Description 更新系统基础URL，仅管理员可操作
// @Tags 系统设置
// @Accept json
// @Produce json
// @Param request body modifyBaseURLRequest true "新的系统基础URL"
// @Success 200 {object} httpcontext.Response "系统基础URL更新成功"
// @Failure 400 {object} httpcontext.Response "参数验证失败，code=99998"
// @Failure 400 {object} httpcontext.Response "更新系统基础URL失败，code=6004"
// @Failure 401 {object} httpcontext.Response "未授权访问"
// @Failure 403 {object} httpcontext.Response "权限不足"
// @Router /api/setting/modify_base_url [post]
func (h *handler) ModifyBaseURL() httpcontext.HandlerFunc {
	return func(ctx *httpcontext.Context) {
		req := new(modifyBaseURLRequest)
		if err := ctx.ShouldBindJSON(req); err != nil {
			ctx.AbortWithInvalidParams(err)

			return
		}

		if err := h.settingService.Update(ctx.GetContext(),
			utils.WithField("base_url", req.BaseURL),
		); err != nil {
			ctx.Fail(codeModifyBaseURLFailed.WithError(err))

			return
		}

		ctx.Success()
	}
}
