package user

import (
	"errors"

	"github.com/xxcheng123/cloudpan189-share/internal/framework/httpcontext"
	userSvi "github.com/xxcheng123/cloudpan189-share/internal/services/user"
	"gorm.io/gorm"
)

type bindGroupRequest = userSvi.BindGroupRequest

type bindGroupResponse struct {
	UserID    int64  `json:"userId" example:"1001"`    // 用户ID
	GroupID   int64  `json:"groupId" example:"2"`      // 用户组ID
	GroupName string `json:"groupName" example:"管理员组"` // 用户组名称
}

// BindGroup 绑定用户到用户组
// @Summary 绑定用户到用户组
// @Description 将指定用户绑定到指定用户组，需要管理员权限。GroupID为0表示绑定到默认用户组
// @Tags 用户管理
// @Accept json
// @Produce json
// @Param Authorization header string true "Bearer token"
// @Param request body bindGroupRequest true "绑定用户组请求"
// @Success 200 {object} httpcontext.Response{data=bindGroupResponse} "用户组绑定成功"
// @Failure 400 {object} httpcontext.Response "参数验证失败，code=99998"
// @Failure 400 {object} httpcontext.Response "绑定用户组失败，code=1011"
// @Failure 401 {object} httpcontext.Response "未授权访问"
// @Failure 403 {object} httpcontext.Response "权限不足"
// @Failure 404 {object} httpcontext.Response "用户不存在或用户组不存在"
// @Router /api/user/bind_group [post]
func (h *handler) BindGroup() httpcontext.HandlerFunc {
	return func(ctx *httpcontext.Context) {
		req := new(bindGroupRequest)
		if err := ctx.ShouldBindJSON(req); err != nil {
			ctx.AbortWithInvalidParams(err)

			return
		}

		if !h.ensureUserService(ctx, codeBindGroupFailed) {
			return
		}

		serviceReq := &bindGroupRequest{
			UserID:  req.UserID,
			GroupID: req.GroupID,
		}

		// 检查用户是否存在
		userInfo, err := h.userService.Query(ctx.GetContext(), req.UserID)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				ctx.Fail(codeUserResourceMissing.WithError(err))

				return
			}

			ctx.Fail(codeBindGroupFailed.WithError(err))

			return
		}

		if userInfo == nil {
			ctx.Fail(codeUserResourceMissing.WithError(gorm.ErrRecordNotFound))

			return
		}

		var groupName = "默认用户组"

		// 检查用户组是否存在
		if req.GroupID > 0 {
			if !h.ensureUserGroupService(ctx, codeBindGroupFailed) {
				return
			}

			if groupInfo, err := h.userGroupService.Query(ctx.GetContext(), req.GroupID); err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					ctx.Fail(codeUserGroupMissing.WithError(err))

					return
				}

				ctx.Fail(codeBindGroupFailed.WithError(err))

				return
			} else {
				if groupInfo == nil {
					ctx.Fail(codeUserGroupMissing.WithError(gorm.ErrRecordNotFound))

					return
				}

				groupName = groupInfo.Name
			}
		}

		if err := h.userService.BindGroup(ctx.GetContext(), serviceReq); err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				ctx.Fail(codeUserResourceMissing.WithError(err))

				return
			}

			ctx.Fail(codeBindGroupFailed.WithError(err))

			return
		}

		ctx.Success(&bindGroupResponse{
			UserID:    req.UserID,
			GroupID:   req.GroupID,
			GroupName: groupName,
		})
	}
}
