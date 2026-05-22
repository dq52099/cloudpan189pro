package scheduler

import (
	"fmt"
	"runtime/debug"
	"sync"
	"time"

	"github.com/bytedance/gopkg/util/gopool"
	"github.com/xxcheng123/cloudpan189-share/internal/framework/context"
	"github.com/xxcheng123/cloudpan189-share/internal/pkgs/utils"

	filetasklogSvi "github.com/xxcheng123/cloudpan189-share/internal/services/filetasklog"

	"go.uber.org/zap"
)

type FileTaskLogCheckScheduler struct {
	running            bool
	mu                 sync.Mutex
	ctx                context.Context
	cancel             context.CancelFunc
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

	if s.running {
		return nil
	}

	s.ctx, s.cancel = context.WithCancel(ctx)

	s.running = true

	gopool.Go(func() {
		for s.doJob() {
		}

		ctx.Info("日志检查器已停止~")
	})

	return nil
}

func (s *FileTaskLogCheckScheduler) doJob() bool {
	ctx := s.ctx

	defer func() {
		if r := recover(); r != nil {
			ctx.Error("日志检查器发生异常",
				zap.Any("panic", r),
				zap.String("stack", string(debug.Stack())))
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

		for _, task := range tasks {
			if err := s.fileTaskLogService.Failed(ctx, filetasklogSvi.NewLogID(task.ID), utils.WithField("result", fmt.Sprintf("任务执行超时, 系统强制回收任务, 回收前状态: %s", task.Status))); err != nil {
				ctx.Error("回收超时任务失败", zap.Int64("task_id", task.ID), zap.Error(err))

				continue
			}

			ctx.Info("发现超时任务", zap.Int64("task_id", task.ID))
		}
	}

	return true
}

func (s *FileTaskLogCheckScheduler) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		return
	}

	s.cancel()
	s.running = false
}
