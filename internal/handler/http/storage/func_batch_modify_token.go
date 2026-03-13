package storage

import (
	"encoding/json"
	"fmt"

	"github.com/pkg/errors"
	"github.com/xxcheng123/cloudpan189-share/internal/consts"
	"github.com/xxcheng123/cloudpan189-share/internal/framework/httpcontext"
	"github.com/xxcheng123/cloudpan189-share/internal/types/topic"
	"gorm.io/gorm"
)

type batchModifyTokenRequest struct {
	IDs     []int64 `json:"ids" binding:"required,min=1"`
	TokenID int64   `json:"tokenId"` // 新的令牌ID，0 表示解绑
}

// BatchModifyToken 批量修改存储挂载点令牌
// @Summary 批量修改存储挂载点令牌
// @Description 用户批量绑定自己的令牌到挂载点，异步提交后台处理
// @Tags 存储管理
// @Accept json
// @Produce json
// @Param Authorization header string true "Bearer token"
// @Param request body batchModifyTokenRequest true "批量修改令牌请求参数"
// @Success 200 {object} httpcontext.Response "令牌修改任务已提交"
// @Failure 400 {object} httpcontext.Response "参数验证失败，code=99998"
// @Failure 400 {object} httpcontext.Response "云盘令牌不存在，code=4013"
// @Failure 400 {object} httpcontext.Response "查询云盘令牌失败，code=4019"
// @Failure 400 {object} httpcontext.Response "下发任务失败，code=4024"
// @Failure 401 {object} httpcontext.Response "未授权访问"
// @Failure 403 {object} httpcontext.Response "权限不足"
// @Router /api/storage/batch_modify_token [post]
func (h *handler) BatchModifyToken() httpcontext.HandlerFunc {
	return func(ctx *httpcontext.Context) {
		req := new(batchModifyTokenRequest)
		if err := ctx.ShouldBindJSON(req); err != nil {
			ctx.AbortWithInvalidParams(err)
			return
		}

		userID := ctx.GetInt64(consts.CtxKeyUserId)
		isAdmin := ctx.GetBool(consts.CtxKeyIsAdmin)
		userGroupID := ctx.GetInt64(consts.CtxKeyUserGroupId)

		if req.TokenID != 0 {
			token, err := h.cloudTokenService.Query(ctx.GetContext(), req.TokenID)
			if err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					ctx.Fail(busCodeStorageCloudTokenNotExist.WithError(err))
				} else {
					ctx.Fail(busCodeStorageQueryCloudTokenError.WithError(err))
				}
				return
			}

			if !isAdmin && token.UserID != userID {
				ctx.Fail(busCodeStorageCloudTokenNotExist.WithMessage("只能绑定自己的令牌"))
				return
			}
		}

		taskReq := &topic.FileBatchModifyTokenRequest{
			IDs:         req.IDs,
			TokenID:     req.TokenID,
			UserID:      userID,
			IsAdmin:     isAdmin,
			UserGroupID: userGroupID,
		}

		body, _ := json.Marshal(taskReq)
		if err := h.taskEngine.PushMessage(
			ctx.GetContext().WithValue(consts.CtxKeyInvokeHandlerName, "批量修改令牌"),
			taskReq.Topic(),
			body,
		); err != nil {
			ctx.Fail(busCodeStorageSendTaskFail.WithError(err))
			return
		}

		actionText := "绑定"
		if req.TokenID == 0 {
			actionText = "解绑"
		}

		ctx.Success(fmt.Sprintf("批量%s令牌任务已提交，共 %d 个挂载点", actionText, len(req.IDs)))
	}
}
