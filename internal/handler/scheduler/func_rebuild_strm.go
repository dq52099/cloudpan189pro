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
	"github.com/xxcheng123/cloudpan189-share/internal/pkgs/utils"
	mediaconfigSvi "github.com/xxcheng123/cloudpan189-share/internal/services/mediaconfig"
	"github.com/xxcheng123/cloudpan189-share/internal/shared"
	"github.com/xxcheng123/cloudpan189-share/internal/types/topic"
	"go.uber.org/zap"
)

// defaultRebuildStrmCron 默认每天凌晨 2 点执行一次
const defaultRebuildStrmCron = "0 2 * * *"

type RebuildStrmScheduler struct {
	running     bool
	stopping    bool
	mu          sync.Mutex
	ctx         context.Context
	cancel      context.CancelFunc
	done        chan struct{}
	taskEngine  taskengine.TaskEngine
	mediaConfig mediaconfigSvi.Service
	lastRun     time.Time
	currentCron string
	nextRunAt   time.Time
}

func NewRebuildStrmScheduler(taskEngine taskengine.TaskEngine, mediaConfig mediaconfigSvi.Service) Scheduler {
	return &RebuildStrmScheduler{
		taskEngine:  taskEngine,
		mediaConfig: mediaConfig,
		running:     false,
	}
}

func (s *RebuildStrmScheduler) Start(ctx context.Context) error {
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

	if isNilDependency(s.taskEngine) {
		return ErrSchedulerTaskEngineMissing
	}

	s.ctx, s.cancel = context.WithCancel(ctx)
	done := markSchedulerRunStarted(&s.running, &s.stopping, &s.done)

	gopool.Go(func() {
		defer finishSchedulerRun(&s.mu, &s.running, &s.stopping, &s.cancel, &s.done, done)

		s.loop()

		ctx.Info("STRM定时重建执行器已停止~")
	})

	return nil
}

func (s *RebuildStrmScheduler) Stop() {
	cancel, done, ok := beginSchedulerStop(&s.mu, &s.running, &s.stopping, &s.cancel, &s.done)
	if !ok {
		return
	}

	waitSchedulerStop(cancel, done)
}

// loop 每分钟检查一次是否到达下一次执行时间。
// 相较原实现，改为真正的"调度表驱动"：计算 nextRunAt，到时触发一次后再计算下一次。
func (s *RebuildStrmScheduler) loop() {
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

func (s *RebuildStrmScheduler) tick() {
	defer func() {
		if r := recover(); r != nil {
			s.ctx.Error("STRM定时重建执行器发生异常",
				zap.String("panic", sanitizeSchedulerPanicValue(r)),
				zap.String("stack", string(debug.Stack())))
		}
	}()

	// 检查是否启用自动重建
	mediaConfig := shared.GetMediaConfig()
	if mediaConfig == nil || !mediaConfig.Enable || !mediaConfig.AutoRebuildEnable {
		return
	}

	cronExpr := mediaConfig.AutoRebuildCron
	if cronExpr == "" {
		cronExpr = defaultRebuildStrmCron
	}

	// 若 cron 表达式变化则重新计算下一次执行时间
	if cronExpr != s.currentCron {
		schedule, err := cron.ParseStandard(cronExpr)
		if err != nil {
			s.ctx.Error("解析cron表达式失败", zap.String("cron", cronExpr), zap.Error(err))

			return
		}

		s.currentCron = cronExpr
		s.nextRunAt = schedule.Next(time.Now())
	}

	now := time.Now()
	if now.Before(s.nextRunAt) {
		return
	}

	if err := s.doRebuild(); err != nil {
		s.ctx.Error("STRM定时重建任务下发失败，保留下次重试时间", zap.Error(err))

		return
	}

	// 计算下一次执行时间
	schedule, err := cron.ParseStandard(s.currentCron)
	if err != nil {
		s.ctx.Error("解析cron表达式失败", zap.String("cron", s.currentCron), zap.Error(err))

		return
	}

	s.nextRunAt = schedule.Next(now)
	s.lastRun = now

	s.syncLastRebuildTime(now)
}

func (s *RebuildStrmScheduler) syncLastRebuildTime(now time.Time) {
	if s.mediaConfig == nil {
		shared.SetMediaConfigLastRebuildTime(now)

		return
	}

	if err := s.mediaConfig.Update(s.ctx, utils.WithField("last_rebuild_time", now)); err != nil {
		s.ctx.Error("更新 STRM 上次重建时间失败", zap.Error(err), zap.Time("last_rebuild_time", now))
		shared.SetMediaConfigLastRebuildTime(now)

		return
	}

	shared.SetMediaConfigLastRebuildTime(now)
}

func (s *RebuildStrmScheduler) doRebuild() error {
	logger := s.ctx.Logger

	logger.Info("准备下发STRM定时重建任务",
		zap.String("cron", s.currentCron),
		zap.Time("next_run_at", s.nextRunAt),
	)

	// 统一走全量重建任务，消费侧自带并发。
	// 曾经按挂载点派发会把 STRM 全量重建拆成 N 条，
	// 对 task_logs 产生大量噪声且难以聚合成单次运行摘要。
	taskReq := &topic.MediaRebuildStrmFileRequest{}

	body, err := json.Marshal(taskReq)
	if err != nil {
		logger.Error("序列化STRM定时重建任务失败", zap.Error(err))

		return err
	}

	if err := s.taskEngine.PushMessage(
		s.ctx.
			WithValue(consts.CtxKeyInvokeHandlerName, "STRM定时重建执行器"),
		taskReq.Topic(), body); err != nil {
		logger.Error("下发STRM定时重建任务失败", zap.Error(err))

		return err
	}

	logger.Info("STRM定时重建任务已下发",
		zap.String("cron", s.currentCron),
		zap.Time("next_run_at", s.nextRunAt),
	)

	return nil
}
