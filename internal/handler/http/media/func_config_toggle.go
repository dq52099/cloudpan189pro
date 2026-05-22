package media

import (
	"github.com/xxcheng123/cloudpan189-share/internal/framework/httpcontext"
)

// configToggleRequest 切换媒体配置启用状态
type configToggleRequest struct {
	Enable *bool `json:"enable" binding:"required" example:"true"`
}

// ConfigToggle 切换媒体配置启用状态
// @Summary 启用/禁用媒体配置
// @Tags 媒体配置
// @Accept json
// @Produce json
// @Param Authorization header string true "Bearer token"
// @Param body body configToggleRequest true "切换启用状态请求体"
// @Success 200 {object} httpcontext.Response "切换成功"
// @Failure 400 {object} httpcontext.Response "参数验证失败，code=99998"
// @Failure 400 {object} httpcontext.Response "切换媒体配置启用状态失败"
// @Router /api/media/config/toggle [post]
func (h *handler) ConfigToggle() httpcontext.HandlerFunc {
	return func(ctx *httpcontext.Context) {
		req := new(configToggleRequest)
		if err := ctx.ShouldBindJSON(req); err != nil {
			ctx.AbortWithInvalidParams(err)

			return
		}

		if err := h.mediaConfigService.Toggle(ctx.GetContext(), *req.Enable); err != nil {
			ctx.Fail(codeConfigToggleFailed.WithError(err))

			return
		}

		ctx.Success()
	}
}
