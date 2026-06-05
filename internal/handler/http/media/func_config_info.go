package media

import (
	"errors"

	"github.com/xxcheng123/cloudpan189-share/internal/framework/httpcontext"
	"github.com/xxcheng123/cloudpan189-share/internal/pkgs/utils"
	"github.com/xxcheng123/cloudpan189-share/internal/repository/models"
	"gorm.io/gorm"
)

type configInfoResponse struct {
	Initialized bool                `json:"initialized"`
	Config      *models.MediaConfig `json:"config,omitempty"`
}

// ConfigInfo 获取媒体配置
// @Summary 获取媒体配置
// @Tags 媒体配置
// @Accept json
// @Produce json
// @Param Authorization header string true "Bearer token"
// @Success 200 {object} httpcontext.Response{data=configInfoResponse} "获取成功"
// @Failure 400 {object} httpcontext.Response "查询媒体配置失败"
// @Router /api/media/config/info [get]
func (h *handler) ConfigInfo() httpcontext.HandlerFunc {
	return func(ctx *httpcontext.Context) {
		if !h.ensureMediaConfigService(ctx, codeConfigQueryFailed) {
			return
		}

		cfg, err := h.mediaConfigService.Query(ctx.GetContext())
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				ctx.Success(&configInfoResponse{
					Initialized: false,
					Config:      nil,
				})

				return
			}

			ctx.Fail(codeConfigQueryFailed.WithError(err))

			return
		}

		if cfg == nil {
			ctx.Success(&configInfoResponse{
				Initialized: false,
				Config:      nil,
			})

			return
		}

		ctx.Success(&configInfoResponse{
			Initialized: true,
			Config:      mediaConfigResponse(cfg),
		})
	}
}

func mediaConfigResponse(cfg *models.MediaConfig) *models.MediaConfig {
	if cfg == nil {
		return nil
	}

	response := *cfg
	response.BaseURL = utils.RedactURLForLog(response.BaseURL)

	return &response
}
