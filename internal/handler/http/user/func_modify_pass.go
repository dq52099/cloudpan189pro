package user

import (
	"errors"

	"github.com/xxcheng123/cloudpan189-share/internal/framework/httpcontext"
	"gorm.io/gorm"
)

type modifyPassRequest struct {
	ID       int64  `json:"id" binding:"required,min=1" example:"1001"`                    // 用户ID，必须大于0
	Password string `json:"password" binding:"required,min=6,max=20" example:"newpass123"` // 新密码，长度6-20位
}

// ModifyPass 修改用户密码
// @Summary 修改用户密码
// @Description 根据用户ID修改用户密码，需要管理员权限。密码会进行MD5加密存储，同时会自动增加用户版本号
// @Tags 用户管理
// @Accept json
// @Produce json
// @Param Authorization header string true "Bearer token"
// @Param request body modifyPassRequest true "修改密码请求"
// @Success 200 {object} httpcontext.Response "密码修改成功"
// @Failure 400 {object} httpcontext.Response "参数验证失败，code=99998"
// @Failure 400 {object} httpcontext.Response "密码修改失败，code=1010"
// @Failure 401 {object} httpcontext.Response "未授权访问"
// @Failure 403 {object} httpcontext.Response "权限不足"
// @Failure 404 {object} httpcontext.Response "用户不存在"
// @Router /api/user/modify_pass [post]
func (h *handler) ModifyPass() httpcontext.HandlerFunc {
	return func(ctx *httpcontext.Context) {
		req := new(modifyPassRequest)
		if err := ctx.ShouldBindJSON(req); err != nil {
			ctx.AbortWithInvalidParams(err)

			return
		}

		if err := h.userService.ModifyPass(ctx.GetContext(), req.ID, req.Password); err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				ctx.Fail(codeUserResourceMissing.WithError(err))

				return
			}

			ctx.Fail(codeModifyPassFailed.WithError(err))

			return
		}

		ctx.Success()
	}
}
