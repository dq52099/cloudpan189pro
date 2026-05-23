package autoingest

import (
	"errors"

	"github.com/xxcheng123/cloudpan189-share/internal/framework/httpcontext"
	"github.com/xxcheng123/cloudpan189-share/internal/pkgs/utils"
	"github.com/xxcheng123/cloudpan189-share/internal/types/autoingest"
	"gorm.io/gorm"
)

type refreshStrategyUpdateRequest struct {
	EnableAutoRefresh *bool `json:"enableAutoRefresh" binding:"omitempty" example:"false"`
	AutoRefreshDays   *int  `json:"autoRefreshDays" binding:"omitempty,min=1" example:"7"`
	RefreshInterval   *int  `json:"refreshInterval" binding:"omitempty,min=30,max=1440" example:"30"` // 单位分钟，最小30，最大1440
	EnableDeepRefresh *bool `json:"enableDeepRefresh" binding:"omitempty" example:"false"`
}

type updatePlanRequest struct {
	ID                 int64                         `json:"id" binding:"required,min=1" example:"1"`
	Name               *string                       `json:"name" binding:"omitempty,max=255" example:"订阅计划A"`
	AutoIngestInterval *int64                        `json:"autoIngestInterval" binding:"omitempty,min=5" example:"30"` // 单位分钟
	ParentPath         *string                       `json:"parentPath" binding:"omitempty" example:"/Movies"`
	OnConflict         *string                       `json:"onConflict" binding:"omitempty,oneof=rename abandon" example:"rename"` // 冲突时的解决策略
	RefreshStrategy    *refreshStrategyUpdateRequest `json:"refreshStrategy"`
	TokenId            *int64                        `json:"tokenId"`
}

// UpdatePlan 修改自动挂载计划（仅允许指定字段）
// @Summary 修改自动挂载计划
// @Description 仅允许修改字段：parentPath、refreshStrategy、tokenId、autoIngestInterval、name、onConflict
// @Tags 自动挂载管理
// @Accept json
// @Produce json
// @Param Authorization header string true "Bearer token"
// @Param request body updatePlanRequest true "修改计划参数"
// @Success 200 {object} httpcontext.Response "修改成功"
// @Failure 400 {object} httpcontext.Response "参数验证失败，code=99998"
// @Failure 400 {object} httpcontext.Response "查询自动挂载计划失败，code=xxxx"
// @Failure 400 {object} httpcontext.Response "自动挂载计划不存在，code=xxxx"
// @Failure 400 {object} httpcontext.Response "更新自动挂载计划失败，code=xxxx"
// @Failure 401 {object} httpcontext.Response "未授权访问"
// @Failure 403 {object} httpcontext.Response "权限不足"
// @Router /api/auto_ingest/plan/update [post]
func (h *handler) UpdatePlan() httpcontext.HandlerFunc {
	return func(ctx *httpcontext.Context) {
		req := new(updatePlanRequest)
		if err := ctx.ShouldBindJSON(req); err != nil {
			ctx.AbortWithInvalidParams(err)

			return
		}

		// 查询计划是否存在
		plan, err := h.planService.Query(ctx.GetContext(), req.ID)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				ctx.Fail(codePlanNotFound.WithError(err))

				return
			}

			ctx.Fail(codePlanQueryFailed.WithError(err))

			return
		}

		if plan == nil || plan.ID == 0 {
			ctx.Fail(codePlanNotFound)

			return
		}

		if !ensurePlanAccess(ctx, plan) {
			return
		}

		// 仅收集允许的字段
		fields := make([]utils.Field, 0, 10)

		if req.Name != nil {
			fields = append(fields, utils.Field{Key: "name", Value: *req.Name})
		}

		if req.AutoIngestInterval != nil {
			fields = append(fields, utils.Field{Key: "auto_ingest_interval", Value: *req.AutoIngestInterval})
		}

		if req.ParentPath != nil {
			fields = append(fields, utils.Field{Key: "parent_path", Value: *req.ParentPath})
		}

		if req.OnConflict != nil {
			fields = append(fields, utils.Field{Key: "on_conflict", Value: autoingest.OnConflict(*req.OnConflict)})
		}

		if req.TokenId != nil {
			if err := h.validateCloudTokenAccess(ctx, *req.TokenId); err != nil {
				ctx.Fail(codePlanUpdateFailed.WithError(err))

				return
			}

			fields = append(fields, utils.Field{Key: "token_id", Value: *req.TokenId})
		}

		// 刷新策略映射到嵌入列
		if req.RefreshStrategy != nil {
			// 设置默认/兜底规则，仅在传入某项时应用该项
			if req.RefreshStrategy.EnableAutoRefresh != nil {
				fields = append(fields, utils.Field{Key: "refresh_strategy_enable_auto_refresh", Value: *req.RefreshStrategy.EnableAutoRefresh})
			}

			if req.RefreshStrategy.AutoRefreshDays != nil {
				days := *req.RefreshStrategy.AutoRefreshDays
				if days <= 0 {
					days = 7
				}

				fields = append(fields, utils.Field{Key: "refresh_strategy_auto_refresh_days", Value: days})
			}

			if req.RefreshStrategy.RefreshInterval != nil {
				interval := *req.RefreshStrategy.RefreshInterval
				if interval < 30 {
					interval = 30
				}

				fields = append(fields, utils.Field{Key: "refresh_strategy_refresh_interval", Value: interval})
			}

			if req.RefreshStrategy.EnableDeepRefresh != nil {
				fields = append(fields, utils.Field{Key: "refresh_strategy_enable_deep_refresh", Value: *req.RefreshStrategy.EnableDeepRefresh})
			}
		}

		// 如果没有可更新字段，直接返回成功（不做任何变更）
		if len(fields) == 0 {
			ctx.Success()

			return
		}

		if err := h.planService.Update(ctx.GetContext(), req.ID, fields...); err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				ctx.Fail(codePlanNotFound.WithError(err))

				return
			}

			ctx.Fail(codePlanUpdateFailed.WithError(err))

			return
		}

		ctx.Success()
	}
}
