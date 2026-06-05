package user

import (
	"github.com/samber/lo"
	"github.com/xxcheng123/cloudpan189-share/internal/framework/httpcontext"
	"github.com/xxcheng123/cloudpan189-share/internal/repository/models"
	userSvi "github.com/xxcheng123/cloudpan189-share/internal/services/user"
)

type (
	listRequest = userSvi.ListRequest

	// 自定义用户响应结构体
	userInfo struct {
		*models.User
		GroupName string `json:"groupName" example:"管理员组"` // 用户组名称
	}

	listResponse struct {
		Total       int64       `json:"total" example:"100"`     // 总记录数
		CurrentPage int         `json:"currentPage" example:"1"` // 当前页码
		PageSize    int         `json:"pageSize" example:"10"`   // 每页大小
		Data        []*userInfo `json:"data"`                    // 用户列表数据
	}
)

func compactUsers(list []*models.User) []*models.User {
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

// List 获取用户列表
// @Summary 获取用户列表
// @Description 分页获取用户列表，支持按用户名模糊搜索，需要管理员权限
// @Tags 用户管理
// @Accept json
// @Produce json
// @Param Authorization header string true "Bearer token"
// @Param currentPage query int false "当前页码，默认为1" default(1)
// @Param pageSize query int false "每页大小，默认为10" default(10)
// @Param noPaginate query bool false "是否不分页，默认false" default(false)
// @Param username query string false "用户名模糊搜索"
// @Success 200 {object} httpcontext.Response{data=listResponse} "获取用户列表成功"
// @Failure 400 {object} httpcontext.Response "参数验证失败，code=99998"
// @Failure 400 {object} httpcontext.Response "用户列表获取失败，code=1009"
// @Failure 401 {object} httpcontext.Response "未授权访问"
// @Failure 403 {object} httpcontext.Response "权限不足"
// @Router /api/user/list [get]
func (h *handler) List() httpcontext.HandlerFunc {
	return func(ctx *httpcontext.Context) {
		req := new(listRequest)
		if err := ctx.ShouldBindQuery(req); err != nil {
			ctx.AbortWithInvalidParams(err)

			return
		}

		if !h.ensureUserService(ctx, codeListUserFailed) {
			return
		}

		userList, err := h.userService.List(ctx.GetContext(), req)
		if err != nil {
			ctx.Fail(codeListUserFailed.WithError(err))

			return
		}

		userList = compactUsers(userList)

		var total int64
		if !req.NoPaginate {
			total, err = h.userService.Count(ctx.GetContext(), req)
			if err != nil {
				ctx.Fail(codeListUserFailed.WithError(err))

				return
			}
		} else {
			total = int64(len(userList))
		}

		groupIds := make([]int64, 0, len(userList))
		for _, user := range userList {
			if user.GroupID == 0 {
				continue
			}

			groupIds = append(groupIds, user.GroupID)
		}

		groupIds = lo.Uniq(groupIds)

		groupList := make([]*models.UserGroup, 0)

		if len(groupIds) > 0 {
			if !h.ensureUserGroupService(ctx, codeListUserFailed) {
				return
			}

			groupList, err = h.userGroupService.BatchQuery(ctx.GetContext(), groupIds)
			if err != nil {
				ctx.Fail(codeListUserFailed.WithError(err))

				return
			}
		}

		groupMap := make(map[int64]*models.UserGroup, len(groupList))
		for _, item := range groupList {
			if item == nil {
				continue
			}

			groupMap[item.ID] = item
		}

		var respUserList = make([]*userInfo, 0)

		for _, user := range userList {
			groupName := "用户组查询失败"
			if user.GroupID == 0 {
				groupName = "默认用户组"
			} else if v, ok := groupMap[user.GroupID]; ok {
				groupName = v.Name
			}

			respUserList = append(respUserList, &userInfo{
				User:      user,
				GroupName: groupName,
			})
		}

		ctx.Success(&listResponse{
			Total:       total,
			Data:        respUserList,
			PageSize:    req.PageSize,
			CurrentPage: req.CurrentPage,
		})
	}
}
