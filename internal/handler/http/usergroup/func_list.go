package usergroup

import (
	"github.com/xxcheng123/cloudpan189-share/internal/framework/httpcontext"
	"github.com/xxcheng123/cloudpan189-share/internal/repository/models"
	userSvi "github.com/xxcheng123/cloudpan189-share/internal/services/user"
	"github.com/xxcheng123/cloudpan189-share/internal/services/usergroup"
)

type (
	listRequest = usergroup.ListRequest

	// 自定义用户组响应结构体
	userGroupInfo struct {
		*models.UserGroup
		UserCount int64 `json:"userCount" example:"5"` // 该用户组下的用户数量
	}

	listResponse struct {
		Total       int64            `json:"total" example:"100"`     // 总记录数
		CurrentPage int              `json:"currentPage" example:"1"` // 当前页码
		PageSize    int              `json:"pageSize" example:"10"`   // 每页大小
		Data        []*userGroupInfo `json:"data"`                    // 用户组列表数据
	}
)

func compactUserGroups(list []*models.UserGroup) []*models.UserGroup {
	if len(list) == 0 {
		return list
	}

	writeIndex := 0

	for _, item := range list {
		if item == nil {
			continue
		}

		list[writeIndex] = item
		writeIndex++
	}

	return list[:writeIndex]
}

// List 获取用户组列表
// @Summary 获取用户组列表
// @Description 获取用户组列表，支持分页和搜索
// @Tags 用户组管理
// @Accept json
// @Produce json
// @Param Authorization header string true "Bearer token"
// @Param currentPage query int false "当前页码，默认为1" default(1)
// @Param pageSize query int false "每页大小，默认为10" default(10)
// @Param noPaginate query bool false "是否不分页，默认false" default(false)
// @Param name query string false "用户组名称模糊搜索"
// @Success 200 {object} httpcontext.Response{data=listResponse} "获取用户组列表成功"
// @Failure 400 {object} httpcontext.Response "参数验证失败，code=99998"
// @Failure 400 {object} httpcontext.Response "用户组列表获取失败，code=3006"
// @Failure 401 {object} httpcontext.Response "未授权访问"
// @Failure 403 {object} httpcontext.Response "权限不足"
// @Router /api/user_group/list [get]
func (h *handler) List() httpcontext.HandlerFunc {
	return func(ctx *httpcontext.Context) {
		req := new(listRequest)
		if err := ctx.ShouldBindQuery(req); err != nil {
			ctx.AbortWithInvalidParams(err)

			return
		}

		if !h.ensureUserGroupService(ctx, codeListUserGroupFailed) {
			return
		}

		if !h.ensureUserService(ctx, codeListUserGroupFailed) {
			return
		}

		list, err := h.userGroupService.List(ctx.GetContext(), req)
		if err != nil {
			ctx.Fail(codeListUserGroupFailed.WithError(err))

			return
		}

		list = compactUserGroups(list)

		var total int64
		if !req.NoPaginate {
			total, err = h.userGroupService.Count(ctx.GetContext(), req)
			if err != nil {
				ctx.Fail(codeListUserGroupFailed.WithError(err))

				return
			}
		} else {
			total = int64(len(list))
		}

		// 获取每个用户组的用户数量
		groupIds := make([]int64, 0, len(list))
		for _, item := range list {
			groupIds = append(groupIds, item.ID)
		}

		// 查询每个用户组的用户数量
		userCountMap := make(map[int64]int64)

		for _, groupId := range groupIds {
			userCountReq := &userSvi.ListRequest{
				GroupId: &groupId,
			}

			count, err := h.userService.Count(ctx.GetContext(), userCountReq)
			if err != nil {
				ctx.Fail(codeListUserGroupFailed.WithError(err))

				return
			}

			userCountMap[groupId] = count
		}

		// 构造响应数据
		respList := make([]*userGroupInfo, 0, len(list))
		for _, group := range list {
			respList = append(respList, &userGroupInfo{
				UserGroup: group,
				UserCount: userCountMap[group.ID],
			})
		}

		ctx.Success(&listResponse{
			Total:       total,
			Data:        respList,
			PageSize:    req.PageSize,
			CurrentPage: req.CurrentPage,
		})
	}
}
