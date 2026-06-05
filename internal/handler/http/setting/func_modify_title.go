package setting

import (
	"github.com/xxcheng123/cloudpan189-share/internal/framework/httpcontext"
	"github.com/xxcheng123/cloudpan189-share/internal/pkgs/utils"
)

type modifyTitleRequest struct {
	Title string `json:"title" binding:"required,min=1,max=255" example:"我的云盘系统"` // 系统标题
}

// ModifyTitle 修改系统标题
// @Summary 修改系统标题
// @Description 更新系统标题，仅管理员可操作
// @Tags 系统设置
// @Accept json
// @Produce json
// @Param request body modifyTitleRequest true "新的系统标题"
// @Success 200 {object} httpcontext.Response "系统标题更新成功"
// @Failure 400 {object} httpcontext.Response "参数验证失败，code=99998"
// @Failure 400 {object} httpcontext.Response "更新系统标题失败，code=6003"
// @Failure 401 {object} httpcontext.Response "未授权访问"
// @Failure 403 {object} httpcontext.Response "权限不足"
// @Router /api/setting/modify_title [post]
func (h *handler) ModifyTitle() httpcontext.HandlerFunc {
	return func(ctx *httpcontext.Context) {
		req := new(modifyTitleRequest)
		if err := ctx.ShouldBindJSON(req); err != nil {
			ctx.AbortWithInvalidParams(err)

			return
		}

		if !h.ensureSettingService(ctx, codeModifyTitleFailed) {
			return
		}

		if err := h.settingService.Update(ctx.GetContext(),
			utils.WithField("title", req.Title),
		); err != nil {
			ctx.Fail(codeModifyTitleFailed.WithError(err))

			return
		}

		ctx.Success()
	}
}
