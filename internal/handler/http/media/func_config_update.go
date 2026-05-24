package media

import (
	"errors"

	"github.com/xxcheng123/cloudpan189-share/internal/framework/httpcontext"
	"github.com/xxcheng123/cloudpan189-share/internal/pkgs/datatypes"
	"github.com/xxcheng123/cloudpan189-share/internal/pkgs/utils"
	"github.com/xxcheng123/cloudpan189-share/internal/types/media"
	"gorm.io/gorm"
)

// configUpdateRequest 媒体配置更新请求（部分字段可选）
type configUpdateRequest struct {
	Enable              *bool                     `json:"enable" binding:"omitempty" example:"false"`
	StoragePath         *string                   `json:"storagePath" binding:"omitempty" example:"/opt/media"`
	AutoClean           *bool                     `json:"autoClean" binding:"omitempty" example:"true"`
	ConflictPolicy      *media.FileConflictPolicy `json:"conflictPolicy" binding:"omitempty,oneof=skip replace" example:"skip"`
	BaseURL             *string                   `json:"baseURL" binding:"omitempty" example:"http://localhost:12395"`
	IncludedSuffixes    *[]string                 `json:"includedSuffixes" binding:"omitempty" example:"['.mp4','.mkv','.avi']"`
	AutoRebuildEnable   *bool                     `json:"autoRebuildEnable" binding:"omitempty" example:"false"`
	AutoRebuildInterval *int                      `json:"autoRebuildInterval" binding:"omitempty" example:"24"`
	AutoRebuildCron     *string                   `json:"autoRebuildCron" binding:"omitempty" example:"0 2 * * *"`
}

// ConfigUpdate 更新媒体配置指定字段
// @Summary 更新媒体配置
// @Tags 媒体配置
// @Accept json
// @Produce json
// @Param Authorization header string true "Bearer token"
// @Param body body configUpdateRequest true "更新请求体（任意字段可选）"
// @Success 200 {object} httpcontext.Response "更新成功"
// @Failure 400 {object} httpcontext.Response "参数验证失败，code=99998"
// @Failure 400 {object} httpcontext.Response "更新媒体配置失败"
// @Failure 404 {object} httpcontext.Response "媒体配置未初始化"
// @Router /api/media/config/update [post]
func (h *handler) ConfigUpdate() httpcontext.HandlerFunc {
	return func(ctx *httpcontext.Context) {
		req := new(configUpdateRequest)
		if err := ctx.ShouldBindJSON(req); err != nil {
			ctx.AbortWithInvalidParams(err)

			return
		}

		fields := make([]utils.Field, 0, 5)

		if req.Enable != nil {
			fields = append(fields, utils.WithField("enable", *req.Enable))
		}

		if req.StoragePath != nil {
			fields = append(fields, utils.WithField("storage_path", *req.StoragePath))
		}

		if req.AutoClean != nil {
			fields = append(fields, utils.WithField("auto_clean", *req.AutoClean))
		}

		if req.ConflictPolicy != nil {
			fields = append(fields, utils.WithField("conflict_policy", *req.ConflictPolicy))
		}

		if req.BaseURL != nil {
			fields = append(fields, utils.WithField("base_url", *req.BaseURL))
		}

		if req.IncludedSuffixes != nil {
			fields = append(fields, utils.WithField("included_suffixes", datatypes.NewJSONSlice(*req.IncludedSuffixes)))
		}

		if req.AutoRebuildEnable != nil {
			fields = append(fields, utils.WithField("auto_rebuild_enable", *req.AutoRebuildEnable))
		}

		if req.AutoRebuildInterval != nil {
			fields = append(fields, utils.WithField("auto_rebuild_interval", *req.AutoRebuildInterval))
		}

		if req.AutoRebuildCron != nil {
			fields = append(fields, utils.WithField("auto_rebuild_cron", *req.AutoRebuildCron))
		}

		if len(fields) == 0 {
			ctx.Success()

			return
		}

		if err := h.mediaConfigService.Update(ctx.GetContext(), fields...); err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				ctx.Fail(codeConfigNotInit.WithError(err))

				return
			}

			ctx.Fail(codeConfigUpdateFailed.WithError(err))

			return
		}

		ctx.Success()
	}
}
