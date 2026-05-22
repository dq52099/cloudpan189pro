package autoingest

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/xxcheng123/cloudpan189-share/internal/framework/httpcontext"
	"github.com/xxcheng123/cloudpan189-share/internal/pkgs/utils"
	"github.com/xxcheng123/cloudpan189-share/internal/repository/models"
	"github.com/xxcheng123/cloudpan189-share/internal/types/topic"
	"go.uber.org/zap"
)

type autoIngestRetrySnapshot struct {
	offset      int64
	addCount    int64
	failedCount int64
}

type autoIngestRetryErrorPhase string

const (
	autoIngestRetryPhaseReset   autoIngestRetryErrorPhase = "reset"
	autoIngestRetryPhaseEnqueue autoIngestRetryErrorPhase = "enqueue"
)

type autoIngestRetryError struct {
	phase autoIngestRetryErrorPhase
	err   error
}

func (e *autoIngestRetryError) Error() string {
	return e.err.Error()
}

func (e *autoIngestRetryError) Unwrap() error {
	return e.err
}

func snapshotAutoIngestRetryState(plan *models.AutoIngestPlan) autoIngestRetrySnapshot {
	return autoIngestRetrySnapshot{
		offset:      plan.Offset,
		addCount:    plan.AddCount,
		failedCount: plan.FailedCount,
	}
}

func failAutoIngestRetryError(ctx *httpcontext.Context, err error) {
	var retryErr *autoIngestRetryError
	if errors.As(err, &retryErr) && retryErr.phase == autoIngestRetryPhaseReset {
		ctx.Fail(codePlanUpdateFailed.WithError(err))

		return
	}

	ctx.Fail(codePlanRefreshFailed.WithError(err))
}

func (h *handler) resetAutoIngestRetryState(ctx *httpcontext.Context, planID int64, offset int64, resetCounters bool) error {
	if !resetCounters {
		return h.planService.UpdateOffset(ctx.GetContext(), planID, offset)
	}

	return h.planService.Update(ctx.GetContext(), planID,
		utils.WithField("offset", offset),
		utils.WithField("add_count", int64(0)),
		utils.WithField("failed_count", int64(0)),
	)
}

func (h *handler) restoreAutoIngestRetryState(ctx *httpcontext.Context, planID int64, snapshot autoIngestRetrySnapshot) {
	if err := h.planService.Update(ctx.GetContext(), planID,
		utils.WithField("offset", snapshot.offset),
		utils.WithField("add_count", snapshot.addCount),
		utils.WithField("failed_count", snapshot.failedCount),
	); err != nil {
		ctx.GetContext().Error("恢复自动入库重试状态失败",
			zap.Int64("plan_id", planID),
			zap.Int64("offset", snapshot.offset),
			zap.Int64("add_count", snapshot.addCount),
			zap.Int64("failed_count", snapshot.failedCount),
			zap.Error(err))
	}
}

func (h *handler) enqueueAutoIngestRetryTask(ctx *httpcontext.Context, planID int64, isRetry bool) error {
	taskReq := &topic.AutoIngestRefreshSubscribeRequest{
		PlanId:  planID,
		IsRetry: isRetry,
	}

	body, err := json.Marshal(taskReq)
	if err != nil {
		return fmt.Errorf("序列化自动入库重试任务失败: %w", err)
	}

	if err := h.taskEngine.PushMessage(ctx.GetContext(), taskReq.Topic(), body); err != nil {
		return fmt.Errorf("下发自动入库重试任务失败: %w", err)
	}

	return nil
}

func (h *handler) dispatchRetryWithRollback(ctx *httpcontext.Context, plan *models.AutoIngestPlan, offset int64, isRetry bool, resetCounters bool) error {
	if plan == nil || plan.ID <= 0 {
		return fmt.Errorf("自动挂载计划不存在")
	}

	snapshot := snapshotAutoIngestRetryState(plan)
	if err := h.resetAutoIngestRetryState(ctx, plan.ID, offset, resetCounters); err != nil {
		return &autoIngestRetryError{phase: autoIngestRetryPhaseReset, err: err}
	}

	if err := h.enqueueAutoIngestRetryTask(ctx, plan.ID, isRetry); err != nil {
		h.restoreAutoIngestRetryState(ctx, plan.ID, snapshot)

		return &autoIngestRetryError{phase: autoIngestRetryPhaseEnqueue, err: err}
	}

	return nil
}
