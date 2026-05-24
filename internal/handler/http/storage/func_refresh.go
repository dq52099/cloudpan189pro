package storage

import (
	"encoding/json"

	"github.com/xxcheng123/cloudpan189-share/internal/consts"
	"github.com/xxcheng123/cloudpan189-share/internal/framework/httpcontext"
	"github.com/xxcheng123/cloudpan189-share/internal/types/topic"
)

type refreshRequest struct {
	ID   int64 `json:"id" binding:"required" example:"1001"` // 挂载点表主键
	Deep bool  `json:"deep" example:"true"`                  // 深度刷新
}

// Refresh 刷新存储挂载
// @Summary 刷新存储挂载
// @Description 通过请求体 id 指定挂载点表主键，触发文件扫描任务重新同步文件信息
// @Tags 存储管理
// @Accept json
// @Produce json
// @Param Authorization header string true "Bearer token"
// @Param request body refreshRequest true "刷新请求参数"
// @Success 200 {object} httpcontext.Response "存储挂载刷新成功"
// @Failure 400 {object} httpcontext.Response "参数验证失败，code=99998"
// @Failure 404 {object} httpcontext.Response "挂载点不存在，code=4022"
// @Failure 400 {object} httpcontext.Response "查询挂载点失败，code=4018"
// @Failure 400 {object} httpcontext.Response "添加扫描任务失败，code=4015"
// @Failure 401 {object} httpcontext.Response "未授权访问"
// @Failure 403 {object} httpcontext.Response "权限不足"
// @Router /api/storage/refresh [post]
func (h *handler) Refresh() httpcontext.HandlerFunc {
	return func(ctx *httpcontext.Context) {
		var req refreshRequest
		if err := ctx.ShouldBindJSON(&req); err != nil {
			ctx.AbortWithInvalidParams(err)

			return
		}

		mountPoint, ok := h.queryOwnedMountPoint(ctx, req.ID)
		if !ok {
			return
		}

		taskReq := &topic.FileScanFileRequest{
			FileId: mountPoint.FileId,
			Deep:   req.Deep,
		}

		body, err := json.Marshal(taskReq)
		if err != nil {
			ctx.Fail(busCodeStorageAddTaskFailed.WithError(err))

			return
		}

		if err = h.taskEngine.PushMessage(ctx.GetContext().
			WithValue(consts.CtxKeyFullPath, mountPoint.FullPath).
			WithValue(consts.CtxKeyInvokeHandlerName, "手动刷新"),
			taskReq.Topic(), body); err != nil {
			ctx.Fail(busCodeStorageAddTaskFailed.WithError(err))

			return
		}

		ctx.Success()
	}
}
