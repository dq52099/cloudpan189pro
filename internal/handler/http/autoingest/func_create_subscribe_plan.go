package autoingest

import (
	"encoding/json"
	"time"

	"github.com/xxcheng123/cloudpan189-share/internal/consts"
	"github.com/xxcheng123/cloudpan189-share/internal/framework/httpcontext"
	"github.com/xxcheng123/cloudpan189-share/internal/repository/models"
	"github.com/xxcheng123/cloudpan189-share/internal/types/autoingest"
	"github.com/xxcheng123/cloudpan189-share/internal/types/topic"
)

// CreateSubscribePlan 创建订阅型自动挂载计划（占位实现）
// 说明：请求结构体与处理逻辑由你后续补充，这里仅提供占位使编译通过并符合注释规范。
type refreshStrategyRequest struct {
	EnableAutoRefresh bool `json:"enableAutoRefresh" binding:"omitempty" example:"false"`
	AutoRefreshDays   int  `json:"autoRefreshDays" binding:"omitempty,min=1" example:"7"`
	RefreshInterval   int  `json:"refreshInterval" binding:"omitempty,min=30" example:"30"` // 单位分钟，最小30
	EnableDeepRefresh bool `json:"enableDeepRefresh" binding:"omitempty" example:"false"`
}

type createSubscribePlanRequest struct {
	Name               string                 `json:"name" binding:"required,max=255" example:"订阅计划A"`
	AutoIngestInterval int64                  `json:"autoIngestInterval" binding:"omitempty,min=5" example:"30"` // 单位分钟
	ParentPath         string                 `json:"parentPath" binding:"required" example:"/Movies"`
	OnConflict         string                 `json:"onConflict" binding:"omitempty,oneof=rename abandon" example:"rename"` // 冲突时的解决策略
	UpUserId           string                 `json:"upUserId" binding:"required" example:"123456"`                         // 上传用户ID
	OneClickAddHistory bool                   `json:"oneClickAddHistory" binding:"omitempty" example:"true"`                // 是否一键添加之前的
	RefreshStrategy    refreshStrategyRequest `json:"refreshStrategy"`
	CloudToken         int64                  `json:"cloudToken"`
}

type createSubscribePlanResponse struct {
	ID int64 `json:"id" example:"1"`
}

// CreateSubscribePlan 创建订阅计划（占位）
// @Summary 创建订阅型自动挂载计划
// @Description 创建订阅计划（占位实现，后续由你补充具体逻辑）
// @Tags 自动挂载管理
// @Accept json
// @Produce json
// @Param Authorization header string true "Bearer token"
// @Param request body createSubscribePlanRequest true "订阅计划参数（占位）"
// @Success 200 {object} httpcontext.Response{data=createSubscribePlanResponse} "创建成功"
// @Failure 400 {object} httpcontext.Response "参数验证失败，code=99998"
// @Failure 401 {object} httpcontext.Response "未授权访问"
// @Failure 403 {object} httpcontext.Response "权限不足"
// @Router /api/auto_ingest/plan/create_subscribe [post]
func (h *handler) CreateSubscribePlan() httpcontext.HandlerFunc {
	return func(ctx *httpcontext.Context) {
		req := new(createSubscribePlanRequest)
		if err := ctx.ShouldBindJSON(req); err != nil {
			ctx.AbortWithInvalidParams(err)

			return
		}

		// 先检查这个 UpUserId
		if _, err := h.cloudBridgeService.GetSubscribeUserInfo(ctx.GetContext(), req.UpUserId); err != nil {
			ctx.Fail(codeUpUserIdInvalid.WithError(err))

			return
		}

		enable := req.AutoIngestInterval > 0

		addition := &models.AutoIngestPlanSubscribeAddition{
			UpUserId: req.UpUserId,
		}

		// 构造刷新策略，做兜底
		rs := models.RefreshStrategy{
			EnableAutoRefresh: false,
			AutoRefreshDays:   0,
			RefreshInterval:   0,
			EnableDeepRefresh: false,
		}
		if req.RefreshStrategy.EnableAutoRefresh {
			rs.EnableAutoRefresh = true
			// 天数兜底
			days := req.RefreshStrategy.AutoRefreshDays
			if days <= 0 {
				days = 7
			}

			rs.AutoRefreshDays = days

			// 间隔兜底（最小30）
			interval := req.RefreshStrategy.RefreshInterval
			if interval < 30 {
				interval = 30
			}

			rs.RefreshInterval = interval

			rs.EnableDeepRefresh = req.RefreshStrategy.EnableDeepRefresh
		}

		offset := time.Now().Unix()
		if req.OneClickAddHistory {
			offset = 1
		}

		// 获取当前用户ID
		userID := ctx.GetInt64(consts.CtxKeyUserId)

		id, err := h.planService.Create(ctx.GetContext(), &models.AutoIngestPlan{
			Name:               req.Name,
			Enabled:            enable,
			AutoIngestInterval: req.AutoIngestInterval,
			SourceType:         autoingest.SourceTypeSubscribe,
			Offset:             offset,
			ParentPath:         req.ParentPath,
			OnConflict:         autoingest.OnConflict(req.OnConflict),
			AddCount:           0,
			FailedCount:        0,
			Addition:           addition.JSONMap(),
			RefreshStrategy:    rs,
			TokenId:            req.CloudToken,
			UserID:             userID,
		})
		if err != nil {
			ctx.Fail(codeCreatePlanFailed.WithError(err))

			return
		}

		if req.OneClickAddHistory {
			taskReq := &topic.AutoIngestRefreshSubscribeRequest{
				PlanId: id,
			}

			taskBody, _ := json.Marshal(taskReq)

			_ = h.taskEngine.PushMessage(ctx.GetContext(), taskReq.Topic(), taskBody)
		}

		ctx.Success(&createSubscribePlanResponse{ID: id})
	}
}
