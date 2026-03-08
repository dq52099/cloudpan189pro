package scheduler

import (
	"encoding/json"
	"runtime/debug"
	"sync"
	"time"

	"github.com/bytedance/gopkg/util/gopool"
	"github.com/robfig/cron/v3"
	"github.com/xxcheng123/cloudpan189-share/internal/consts"
	"github.com/xxcheng123/cloudpan189-share/internal/framework/context"
	"github.com/xxcheng123/cloudpan189-share/internal/pkgs/taskengine"
	"github.com/xxcheng123/cloudpan189-share/internal/shared"
	"github.com/xxcheng123/cloudpan189-share/internal/types/topic"
	"go.uber.org/zap"
)

type RebuildStrmScheduler struct {
	running    bool
	mu         sync.Mutex
	ctx        context.Context
	cancel     context.CancelFunc
	taskEngine taskengine.TaskEngine
	cron       *cron.Cron
}

func NewRebuildStrmScheduler(taskEngine taskengine.TaskEngine) Scheduler {
	return &RebuildStrmScheduler{
		taskEngine: taskEngine,
		running:    false,
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

	// 检查是否启用自动重建
	if shared.MediaConfig == nil || !shared.MediaConfig.Enable || !shared.MediaConfig.AutoRebuildEnable {
		return true
	}

	// 检查cron表达式
	cronExpr := shared.MediaConfig.AutoRebuildCron
	if cronExpr == "" {
		cronExpr = "0 2 * * *" // 默认每天凌晨2点
	}

	// 解析cron表达式
	schedule, err := cron.ParseStandard(cronExpr)
	if err != nil {
		s.ctx.Error("解析cron表达式失败", zap.String("cron", cronExpr), zap.Error(err))
		return true
	}

	now := time.Now()
	nextRun := schedule.Next(now.Add(-time.Hour)) // 获取上次应该运行的时间
	lastRun := schedule.Next(now.Add(-time.Hour * 24))

	// 检查是否应该执行
	if now.Sub(lastRun) < time.Hour && now.After(nextRun) {
		s.doRebuild()
	}

	return true
}

func (s *RebuildStrmScheduler) doRebuild() {
	logger := s.ctx.Logger

	logger.Info("STRM定时重建任务已下发", zap.String("cron", shared.MediaConfig.AutoRebuildCron))

	taskReq := &topic.MediaRebuildStrmFileRequest{}
	body, _ := json.Marshal(taskReq)

	if err := s.taskEngine.PushMessage(
		s.ctx.
			WithValue(consts.CtxKeyInvokeHandlerName, "STRM定时重建执行器"),
		taskReq.Topic(), body); err != nil {
		logger.Error("下发STRM定时重建任务失败", zap.Error(err))
	}
}
