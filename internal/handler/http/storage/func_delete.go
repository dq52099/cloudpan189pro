package storage

import (
	"encoding/json"

	"github.com/pkg/errors"
	"github.com/xxcheng123/cloudpan189-share/internal/consts"
	"github.com/xxcheng123/cloudpan189-share/internal/framework/httpcontext"
	"github.com/xxcheng123/cloudpan189-share/internal/types/topic"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type deleteRequest struct {
	ID int64 `json:"id" binding:"required" example:"1"` // 存储节点ID
}

// Delete 删除存储挂载
// @Summary 删除存储挂载
// @Description 删除指定的存储挂载点，同时清理相关文件
// @Tags 存储管理
// @Accept json
// @Produce json
// @Param Authorization header string true "Bearer token"
// @Param request body deleteRequest true "删除请求参数"
// @Success 200 {object} httpcontext.Response "存储挂载删除成功"
// @Failure 400 {object} httpcontext.Response "参数验证失败，code=99998"
// @Failure 400 {object} httpcontext.Response "挂载点不存在，code=4022"
// @Failure 400 {object} httpcontext.Response "查询挂载点失败，code=4021"
// @Failure 400 {object} httpcontext.Response "删除挂载点失败，code=4023"
// @Failure 400 {object} httpcontext.Response "发送清理任务失败，code=4024"
// @Failure 401 {object} httpcontext.Response "未授权访问"
// @Failure 403 {object} httpcontext.Response "权限不足"
// @Router /api/storage/delete [post]
func (h *handler) Delete() httpcontext.HandlerFunc {
	return func(ctx *httpcontext.Context) {
		req := new(deleteRequest)
		if err := ctx.ShouldBindJSON(req); err != nil {
			ctx.AbortWithInvalidParams(err)

			return
		}

		mountPointInfo, err := h.mountPointService.Query(ctx.GetContext(), req.ID)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				ctx.Fail(busCodeStorageMountPointNotFound.WithError(err))
			} else {
				ctx.Fail(busCodeStorageQueryMountPointError.WithError(err))
			}

			return
		}

		// 获取当前用户信息用于权限控制
		userID := ctx.GetInt64(consts.CtxKeyUserId)
		isAdmin := ctx.GetBool(consts.CtxKeyIsAdmin)

		if !isAdmin && (userID <= 0 || mountPointInfo.CreatorUserID != userID) {
			ctx.Fail(busCodeStorageMountPointDeleteFail.WithError(errors.New("挂载点不存在或无权限删除")))

			return
		}

		taskReq := &topic.FileBatchDeleteRequest{
			IDs: []int64{mountPointInfo.FileId},
		}

		body, err := json.Marshal(taskReq)
		if err != nil {
			ctx.GetContext().Error("序列化存储删除任务失败", zap.Error(err))
			ctx.Fail(busCodeStorageSendTaskFail.WithError(err))

			return
		}

		if err := h.taskEngine.PushMessage(
			ctx.GetContext().
				WithValue(consts.CtxKeyFullPath, mountPointInfo.FullPath).
				WithValue(consts.CtxKeyInvokeHandlerName, "删除接口"),
			taskReq.Topic(),
			body,
		); err != nil {
			ctx.Fail(busCodeStorageSendTaskFail.WithError(err))

			return
		}

		ctx.Success()
	}
}
