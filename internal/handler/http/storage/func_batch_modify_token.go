package storage

import (
	"encoding/json"
	"fmt"

	"github.com/pkg/errors"
	"github.com/xxcheng123/cloudpan189-share/internal/consts"
	"github.com/xxcheng123/cloudpan189-share/internal/framework/httpcontext"
	"github.com/xxcheng123/cloudpan189-share/internal/repository/models"
	"github.com/xxcheng123/cloudpan189-share/internal/types/topic"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type batchModifyTokenRequest struct {
	IDs     []int64 `json:"ids" binding:"required,min=1"`
	TokenID int64   `json:"tokenId" binding:"min=0"` // 新的令牌ID，0 表示解绑
}

// BatchModifyToken 批量修改存储挂载点令牌
// @Summary 批量修改存储挂载点令牌
// @Description 用户批量绑定自己的令牌到挂载点，异步提交后台处理（上限 500 个）
// @Tags 存储管理
// @Accept json
// @Produce json
// @Param Authorization header string true "Bearer token"
// @Param request body batchModifyTokenRequest true "批量修改令牌请求参数"
// @Success 200 {object} httpcontext.Response "令牌修改任务已提交"
// @Failure 400 {object} httpcontext.Response "参数验证失败，code=99998"
// @Failure 404 {object} httpcontext.Response "挂载点不存在，code=4022；云盘令牌不存在，code=4013"
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

		requestIDs, err := normalizeBatchIDs(req.IDs, maxBatchModifyTokenIDs)
		if err != nil {
			ctx.AbortWithInvalidParams(err)

			return
		}

		if req.TokenID != 0 {
			if !h.ensureCloudTokenService(ctx, busCodeStorageQueryCloudTokenError) {
				return
			}

			token, err := h.cloudTokenService.QueryAccessible(ctx.GetContext(), req.TokenID, userID, isAdmin)
			if err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					ctx.Fail(busCodeStorageCloudTokenNotExist.WithError(err))
				} else {
					ctx.Fail(busCodeStorageQueryCloudTokenError.WithError(err))
				}

				return
			}

			if token == nil {
				ctx.Fail(busCodeStorageCloudTokenNotExist.WithError(gorm.ErrRecordNotFound))

				return
			}
		}

		mountPoints := make(map[int64]*models.MountPoint, len(requestIDs))
		fileIDs := make([]int64, 0, len(requestIDs))

		if !h.ensureMountPointService(ctx, busCodeStorageQueryMountPointError) {
			return
		}

		for _, id := range requestIDs {
			mp, err := h.mountPointService.QueryByID(ctx.GetContext(), id)
			if err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					ctx.Fail(busCodeStorageMountPointNotFound.WithError(err))
				} else {
					ctx.Fail(busCodeStorageQueryMountPointError.WithError(err))
				}

				return
			}

			if mp == nil {
				ctx.Fail(busCodeStorageMountPointNotFound.WithError(gorm.ErrRecordNotFound))

				return
			}

			mountPoints[id] = mp
			fileIDs = append(fileIDs, mp.FileId)
		}

		// 校验每个挂载点是否在当前用户的可见范围内（非管理员）
		if !isAdmin {
			var groupFileIds []int64

			if userGroupID > 0 {
				if !h.ensureGroup2FileService(ctx, busCodeStorageQueryMountPointError) {
					return
				}

				groupFileIDs, err := h.group2FileService.GetBindFiles(ctx.GetContext(), userGroupID)
				if err != nil {
					ctx.Fail(busCodeStorageQueryMountPointError.WithError(err))

					return
				}

				groupFileIds = groupFileIDs
			}

			groupFileSet := make(map[int64]struct{}, len(groupFileIds))
			for _, fid := range groupFileIds {
				groupFileSet[fid] = struct{}{}
			}

			for _, id := range requestIDs {
				mp := mountPoints[id]
				if mp == nil {
					ctx.Fail(busCodeStorageMountPointNotFound.WithError(gorm.ErrRecordNotFound))

					return
				}

				if mp.CreatorUserID == userID {
					continue
				}

				if _, ok := groupFileSet[mp.FileId]; ok {
					continue
				}

				if h.hasUserMountPointTokenService() {
					tokenID, err := h.userMountPointTokenService.GetTokenID(ctx.GetContext(), userID, mp.ID)
					if err != nil {
						ctx.Fail(busCodeStorageQueryMountPointError.WithError(err))

						return
					}

					if tokenID > 0 {
						continue
					}
				}

				ctx.Fail(busCodeStorageMountPointNotFound.WithMessage(fmt.Sprintf("无权操作挂载点 %d", id)))

				return
			}
		}

		taskReq := &topic.FileBatchModifyTokenRequest{
			IDs:           fileIDs,
			MountPointIDs: requestIDs,
			TokenID:       req.TokenID,
			UserID:        userID,
			IsAdmin:       isAdmin,
			UserGroupID:   userGroupID,
		}

		body, err := json.Marshal(taskReq)
		if err != nil {
			ctx.Fail(busCodeStorageSendTaskFail.WithError(err))

			return
		}

		if !h.ensureTaskEngine(ctx, busCodeStorageSendTaskFail) {
			return
		}

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

		ctx.GetContext().Info(
			"批量修改令牌已提交",
			zap.Int("count", len(requestIDs)),
			zap.Int("original_count", len(req.IDs)),
			zap.Int64("token_id", req.TokenID),
			zap.Int64("user_id", userID),
		)

		ctx.Success(fmt.Sprintf("批量%s令牌任务已提交，共 %d 个挂载点", actionText, len(requestIDs)))
	}
}
