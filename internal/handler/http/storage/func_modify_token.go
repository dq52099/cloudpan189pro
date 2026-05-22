package storage

import (
	"github.com/pkg/errors"
	"github.com/xxcheng123/cloudpan189-share/internal/consts"
	"github.com/xxcheng123/cloudpan189-share/internal/framework/httpcontext"
	"gorm.io/gorm"
)

type modifyTokenRequest struct {
	ID      int64 `json:"id" binding:"required,min=1" example:"1001"` // 挂载点文件ID
	TokenID int64 `json:"tokenId" binding:"min=0" example:"123"`      // 新的令牌ID，0 表示解绑
}

// ModifyToken 修改存储挂载点令牌
// @Summary 修改存储挂载点令牌
// @Description 通过请求体 id 指定挂载点文件ID，用户绑定自己的令牌到挂载点（不影响其他用户）
// @Tags 存储管理
// @Accept json
// @Produce json
// @Param Authorization header string true "Bearer token"
// @Param request body modifyTokenRequest true "修改令牌请求参数"
// @Success 200 {object} httpcontext.Response "令牌修改成功"
// @Failure 400 {object} httpcontext.Response "参数验证失败，code=99998"
// @Failure 400 {object} httpcontext.Response "挂载点不存在，code=4022"
// @Failure 400 {object} httpcontext.Response "查询挂载点失败，code=4021"
// @Failure 400 {object} httpcontext.Response "云盘令牌不存在，code=4013"
// @Failure 400 {object} httpcontext.Response "查询云盘令牌失败，code=4019"
// @Failure 400 {object} httpcontext.Response "修改令牌失败，code=4030"
// @Failure 401 {object} httpcontext.Response "未授权访问"
// @Failure 403 {object} httpcontext.Response "权限不足"
// @Router /api/storage/modify_token [post]
func (h *handler) ModifyToken() httpcontext.HandlerFunc {
	return func(ctx *httpcontext.Context) {
		req := new(modifyTokenRequest)
		if err := ctx.ShouldBindJSON(req); err != nil {
			ctx.AbortWithInvalidParams(err)

			return
		}

		// 获取当前用户信息用于权限控制
		userID := ctx.GetInt64(consts.CtxKeyUserId)
		isAdmin := ctx.GetBool(consts.CtxKeyIsAdmin)

		// 验证挂载点是否存在
		mp, err := h.mountPointService.Query(ctx.GetContext(), req.ID)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				ctx.Fail(busCodeStorageMountPointNotFound.WithError(err))
			} else {
				ctx.Fail(busCodeStorageQueryMountPointError.WithError(err))
			}

			return
		}

		// 验证用户是否有权限访问这个挂载点
		// 非管理员：只能访问自己创建的或用户组分享的挂载点
		if !isAdmin {
			hasAccess := false
			if mp.CreatorUserID == userID {
				hasAccess = true
			} else {
				// 检查用户组是否有权限
				userGroupId := ctx.GetInt64(consts.CtxKeyUserGroupId)
				if userGroupId > 0 {
					groupFileIds, err := h.group2FileService.GetBindFiles(ctx.GetContext(), userGroupId)
					if err != nil {
						ctx.Fail(busCodeStorageQueryMountPointError.WithError(err))

						return
					}

					for _, fid := range groupFileIds {
						if fid == mp.FileId {
							hasAccess = true

							break
						}
					}
				}
			}

			if !hasAccess && h.userMountPointTokenService != nil {
				tokenID, err := h.userMountPointTokenService.GetTokenID(ctx.GetContext(), userID, mp.ID)
				if err != nil {
					ctx.Fail(busCodeStorageQueryMountPointError.WithError(err))

					return
				}

				hasAccess = tokenID > 0
			}

			if !hasAccess {
				ctx.Fail(busCodeStorageMountPointNotFound)

				return
			}
		}

		// 验证云盘令牌是否存在（必须是用户自己的令牌）
		if req.TokenID != 0 {
			_, err := h.cloudTokenService.QueryAccessible(ctx.GetContext(), req.TokenID, userID, isAdmin)
			if err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					ctx.Fail(busCodeStorageCloudTokenNotExist.WithError(err))
				} else {
					ctx.Fail(busCodeStorageQueryCloudTokenError.WithError(err))
				}

				return
			}
		}

		// 绑定用户的令牌到挂载点
		if req.TokenID == 0 {
			// 解除绑定
			if err := h.userMountPointTokenService.UnbindToken(ctx.GetContext(), userID, mp.ID); err != nil {
				ctx.Fail(busCodeStorageModifyTokenFailed.WithError(err))

				return
			}
		} else {
			// 绑定令牌
			if err := h.userMountPointTokenService.BindToken(ctx.GetContext(), userID, mp.ID, req.TokenID); err != nil {
				ctx.Fail(busCodeStorageModifyTokenFailed.WithError(err))

				return
			}
		}

		ctx.Success()
	}
}
