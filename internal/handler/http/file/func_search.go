package file

import (
	"path"

	"github.com/xxcheng123/cloudpan189-share/internal/consts"
	"github.com/xxcheng123/cloudpan189-share/internal/framework/httpcontext"
	"github.com/xxcheng123/cloudpan189-share/internal/pkgs/ptr"
	"github.com/xxcheng123/cloudpan189-share/internal/repository/models"
	virtualfileSvi "github.com/xxcheng123/cloudpan189-share/internal/services/virtualfile"
)

type searchRequest struct {
	Keyword     string `form:"keyword" binding:"omitempty" example:"test.txt"`                    // 搜索关键词
	PID         int64  `form:"pid" binding:"omitempty,min=0" example:"0"`                         // 父级ID，0表示根目录
	Global      bool   `form:"global" example:"false"`                                            // 全局搜索（如果为true，忽略pid）
	PageSize    int    `form:"pageSize,default=10" binding:"required,min=1,max=100" example:"10"` // 每页大小
	CurrentPage int    `form:"currentPage,default=1" binding:"required,min=1" example:"1"`        // 当前页码
}

type searchDTO struct {
	*models.VirtualFile
	FullPath string `json:"fullPath" example:"/folder1/test.txt"` // 文件完整路径
}

type searchResponse struct {
	Total       int64        `json:"total" example:"100"`     // 总记录数
	CurrentPage int          `json:"currentPage" example:"1"` // 当前页码
	PageSize    int          `json:"pageSize" example:"10"`   // 每页大小
	Data        []*searchDTO `json:"data"`                    // 搜索结果列表
}

// Search 搜索文件
// @Summary 搜索文件
// @Description 根据关键词搜索虚拟文件，支持全局搜索或指定目录搜索
// @Tags 文件管理
// @Accept json
// @Produce json
// @Param Authorization header string true "Bearer token"
// @Param keyword query string false "搜索关键词" example("test.txt")
// @Param pid query int false "父级目录ID，0表示根目录" default(0) example(0)
// @Param global query bool false "是否全局搜索，true时忽略pid参数" default(false) example(false)
// @Param pageSize query int true "每页大小，范围1-100" minimum(1) maximum(100) default(10) example(10)
// @Param currentPage query int true "当前页码，从1开始" minimum(1) default(1) example(1)
// @Success 200 {object} httpcontext.Response{data=searchResponse} "搜索文件成功"
// @Failure 400 {object} httpcontext.Response "参数验证失败，code=99998"
// @Failure 400 {object} httpcontext.Response "查询文件列表失败，code=6001"
// @Failure 400 {object} httpcontext.Response "查询文件数量失败，code=6002"
// @Failure 400 {object} httpcontext.Response "计算文件完整路径失败，code=6003"
// @Failure 401 {object} httpcontext.Response "未授权访问"
// @Failure 403 {object} httpcontext.Response "权限不足"
// @Router /api/file/search [get]
func (h *handler) Search() httpcontext.HandlerFunc {
	return func(ctx *httpcontext.Context) {
		req := new(searchRequest)
		if err := ctx.ShouldBindQuery(req); err != nil {
			ctx.AbortWithInvalidParams(err)

			return
		}

		userID := ctx.GetInt64(consts.CtxKeyUserId)
		isAdmin := ctx.GetBool(consts.CtxKeyIsAdmin)

		groupFileIds, err := h.getUserGroupFileIDs(ctx)
		if err != nil {
			ctx.Fail(busCodeQueryTopIdError.WithError(err))

			return
		}

		if !h.ensureMountPointService(ctx, busCodeQueryTopIdError) {
			return
		}

		allowTopIds, err := h.mountPointService.GetAccessibleMountPointIDs(ctx.GetContext(), userID, isAdmin, groupFileIds)
		if err != nil {
			ctx.Fail(busCodeQueryTopIdError.WithError(err))

			return
		}

		if len(allowTopIds) == 0 {
			ctx.Success(&searchResponse{
				Total:       0,
				CurrentPage: req.CurrentPage,
				PageSize:    req.PageSize,
				Data:        make([]*searchDTO, 0),
			})

			return
		}

		if !h.ensureVirtualFileService(ctx, busCodeList) {
			return
		}

		listReq := &virtualfileSvi.ListRequest{
			Name:        req.Keyword,
			PageSize:    req.PageSize,
			CurrentPage: req.CurrentPage,
			TopIdList:   allowTopIds,
		}

		if shouldApplyFileSuffixLimit(isAdmin) {
			listReq.AllowedSuffixes = allowedUserFileSuffixes()
		}

		if !req.Global {
			listReq.ParentId = ptr.Of(req.PID)
		}

		list, err := h.virtualFileService.List(ctx.GetContext(), listReq)
		if err != nil {
			ctx.Fail(busCodeList.WithError(err))

			return
		}

		list = compactVirtualFiles(list)

		count, err := h.virtualFileService.Count(ctx.GetContext(), listReq)
		if err != nil {
			ctx.Fail(busCodeCount.WithError(err))

			return
		}

		pidMap := make(map[int64]string)

		for _, v := range list {
			if _, ok := pidMap[v.ParentId]; !ok {
				pidMap[v.ParentId], err = h.virtualFileService.CalFullPath(ctx.GetContext(), v.ParentId)
				if err != nil {
					ctx.Fail(busCodeCalFullPath.WithError(err))

					return
				}
			}
		}

		respList := make([]*searchDTO, 0, len(list))

		for _, v := range list {
			fullPath := path.Join(pidMap[v.ParentId], v.Name)

			respList = append(respList, &searchDTO{
				VirtualFile: v,
				FullPath:    fullPath,
			})
		}

		ctx.Success(&searchResponse{
			Total:       count,
			CurrentPage: req.CurrentPage,
			PageSize:    req.PageSize,
			Data:        respList,
		})
	}
}
