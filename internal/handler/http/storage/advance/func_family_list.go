package advance

import (
	"github.com/xxcheng123/cloudpan189-share/internal/consts"
	"github.com/xxcheng123/cloudpan189-share/internal/framework/httpcontext"
	"github.com/xxcheng123/cloudpan189-share/internal/services/cloudbridge"
)

type familyListRequest struct {
	CloudToken int64 `form:"cloudToken" binding:"required"`
}

// FamilyList 获取家庭云列表
// @Summary 获取家庭云列表
// @Description 根据云盘令牌获取用户的家庭云列表
// @Tags 存储管理-高级功能
// @Accept json
// @Produce json
// @Param Authorization header string true "Bearer token"
// @Param cloudToken query int true "云盘令牌ID"
// @Success 200 {object} httpcontext.Response{data=cloudbridge.GetFamilyListResponse} "获取家庭云列表成功"
// @Failure 400 {object} httpcontext.Response "参数验证失败，code=99998"
// @Failure 400 {object} httpcontext.Response "云盘令牌不存在，code=8001"
// @Failure 400 {object} httpcontext.Response "获取家庭云列表失败，code=8002"
// @Failure 401 {object} httpcontext.Response "未授权访问"
// @Failure 403 {object} httpcontext.Response "权限不足"
// @Router /api/storage/advance/family/list [get]
func (h *handler) FamilyList() httpcontext.HandlerFunc {
	return func(ctx *httpcontext.Context) {
		req := new(familyListRequest)
		if err := ctx.ShouldBindQuery(req); err != nil {
			ctx.AbortWithInvalidParams(err)

			return
		}

		token, err := h.cloudTokenService.QueryAccessible(
			ctx.GetContext(),
			req.CloudToken,
			ctx.GetInt64(consts.CtxKeyUserId),
			ctx.GetBool(consts.CtxKeyIsAdmin),
		)
		if err != nil {
			ctx.Fail(codeStorageAdvanceCloudTokenNotExist.WithError(err))

			return
		}

		// 构造认证令牌
		authToken := cloudbridge.NewAuthToken(token.AccessToken, token.ExpiresIn)

		// 获取家庭云列表
		familyList, err := h.cloudBridgeService.FamilyList(ctx.GetContext(), authToken)
		if err != nil {
			ctx.Fail(codeStorageAdvanceQueryPathFailed.WithError(err))

			return
		}

		ctx.Success(familyList)
	}
}
