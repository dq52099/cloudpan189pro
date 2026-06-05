package setting

import (
	"time"

	"github.com/xxcheng123/cloudpan189-share/internal/framework/httpcontext"
	"github.com/xxcheng123/cloudpan189-share/internal/pkgs/utils"
	"gorm.io/gorm"
)

type infoResponse struct {
	RunTime      int64  `json:"runTime"`      // 运行时间 单位 s
	RunTimeHuman string `json:"runTimeHuman"` // 运行时间 格式 例如：1年2月3天4小时5分6秒
	BaseURL      string `json:"baseURL"`
	Initialized  bool   `json:"initialized"`
	EnableAuth   bool   `json:"enableAuth"`
	Title        string `json:"title"`
}

// Info 获取系统信息
// @Summary 获取系统信息
// @Description 获取系统运行状态信息，包括运行时间、基础URL、初始化状态和认证启用状态
// @Tags 系统设置
// @Accept json
// @Produce json
// @Success 200 {object} httpcontext.Response{data=infoResponse} "获取系统信息成功"
// @Failure 400 {object} httpcontext.Response "查询系统配置失败，code=2003"
// @Router /api/setting/info [get]
func (h *handler) Info() httpcontext.HandlerFunc {
	return func(ctx *httpcontext.Context) {
		if !h.ensureSettingService(ctx, codeQueryFailed) {
			return
		}

		setting, err := h.settingService.Query(ctx.GetContext())
		if err != nil {
			ctx.Fail(codeQueryFailed.WithError(err))

			return
		}

		if setting == nil {
			ctx.Fail(codeQueryFailed.WithError(gorm.ErrRecordNotFound))

			return
		}

		duration := time.Since(h.initTime)

		resp := &infoResponse{
			BaseURL:      setting.BaseURL,
			Initialized:  setting.Initialized,
			EnableAuth:   setting.EnableAuth,
			RunTime:      int64(duration.Seconds()),
			RunTimeHuman: utils.FormatDuration(duration),
			Title:        setting.Title,
		}

		ctx.Success(resp)
	}
}
