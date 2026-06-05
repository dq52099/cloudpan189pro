package cloudtoken

import (
	"errors"

	"github.com/xxcheng123/cloudpan189-share/internal/consts"
	"github.com/xxcheng123/cloudpan189-share/internal/framework/httpcontext"
	"github.com/xxcheng123/cloudpan189-share/internal/services/cloudtoken"
	"gorm.io/gorm"
)

type checkQrcodeRequest = cloudtoken.CheckQrcodeRequest

// CheckQrcode 检查二维码状态
// @Summary 检查二维码状态
// @Description 检查二维码扫码状态，如果扫码成功则创建或更新云盘令牌
// @Tags 云盘令牌管理
// @Accept json
// @Produce json
// @Param Authorization header string true "Bearer token"
// @Param request body checkQrcodeRequest true "检查二维码请求"
// @Success 200 {object} httpcontext.Response "二维码检查成功"
// @Failure 400 {object} httpcontext.Response "参数验证失败，code=99998"
// @Failure 400 {object} httpcontext.Response "检查二维码失败，code=5002"
// @Failure 401 {object} httpcontext.Response "未授权访问"
// @Failure 403 {object} httpcontext.Response "权限不足"
// @Router /api/cloud_token/check_qrcode [post]
func (h *handler) CheckQrcode() httpcontext.HandlerFunc {
	return func(ctx *httpcontext.Context) {
		req := new(checkQrcodeRequest)
		if err := ctx.ShouldBindJSON(req); err != nil {
			ctx.AbortWithInvalidParams(err)

			return
		}

		// 设置当前用户ID
		req.UserID = ctx.GetInt64(consts.CtxKeyUserId)
		req.IsAdmin = ctx.GetBool(consts.CtxKeyIsAdmin)

		if !h.ensureCloudTokenService(ctx, codeCheckQrcodeFailed) {
			return
		}

		if err := h.cloudTokenService.CheckQrcode(ctx.GetContext(), req); err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				ctx.Fail(codeTokenNotFound.WithError(err))

				return
			}

			ctx.Fail(codeCheckQrcodeFailed.WithError(err))

			return
		}

		ctx.Success()
	}
}
