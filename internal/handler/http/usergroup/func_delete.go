package usergroup

import (
	"errors"

	"github.com/xxcheng123/cloudpan189-share/internal/framework/httpcontext"
	"github.com/xxcheng123/cloudpan189-share/internal/services/usergroup"
	"gorm.io/gorm"
)

type deleteRequest = usergroup.DeleteRequest

// Delete 删除用户组
// @Summary 删除用户组
// @Description 删除指定用户组，需要管理员权限
// @Tags 用户组管理
// @Accept json
// @Produce json
// @Param Authorization header string true "Bearer token"
// @Param request body deleteRequest true "删除请求"
// @Success 200 {object} httpcontext.Response "用户组删除成功"
// @Failure 400 {object} httpcontext.Response "参数验证失败，code=99998"
// @Failure 400 {object} httpcontext.Response "用户组删除失败，code=3002"
// @Failure 404 {object} httpcontext.Response "用户组不存在"
// @Failure 401 {object} httpcontext.Response "未授权访问"
// @Failure 403 {object} httpcontext.Response "权限不足"
// @Router /api/user_group/delete [post]
func (h *handler) Delete() httpcontext.HandlerFunc {
	return func(ctx *httpcontext.Context) {
		req := new(deleteRequest)
		if err := ctx.ShouldBindJSON(req); err != nil {
			ctx.AbortWithInvalidParams(err)

			return
		}

		if err := h.userGroupService.Delete(ctx.GetContext(), req); err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				ctx.Fail(codeUserGroupNotFound.WithError(err))

				return
			}

			ctx.Fail(codeDeleteUserGroupFailed.WithError(err))

			return
		}

		ctx.Success()
	}
}
