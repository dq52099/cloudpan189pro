package user

import (
	"errors"

	"github.com/xxcheng123/cloudpan189-share/internal/consts"
	"github.com/xxcheng123/cloudpan189-share/internal/framework/httpcontext"
	_ "github.com/xxcheng123/cloudpan189-share/internal/repository/models"
	"gorm.io/gorm"
)

// Info 获取当前用户信息
// @Summary 获取当前用户信息
// @Description 获取当前登录用户的详细信息，需要基础权限。从JWT token中获取用户ID
// @Tags 用户管理
// @Accept json
// @Produce json
// @Param Authorization header string true "Bearer token"
// @Success 200 {object} httpcontext.Response{data=userInfo} "获取用户信息成功"
// @Failure 400 {object} httpcontext.Response "用户信息获取失败，code=1012"
// @Failure 401 {object} httpcontext.Response "未授权访问"
// @Router /api/user/info [get]
func (h *handler) Info() httpcontext.HandlerFunc {
	return func(ctx *httpcontext.Context) {
		uid := ctx.GetInt64(consts.CtxKeyUserId)
		if uid == 0 {
			ctx.Fail(codeUserInfoFailed)

			return
		}

		if !h.ensureUserService(ctx, codeUserInfoFailed) {
			return
		}

		user, err := h.userService.Query(ctx.GetContext(), uid)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				ctx.Fail(codeUserResourceMissing.WithError(err))

				return
			}

			ctx.Fail(codeUserInfoFailed.WithError(err))

			return
		}

		if user == nil {
			ctx.Fail(codeUserResourceMissing.WithError(gorm.ErrRecordNotFound))

			return
		}

		groupName := "默认用户组"

		if user.GroupID > 0 {
			if !h.ensureUserGroupService(ctx, codeUserInfoFailed) {
				return
			}

			group, err := h.userGroupService.Query(ctx.GetContext(), user.GroupID)
			if err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					groupName = "用户组不存在"
				} else {
					ctx.Fail(codeUserInfoFailed.WithError(err))

					return
				}
			} else if group != nil {
				groupName = group.Name
			} else {
				groupName = "用户组不存在"
			}
		}

		info := &userInfo{
			User:      user,
			GroupName: groupName,
		}

		ctx.Success(info)
	}
}
