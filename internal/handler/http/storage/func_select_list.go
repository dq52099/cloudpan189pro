package storage

import (
	"github.com/xxcheng123/cloudpan189-share/internal/consts"
	"github.com/xxcheng123/cloudpan189-share/internal/framework/httpcontext"
	mountpointSvi "github.com/xxcheng123/cloudpan189-share/internal/services/mountpoint"
)

// selectListRequest 简化选择列表查询参数
type selectListRequest struct {
	CurrentPage int    `form:"currentPage,default=1" binding:"omitempty,min=1" example:"1"`
	PageSize    int    `form:"pageSize,default=10" binding:"omitempty,min=1" example:"10"`
	NoPaginate  bool   `form:"noPaginate" binding:"omitempty" example:"false"`
	Path        string `form:"path" example:"/aaa"`
	Name        string `form:"name" example:"挂载点名称"`
}

// selectItem 下拉选择所需最简项
type selectItem struct {
	ID   int64  `json:"id" example:"123"`         // fileId
	Name string `json:"name" example:"我的挂载点"`     // 展示名称
	Path string `json:"path" example:"/path/aaa"` // 完整路径
}

type selectListResponse struct {
	Total       int64         `json:"total" example:"100"`
	CurrentPage int           `json:"currentPage" example:"1"`
	PageSize    int           `json:"pageSize" example:"10"`
	Data        []*selectItem `json:"data"`
}

// SelectList 获取存储挂载点简化列表（用于下拉选择）
// @Summary 获取存储挂载点简化列表
// @Description 返回用于选择的简化数据，支持分页和搜索，仅包含必要字段
// @Tags 存储管理
// @Accept json
// @Produce json
// @Param Authorization header string true "Bearer token"
// @Param currentPage query int false "当前页码，默认为1" default(1)
// @Param pageSize query int false "每页大小，默认为10" default(10)
// @Param noPaginate query bool false "是否不分页" default(false)
// @Param path query string false "路径过滤（模糊匹配）" example("/aaa")
// @Param name query string false "名称过滤（模糊匹配）" example("挂载点")
// @Success 200 {object} httpcontext.Response{data=selectListResponse} "获取简化列表成功"
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

		var groupFileIds []int64
		if userGroupId > 0 {
			groupFileIds, _ = h.group2FileService.GetBindFiles(ctx.GetContext(), userGroupId)
		}

		mpReq := &mountpointSvi.ListRequest{
			CurrentPage:  req.CurrentPage,
			PageSize:     req.PageSize,
			NoPaginate:   req.NoPaginate,
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

		var total int64
		if req.NoPaginate {
			total = int64(len(list))
		} else {
			total, err = h.mountPointService.Count(ctx.GetContext(), mpReq)
			if err != nil {
				ctx.Fail(busCodeStorageQueryMountPointError.WithError(err))

				return
			}
		}

		mountPointIDs := make([]int64, 0, len(list))
		for _, mp := range list {
			mountPointIDs = append(mountPointIDs, mp.ID)
		}

		userTokenMap := make(map[int64]int64)
		if len(mountPointIDs) > 0 {
			userTokenMap, _ = h.userMountPointTokenService.GetUserTokens(ctx.GetContext(), userID, mountPointIDs)
		}

		for _, mp := range list {
			if userTokenID, ok := userTokenMap[mp.ID]; ok && userTokenID > 0 {
				mp.TokenId = userTokenID
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

		ctx.Success(&selectListResponse{
			Total:       total,
			CurrentPage: req.CurrentPage,
			PageSize:    req.PageSize,
			Data:        items,
		})
	}
}
