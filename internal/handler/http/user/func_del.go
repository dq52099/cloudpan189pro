package user

import (
	"errors"

	"gorm.io/gorm"

	"github.com/xxcheng123/cloudpan189-share/internal/framework/httpcontext"
	"github.com/xxcheng123/cloudpan189-share/internal/services/user"
)

type delRequest = user.DelRequest

// Del 删除用户
// @Summary 删除用户
// @Description 根据用户ID删除用户，需要管理员权限。注意：ID为1的创始人用户不能删除
// @Tags 用户管理
// @Accept json
// @Produce json
// @Param Authorization header string true "Bearer token"
// @Param request body delRequest true "删除用户信息"
// @Success 200 {object} httpcontext.Response "用户删除成功"
// @Failure 400 {object} httpcontext.Response "参数验证失败，code=99998"
// @Failure 400 {object} httpcontext.Response "用户删除失败，code=1007"
// @Failure 401 {object} httpcontext.Response "未授权访问"
// @Failure 403 {object} httpcontext.Response "权限不足"
// @Failure 404 {object} httpcontext.Response "用户不存在"
// @Router /api/user/del [post]
func (h *handler) Del() httpcontext.HandlerFunc {
	return func(ctx *httpcontext.Context) {
		req := new(delRequest)
		if err := ctx.ShouldBindJSON(req); err != nil {
			ctx.AbortWithInvalidParams(err)

			return
		}

		if !h.ensureUserService(ctx, codeDelUserFailed) {
			return
		}

		if err := h.userService.Del(ctx.GetContext(), req); err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				ctx.Fail(codeUserResourceMissing.WithError(err))

				return
			}

			ctx.Fail(codeDelUserFailed.WithError(err))

			return
		}

		ctx.Success()
	}
}
