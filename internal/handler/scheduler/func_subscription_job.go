package scheduler

import (
	"runtime/debug"
	"strings"
	"sync"
	"time"

	"github.com/bytedance/gopkg/util/gopool"
	"github.com/robfig/cron/v3"
	"github.com/xxcheng123/cloudpan189-share/internal/consts"
	"github.com/xxcheng123/cloudpan189-share/internal/framework/context"
	"github.com/xxcheng123/cloudpan189-share/internal/services/subscription"
	"go.uber.org/zap"
)

// defaultSubscriptionCron 默认每天凌晨 2 点执行订阅任务。
const defaultSubscriptionCron = consts.DefaultSubscriptionCronExpression

type SubscriptionScheduler struct {
	running         bool
	stopping        bool
	mu              sync.Mutex
	ctx             context.Context
	cancel          context.CancelFunc
	done            chan struct{}
	subscriptionSvc subscription.Service
	startupDelay    time.Duration
	enabled         bool
	cronExpr        string
	nextRunAt       time.Time
}

func NewSubscriptionScheduler(subscriptionSvc subscription.Service) *SubscriptionScheduler {
	return &SubscriptionScheduler{
		subscriptionSvc: subscriptionSvc,
		running:         false,
		startupDelay:    10 * time.Minute, // 启动后延迟10分钟再执行
		enabled:         false,
		cronExpr:        defaultSubscriptionCron,
	}
}

func (s *SubscriptionScheduler) UpdateConfig(config subscription.SubscriptionConfig) {
	cronExpr := strings.TrimSpace(config.CronExpression)
	if cronExpr == "" {
		cronExpr = defaultSubscriptionCron
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.enabled = config.Enabled
	s.cronExpr = cronExpr

	if s.running {
		s.nextRunAt = s.computeNextRunLocked(time.Now())
	}
}

func (s *SubscriptionScheduler) Start(ctx context.Context) error {
	if !s.mu.TryLock() {
		return ErrSchedulerRunning
	}
	defer s.mu.Unlock()

	shouldStart, err := shouldStartScheduler(s.running, s.stopping)
	if err != nil {
		return err
	}

	if !shouldStart {
		return nil
	}

	if isNilDependency(s.subscriptionSvc) {
		return ErrSchedulerSubscriptionServiceMissing
	}

	s.ctx, s.cancel = context.WithCancel(ctx)
	done := markSchedulerRunStarted(&s.running, &s.stopping, &s.done)

	gopool.Go(func() {
		defer finishSchedulerRun(&s.mu, &s.running, &s.stopping, &s.cancel, &s.done, done)

		s.loop()

		ctx.Info("订阅定时任务执行器已停止~")
	})

	return nil
}

func (s *SubscriptionScheduler) Stop() {
	cancel, done, ok := beginSchedulerStop(&s.mu, &s.running, &s.stopping, &s.cancel, &s.done)
	if !ok {
		return
	}

	waitSchedulerStop(cancel, done)
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
	s.scheduleNextRun(time.Now())

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
				zap.String("panic", sanitizeSchedulerPanicValue(r)),
				zap.String("stack", string(debug.Stack())))
		}
	}()

	now := time.Now()

	scheduledAt, shouldRun := s.shouldRun(now)
	if !shouldRun {
		return
	}

	s.ctx.Info("执行每日订阅任务...", zap.Time("scheduled_at", scheduledAt))

	if err := s.subscriptionSvc.RunSubscriptionJob(); err != nil {
		s.ctx.Error("订阅任务执行失败", zap.Error(err))
	}

	s.scheduleNextRun(now)
}

// computeNextRun 计算下一次调度时间。当前 cron 无法解析时退回默认 24h 间隔。
func (s *SubscriptionScheduler) computeNextRun(after time.Time) time.Time {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.computeNextRunLocked(after)
}

func (s *SubscriptionScheduler) shouldRun(now time.Time) (time.Time, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.enabled {
		return time.Time{}, false
	}

	if now.Before(s.nextRunAt) {
		return time.Time{}, false
	}

	return s.nextRunAt, true
}

func (s *SubscriptionScheduler) scheduleNextRun(after time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.nextRunAt = s.computeNextRunLocked(after)
}

func (s *SubscriptionScheduler) computeNextRunLocked(after time.Time) time.Time {
	schedule, err := cron.ParseStandard(s.cronExpr)
	if err != nil {
		if s.ctx.Logger != nil {
			s.ctx.Warn("解析订阅 cron 失败，使用默认 24h 间隔", zap.String("cron", s.cronExpr), zap.Error(err))
		}

		return after.Add(24 * time.Hour)
	}

	return schedule.Next(after)
}
