package dav

import (
	"errors"
	"fmt"
	"net/http"
	"path"
	"slices"
	"strings"

	"github.com/samber/lo"
	"github.com/xxcheng123/cloudpan189-share/internal/consts"
	"github.com/xxcheng123/cloudpan189-share/internal/framework/httpcontext"
	"github.com/xxcheng123/cloudpan189-share/internal/pkgs/ptr"
	"github.com/xxcheng123/cloudpan189-share/internal/pkgs/utils"
	"github.com/xxcheng123/cloudpan189-share/internal/repository/models"
	"github.com/xxcheng123/cloudpan189-share/internal/shared"
	"gorm.io/gorm"

	group2fileSvi "github.com/xxcheng123/cloudpan189-share/internal/services/group2file"
	mountpointSvi "github.com/xxcheng123/cloudpan189-share/internal/services/mountpoint"
	verifySvi "github.com/xxcheng123/cloudpan189-share/internal/services/verify"
	virtualfileSvi "github.com/xxcheng123/cloudpan189-share/internal/services/virtualfile"
)

type workEngine struct {
	virtualFileService virtualfileSvi.Service
	verifyService      verifySvi.Service
	group2FileService  group2fileSvi.Service
	mountPointService  mountpointSvi.Service
}

var bi = httpcontext.NewBusinessGenerator(consts.BusCodeDavStartCode)

var (
	busCodeFileQueryError     = bi.Next("查询文件失败")
	busCodeFileSignError      = bi.Next("文件签名失败")
	busCodeFilePathSplitError = bi.Next("路径切割失败")
	busCodeFileInvalidPath    = bi.Next("路径不合法，需要 / 开头的路径")
	busCodeFileNotFound       = bi.Next("文件不存在")
	busCodeQueryTopIdError    = bi.Next("查询 TopId 失败")
)

const downloadURLFormat = "/api/file/download/%d?%s"

func (e *workEngine) Open() httpcontext.HandlerFunc {
	return func(ctx *httpcontext.Context) {
		fullPath := ctx.Param("path")

		paths, err := utils.SplitPath(fullPath)
		if err != nil {
			ctx.Fail(busCodeFilePathSplitError.WithError(err))
			return
		}

		var file *models.VirtualFile

		if len(paths) == 0 {
			file = models.RootFile()
		} else if !utils.CheckIsPath(fullPath) {
			ctx.Fail(busCodeFileInvalidPath)
			return
		} else if file, err = e.virtualFileService.QueryByPath(ctx.GetContext(), fullPath); err != nil {
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

		if e.shouldLimitFileByStrm(file, isAdmin) {
			ctx.Fail(busCodeFileNotFound)

			return
		}

		// 获取用户组绑定的文件ID
		var groupFileIds []int64
		if userGroupId > 0 {
			groupFileIds, _ = e.group2FileService.GetBindFiles(ctx.GetContext(), userGroupId)
		}

		accessibleIds, err := e.mountPointService.GetAccessibleMountPointIDs(ctx.GetContext(), userID, isAdmin, groupFileIds)
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

		var children []*models.VirtualFile

		if file.IsDir {
			childReq := &virtualfileSvi.ListRequest{
				ParentId: ptr.Of(file.ID),
			}

			if children, err = e.virtualFileService.List(ctx.GetContext(), childReq); err != nil {
				ctx.Fail(busCodeFileQueryError.WithError(err))
				return
			}

			children = lo.Filter(children, func(child *models.VirtualFile, _ int) bool {
				return !e.shouldLimitFileByStrm(child, isAdmin)
			})
		} else if ctx.Request.Method == http.MethodGet || ctx.Request.Method == http.MethodHead || ctx.Request.Method == http.MethodPost {
			values, err := e.verifyService.SignV1(ctx.GetContext(), file.ID)
			if err != nil {
				ctx.Fail(busCodeFileSignError.WithError(err))
				return
			}

			downloadURL := fmt.Sprintf(downloadURLFormat, file.ID, values.Encode())
			ctx.Redirect(http.StatusFound, fmt.Sprintf("%s%s", shared.BaseURL, downloadURL))
			return
		}

		if len(allowTopIds) > 0 {
			children = lo.Filter(children, func(child *models.VirtualFile, index int) bool {
				return child.OsType == models.OsTypeFolder || lo.Contains(allowTopIds, child.TopId)
			})
		}

		ctx.Header("Content-Type", "application/xml; charset=utf-8")
		ctx.Header("DAV", "1, 2")

		var xmlResponse strings.Builder
		xmlResponse.WriteString(`<?xml version="1.0" encoding="utf-8"?>`)
		xmlResponse.WriteString(`<D:multistatus xmlns:D="DAV:">`)

		currentPath := e.normalizeWebDAVPath(ctx.Request.URL.Path)
		if file.IsDir && !strings.HasSuffix(currentPath, "/") {
			currentPath += "/"
		}

		e.addPropResponse(&xmlResponse, file, currentPath)

		if file.IsDir && ctx.Request.Header.Get("Depth") != "0" && len(children) > 0 {
			for _, child := range children {
				childPath := e.buildChildPath(currentPath, child.Name, child.IsDir)
				e.addPropResponse(&xmlResponse, child, childPath)
			}
		}

		xmlResponse.WriteString(`</D:multistatus>`)

		ctx.Data(http.StatusMultiStatus, "application/xml; charset=utf-8", []byte(xmlResponse.String()))
	}
}

func (e *workEngine) shouldLimitFileByStrm(file *models.VirtualFile, isAdmin bool) bool {
	if isAdmin || file == nil || file.IsDir || !shared.SettingAddition.WebDAVUserStrmOnly {
		return false
	}

	extName := strings.ToLower(path.Ext(file.Name))
	if extName == "" {
		return true
	}

	allowed := models.NormalizeSuffixes(shared.SettingAddition.WebDAVAllowedSuffixes)
	if len(allowed) == 0 {
		allowed = append([]string(nil), models.DefaultWebDAVAllowedSuffixes...)
	}

	return !slices.Contains(allowed, extName)
}
