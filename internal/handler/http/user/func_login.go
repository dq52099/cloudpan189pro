package user

import (
	"errors"

	"github.com/xxcheng123/cloudpan189-share/internal/consts"
	"github.com/xxcheng123/cloudpan189-share/internal/framework/httpcontext"
	"github.com/xxcheng123/cloudpan189-share/internal/pkgs/utils"
	"github.com/xxcheng123/cloudpan189-share/internal/repository/models"
	"gorm.io/gorm"
)

type loginRequest struct {
	Username string `json:"username" binding:"required,min=3,max=20" example:"tom123"`   // 用户名，长度3-20位
	Password string `json:"password" binding:"required,min=6,max=20" example:"12345678"` // 密码，长度6-20位
}

type loginResponse struct {
	AccessToken  string       `json:"accessToken" example:"eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."`  // 访问令牌
	RefreshToken string       `json:"refreshToken" example:"eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."` // 刷新令牌
	TokenType    string       `json:"tokenType" example:"Bearer"`                                     // 令牌类型
	ExpiresIn    int64        `json:"expiresIn" example:"3600"`                                       // 过期时间（秒）
	User         *models.User `json:"user"`                                                           // 用户信息
}

// Login 用户登录
// @Summary 用户登录
// @Description 用户通过用户名和密码登录系统，获取访问令牌
// @Tags 用户认证
// @Accept json
// @Produce json
// @Param request body loginRequest true "登录信息"
// @Success 200 {object} httpcontext.Response{data=loginResponse} "登录成功"
// @Failure 400 {object} httpcontext.Response "参数验证失败，code=99998"
// @Failure 400 {object} httpcontext.Response "登录失败，code=1002"
// @Failure 400 {object} httpcontext.Response "用户被禁用，code=1003"
// @Failure 400 {object} httpcontext.Response "Token生成失败，code=1004"
// @Router /api/user/login [post]
func (h *handler) Login() httpcontext.HandlerFunc {
	return func(ctx *httpcontext.Context) {
		req := new(loginRequest)
		if err := ctx.ShouldBindJSON(req); err != nil {
			ctx.AbortWithInvalidParams(err)

			return
		}

		ctx.Set(consts.CtxKeyUsername, req.Username)

		if !h.ensureUserService(ctx, codeLoginFailed) {
			return
		}

		user, err := h.userService.QueryByUsername(ctx.GetContext(), req.Username)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				ctx.Fail(codeUserNotFound.WithError(err))

				return
			}

			ctx.Fail(codeLoginFailed.WithError(err))

			return
		}

		if user == nil {
			ctx.Fail(codeUserNotFound.WithError(gorm.ErrRecordNotFound))

			return
		}

		ctx.Set(consts.CtxKeyUserId, user.ID)

		// 验证密码
		if user.Password != utils.MD5(req.Password) {
			ctx.Fail(codeUserPasswordFailed)

			return
		}

		if !user.Valid() {
			ctx.Fail(codeUserDisabled)

			return
		}

		// 生成访问Token
		accessToken, err := h.userService.GenerateAccessToken(user.ID, user.Username, user.Version)
		if err != nil {
			ctx.Fail(codeTokenGenerateFailed.WithError(err))

			return
		}

		// 生成刷新Token
		refreshToken, err := h.userService.GenerateRefreshToken(user.ID, user.Username, user.Version)
		if err != nil {
			ctx.Fail(codeTokenGenerateFailed.WithError(err))

			return
		}

		ctx.Success(&loginResponse{
			AccessToken:  accessToken,
			RefreshToken: refreshToken,
			TokenType:    "Bearer",
			ExpiresIn:    h.userService.GetExpire(),
			User:         user,
		})
	}
}
