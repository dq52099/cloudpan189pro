package user

import (
	"errors"

	"github.com/xxcheng123/cloudpan189-share/internal/consts"
	"github.com/xxcheng123/cloudpan189-share/internal/framework/httpcontext"
	"github.com/xxcheng123/cloudpan189-share/internal/pkgs/utils"
	"gorm.io/gorm"
)

type modifyOwnPassRequest struct {
	OldPassword string `json:"oldPassword" binding:"required,min=6,max=20" example:"oldpass123"` // 旧密码，长度6-20位
	Password    string `json:"password" binding:"required,min=6,max=20" example:"newpass123"`    // 新密码，长度6-20位
}

// ModifyOwnPass 修改当前用户密码
// @Summary 修改当前用户密码
// @Description 用户修改自己的密码，需要提供旧密码进行验证。需要基础权限，密码会进行MD5加密存储，同时会自动增加用户版本号
// @Tags 用户管理
// @Accept json
// @Produce json
// @Param Authorization header string true "Bearer token"
// @Param request body modifyOwnPassRequest true "修改密码请求"
// @Success 200 {object} httpcontext.Response "密码修改成功"
// @Failure 400 {object} httpcontext.Response "参数验证失败，code=99998"
// @Failure 400 {object} httpcontext.Response "密码修改失败，code=1010（包括旧密码错误）"
// @Failure 401 {object} httpcontext.Response "未授权访问"
// @Failure 404 {object} httpcontext.Response "用户不存在"
// @Router /api/user/modify_own_pass [post]
func (h *handler) ModifyOwnPass() httpcontext.HandlerFunc {
	return func(ctx *httpcontext.Context) {
		req := new(modifyOwnPassRequest)
		if err := ctx.ShouldBindJSON(req); err != nil {
			ctx.AbortWithInvalidParams(err)

			return
		}

		uid := ctx.GetInt64(consts.CtxKeyUserId)
		if uid == 0 {
			ctx.Fail(codeModifyPassFailed)

			return
		}

		if !h.ensureUserService(ctx, codeModifyPassFailed) {
			return
		}

		// 查询用户信息进行密码验证
		user, err := h.userService.Query(ctx.GetContext(), uid)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				ctx.Fail(codeUserResourceMissing.WithError(err))

				return
			}

			ctx.Fail(codeModifyPassFailed.WithError(err))

			return
		}

		if user == nil {
			ctx.Fail(codeUserResourceMissing.WithError(gorm.ErrRecordNotFound))

			return
		}

		// 验证旧密码是否正确
		if user.Password != utils.MD5(req.OldPassword) {
			ctx.Fail(codeModifyPassFailed)

			return
		}

		// 调用 ModifyPass service 方法修改密码
		if err = h.userService.ModifyPass(ctx.GetContext(), uid, req.Password); err != nil {
			ctx.Fail(codeModifyPassFailed.WithError(err))

			return
		}

		ctx.Success()
	}
}
