package cloudtoken

import (
	"github.com/xxcheng123/cloudpan189-share/internal/framework/httpcontext"
	cloudtokenSvi "github.com/xxcheng123/cloudpan189-share/internal/services/cloudtoken"
)

// InitQrcodeResponse 初始化二维码响应类型别名
type InitQrcodeResponse = cloudtokenSvi.InitQrcodeResponse

// InitQrcode 初始化二维码
// @Summary 初始化二维码
// @Description 初始化登录二维码，获取二维码UUID用于后续扫码登录
// @Tags 云盘令牌管理
// @Accept json
// @Produce json
// @Param Authorization header string true "Bearer token"
// @Success 200 {object} httpcontext.Response{data=cloudtoken.InitQrcodeResponse} "二维码初始化成功"
// @Failure 400 {object} httpcontext.Response "参数验证失败，code=99998"
// @Failure 400 {object} httpcontext.Response "初始化二维码失败，code=5001"
// @Failure 401 {object} httpcontext.Response "未授权访问"
// @Failure 403 {object} httpcontext.Response "权限不足"
// @Router /api/cloud_token/init_qrcode [post]
func (h *handler) InitQrcode() httpcontext.HandlerFunc {
	return func(ctx *httpcontext.Context) {
		if !h.ensureCloudTokenService(ctx, codeInitQrcodeFailed) {
			return
		}

		resp, err := h.cloudTokenService.InitQrcode(ctx.GetContext())
		if err != nil {
			ctx.Fail(codeInitQrcodeFailed.WithError(err))

			return
		}

		ctx.Success(resp)
	}
}
