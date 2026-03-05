package scheduler

import (
	"runtime/debug"
	"sync"
	"time"

	"github.com/bytedance/gopkg/util/gopool"
	"github.com/xxcheng123/cloudpan189-share/internal/framework/context"
	"github.com/xxcheng123/cloudpan189-share/internal/services/subscription"
	"go.uber.org/zap"
)

type SubscriptionScheduler struct {
	running         bool
	mu              sync.Mutex
	ctx             context.Context
	cancel          context.CancelFunc
	subscriptionSvc subscription.Service
	firstRunSkipped bool
	startupDelay    time.Duration
}

func NewSubscriptionScheduler(subscriptionSvc subscription.Service) Scheduler {
	return &SubscriptionScheduler{
		subscriptionSvc: subscriptionSvc,
		running:         false,
		firstRunSkipped: false,
		startupDelay:    10 * time.Minute, // 启动后延迟10分钟再执行
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
		for s.doJob() {
		}

		ctx.Info("订阅定时任务执行器已停止~")
	})

	return nil
}

func (s *SubscriptionScheduler) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.cancel != nil {
		s.cancel()
	}

	s.running = false
}

func (s *SubscriptionScheduler) doJob() bool {
	s.mu.Lock()
	running := s.running
	s.mu.Unlock()

	if !running {
		return false
	}

	defer func() {
		if r := recover(); r != nil {
			s.ctx.Error("订阅定时任务执行器发生异常",
				zap.Any("panic", r),
				zap.String("stack", string(debug.Stack())))
		}
	}()

	// 启动后首次执行，跳过并记录延迟时间
	if !s.firstRunSkipped {
		s.firstRunSkipped = true
		s.ctx.Info("订阅定时任务执行器启动，已跳过首次执行", zap.Duration("delay", s.startupDelay))
		select {
		case <-s.ctx.Done():
			return false
		case <-time.After(s.startupDelay):
			return true
		}
	}

	// 检查是否到了执行时间（每天凌晨2点）
	now := time.Now()
	if now.Hour() == 2 && now.Minute() < 5 {
		s.ctx.Info("执行每日订阅任务...")
		if err := s.subscriptionSvc.RunSubscriptionJob(); err != nil {
			s.ctx.Error("订阅任务执行失败", zap.Error(err))
		}
	}

	// 每小时检查一次
	select {
	case <-s.ctx.Done():
		return false
	case <-time.After(time.Hour):
		return true
	}
}
