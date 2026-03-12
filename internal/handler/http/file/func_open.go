package file

import (
	"path"

	"github.com/pkg/errors"
	"github.com/samber/lo"
	"github.com/xxcheng123/cloudpan189-share/internal/consts"
	"github.com/xxcheng123/cloudpan189-share/internal/framework/httpcontext"
	"github.com/xxcheng123/cloudpan189-share/internal/pkgs/ptr"
	"github.com/xxcheng123/cloudpan189-share/internal/pkgs/utils"
	"github.com/xxcheng123/cloudpan189-share/internal/repository/models"
	virtualfileSvi "github.com/xxcheng123/cloudpan189-share/internal/services/virtualfile"
	"gorm.io/gorm"
)

type openRequest struct {
	FullPath string `uri:"fullPath"`
}

const (
	openBaseURL = "/api/file/open"
)

type childDTO struct {
	*models.VirtualFile
	Href    string `json:"href"`
	ApiPath string `json:"apiPath"`
}

type breadcrumbItem struct {
	Href string `json:"href"`
	Name string `json:"name"`
}

type openResponse struct {
	*models.VirtualFile
	Href          string            `json:"href"`
	ApiPath       string            `json:"apiPath"`
	Children      []*childDTO       `json:"children,omitempty"`
	ChildrenTotal int64             `json:"childrenTotal"`
	Breadcrumbs   []*breadcrumbItem `json:"breadcrumbs"`
}

// Open 打开文件或目录
// @Summary 打开文件或目录
// @Description 根据完整路径打开文件或目录，返回文件信息、子文件列表和面包屑导航
// @Tags 文件管理
// @Accept json
// @Produce json
// @Param Authorization header string true "Bearer token"
// @Param fullPath path string true "文件或目录的完整路径" example("/folder1/subfolder")
// @Success 200 {object} httpcontext.Response{data=openResponse} "打开文件或目录成功"
// @Failure 400 {object} httpcontext.Response "参数验证失败，code=99998"
// @Failure 400 {object} httpcontext.Response "路径切割失败，code=6016"
// @Failure 400 {object} httpcontext.Response "路径不合法，需要 / 开头的路径，code=6017"
// @Failure 400 {object} httpcontext.Response "文件不存在，code=6018"
// @Failure 400 {object} httpcontext.Response "查询文件失败，code=6004"
// @Failure 401 {object} httpcontext.Response "未授权访问"
// @Failure 403 {object} httpcontext.Response "权限不足"
// @Router /api/file/open/{fullPath} [get]
func (h *handler) Open() httpcontext.HandlerFunc {
	return func(ctx *httpcontext.Context) {
		req := new(openRequest)
		if err := ctx.ShouldBindUri(req); err != nil {
			ctx.AbortWithInvalidParams(err)

			return
		}

		// 切分路径查找文件
		paths, err := utils.SplitPath(req.FullPath)
		if err != nil {
			ctx.Fail(busCodeFilePathSplitError.WithError(err))

			return
		}

		var (
			file *models.VirtualFile
		)

		// 根节点
		if len(paths) == 0 {
			file = models.RootFile()
		} else if !utils.CheckIsPath(req.FullPath) {
			ctx.Fail(busCodeFileInvalidPath)

			return
		} else if file, err = h.virtualFileService.QueryByPath(ctx.GetContext(), req.FullPath); err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				ctx.Fail(busCodeFileNotFound.WithError(err))
			} else {
				ctx.Fail(busCodeFileQueryError.WithError(err))
			}

			return
		}

		var allowTopIds []int64

		userID := ctx.GetInt64(consts.CtxKeyUserId)
		isAdmin := ctx.GetBool(consts.CtxKeyIsAdmin)
		userGroupId := ctx.GetInt64(consts.CtxKeyUserGroupId)

		// 获取用户组绑定的文件ID
		var groupFileIds []int64
		if userGroupId > 0 {
			groupFileIds, _ = h.group2FileService.GetBindFiles(ctx.GetContext(), userGroupId)
		}

		accessibleIds, err := h.mountPointService.GetAccessibleMountPointIDs(ctx.GetContext(), userID, isAdmin, groupFileIds)
		if err != nil {
			ctx.Fail(busCodeQueryTopIdError.WithError(err))

			return
		}

		// 合并用户组绑定的文件ID
		if len(groupFileIds) > 0 {
			accessibleIds = append(accessibleIds, groupFileIds...)
		}

		accessibleIds = lo.Uniq(accessibleIds)

		if len(accessibleIds) == 0 || (!lo.Contains(accessibleIds, file.TopId) && file.OsType != models.OsTypeFolder) {
			ctx.Unauthorized("无权限访问")

			return
		}

		allowTopIds = accessibleIds

		var (
			children      []*models.VirtualFile
			childrenCount int64 = 0
		)

		// 查询子节点
		if file.IsDir {
			childReq := &virtualfileSvi.ListRequest{
				ParentId: ptr.Of(file.ID),
			}

			// 目录查询子节点
			if children, err = h.virtualFileService.List(ctx.GetContext(), childReq); err != nil {
				ctx.Fail(busCodeFileQueryError.WithError(err))

				return
			}

			// 子节点数量
			if childrenCount, err = h.virtualFileService.Count(ctx.GetContext(), childReq); err != nil {
				ctx.Fail(busCodeFileQueryError.WithError(err))

				return
			}
		}

		if len(allowTopIds) > 0 {
			children = lo.Filter(children, func(child *models.VirtualFile, index int) bool {
				return child.OsType == models.OsTypeFolder || lo.Contains(allowTopIds, child.TopId)
			})
		}

		// 转换为 DTO
		childrenDTO := make([]*childDTO, 0, len(children))
		for _, child := range children {
			childrenDTO = append(childrenDTO, &childDTO{
				VirtualFile: child,
				ApiPath:     utils.PathEscape(openBaseURL, path.Join(req.FullPath, child.Name)),
				Href:        utils.PathEscape(path.Join(req.FullPath, child.Name)),
			})
		}

		var breadcrumbs []*breadcrumbItem

		var currentHref string

		for _, p := range paths {
			breadcrumbs = append(breadcrumbs, &breadcrumbItem{
				Href: utils.PathEscape(openBaseURL, path.Join(currentHref, p)),
				Name: p,
			})
			currentHref = path.Join(currentHref, p)
		}

		ctx.Success(&openResponse{
			VirtualFile:   file,
			ApiPath:       utils.PathEscape(openBaseURL, req.FullPath),
			Href:          utils.PathEscape(req.FullPath),
			Children:      childrenDTO,
			ChildrenTotal: childrenCount,
			Breadcrumbs:   breadcrumbs,
		})
	}
}
