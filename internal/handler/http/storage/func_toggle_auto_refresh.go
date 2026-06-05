package storage

import (
	"time"

	"github.com/xxcheng123/cloudpan189-share/internal/framework/httpcontext"
	"github.com/xxcheng123/cloudpan189-share/internal/pkgs/ptr"
	"github.com/xxcheng123/cloudpan189-share/internal/services/mountpoint"
)

type toggleAutoRefreshRequest struct {
	ID                int64  `json:"id" binding:"required,gt=0" example:"1"`                                                // 挂载点ID
	EnableAutoRefresh *bool  `json:"enableAutoRefresh" binding:"required" example:"true"`                                   // 是否启用自动刷新
	AutoRefreshDays   int    `json:"autoRefreshDays,omitempty" binding:"omitempty,min=1,max=365" example:"7"`               // 自动刷新持续天数，单位天，最小值1，最大值365
	RefreshInterval   int    `json:"refreshInterval,omitempty" binding:"omitempty,min=30,max=1440" example:"30"`            // 刷新间隔，单位分钟，最小值30，最大值1440
	RefreshBeginAt    string `json:"refreshBeginAt,omitempty" binding:"omitempty,datetime=2006-01-02" example:"2023-01-01"` // 自动刷新开始时间，格式：yyyy-MM-dd，默认为当前时间
	EnableDeepRefresh bool   `json:"enableDeepRefresh,omitempty" example:"false"`                                           // 是否启用深度刷新
}

// ToggleAutoRefresh 切换自动刷新配置
// @Summary 切换自动刷新配置
// @Description 启用或禁用存储挂载点的自动刷新功能，并可配置相关参数
// @Tags 存储管理
// @Accept json
// @Produce json
// @Param Authorization header string true "Bearer token"
// @Param request body toggleAutoRefreshRequest true "自动刷新配置信息"
// @Success 200 {object} httpcontext.Response "自动刷新配置更新成功"
// @Failure 400 {object} httpcontext.Response "参数验证失败，code=99998"
// @Failure 400 {object} httpcontext.Response "切换自动刷新失败，code=4025"
// @Failure 400 {object} httpcontext.Response "更新自动刷新持续天数失败，code=4026"
// @Failure 400 {object} httpcontext.Response "更新刷新间隔失败，code=4027"
// @Failure 400 {object} httpcontext.Response "时间格式错误，code=4028"
// @Failure 400 {object} httpcontext.Response "更新自动刷新开始时间失败，code=4029"
// @Failure 401 {object} httpcontext.Response "未授权访问"
// @Failure 403 {object} httpcontext.Response "权限不足"
// @Router /api/storage/toggle_auto_refresh [post]
func (h *handler) ToggleAutoRefresh() httpcontext.HandlerFunc {
	return func(ctx *httpcontext.Context) {
		req := new(toggleAutoRefreshRequest)
		if err := ctx.ShouldBindJSON(req); err != nil {
			ctx.AbortWithInvalidParams(err)

			return
		}

		mountPoint, ok := h.queryOwnedMountPoint(ctx, req.ID)
		if !ok {
			return
		}

		if !*req.EnableAutoRefresh {
			if !h.ensureMountPointService(ctx, busCodeStorageToggleAutoRefreshError) {
				return
			}

			if err := h.mountPointService.EnableAutoRefresh(ctx.GetContext(), mountPoint.FileId, false); err != nil {
				ctx.Fail(busCodeStorageToggleAutoRefreshError.WithError(err))

				return
			}

			ctx.Success()

			return
		}

		// 构建刷新配置
		config := mountpoint.RefreshConfig{}

		if req.AutoRefreshDays > 0 {
			config.AutoRefreshDays = ptr.Of(req.AutoRefreshDays)
		}

		if req.RefreshInterval > 0 {
			config.RefreshInterval = ptr.Of(req.RefreshInterval)
		}

		if req.RefreshBeginAt != "" {
			formatTime, err := time.Parse(time.DateOnly, req.RefreshBeginAt)
			if err != nil {
				ctx.Fail(busCodeStorageTimeFormatErr.WithError(err))

				return
			}

			config.AutoRefreshBeginAt = ptr.Of(formatTime)
		}

		// 设置深度刷新配置
		config.EnableDeepRefresh = ptr.Of(req.EnableDeepRefresh)

		// 使用合并后的方法更新刷新配置
		if !h.ensureMountPointService(ctx, busCodeStorageUpdateRefreshIntervalErr) {
			return
		}

		if err := h.mountPointService.UpdateRefreshConfig(ctx.GetContext(), mountPoint.FileId, config); err != nil {
			ctx.Fail(busCodeStorageUpdateRefreshIntervalErr.WithError(err))

			return
		}

		if err := h.mountPointService.EnableAutoRefresh(ctx.GetContext(), mountPoint.FileId, true); err != nil {
			ctx.Fail(busCodeStorageToggleAutoRefreshError.WithError(err))

			return
		}

		ctx.Success()
	}
}
