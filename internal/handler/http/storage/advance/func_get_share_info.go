package advance

import (
	"github.com/xxcheng123/cloudpan189-share/internal/framework/httpcontext"
	"github.com/xxcheng123/cloudpan189-share/internal/pkgs/utils"
)

type getShareInfoRequest struct {
	ShareCode       string `form:"shareCode" binding:"required"`
	ShareAccessCode string `form:"shareAccessCode"`
}

// GetShareInfo 获取分享信息
// @Summary 获取分享信息
// @Description 根据分享码获取分享的详细信息，支持直接传入完整分享链接
// @Tags 存储高级功能
// @Accept json
// @Produce json
// @Param Authorization header string true "Bearer token"
// @Param shareCode query string true "分享码或完整链接" example("https://cloud.189.cn/t/abc12345")
// @Param shareAccessCode query string false "分享访问码" example("1234")
// @Success 200 {object} httpcontext.Response{data=cloudbridge.ShareInfo} "获取分享信息成功"
// @Failure 400 {object} httpcontext.Response "参数验证失败"
// @Failure 400 {object} httpcontext.Response "获取分享详情失败"
// @Failure 401 {object} httpcontext.Response "未授权访问"
// @Router /api/storage/advance/share_info [get]
// @Router /api/public/share_info [get]
func (h *handler) GetShareInfo() httpcontext.HandlerFunc {
	return func(ctx *httpcontext.Context) {
		req := new(getShareInfoRequest)
		if err := ctx.ShouldBindQuery(req); err != nil {
			ctx.AbortWithInvalidParams(err)

			return
		}

		pureShareCode, pureAccessCode := utils.ParseCloud189ShareCode(req.ShareCode, req.ShareAccessCode)
		if !utils.IsCloud189ShareCode(pureShareCode) {
			ctx.Fail(codeStorageAdvanceGetShareInfoError.WithMessage("无法从分享链接中提取有效分享码"))

			return
		}

		if pureAccessCode != "" && !utils.IsCloud189AccessCode(pureAccessCode) {
			ctx.Fail(codeStorageAdvanceGetShareInfoError.WithMessage("访问码格式无效"))

			return
		}

		if !h.ensureCloudBridgeService(ctx, codeStorageAdvanceGetShareInfoError) {
			return
		}

		shareInfo, err := h.cloudBridgeService.GetShareInfo(ctx.GetContext(), pureShareCode, pureAccessCode)
		if err != nil {
			ctx.Fail(codeStorageAdvanceGetShareInfoError.WithError(err))

			return
		}

		if shareInfo == nil {
			ctx.Fail(codeStorageAdvanceGetShareInfoError.WithMessage("未查询到分享信息"))

			return
		}

		ctx.Success(shareInfo)
	}
}
