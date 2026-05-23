package usergroup

import (
	"errors"

	"github.com/xxcheng123/cloudpan189-share/internal/framework/httpcontext"
	"github.com/xxcheng123/cloudpan189-share/internal/services/usergroup"
	"gorm.io/gorm"
)

type modifyNameRequest = usergroup.ModifyNameRequest

// ModifyName 修改用户组名称
// @Summary 修改用户组名称
// @Description 修改指定用户组的名称，需要管理员权限
// @Tags 用户组管理
// @Accept json
// @Produce json
// @Param Authorization header string true "Bearer token"
// @Param request body modifyNameRequest true "修改名称请求"
// @Success 200 {object} httpcontext.Response "用户组名称修改成功"
// @Failure 400 {object} httpcontext.Response "参数验证失败，code=99998"
// @Failure 400 {object} httpcontext.Response "用户组名称修改失败，code=3003"
// @Failure 401 {object} httpcontext.Response "未授权访问"
// @Failure 403 {object} httpcontext.Response "权限不足"
// @Router /api/user_group/modify_name [post]
func (h *handler) ModifyName() httpcontext.HandlerFunc {
	return func(ctx *httpcontext.Context) {
		req := new(modifyNameRequest)
		if err := ctx.ShouldBindJSON(req); err != nil {
			ctx.AbortWithInvalidParams(err)

			return
		}

		if err := h.userGroupService.ModifyName(ctx.GetContext(), req); err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				ctx.Fail(codeUserGroupNotFound.WithError(err))

				return
			}

			ctx.Fail(codeModifyNameFailed.WithError(err))

			return
		}

		ctx.Success()
	}
}
