package file

import (
	"errors"

	"github.com/samber/lo"
	"github.com/xxcheng123/cloudpan189-share/internal/consts"
	"github.com/xxcheng123/cloudpan189-share/internal/framework/httpcontext"
	"github.com/xxcheng123/cloudpan189-share/internal/shared"
	"gorm.io/gorm"
)

type createDownloadURLRequest struct {
	FileID int64 `json:"fileId" binding:"required,min=1" example:"123456"`
}

type createDownloadURLResponse struct {
	DownloadURL string `json:"downloadUrl" example:"/api/file/download/123456?sign=abc&uuid=def&timestamp=1234567890&signer=v1"`
}

// CreateDownloadURL 创建文件下载链接
// @Summary 创建文件下载链接
// @Description 为指定文件生成带签名的下载链接
// @Tags 文件管理
// @Accept json
// @Produce json
// @Param Authorization header string true "Bearer token"
// @Param request body createDownloadURLRequest true "文件ID信息"
// @Success 200 {object} httpcontext.Response{data=createDownloadURLResponse} "下载链接创建成功"
// @Failure 400 {object} httpcontext.Response "参数验证失败，code=99998"
// @Failure 400 {object} httpcontext.Response "文件签名失败，code=6015"
// @Failure 401 {object} httpcontext.Response "未授权访问"
// @Failure 403 {object} httpcontext.Response "权限不足"
// @Router /api/file/create_download_url [post]
func (h *handler) CreateDownloadURL() httpcontext.HandlerFunc {
	return func(ctx *httpcontext.Context) {
		var req createDownloadURLRequest
		if err := ctx.ShouldBindJSON(&req); err != nil {
			ctx.AbortWithInvalidParams(err)

			return
		}

		if !h.ensureVirtualFileService(ctx, busCodeFileQueryError) {
			return
		}

		file, err := h.virtualFileService.Query(ctx.GetContext(), req.FileID)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				ctx.Fail(busCodeFileNotFound.WithError(err))

				return
			}

			ctx.Fail(busCodeFileQueryError.WithError(err))

			return
		}

		if file == nil {
			ctx.Fail(busCodeFileNotFound.WithError(gorm.ErrRecordNotFound))

			return
		}

		groupFileIds, err := h.getUserGroupFileIDs(ctx)
		if err != nil {
			ctx.Fail(busCodeQueryTopIdError.WithError(err))

			return
		}

		userID := ctx.GetInt64(consts.CtxKeyUserId)
		isAdmin := ctx.GetBool(consts.CtxKeyIsAdmin)

		if !h.ensureMountPointService(ctx, busCodeQueryTopIdError) {
			return
		}

		allowTopIds, err := h.mountPointService.GetAccessibleMountPointIDs(ctx.GetContext(), userID, isAdmin, groupFileIds)
		if err != nil {
			ctx.Fail(busCodeQueryTopIdError.WithError(err))

			return
		}

		if !lo.Contains(allowTopIds, file.TopId) {
			ctx.Unauthorized("无权限访问")

			return
		}

		if file.IsDir {
			ctx.Fail(busCodeFileIsDirNotSupport)

			return
		}

		if shouldLimitFileBySuffix(file, isAdmin) {
			ctx.Fail(busCodeFileNotFound)

			return
		}

		if !h.ensureVerifyService(ctx, busCodeFileSignError) {
			return
		}

		values, err := h.verifyService.SignV1(ctx.GetContext(), req.FileID)
		if err != nil {
			ctx.Fail(busCodeFileSignError.WithError(err))

			return
		}

		ctx.Success(&createDownloadURLResponse{
			DownloadURL: shared.JoinDownloadURL(req.FileID, values),
		})
	}
}

func (h *handler) getUserGroupFileIDs(ctx *httpcontext.Context) ([]int64, error) {
	userGroupId := ctx.GetInt64(consts.CtxKeyUserGroupId)
	if userGroupId == 0 {
		return nil, nil
	}

	if !h.hasGroup2FileService() {
		return nil, errors.New("用户组绑定服务未初始化")
	}

	return h.group2FileService.GetBindFiles(ctx.GetContext(), userGroupId)
}
