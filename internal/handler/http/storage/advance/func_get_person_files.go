package advance

import (
	"github.com/xxcheng123/cloudpan189-share/internal/framework/httpcontext"
	"github.com/xxcheng123/cloudpan189-share/internal/services/cloudbridge"
)

type getPersonFilesRequest struct {
	PageNum    int    `form:"pageNum,default=1" binding:"min=1"`
	PageSize   int    `form:"pageSize,default=10" binding:"min=1,max=100"`
	CloudToken int64  `form:"cloudToken" binding:"required,min=1"`
	ParentId   string `form:"parentId,omitempty,default=-11"`
}

type getPersonFilesResponse struct {
	Data        []*cloudbridge.FileNode `json:"data"`
	Total       int64                   `json:"total"`
	CurrentPage int                     `json:"currentPage"`
	PageSize    int                     `json:"pageSize"`
}

// GetPersonFiles 获取个人文件列表
// @Summary 获取个人文件列表
// @Description 根据云盘令牌获取个人文件列表，支持分页查询
// @Tags 存储管理-高级功能
// @Accept json
// @Produce json
// @Param Authorization header string true "Bearer token"
// @Param pageNum query int true "页码，从1开始" minimum(1)
// @Param pageSize query int true "每页数量，最大100" minimum(1) maximum(100)
// @Param cloudToken query int true "云盘令牌ID"
// @Param parentId query string false "父目录ID，默认为-11（根目录）" default(-11)
// @Success 200 {object} httpcontext.Response{data=getPersonFilesResponse} "获取文件列表成功"
// @Failure 400 {object} httpcontext.Response "参数验证失败，code=99998"
// @Failure 404 {object} httpcontext.Response "云盘令牌不存在，code=8001"
// @Failure 400 {object} httpcontext.Response "获取文件列表失败，code=8002"
// @Failure 401 {object} httpcontext.Response "未授权访问"
// @Failure 403 {object} httpcontext.Response "权限不足"
// @Router /api/storage/advance/person/files [get]
func (h *handler) GetPersonFiles() httpcontext.HandlerFunc {
	return func(ctx *httpcontext.Context) {
		req := new(getPersonFilesRequest)
		if err := ctx.ShouldBindQuery(req); err != nil {
			ctx.AbortWithInvalidParams(err)

			return
		}

		token, ok := h.queryAccessibleCloudToken(ctx, req.CloudToken)
		if !ok {
			return
		}

		// 构造认证令牌
		authToken := cloudbridge.NewAuthToken(token.AccessToken, token.ExpiresIn)

		// 获取文件列表
		fileList, err := h.cloudBridgeService.PersonFileList(ctx.GetContext(), authToken, req.ParentId, req.PageNum, req.PageSize)
		if err != nil {
			ctx.Fail(codeStorageAdvanceQueryPathFailed.WithError(err))

			return
		}

		// 获取文件总数
		total, err := h.cloudBridgeService.PersonFileCount(ctx.GetContext(), authToken, req.ParentId)
		if err != nil {
			ctx.Fail(codeStorageAdvanceQueryPathFailed.WithError(err))

			return
		}

		// 构造响应
		response := &getPersonFilesResponse{
			Data:        fileList.Data,
			Total:       total,
			CurrentPage: req.PageNum,
			PageSize:    req.PageSize,
		}

		ctx.Success(response)
	}
}
