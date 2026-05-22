package cloudtoken

import (
	"errors"
	"strconv"

	"github.com/xxcheng123/cloudpan189-share/internal/consts"
	"github.com/xxcheng123/cloudpan189-share/internal/framework/httpcontext"
	"github.com/xxcheng123/cloudpan189-share/internal/repository/models"
	"gorm.io/gorm"
)

// CloudToken 云盘令牌模型类型别名
type CloudToken = models.CloudToken

// Query 查询云盘令牌详情
// @Summary 查询云盘令牌详情
// @Description 根据云盘令牌ID查询详细信息
// @Tags 云盘令牌管理
// @Accept json
// @Produce json
// @Param Authorization header string true "Bearer token"
// @Param id path int true "云盘令牌ID"
// @Success 200 {object} httpcontext.Response{data=models.CloudToken} "查询成功"
// @Failure 400 {object} httpcontext.Response "参数验证失败，code=99998"
// @Failure 400 {object} httpcontext.Response "查询云盘令牌失败，code=5007"
// @Failure 401 {object} httpcontext.Response "未授权访问"
// @Failure 403 {object} httpcontext.Response "权限不足"
// @Failure 404 {object} httpcontext.Response "云盘令牌不存在"
// @Router /api/cloud_token/{id} [get]
func (h *handler) Query() httpcontext.HandlerFunc {
	return func(ctx *httpcontext.Context) {
		idStr := ctx.Param("id")

		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			ctx.AbortWithInvalidParams(err)

			return
		}

		cloudToken, err := h.cloudTokenService.QueryAccessible(
			ctx.GetContext(),
			id,
			ctx.GetInt64(consts.CtxKeyUserId),
			ctx.GetBool(consts.CtxKeyIsAdmin),
		)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				ctx.Fail(codeTokenNotFound.WithError(err))

				return
			}

			ctx.Fail(codeQueryFailed.WithError(err))

			return
		}

		ctx.Success(cloudToken)
	}
}
