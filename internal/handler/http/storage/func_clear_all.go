package storage

import (
	"github.com/xxcheng123/cloudpan189-share/internal/framework/httpcontext"
)

type ClearAllRequest struct {
	DeleteFiles bool `json:"deleteFiles"` // 是否同时删除源文件
}

// ClearAll 清空所有挂载点
// @Summary 清空所有挂载点
// @Description 删除所有存储挂载点（可选是否删除源文件）
// @Tags 存储管理
// @Accept json
// @Produce json
// @Param Authorization header string true "Bearer token"
// @Param request body ClearAllRequest false "清空请求参数"
// @Success 200 {object} httpcontext.Response "清空成功"
// @Failure 400 {object} httpcontext.Response "参数验证失败"
// @Failure 401 {object} httpcontext.Response "未授权访问"
// @Router /api/storage/clear_all [post]
func (h *handler) ClearAll() httpcontext.HandlerFunc {
	return func(ctx *httpcontext.Context) {
		_ = ctx.ShouldBindJSON(&ClearAllRequest{})

		count, err := h.mountPointService.ClearAll(ctx.GetContext())
		if err != nil {
			ctx.Fail(busCodeStorageMountPointDeleteFail.WithError(err))
			return
		}

		if err := h.virtualFileService.ClearAll(ctx.GetContext()); err != nil {
			ctx.Fail(busCodeStorageMountPointDeleteFail.WithError(err))
			return
		}

		if err := h.mediaFileService.ClearAll(ctx.GetContext()); err != nil {
			ctx.Fail(busCodeStorageMountPointDeleteFail.WithError(err))
			return
		}

		ctx.Success(count)
	}
}
