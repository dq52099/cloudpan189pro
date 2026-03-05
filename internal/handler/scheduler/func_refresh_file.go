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
	"github.com/xxcheng123/cloudpan189-share/internal/services/mountpoint"
	"github.com/xxcheng123/cloudpan189-share/internal/types/topic"
	"go.uber.org/zap"
)

type RefreshFileScheduler struct {
	running           bool
	mu                sync.Mutex
	ctx               context.Context
	cancel            context.CancelFunc
	mountPointService mountpoint.Service
	taskEngine        taskengine.TaskEngine
	firstRunSkipped   bool
	startupDelay      time.Duration
}

func NewRefreshFileScheduler(mountPointService mountpoint.Service, taskEngine taskengine.TaskEngine) Scheduler {
	return &RefreshFileScheduler{
		mountPointService: mountPointService,
		taskEngine:        taskEngine,
		running:           false,
		firstRunSkipped:   false,
		startupDelay:      5 * time.Minute, // 启动后延迟5分钟再执行
	}
}

func (s *RefreshFileScheduler) Start(ctx context.Context) error {
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

		ctx.Info("文件刷新执行器已停止~")
	})

	return nil
}

func (s *RefreshFileScheduler) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		return
	}

	s.cancel()
	s.running = false
}

func (s *RefreshFileScheduler) doJob() bool {
	ctx := s.ctx

	defer func() {
		if r := recover(); r != nil {
			ctx.Error("文件刷新执行器发生异常",
				zap.Any("panic", r),
				zap.String("stack", string(debug.Stack())))
		}
	}()

	// 启动后首次执行，跳过并记录延迟时间
	if !s.firstRunSkipped {
		s.firstRunSkipped = true
		ctx.Info("文件刷新执行器启动，已跳过首次执行", zap.Duration("delay", s.startupDelay))
		select {
		case <-ctx.Done():
			return false
		case <-time.After(s.startupDelay):
			return true
		}
	}

	select {
	case <-ctx.Done():
		ctx.Info("文件刷新执行器停止")

		return false
	case <-time.After(time.Minute):
		mountPoints, err := s.mountPointService.GetAutoRefreshList(ctx, &mountpoint.GetAutoRefreshListRequest{})
		if err != nil {
			ctx.Error("查询挂载点失败", zap.Error(err))

			return true
		}

		ctx.Debug("文件刷新执行器查询到挂载点数量", zap.Int("count", len(mountPoints)))

		now := time.Now()

		// 获取今天零点
		midnight := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

		// 计算从零点到现在的分钟数
		minutesSinceMidnight := int(now.Sub(midnight).Minutes())

		for _, mp := range mountPoints {
			// 只处理启用自动刷新的挂载点
			if !mp.EnableAutoRefresh {
				continue
			}

			if minutesSinceMidnight%mp.RefreshInterval != 0 {
				continue
			}

			ctx.Info("文件扫描执行器触发",
				zap.Int64("mount_point_id", mp.ID),
				zap.Int64("file_id", mp.FileId),
				zap.String("full_path", mp.FullPath),
				zap.Int("refresh_interval", mp.RefreshInterval))

			// 创建文件扫描任务
			taskReq := &topic.FileScanFileRequest{
				FileId: mp.FileId,
				Deep:   mp.EnableDeepRefresh,
			}

			body, _ := json.Marshal(taskReq)
			taskCtx := ctx.
				WithValue(consts.CtxKeyFullPath, mp.FullPath).
				WithValue(consts.CtxKeyInvokeHandlerName, "定时任务")

			if err = s.taskEngine.PushMessage(taskCtx, taskReq.Topic(), body); err != nil {
				ctx.Error("推送文件扫描任务失败",
					zap.Int64("mount_point_id", mp.ID),
					zap.Int64("file_id", mp.FileId),
					zap.String("full_path", mp.FullPath),
					zap.Error(err))
			} else {
				ctx.Info("下发文件扫描任务成功",
					zap.Int64("mount_point_id", mp.ID),
					zap.Int64("file_id", mp.FileId),
					zap.String("full_path", mp.FullPath))
			}
		}
	}

	return true
}
