package cloudtoken

import (
	"github.com/xxcheng123/cloudpan189-share/internal/consts"
	"github.com/xxcheng123/cloudpan189-share/internal/framework/httpcontext"
	"github.com/xxcheng123/cloudpan189-share/internal/repository/models"
	"github.com/xxcheng123/cloudpan189-share/internal/services/cloudtoken"
)

type (
	usernameLoginRequest = struct {
		ID       int64  `json:"id" binding:"omitempty" example:"1"`          // 云盘令牌ID，可选
		Username string `json:"username"  binding:"omitempty" example:"用户名"` // 用户名，添加时必填
		Password string `json:"password"  binding:"omitempty" example:"密码"`  // 密码，添加时必填
		Name     string `json:"name" binding:"omitempty" example:"云盘令牌"`     // 令牌名称，可选
	}
	UsernameLoginResponse = cloudtoken.UsernameLoginResponse
)

// UsernameLogin 用户名密码登录
// @Summary 用户名密码登录
// @Description 使用用户名和密码登录云盘，创建或更新云盘令牌
// @Tags 云盘令牌管理
// @Accept json
// @Produce json
// @Param Authorization header string true "Bearer token"
// @Param request body usernameLoginRequest true "用户名登录请求"
// @Success 200 {object} httpcontext.Response{data=cloudtoken.UsernameLoginResponse} "登录成功"
// @Failure 400 {object} httpcontext.Response "参数验证失败，code=99998"
// @Failure 400 {object} httpcontext.Response "用户名登录失败，code=5006"
// @Failure 401 {object} httpcontext.Response "未授权访问"
// @Failure 403 {object} httpcontext.Response "权限不足"
// @Router /api/cloud_token/username_login [post]
func (h *handler) UsernameLogin() httpcontext.HandlerFunc {
	return func(ctx *httpcontext.Context) {
		req := new(usernameLoginRequest)
		if err := ctx.ShouldBindJSON(req); err != nil {
			ctx.AbortWithInvalidParams(err)

			return
		}

		var (
			username      = req.Username
			password      = req.Password
			currentUserID = ctx.GetInt64(consts.CtxKeyUserId)
			isAdmin       = ctx.GetBool(consts.CtxKeyIsAdmin)
		)

		// 检查账号密码是否完整
		if req.ID == 0 {
			if req.Username == "" {
				ctx.Fail(codeMissUsername)

				return
			} else if req.Password == "" {
				ctx.Fail(codeMissPassword)

				return
			}
		} else if req.Username == "" || req.Password == "" {
			// 去查询账号密码
			token, err := h.cloudTokenService.QueryAccessible(ctx.GetContext(), req.ID, currentUserID, isAdmin)
			if err != nil {
				ctx.Fail(codeQueryFailed.WithError(err))

				return
			} else if token.LoginType != models.LoginTypePassword {
				ctx.Fail(codeNotMatchLoginType)

				return
			}

			if username == "" {
				username = token.Username
			}

			if password == "" {
				password = token.Password
			}
		}

		name := "云盘令牌（账密）"
		if req.Name != "" {
			name = req.Name
		}

		resp, err := h.cloudTokenService.UsernameLogin(ctx.GetContext(), &cloudtoken.UsernameLoginRequest{
			Username: username,
			Password: password,
			Name:     name,
			ID:       req.ID,
			UserID:   currentUserID,
			IsAdmin:  isAdmin,
		})
		if err != nil {
			ctx.Fail(codeUsernameLoginFailed.WithError(err))

			return
		}

		ctx.Success(resp)
	}
}
