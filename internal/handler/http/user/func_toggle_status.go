package user

import (
	"errors"

	"github.com/xxcheng123/cloudpan189-share/internal/framework/httpcontext"
	"github.com/xxcheng123/cloudpan189-share/internal/pkgs/utils"
	"gorm.io/gorm"
)

type toggleStatusRequest struct {
	ID     int64 `json:"id" binding:"required,min=1" example:"1001"`      // 用户ID，必须大于0
	Status int8  `json:"status" binding:"required,oneof=1 2" example:"1"` // 用户状态：1=启用，2=禁用
}

// ToggleStatus 切换用户状态
// @Summary 切换用户状态
// @Description 设置用户的启用/禁用状态，需要管理员权限。状态为1表示启用，状态为2表示禁用
// @Tags 用户管理
// @Accept json
// @Produce json
// @Param Authorization header string true "Bearer token"
// @Param request body toggleStatusRequest true "用户状态信息"
// @Success 200 {object} httpcontext.Response "用户状态设置成功"
// @Failure 400 {object} httpcontext.Response "参数验证失败，code=99998"
// @Failure 400 {object} httpcontext.Response "用户更新失败，code=1008"
// @Failure 401 {object} httpcontext.Response "未授权访问"
// @Failure 403 {object} httpcontext.Response "权限不足"
// @Router /api/user/toggle_status [post]
func (h *handler) ToggleStatus() httpcontext.HandlerFunc {
	return func(ctx *httpcontext.Context) {
		req := new(toggleStatusRequest)
		if err := ctx.ShouldBindJSON(req); err != nil {
			ctx.AbortWithInvalidParams(err)

			return
		}

		// 更新用户状态
		err := h.userService.Update(ctx.GetContext(), req.ID, utils.Field{Key: "status", Value: req.Status})
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				ctx.Fail(codeUserResourceMissing.WithError(err))

				return
			}

			ctx.Fail(codeUpdateUserFailed.WithError(err))

			return
		}

		ctx.Success()
	}
}
