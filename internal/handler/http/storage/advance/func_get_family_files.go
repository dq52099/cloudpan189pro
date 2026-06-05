package advance

import (
	"errors"
	"time"

	"github.com/xxcheng123/cloudpan189-share/internal/framework/httpcontext"
	"github.com/xxcheng123/cloudpan189-share/internal/services/cloudbridge"
)

type getFamilyFilesRequest struct {
	PageNum    int    `form:"pageNum,default=1" binding:"min=1"`
	PageSize   int    `form:"pageSize,default=10" binding:"min=1,max=100"`
	CloudToken int64  `form:"cloudToken" binding:"required,min=1"`
	FamilyId   string `form:"familyId" binding:"required"`
	ParentId   string `form:"parentId" default:""`
}

type getFamilyFilesResponse struct {
	Data        []*cloudbridge.FileNode `json:"data"`
	Total       int64                   `json:"total"`
	CurrentPage int                     `json:"currentPage"`
	PageSize    int                     `json:"pageSize"`
}

// GetFamilyFiles 获取家庭云文件列表
// @Summary 获取家庭云文件列表
// @Description 根据云盘令牌和家庭云ID获取家庭云文件列表，支持分页查询
// @Tags 存储管理-高级功能
// @Accept json
// @Produce json
// @Param Authorization header string true "Bearer token"
// @Param pageNum query int true "页码，从1开始" minimum(1)
// @Param pageSize query int true "每页数量，最大100" minimum(1) maximum(100)
// @Param cloudToken query int true "云盘令牌ID"
// @Param familyId query string true "家庭云ID"
// @Param parentId query string false "父目录ID，默认为空（根目录）" default("")
// @Success 200 {object} httpcontext.Response{data=getFamilyFilesResponse} "获取家庭云文件列表成功"
// @Failure 400 {object} httpcontext.Response "参数验证失败，code=99998"
// @Failure 404 {object} httpcontext.Response "云盘令牌不存在，code=8001"
// @Failure 400 {object} httpcontext.Response "获取家庭云文件列表失败，code=8002"
// @Failure 401 {object} httpcontext.Response "未授权访问"
// @Failure 403 {object} httpcontext.Response "权限不足"
// @Router /api/storage/advance/family/files [get]
func (h *handler) GetFamilyFiles() httpcontext.HandlerFunc {
	return func(ctx *httpcontext.Context) {
		req := new(getFamilyFilesRequest)
		if err := ctx.ShouldBindQuery(req); err != nil {
			ctx.AbortWithInvalidParams(err)

			return
		}

		token, ok := h.queryAccessibleCloudToken(ctx, req.CloudToken)
		if !ok {
			return
		}

		if !h.ensureCloudBridgeService(ctx, codeStorageAdvanceQueryPathFailed) {
			return
		}

		// 构造认证令牌
		authToken := cloudbridge.NewAuthToken(token.AccessToken, token.AuthExpiresAtMillis(time.Now()))

		// 获取家庭云文件列表
		fileList, err := h.cloudBridgeService.FamilyFileList(ctx.GetContext(), authToken, req.FamilyId, req.ParentId, req.PageNum, req.PageSize)
		if err != nil {
			ctx.Fail(codeStorageAdvanceQueryPathFailed.WithError(err))

			return
		}

		if fileList == nil {
			ctx.Fail(codeStorageAdvanceQueryPathFailed.WithError(errors.New("家庭云文件列表返回为空")))

			return
		}

		// 获取家庭云文件总数
		total, err := h.cloudBridgeService.FamilyFileCount(ctx.GetContext(), authToken, req.FamilyId, req.ParentId)
		if err != nil {
			ctx.Fail(codeStorageAdvanceQueryPathFailed.WithError(err))

			return
		}

		// 构造响应
		response := &getFamilyFilesResponse{
			Data:        fileList.Data,
			Total:       total,
			CurrentPage: req.PageNum,
			PageSize:    req.PageSize,
		}

		ctx.Success(response)
	}
}
