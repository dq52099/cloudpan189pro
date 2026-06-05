package autoingest

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/xxcheng123/cloudpan189-share/internal/consts"
	"github.com/xxcheng123/cloudpan189-share/internal/framework/httpcontext"
	"github.com/xxcheng123/cloudpan189-share/internal/repository/models"
	"github.com/xxcheng123/cloudpan189-share/internal/types/topic"
)

type autoIngestRetryErrorPhase string

const (
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

func failAutoIngestRetryError(ctx *httpcontext.Context, err error) {
	var retryErr *autoIngestRetryError
	if errors.As(err, &retryErr) && retryErr.phase == autoIngestRetryPhaseEnqueue {
		ctx.Fail(codePlanRefreshFailed.WithError(err))

		return
	}

	ctx.Fail(codePlanUpdateFailed.WithError(err))
}

func (h *handler) enqueueAutoIngestRetryTask(ctx *httpcontext.Context, planID int64, isRetry bool, retryReset *topic.AutoIngestRetryStateReset) error {
	taskReq := newAutoIngestRefreshTask(ctx, planID, isRetry)
	taskReq.RetryReset = retryReset

	body, err := json.Marshal(taskReq)
	if err != nil {
		return fmt.Errorf("序列化自动入库重试任务失败: %w", err)
	}

	if isNilDependency(h.taskEngine) {
		return errors.New("任务引擎未初始化")
	}

	if err := h.taskEngine.PushMessage(ctx.GetContext(), taskReq.Topic(), body); err != nil {
		return fmt.Errorf("下发自动入库重试任务失败: %w", err)
	}

	return nil
}

func newAutoIngestRefreshTask(ctx *httpcontext.Context, planID int64, isRetry bool) *topic.AutoIngestRefreshSubscribeRequest {
	return &topic.AutoIngestRefreshSubscribeRequest{
		PlanId:           planID,
		IsRetry:          isRetry,
		ExpectedUserID:   ctx.GetInt64(consts.CtxKeyUserId),
		TriggeredByAdmin: ctx.GetBool(consts.CtxKeyIsAdmin),
	}
}

func (h *handler) dispatchRetryWithRollback(ctx *httpcontext.Context, plan *models.AutoIngestPlan, offset int64, isRetry bool, resetCounters bool) error {
	if plan == nil || plan.ID <= 0 {
		return fmt.Errorf("自动挂载计划不存在")
	}

	if err := h.enqueueAutoIngestRetryTask(ctx, plan.ID, isRetry, &topic.AutoIngestRetryStateReset{
		Offset:        offset,
		ResetCounters: resetCounters,
	}); err != nil {
		return &autoIngestRetryError{phase: autoIngestRetryPhaseEnqueue, err: err}
	}

	return nil
}
