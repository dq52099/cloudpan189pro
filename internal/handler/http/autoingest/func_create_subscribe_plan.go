package autoingest

import (
	"encoding/json"
	"errors"
	"time"

	"github.com/xxcheng123/cloudpan189-share/internal/consts"
	"github.com/xxcheng123/cloudpan189-share/internal/framework/httpcontext"
	"github.com/xxcheng123/cloudpan189-share/internal/repository/models"
	"github.com/xxcheng123/cloudpan189-share/internal/types/autoingest"
	"go.uber.org/zap"
)

type refreshStrategyRequest struct {
	EnableAutoRefresh bool `json:"enableAutoRefresh" binding:"omitempty" example:"false"`
	AutoRefreshDays   int  `json:"autoRefreshDays" binding:"omitempty,min=1" example:"7"`
	RefreshInterval   int  `json:"refreshInterval" binding:"omitempty,min=30,max=1440" example:"30"` // 单位分钟，最小30，最大1440
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
	// Enable 显式控制是否立即启用；为 nil 时按 AutoIngestInterval 是否 > 0 决定（保留向后兼容）
	Enable *bool `json:"enable"`
}

type createSubscribePlanResponse struct {
	ID            int64  `json:"id" example:"1"`
	HistoryQueued bool   `json:"historyQueued" example:"true"`
	HistoryError  string `json:"historyError,omitempty"`
}

// CreateSubscribePlan 创建订阅型自动挂载计划
// @Summary 创建订阅型自动挂载计划
// @Description 创建订阅计划；enable 字段为空时回退到 AutoIngestInterval > 0 的旧语义，传入 false 则计划创建后保持停用。
// @Tags 自动挂载管理
// @Accept json
// @Produce json
// @Param Authorization header string true "Bearer token"
// @Param request body createSubscribePlanRequest true "订阅计划参数"
// @Success 200 {object} httpcontext.Response{data=createSubscribePlanResponse} "创建成功"
// @Failure 400 {object} httpcontext.Response "参数验证失败，code=99998"
// @Failure 401 {object} httpcontext.Response "未授权访问"
// @Failure 403 {object} httpcontext.Response "权限不足"
// @Failure 404 {object} httpcontext.Response "云盘令牌不存在，code=xxxx"
// @Router /api/auto_ingest/plan/create_subscribe [post]
func (h *handler) CreateSubscribePlan() httpcontext.HandlerFunc {
	return func(ctx *httpcontext.Context) {
		req := new(createSubscribePlanRequest)
		if err := ctx.ShouldBindJSON(req); err != nil {
			ctx.AbortWithInvalidParams(err)

			return
		}

		parentPath, err := normalizeAutoIngestParentPath(req.ParentPath)
		if err != nil {
			ctx.AbortWithInvalidParams(err)

			return
		}

		req.UpUserId = normalizeSubscribeUserID(req.UpUserId)
		if req.UpUserId == "" {
			ctx.AbortWithInvalidParams(errors.New("upUserId 不能为空"))

			return
		}

		if !h.ensureCloudBridgeService(ctx, codeUpUserIdInvalid) {
			return
		}

		// 先检查这个 UpUserId
		if _, err := h.cloudBridgeService.GetSubscribeUserInfo(ctx.GetContext(), req.UpUserId); err != nil {
			ctx.Fail(codeUpUserIdInvalid.WithError(err))

			return
		}

		// 构造启用状态：显式 Enable 优先，否则兼容旧逻辑
		enable := req.AutoIngestInterval > 0
		if req.Enable != nil {
			enable = *req.Enable
		}

		onConflict := autoingest.OnConflict(req.OnConflict)
		if onConflict == "" {
			onConflict = autoingest.OnConflictRename
		}

		if err := h.validateCloudTokenAccess(ctx, req.CloudToken); err != nil {
			failCloudTokenAccessError(ctx, err, codeCreatePlanFailed)

			return
		}

		addition := &models.AutoIngestPlanSubscribeAddition{
			UpUserId: req.UpUserId,
		}

		// 构造刷新策略，统一做兜底；即使 EnableAutoRefresh=false，也保留用户传入的数值
		rs := models.RefreshStrategy{
			EnableAutoRefresh: req.RefreshStrategy.EnableAutoRefresh,
			AutoRefreshDays:   req.RefreshStrategy.AutoRefreshDays,
			RefreshInterval:   req.RefreshStrategy.RefreshInterval,
			EnableDeepRefresh: req.RefreshStrategy.EnableDeepRefresh,
		}

		if rs.EnableAutoRefresh {
			if rs.AutoRefreshDays <= 0 {
				rs.AutoRefreshDays = 7
			}

			if rs.RefreshInterval < 30 {
				rs.RefreshInterval = 30
			}
		}

		offset := time.Now().Unix()
		if req.OneClickAddHistory {
			offset = 1
		} else {
			addition.OffsetResourceID = models.AutoIngestSubscribeOffsetResourceIDMax
		}

		// 获取当前用户ID
		userID := ctx.GetInt64(consts.CtxKeyUserId)
		if !h.ensurePlanService(ctx, codeCreatePlanFailed) {
			return
		}

		id, err := h.planService.Create(ctx.GetContext(), &models.AutoIngestPlan{
			Name:               req.Name,
			Enabled:            enable,
			AutoIngestInterval: req.AutoIngestInterval,
			SourceType:         autoingest.SourceTypeSubscribe,
			Offset:             offset,
			ParentPath:         parentPath,
			OnConflict:         onConflict,
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

		resp := &createSubscribePlanResponse{ID: id}
		if req.OneClickAddHistory {
			taskReq := newAutoIngestRefreshTask(ctx, id, false)

			taskBody, jerr := json.Marshal(taskReq)
			if jerr != nil {
				safeErr := sanitizeAutoIngestHTTPError(jerr)
				ctx.GetContext().Warn("序列化一键入库任务失败", zap.Int64("plan_id", id), zap.String("error", safeErr))
				resp.HistoryError = safeErr
			} else if isNilDependency(h.taskEngine) {
				resp.HistoryError = "任务引擎未初始化"
				ctx.GetContext().Warn("一键入库任务下发失败", zap.Int64("plan_id", id), zap.String("error", resp.HistoryError))
			} else if perr := h.taskEngine.PushMessage(ctx.GetContext(), taskReq.Topic(), taskBody); perr != nil {
				safeErr := sanitizeAutoIngestHTTPError(perr)
				ctx.GetContext().Warn("一键入库任务下发失败", zap.Int64("plan_id", id), zap.String("error", safeErr))
				resp.HistoryError = safeErr
			} else {
				resp.HistoryQueued = true
			}
		}

		ctx.Success(resp)
	}
}
