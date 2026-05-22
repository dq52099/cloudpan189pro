package scheduler

import (
	"encoding/json"
	"runtime/debug"
	"sync"
	"time"

	"github.com/bytedance/gopkg/util/gopool"
	"github.com/xxcheng123/cloudpan189-share/internal/framework/context"
	"github.com/xxcheng123/cloudpan189-share/internal/pkgs/taskengine"
	"github.com/xxcheng123/cloudpan189-share/internal/types/autoingest"
	"github.com/xxcheng123/cloudpan189-share/internal/types/topic"
	"go.uber.org/zap"

	autoingestlogSvi "github.com/xxcheng123/cloudpan189-share/internal/services/autoingestlog"
	autoingestplanSvi "github.com/xxcheng123/cloudpan189-share/internal/services/autoingestplan"
)

type AutoIngestRefreshScheduler struct {
	running               bool
	mu                    sync.Mutex
	ctx                   context.Context
	cancel                context.CancelFunc
	autoIngestPlanService autoingestplanSvi.Service
	autoIngestLogService  autoingestlogSvi.Service
	taskEngine            taskengine.TaskEngine
	firstRunSkipped       bool
	startupDelay          time.Duration
}

func NewAutoIngestRefreshScheduler(
	taskEngine taskengine.TaskEngine,
	autoIngestPlanService autoingestplanSvi.Service,
	autoIngestLogService autoingestlogSvi.Service,
) Scheduler {
	return &AutoIngestRefreshScheduler{
		running:               false,
		taskEngine:            taskEngine,
		autoIngestPlanService: autoIngestPlanService,
		autoIngestLogService:  autoIngestLogService,
		firstRunSkipped:       false,
		startupDelay:          3 * time.Minute, // 启动后延迟3分钟再执行
	}
}

func (s *AutoIngestRefreshScheduler) Start(ctx context.Context) error {
	if !s.mu.TryLock() {
		return ErrSchedulerRunning
	}
	defer s.mu.Unlock()

	if s.running {
		return nil
	}

	s.ctx, s.cancel = context.WithCancel(ctx)
	s.running = true

	gopool.Go(func() {
		for s.doJob() {
		}

		ctx.Info("自动入库执行器已停止~")
	})

	return nil
}

func (s *AutoIngestRefreshScheduler) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		return
	}

	s.cancel()
	s.running = false
}

func (s *AutoIngestRefreshScheduler) doJob() bool {
	ctx := s.ctx

	defer func() {
		if r := recover(); r != nil {
			ctx.Error("自动入库执行器发生异常",
				zap.Any("panic", r),
				zap.String("stack", string(debug.Stack())))
		}
	}()

	// 启动后首次执行，跳过并记录延迟时间
	if !s.firstRunSkipped {
		s.firstRunSkipped = true
		ctx.Info("自动入库执行器启动，已跳过首次执行", zap.Duration("delay", s.startupDelay))

		select {
		case <-ctx.Done():
			return false
		case <-time.After(s.startupDelay):
			return true
		}
	}

	select {
	case <-ctx.Done():
		ctx.Info("自动入库执行器停止")

		return false
	case <-time.After(time.Minute):
		list, err := s.autoIngestPlanService.FindDue(ctx, time.Now())
		if err != nil {
			ctx.Error("查询自动入库计划失败", zap.Error(err))

			if _, logErr := s.autoIngestLogService.Create(ctx, 0, autoingest.LogLevelError, "查询自动入库计划失败"); logErr != nil {
				ctx.Error("写入自动入库失败日志失败", zap.Error(logErr))
			}

			return true
		}

		for _, plan := range list {
			if plan.SourceType != autoingest.SourceTypeSubscribe {
				ctx.Error("不支持的自动入库源类型", zap.String("name", plan.Name), zap.String("type", plan.SourceType.String()), zap.Int64("id", plan.ID))

				continue
			}

			taskReq := &topic.AutoIngestRefreshSubscribeRequest{
				PlanId: plan.ID,
			}

			msgBody, _ := json.Marshal(taskReq)

			if err = s.taskEngine.PushMessage(ctx, taskReq.Topic(), msgBody); err != nil {
				ctx.Error("推送自动入库任务失败", zap.Error(err))
			}

			ctx.Info("查询到需要自动入库的订阅计划", zap.Int64("id", plan.ID), zap.String("name", plan.Name))
		}
	}

	return true
}
