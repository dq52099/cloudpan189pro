package cloudtoken

import (
	"errors"

	"github.com/xxcheng123/cloudpan189-share/internal/consts"
	"github.com/xxcheng123/cloudpan189-share/internal/framework/httpcontext"
	"github.com/xxcheng123/cloudpan189-share/internal/services/cloudtoken"
	"gorm.io/gorm"
)

type modifyNameRequest = cloudtoken.ModifyNameRequest

// ModifyName 修改云盘令牌名称
// @Summary 修改云盘令牌名称
// @Description 根据云盘令牌ID修改其显示名称
// @Tags 云盘令牌管理
// @Accept json
// @Produce json
// @Param Authorization header string true "Bearer token"
// @Param request body modifyNameRequest true "修改名称请求"
// @Success 200 {object} httpcontext.Response "名称修改成功"
// @Failure 400 {object} httpcontext.Response "参数验证失败，code=99998"
// @Failure 400 {object} httpcontext.Response "修改名称失败，code=5003"
// @Failure 401 {object} httpcontext.Response "未授权访问"
// @Failure 403 {object} httpcontext.Response "权限不足"
// @Router /api/cloud_token/modify_name [post]
func (h *handler) ModifyName() httpcontext.HandlerFunc {
	return func(ctx *httpcontext.Context) {
		req := new(modifyNameRequest)
		if err := ctx.ShouldBindJSON(req); err != nil {
			ctx.AbortWithInvalidParams(err)

			return
		}

		req.UserID = ctx.GetInt64(consts.CtxKeyUserId)
		req.IsAdmin = ctx.GetBool(consts.CtxKeyIsAdmin)

		if !h.ensureCloudTokenService(ctx, codeModifyNameFailed) {
			return
		}

		if err := h.cloudTokenService.ModifyName(ctx.GetContext(), req); err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				ctx.Fail(codeTokenNotFound.WithError(err))

				return
			}

			ctx.Fail(codeModifyNameFailed.WithError(err))

			return
		}

		ctx.Success()
	}
}
