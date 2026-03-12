package storage

import (
	"github.com/xxcheng123/cloudpan189-share/internal/consts"
	"github.com/xxcheng123/cloudpan189-share/internal/framework/httpcontext"
	mountpointSvi "github.com/xxcheng123/cloudpan189-share/internal/services/mountpoint"
)

// selectListRequest 简化选择用查询参数
type selectListRequest struct {
	// Path 路径过滤，模糊匹配
	Path string `form:"path" example:"/aaa"`
	// Name 名称过滤，模糊匹配
	Name string `form:"name" example:"挂载点名称"`
}

// selectItem 下拉选择所需最简项
type selectItem struct {
	ID   int64  `json:"id" example:"123"`         // fileId
	Name string `json:"name" example:"我的挂载点"`     // 展示名称（挂载点名称）
	Path string `json:"path" example:"/path/aaa"` // 完整路径
}

// SelectList 获取存储挂载点简化列表（用于下拉选择）
// @Summary 获取存储挂载点简化列表
// @Description 返回用于选择的简化数据（不分页），仅包含必要字段
// @Tags 存储管理
// @Accept json
// @Produce json
// @Param Authorization header string true "Bearer token"
// @Param path query string false "路径过滤（模糊匹配）" example("/aaa")
// @Param name query string false "名称过滤（模糊匹配）" example("挂载点")
// @Success 200 {object} httpcontext.Response{data=[]selectItem} "获取简化列表成功"
// @Failure 400 {object} httpcontext.Response "参数验证失败，code=99998"
// @Failure 400 {object} httpcontext.Response "查询挂载点失败，code=3019"
// @Failure 401 {object} httpcontext.Response "未授权访问"
// @Failure 403 {object} httpcontext.Response "权限不足"
// @Router /api/storage/select_list [get]
func (h *handler) SelectList() httpcontext.HandlerFunc {
	return func(ctx *httpcontext.Context) {
		req := new(selectListRequest)
		if err := ctx.ShouldBindQuery(req); err != nil {
			ctx.AbortWithInvalidParams(err)

			return
		}

		userID := ctx.GetInt64(consts.CtxKeyUserId)
		isAdmin := ctx.GetBool(consts.CtxKeyIsAdmin)
		userGroupId := ctx.GetInt64(consts.CtxKeyUserGroupId)

		// 获取用户组绑定的文件ID
		var groupFileIds []int64
		if userGroupId > 0 {
			groupFileIds, _ = h.group2FileService.GetBindFiles(ctx.GetContext(), userGroupId)
		}

		mpReq := &mountpointSvi.ListRequest{
			NoPaginate:   true,
			FullPath:     req.Path,
			Name:         req.Name,
			UserID:       userID,
			IsAdmin:      isAdmin,
			GroupFileIds: groupFileIds,
		}

		list, err := h.mountPointService.List(ctx.GetContext(), mpReq)
		if err != nil {
			ctx.Fail(busCodeStorageQueryMountPointError.WithError(err))

			return
		}

		// 获取当前用户对这些挂载点的令牌绑定
		mountPointIds := make([]int64, 0, len(list))
		for _, mp := range list {
			mountPointIds = append(mountPointIds, mp.ID)
		}
		userTokenMap := make(map[int64]int64)
		if len(mountPointIds) > 0 {
			userTokenMap, _ = h.userMountPointTokenService.GetUserTokens(ctx.GetContext(), userID, mountPointIds)
		}

		// 用用户绑定的令牌替换原令牌
		for _, mp := range list {
			if userTokenId, ok := userTokenMap[mp.ID]; ok && userTokenId > 0 {
				mp.TokenId = userTokenId
			}
		}

		items := make([]*selectItem, 0, len(list))
		for _, mp := range list {
			items = append(items, &selectItem{
				ID:   mp.FileId,
				Name: mp.Name,
				Path: mp.FullPath,
			})
		}

		ctx.Success(items)
	}
}
