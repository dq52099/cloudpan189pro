package setting

import (
	"github.com/xxcheng123/cloudpan189-share/internal/framework/httpcontext"
	"github.com/xxcheng123/cloudpan189-share/internal/services/setting"
	"github.com/xxcheng123/cloudpan189-share/internal/services/user"
)

type initSystemRequest struct {
	Title         string `json:"title" binding:"required" example:"我的云盘系统"`                      // 系统标题
	EnableAuth    *bool  `json:"enableAuth" binding:"required" example:"true"`                   // 是否启用认证
	BaseURL       string `json:"baseURL" binding:"required,url" example:"https://example.com"`   // 系统基础URL
	SuperUsername string `json:"superUsername" binding:"required,min=3,max=20" example:"admin"`  // 超级管理员用户名，长度3-20位
	SuperPassword string `json:"superPassword" binding:"required,min=6,max=20" example:"123456"` // 超级管理员密码，长度6-20位
}

// InitSystem 初始化系统
// @Summary 初始化系统
// @Description 初始化系统配置并创建超级管理员账户。此接口只能在系统未初始化时调用，初始化后将无法再次调用
// @Tags 系统设置
// @Accept json
// @Produce json
// @Param request body initSystemRequest true "系统初始化信息"
// @Success 200 {object} httpcontext.Response "系统初始化成功"
// @Failure 400 {object} httpcontext.Response "参数验证失败，code=99998"
// @Failure 400 {object} httpcontext.Response "初始化系统时发生错误，code=6001"
// @Failure 400 {object} httpcontext.Response "初始化超级管理员时发生错误，code=6002"
// @Router /api/setting/init_system [post]
func (h *handler) InitSystem() httpcontext.HandlerFunc {
	return func(ctx *httpcontext.Context) {
		req := new(initSystemRequest)
		if err := ctx.ShouldBindJSON(req); err != nil {
			ctx.AbortWithInvalidParams(err)

			return
		}

		if err := h.settingService.InitSystem(ctx.GetContext(), &setting.InitSystemRequest{
			Title:      req.Title,
			EnableAuth: *req.EnableAuth,
			BaseURL:    req.BaseURL,
		}); err != nil {
			ctx.Fail(codeInitSettingErr.WithError(err))

			return
		}

		if _, err := h.userService.Add(ctx.GetContext(), &user.AddRequest{
			Username: req.SuperUsername,
			Password: req.SuperPassword,
		}, user.WithAdmin()); err != nil {
			ctx.Fail(codeInitSuperUserErr.WithError(err))

			return
		}

		ctx.Success()
	}
}
