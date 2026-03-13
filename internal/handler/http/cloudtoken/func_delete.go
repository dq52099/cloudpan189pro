package cloudtoken

import (
	"github.com/xxcheng123/cloudpan189-share/internal/consts"
	"github.com/xxcheng123/cloudpan189-share/internal/framework/httpcontext"
	"github.com/xxcheng123/cloudpan189-share/internal/services/cloudtoken"
	mountPointSvi "github.com/xxcheng123/cloudpan189-share/internal/services/mountpoint"
)

type deleteRequest = cloudtoken.DeleteRequest

// Delete 删除云盘令牌
// @Summary 删除云盘令牌
// @Description 根据云盘令牌ID删除指定的云盘令牌
// @Tags 云盘令牌管理
// @Accept json
// @Produce json
// @Param Authorization header string true "Bearer token"
// @Param request body deleteRequest true "删除请求"
// @Success 200 {object} httpcontext.Response "删除成功"
// @Failure 400 {object} httpcontext.Response "参数验证失败，code=99998"
// @Failure 400 {object} httpcontext.Response "删除云盘令牌失败，code=5004"
// @Failure 401 {object} httpcontext.Response "未授权访问"
// @Failure 403 {object} httpcontext.Response "权限不足"
// @Router /api/cloud_token/delete [post]
func (h *handler) Delete() httpcontext.HandlerFunc {
	return func(ctx *httpcontext.Context) {
		req := new(deleteRequest)
		if err := ctx.ShouldBindJSON(req); err != nil {
			ctx.AbortWithInvalidParams(err)

			return
		}

		// 获取当前用户信息用于权限控制
		req.UserID = ctx.GetInt64(consts.CtxKeyUserId)
		req.IsAdmin = ctx.GetBool(consts.CtxKeyIsAdmin)

		if count, err := h.mountPointService.Count(ctx.GetContext(), &mountPointSvi.ListRequest{
			TokenId: &req.ID,
		}); err != nil {
			ctx.Fail(codeQueryFailed.WithError(err))

			return
		} else if count > 0 && !req.IsAdmin {
			ctx.Fail(codeMountPointUsed)

			return
		}

		if err := h.cloudTokenService.Delete(ctx.GetContext(), req); err != nil {
			ctx.Fail(codeDeleteFailed.WithError(err))

			return
		}

		ctx.Success()
	}
}
