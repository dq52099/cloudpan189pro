package setting

import (
	"github.com/xxcheng123/cloudpan189-share/internal/framework/httpcontext"
	"github.com/xxcheng123/cloudpan189-share/internal/pkgs/utils"
)

type toggleEnableAuthRequest struct {
	EnableAuth *bool `json:"enableAuth" binding:"required" example:"true"` // 是否启用鉴权
}

// ToggleEnableAuth 切换系统鉴权开关
// @Summary 切换系统鉴权开关
// @Description 启用或关闭系统鉴权，仅管理员可操作
// @Tags 系统设置
// @Accept json
// @Produce json
// @Param request body toggleEnableAuthRequest true "鉴权开关设置"
// @Success 200 {object} httpcontext.Response "系统鉴权开关更新成功"
// @Failure 400 {object} httpcontext.Response "参数验证失败，code=99998"
// @Failure 400 {object} httpcontext.Response "更新系统鉴权开关失败，code=6005"
// @Failure 401 {object} httpcontext.Response "未授权访问"
// @Failure 403 {object} httpcontext.Response "权限不足"
// @Router /api/setting/toggle_enable_auth [post]
func (h *handler) ToggleEnableAuth() httpcontext.HandlerFunc {
	return func(ctx *httpcontext.Context) {
		req := new(toggleEnableAuthRequest)
		if err := ctx.ShouldBindJSON(req); err != nil {
			ctx.AbortWithInvalidParams(err)

			return
		}

		if err := h.settingService.Update(ctx.GetContext(),
			utils.WithField("enable_auth", *req.EnableAuth),
		); err != nil {
			ctx.Fail(codeToggleEnableAuthFailed.WithError(err))

			return
		}

		ctx.Success()
	}
}
