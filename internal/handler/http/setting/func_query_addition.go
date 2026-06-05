package setting

import (
	"github.com/xxcheng123/cloudpan189-share/internal/framework/httpcontext"
	"github.com/xxcheng123/cloudpan189-share/internal/pkgs/utils"
	"github.com/xxcheng123/cloudpan189-share/internal/repository/models"
	"gorm.io/gorm"
)

type _ = models.SettingAddition

// Addition 查询系统附加设置（仅登录用户）
// @Summary 查询系统附加设置
// @Description 获取系统的 SettingAddition，仅登录用户可访问
// @Tags 系统设置
// @Accept json
// @Produce json
// @Success 200 {object} httpcontext.Response{data=models.SettingAddition} "获取系统附加设置成功"
// @Failure 400 {object} httpcontext.Response "查询系统配置失败，code=2003"
// @Failure 401 {object} httpcontext.Response "未授权访问"
// @Failure 403 {object} httpcontext.Response "权限不足"
// @Router /api/setting/addition [get]
func (h *handler) Addition() httpcontext.HandlerFunc {
	return func(ctx *httpcontext.Context) {
		if !h.ensureSettingService(ctx, codeQueryFailed) {
			return
		}

		setting, err := h.settingService.Query(ctx.GetContext())
		if err != nil {
			ctx.Fail(codeQueryFailed.WithError(err))

			return
		}

		if setting == nil {
			ctx.Fail(codeQueryFailed.WithError(gorm.ErrRecordNotFound))

			return
		}

		addition := setting.Addition
		addition.LocalProxyURL = utils.RedactURLForLog(addition.LocalProxyURL)

		ctx.Success(addition)
	}
}
