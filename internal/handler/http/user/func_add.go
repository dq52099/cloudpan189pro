package user

import (
	"github.com/xxcheng123/cloudpan189-share/internal/framework/httpcontext"
	"github.com/xxcheng123/cloudpan189-share/internal/services/user"
)

type addRequest = user.AddRequest

type addResponse = user.AddResponse

// Add 添加用户
// @Summary 添加用户
// @Description 创建新用户账户，需要管理员权限。
// @Tags 用户管理
// @Accept json
// @Produce json
// @Param Authorization header string true "Bearer token"
// @Param request body addRequest true "用户信息"
// @Success 200 {object} httpcontext.Response{data=addResponse} "用户创建成功"
// @Failure 400 {object} httpcontext.Response "参数验证失败，code=99998"
// @Failure 400 {object} httpcontext.Response "用户添加失败，code=1001"
// @Failure 401 {object} httpcontext.Response "未授权访问"
// @Failure 403 {object} httpcontext.Response "权限不足"
// @Router /api/user/add [post]
func (h *handler) Add() httpcontext.HandlerFunc {
	return func(ctx *httpcontext.Context) {
		req := new(addRequest)
		if err := ctx.ShouldBindJSON(req); err != nil {
			ctx.AbortWithInvalidParams(err)

			return
		}

		if !h.ensureUserService(ctx, codeAddUserFailed) {
			return
		}

		var (
			resp *addResponse
			err  error
		)
		if resp, err = h.userService.Add(ctx.GetContext(), req); err != nil {
			ctx.Fail(codeAddUserFailed.WithError(err))

			return
		}

		ctx.Success(resp)
	}
}
