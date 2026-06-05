package user

import (
	"errors"

	"github.com/xxcheng123/cloudpan189-share/internal/framework/httpcontext"
	"github.com/xxcheng123/cloudpan189-share/internal/pkgs/utils"
	"gorm.io/gorm"
)

type updateRequest struct {
	ID     int64 `json:"id" binding:"required,min=1" example:"1001"` // 用户ID，必须大于0
	Status *int8 `json:"status" binding:"omitempty,oneof=1 2"`
}

// Update 更新用户信息
// @Summary 更新用户信息
// @Description 根据用户ID更新用户权限等信息，需要管理员权限。目前支持更新权限字段
// @Tags 用户管理
// @Accept json
// @Produce json
// @Param Authorization header string true "Bearer token"
// @Param request body updateRequest true "更新用户信息"
// @Success 200 {object} httpcontext.Response "用户信息更新成功"
// @Failure 400 {object} httpcontext.Response "参数验证失败，code=99998"
// @Failure 400 {object} httpcontext.Response "请填写需要更新的字段，code=1013"
// @Failure 400 {object} httpcontext.Response "用户更新失败，code=1008"
// @Failure 401 {object} httpcontext.Response "未授权访问"
// @Failure 403 {object} httpcontext.Response "权限不足"
// @Router /api/user/update [post]
func (h *handler) Update() httpcontext.HandlerFunc {
	return func(ctx *httpcontext.Context) {
		req := new(updateRequest)
		if err := ctx.ShouldBindJSON(req); err != nil {
			ctx.AbortWithInvalidParams(err)

			return
		}

		fields := make([]utils.Field, 0)

		if req.Status != nil {
			fields = append(fields, utils.Field{Key: "status", Value: *req.Status})
		}

		if len(fields) == 0 {
			ctx.Fail(codeNoUpdateFields)

			return
		}

		if !h.ensureUserService(ctx, codeUpdateUserFailed) {
			return
		}

		err := h.userService.Update(ctx.GetContext(), req.ID, fields...)
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
