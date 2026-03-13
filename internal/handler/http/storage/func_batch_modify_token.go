package storage

import (
	"fmt"

	"github.com/pkg/errors"
	"github.com/xxcheng123/cloudpan189-share/internal/consts"
	"github.com/xxcheng123/cloudpan189-share/internal/framework/httpcontext"
	filetasklogSvi "github.com/xxcheng123/cloudpan189-share/internal/services/filetasklog"
	"gorm.io/gorm"
)

type batchModifyTokenRequest struct {
	IDs     []int64 `json:"ids" binding:"required,min=1"`
	TokenID int64   `json:"tokenId"` // 新的令牌ID，0 表示解绑
}

// BatchModifyToken 批量修改存储挂载点令牌
// @Summary 批量修改存储挂载点令牌
// @Description 用户批量绑定自己的令牌到挂载点（不影响其他用户）
// @Tags 存储管理
// @Accept json
// @Produce json
// @Param Authorization header string true "Bearer token"
// @Param request body batchModifyTokenRequest true "批量修改令牌请求参数"
// @Success 200 {object} httpcontext.Response "令牌修改成功"
// @Failure 400 {object} httpcontext.Response "参数验证失败，code=99998"
// @Failure 400 {object} httpcontext.Response "云盘令牌不存在，code=4013"
// @Failure 400 {object} httpcontext.Response "查询云盘令牌失败，code=4019"
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

		// 获取当前用户信息用于权限控制
		userID := ctx.GetInt64(consts.CtxKeyUserId)
		isAdmin := ctx.GetBool(consts.CtxKeyIsAdmin)
		userGroupId := ctx.GetInt64(consts.CtxKeyUserGroupId)

		// 获取用户组绑定的文件ID
		var groupFileIds []int64
		if userGroupId > 0 {
			groupFileIds, _ = h.group2FileService.GetBindFiles(ctx.GetContext(), userGroupId)
		}

		// 验证云盘令牌是否存在
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

			// 非管理员：只能绑定自己的令牌
			if !isAdmin && token.UserID != userID {
				ctx.Fail(busCodeStorageCloudTokenNotExist.WithMessage("只能绑定自己的令牌"))

				return
			}
		}

		// 批量绑定用户的令牌到挂载点
		successCount := 0
		failCount := 0
		for _, id := range req.IDs {
			// 验证挂载点是否存在
			mp, err := h.mountPointService.Query(ctx.GetContext(), id)
			if err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					failCount++
					continue
				} else {
					failCount++
					continue
				}
			}

			// 验证用户是否有权限访问这个挂载点
			if !isAdmin {
				hasAccess := false
				if mp.CreatorUserID == userID {
					hasAccess = true
				} else {
					for _, fid := range groupFileIds {
						if fid == mp.FileId {
							hasAccess = true
							break
						}
					}
				}
				if !hasAccess {
					failCount++
					continue
				}
			}

			// 绑定用户的令牌
			if req.TokenID == 0 {
				h.userMountPointTokenService.UnbindToken(ctx.GetContext(), userID, mp.ID)
			} else {
				if err := h.userMountPointTokenService.BindToken(ctx.GetContext(), userID, mp.ID, req.TokenID); err != nil {
					failCount++
					continue
				}
			}
			successCount++
		}

		// 创建任务日志
		tracker, _ := h.fileTaskLogService.Create(
			ctx.GetContext(),
			"批量修改令牌",
			fmt.Sprintf("批量修改 %d 个挂载点的令牌", len(req.IDs)),
			filetasklogSvi.WithFile(req.IDs[0]),
			filetasklogSvi.WithDesc(fmt.Sprintf("成功: %d, 失败: %d, 新令牌ID: %d", successCount, failCount, req.TokenID)),
		)
		if tracker != nil {
			_ = h.fileTaskLogService.Completed(ctx.GetContext(), tracker)
		}

		ctx.Success(fmt.Sprintf("修改成功 %d 个，失败 %d 个", successCount, failCount))
	}
}
