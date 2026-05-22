package media

import (
	"github.com/xxcheng123/cloudpan189-share/internal/framework/httpcontext"
	"github.com/xxcheng123/cloudpan189-share/internal/services/mediaconfig"
	"github.com/xxcheng123/cloudpan189-share/internal/types/media"
)

// configInitRequest 初始化/更新媒体配置请求
type configInitRequest struct {
	Enable              bool                     `json:"enable" example:"false"`
	StoragePath         string                   `json:"storagePath" binding:"required" example:"/opt/media"`
	AutoClean           *bool                    `json:"autoClean" binding:"required" example:"true"`
	ConflictPolicy      media.FileConflictPolicy `json:"conflictPolicy" binding:"omitempty,oneof=skip replace" example:"skip"`
	BaseURL             string                   `json:"baseURL" binding:"required" example:"http://localhost:12395"`
	IncludedSuffixes    []string                 `json:"includedSuffixes" binding:"omitempty" example:"['.mp4','.mkv','.avi']"`
	AutoRebuildEnable   bool                     `json:"autoRebuildEnable" example:"false"`
	AutoRebuildInterval int                      `json:"autoRebuildInterval" example:"24"`
}

// ConfigInit 初始化媒体配置
// @Summary 初始化媒体配置
// @Tags 媒体配置
// @Accept json
// @Produce json
// @Param Authorization header string true "Bearer token"
// @Param body body configInitRequest true "初始化请求体"
// @Success 200 {object} httpcontext.Response "初始化成功"
// @Failure 400 {object} httpcontext.Response "参数验证失败，code=99998"
// @Failure 400 {object} httpcontext.Response "初始化媒体配置失败"
// @Router /api/media/config/init [post]
func (h *handler) ConfigInit() httpcontext.HandlerFunc {
	return func(ctx *httpcontext.Context) {
		req := new(configInitRequest)
		if err := ctx.ShouldBindJSON(req); err != nil {
			ctx.AbortWithInvalidParams(err)

			return
		}

		iReq := &mediaconfig.InitRequest{
			Enable:              req.Enable,
			StoragePath:         req.StoragePath,
			AutoClean:           *req.AutoClean,
			ConflictPolicy:      req.ConflictPolicy,
			BaseURL:             req.BaseURL,
			IncludedSuffixes:    req.IncludedSuffixes,
			AutoRebuildEnable:   req.AutoRebuildEnable,
			AutoRebuildInterval: req.AutoRebuildInterval,
		}

		if err := h.mediaConfigService.Init(ctx.GetContext(), iReq); err != nil {
			ctx.Fail(codeConfigInitFailed.WithError(err))

			return
		}

		ctx.Success()
	}
}
