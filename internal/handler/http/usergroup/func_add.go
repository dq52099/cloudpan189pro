package usergroup

import (
	"github.com/xxcheng123/cloudpan189-share/internal/framework/httpcontext"
	"github.com/xxcheng123/cloudpan189-share/internal/services/usergroup"
)

type addRequest = usergroup.AddRequest

type addResponse = usergroup.AddResponse

// Add 添加用户组
// @Summary 添加用户组
// @Description 创建新用户组，需要管理员权限
// @Tags 用户组管理
// @Accept json
// @Produce json
// @Param Authorization header string true "Bearer token"
// @Param request body addRequest true "用户组信息"
// @Success 200 {object} httpcontext.Response{data=addResponse} "用户组创建成功"
// @Failure 400 {object} httpcontext.Response "参数验证失败，code=99998"
// @Failure 400 {object} httpcontext.Response "用户组添加失败，code=3001"
// @Failure 401 {object} httpcontext.Response "未授权访问"
// @Failure 403 {object} httpcontext.Response "权限不足"
// @Router /api/user_group/add [post]
func (h *handler) Add() httpcontext.HandlerFunc {
	return func(ctx *httpcontext.Context) {
		req := new(addRequest)
		if err := ctx.ShouldBindJSON(req); err != nil {
			ctx.AbortWithInvalidParams(err)

			return
		}

		if !h.ensureUserGroupService(ctx, codeAddUserGroupFailed) {
			return
		}

		var (
			resp *addResponse
			err  error
		)
		if resp, err = h.userGroupService.Add(ctx.GetContext(), req); err != nil {
			ctx.Fail(codeAddUserGroupFailed.WithError(err))

			return
		}

		ctx.Success(resp)
	}
}
