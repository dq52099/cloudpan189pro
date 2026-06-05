package user

import (
	"github.com/xxcheng123/cloudpan189-share/internal/consts"
	"github.com/xxcheng123/cloudpan189-share/internal/framework/httpcontext"
	"gorm.io/gorm"
)

// refreshRequest 刷新Token请求结构
type refreshRequest struct {
	RefreshToken string `json:"refreshToken" binding:"required" example:"eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."` // 刷新令牌
}

type refreshResponse = loginResponse

// RefreshToken 刷新访问令牌
// @Summary 刷新访问令牌
// @Description 使用刷新令牌获取新的访问令牌和刷新令牌
// @Tags 用户认证
// @Accept json
// @Produce json
// @Param request body refreshRequest true "刷新令牌信息"
// @Success 200 {object} httpcontext.Response{data=refreshResponse} "令牌刷新成功"
// @Failure 400 {object} httpcontext.Response "参数验证失败，code=99998"
// @Failure 400 {object} httpcontext.Response "刷新令牌无效，code=1005"
// @Failure 400 {object} httpcontext.Response "用户被禁用，code=1003"
// @Failure 400 {object} httpcontext.Response "用户信息已更新，code=1006"
// @Failure 400 {object} httpcontext.Response "Token生成失败，code=1004"
// @Router /api/user/refresh_token [post]
func (h *handler) RefreshToken() httpcontext.HandlerFunc {
	return func(ctx *httpcontext.Context) {
		req := new(refreshRequest)
		if err := ctx.ShouldBindJSON(req); err != nil {
			ctx.AbortWithInvalidParams(err)

			return
		}

		if !h.ensureUserService(ctx, codeRefreshTokenInvalid) {
			return
		}

		// 解析刷新Token
		uid, parsedUsername, version, err := h.userService.ParseRefreshToken(req.RefreshToken)
		if err != nil {
			ctx.Fail(codeRefreshTokenInvalid.WithError(err))

			return
		}

		ctx.Set(consts.CtxKeyUserId, uid)
		ctx.Set(consts.CtxKeyUsername, parsedUsername)

		user, err := h.userService.Query(ctx.GetContext(), uid)
		if err != nil {
			ctx.Fail(codeRefreshTokenInvalid.WithError(err))

			return
		}

		if user == nil {
			ctx.Fail(codeRefreshTokenInvalid.WithError(gorm.ErrRecordNotFound))

			return
		}

		ctx.Set(consts.CtxKeyUsername, user.Username)

		if !user.Valid() {
			ctx.Fail(codeUserDisabled)

			return
		}

		if user.Version > version {
			ctx.Fail(codeUserInfoUpdated)

			return
		}

		accessToken, err := h.userService.GenerateAccessToken(user.ID, user.Username, user.Version)
		if err != nil {
			ctx.Fail(codeTokenGenerateFailed.WithError(err))

			return
		}

		newRefreshToken, err := h.userService.GenerateRefreshToken(user.ID, user.Username, user.Version)
		if err != nil {
			ctx.Fail(codeTokenGenerateFailed.WithError(err))

			return
		}

		ctx.Success(&refreshResponse{
			AccessToken:  accessToken,
			RefreshToken: newRefreshToken,
			TokenType:    "Bearer",
			ExpiresIn:    h.userService.GetExpire(),
			User:         user,
		})
	}
}
