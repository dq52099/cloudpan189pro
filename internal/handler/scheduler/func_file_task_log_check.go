package scheduler

import (
	"errors"
	"fmt"
	"runtime/debug"
	"sync"
	"time"

	"github.com/bytedance/gopkg/util/gopool"
	"github.com/xxcheng123/cloudpan189-share/internal/framework/context"
	"github.com/xxcheng123/cloudpan189-share/internal/pkgs/utils"
	"github.com/xxcheng123/cloudpan189-share/internal/repository/models"

	filetasklogSvi "github.com/xxcheng123/cloudpan189-share/internal/services/filetasklog"

	"go.uber.org/zap"
)

type FileTaskLogCheckScheduler struct {
	running            bool
	stopping           bool
	mu                 sync.Mutex
	ctx                context.Context
	cancel             context.CancelFunc
	done               chan struct{}
	fileTaskLogService filetasklogSvi.Service
}

func NewFileTaskLogCheckScheduler(fileTaskLogService filetasklogSvi.Service) Scheduler {
	return &FileTaskLogCheckScheduler{
		fileTaskLogService: fileTaskLogService,
	}
}

func (s *FileTaskLogCheckScheduler) Start(ctx context.Context) error {
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

	if isNilDependency(s.fileTaskLogService) {
		return ErrSchedulerFileTaskLogServiceMissing
	}

	s.ctx, s.cancel = context.WithCancel(ctx)
	done := markSchedulerRunStarted(&s.running, &s.stopping, &s.done)

	gopool.Go(func() {
		defer finishSchedulerRun(&s.mu, &s.running, &s.stopping, &s.cancel, &s.done, done)

		for s.doJob() {
		}

		ctx.Info("日志检查器已停止~")
	})

	return nil
}

func (s *FileTaskLogCheckScheduler) doJob() (keepRunning bool) {
	ctx := s.ctx
	keepRunning = true

	defer func() {
		if r := recover(); r != nil {
			ctx.Error("日志检查器发生异常",
				zap.String("panic", sanitizeSchedulerPanicValue(r)),
				zap.String("stack", string(debug.Stack())))

			keepRunning = ctx.Err() == nil
		}
	}()

	select {
	case <-s.ctx.Done():
		ctx.Info("日志检查器停止")

		return false
	case <-time.After(time.Minute):
		// 检查有没有超时的任务
		tasks, err := s.fileTaskLogService.FindStaleTasksByDuration(ctx, time.Minute*10)
		if err != nil {
			ctx.Error("查询超时任务失败", zap.Error(err))

			return true
		}

		ctx.Debug("文件刷新执行器查询到超时任务数量", zap.Int("count", len(tasks)))

		s.reclaimStaleTasks(ctx, tasks)
	}

	return true
}

func (s *FileTaskLogCheckScheduler) reclaimStaleTasks(ctx context.Context, tasks []*models.FileTaskLog) {
	for _, task := range tasks {
		if task == nil {
			ctx.Warn("超时任务列表包含空记录，跳过")

			continue
		}

		if err := s.fileTaskLogService.Failed(ctx, filetasklogSvi.NewLogID(task.ID), utils.WithField("result", fmt.Sprintf("任务执行超时, 系统强制回收任务, 回收前状态: %s", task.Status))); err != nil {
			if errors.Is(err, filetasklogSvi.ErrFileTaskLogTerminalState) {
				ctx.Debug("超时任务已处于终态，跳过回收", zap.Int64("task_id", task.ID), zap.Error(err))

				continue
			}

			ctx.Error("回收超时任务失败", zap.Int64("task_id", task.ID), zap.Error(err))

			continue
		}

		ctx.Info("发现超时任务", zap.Int64("task_id", task.ID))
	}
}

func (s *FileTaskLogCheckScheduler) Stop() {
	cancel, done, ok := beginSchedulerStop(&s.mu, &s.running, &s.stopping, &s.cancel, &s.done)
	if !ok {
		return
	}

	waitSchedulerStop(cancel, done)
}
