package scheduler

import (
	"encoding/json"
	"runtime/debug"
	"sync"
	"time"

	"github.com/bytedance/gopkg/util/gopool"
	"github.com/xxcheng123/cloudpan189-share/internal/consts"
	"github.com/xxcheng123/cloudpan189-share/internal/framework/context"
	"github.com/xxcheng123/cloudpan189-share/internal/pkgs/taskengine"
	"github.com/xxcheng123/cloudpan189-share/internal/shared"
	"github.com/xxcheng123/cloudpan189-share/internal/types/topic"
	"go.uber.org/zap"
)

type RebuildStrmScheduler struct {
	running         bool
	mu              sync.Mutex
	ctx             context.Context
	cancel          context.CancelFunc
	taskEngine      taskengine.TaskEngine
	firstRunSkipped bool
	startupDelay    time.Duration
}

func NewRebuildStrmScheduler(taskEngine taskengine.TaskEngine) Scheduler {
	return &RebuildStrmScheduler{
		taskEngine:      taskEngine,
		running:         false,
		firstRunSkipped: false,
		startupDelay:    10 * time.Minute, // 启动后延迟10分钟再执行
	}
}

func (s *RebuildStrmScheduler) Start(ctx context.Context) error {
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

		ctx.Info("STRM定时重建执行器已停止~")
	})

	return nil
}

func (s *RebuildStrmScheduler) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.cancel != nil {
		s.cancel()
	}

	s.running = false
}

func (s *RebuildStrmScheduler) doJob() bool {
	interval := 60 // 默认每60秒检查一次配置

	s.mu.Lock()
	running := s.running
	s.mu.Unlock()

	if !running {
		return false
	}

	defer func() {
		if r := recover(); r != nil {
			s.ctx.Error("STRM定时重建执行器发生异常",
				zap.Any("panic", r),
				zap.String("stack", string(debug.Stack())))
		}
	}()

	// 启动后首次执行，跳过并记录延迟时间
	if !s.firstRunSkipped {
		s.firstRunSkipped = true
		s.ctx.Info("STRM定时重建执行器启动，已跳过首次执行", zap.Duration("delay", s.startupDelay))
		select {
		case <-s.ctx.Done():
			return false
		case <-time.After(s.startupDelay):
			return true
		}
	}

	if shared.MediaConfig != nil && shared.MediaConfig.Enable && shared.MediaConfig.AutoRebuildEnable {
		s.doRebuild()
	}

	select {
	case <-s.ctx.Done():
		return false
	case <-time.After(time.Duration(interval) * time.Second):
		return true
	}
}

func (s *RebuildStrmScheduler) doRebuild() {
	logger := s.ctx.Logger

	if shared.MediaConfig == nil || !shared.MediaConfig.Enable || !shared.MediaConfig.AutoRebuildEnable {
		return
	}

	interval := shared.MediaConfig.AutoRebuildInterval
	if interval <= 0 {
		interval = 24 // 默认24小时
	}

	logger.Debug("检查是否需要定时重建strm", zap.Int("interval_hours", interval))

	taskReq := &topic.MediaRebuildStrmFileRequest{}
	body, _ := json.Marshal(taskReq)

	if err := s.taskEngine.PushMessage(
		s.ctx.
			WithValue(consts.CtxKeyInvokeHandlerName, "STRM定时重建执行器"),
		taskReq.Topic(), body); err != nil {
		logger.Error("下发STRM定时重建任务失败", zap.Error(err))
	} else {
		logger.Info("STRM定时重建任务已下发", zap.Int("interval_hours", interval))
	}
}
