package scheduler

import (
	"runtime/debug"
	"sync"
	"time"

	"github.com/bytedance/gopkg/util/gopool"
	"github.com/robfig/cron/v3"
	"github.com/xxcheng123/cloudpan189-share/internal/framework/context"
	"github.com/xxcheng123/cloudpan189-share/internal/services/subscription"
	"go.uber.org/zap"
)

// defaultSubscriptionCron 默认每天凌晨 2 点执行订阅任务。
const defaultSubscriptionCron = "0 2 * * *"

type SubscriptionScheduler struct {
	running         bool
	mu              sync.Mutex
	ctx             context.Context
	cancel          context.CancelFunc
	subscriptionSvc subscription.Service
	startupDelay    time.Duration
	cronExpr        string
	nextRunAt       time.Time
}

func NewSubscriptionScheduler(subscriptionSvc subscription.Service) Scheduler {
	return &SubscriptionScheduler{
		subscriptionSvc: subscriptionSvc,
		running:         false,
		startupDelay:    10 * time.Minute, // 启动后延迟10分钟再执行
		cronExpr:        defaultSubscriptionCron,
	}
}

func (s *SubscriptionScheduler) Start(ctx context.Context) error {
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
		s.loop()

		ctx.Info("订阅定时任务执行器已停止~")
	})

	return nil
}

func (s *SubscriptionScheduler) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		return
	}

	if s.cancel != nil {
		s.cancel()
	}

	s.running = false
}

// loop 用 ticker 驱动判断是否到达下次执行时间。
func (s *SubscriptionScheduler) loop() {
	// 启动延迟，避免服务刚起来就立即触发任务
	select {
	case <-s.ctx.Done():
		return
	case <-time.After(s.startupDelay):
	}

	// 计算首次 nextRunAt
	s.nextRunAt = s.computeNextRun(time.Now())

	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	for {
		s.tick()

		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (s *SubscriptionScheduler) tick() {
	defer func() {
		if r := recover(); r != nil {
			s.ctx.Error("订阅定时任务执行器发生异常",
				zap.Any("panic", r),
				zap.String("stack", string(debug.Stack())))
		}
	}()

	now := time.Now()
	if now.Before(s.nextRunAt) {
		return
	}

	s.ctx.Info("执行每日订阅任务...", zap.Time("scheduled_at", s.nextRunAt))
	if err := s.subscriptionSvc.RunSubscriptionJob(); err != nil {
		s.ctx.Error("订阅任务执行失败", zap.Error(err))
	}

	s.nextRunAt = s.computeNextRun(now)
}

// computeNextRun 计算下一次调度时间。当前 cron 无法解析时退回默认 24h 间隔。
func (s *SubscriptionScheduler) computeNextRun(after time.Time) time.Time {
	schedule, err := cron.ParseStandard(s.cronExpr)
	if err != nil {
		s.ctx.Warn("解析订阅 cron 失败，使用默认 24h 间隔", zap.String("cron", s.cronExpr), zap.Error(err))
		return after.Add(24 * time.Hour)
	}
	return schedule.Next(after)
}
