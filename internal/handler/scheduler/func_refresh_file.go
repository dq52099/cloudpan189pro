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
	"github.com/xxcheng123/cloudpan189-share/internal/repository/models"
	"github.com/xxcheng123/cloudpan189-share/internal/services/mountpoint"
	"github.com/xxcheng123/cloudpan189-share/internal/shared"
	"github.com/xxcheng123/cloudpan189-share/internal/types/topic"
	"go.uber.org/zap"
)

// RefreshFileScheduler 驱动挂载点的自动刷新（周期 = RefreshInterval 分钟）。
//
// 实现细节：
//   - 启动后延迟 5 分钟执行首轮扫描，避免进程启动瞬间的资源竞争；
//   - 每分钟 tick 一次；每个挂载点使用内存中的 lastDispatchedAt 来判断是否已过 interval，
//     不再使用 `minutesSinceMidnight % RefreshInterval == 0`，避免 47min 等非 1440 因子长期不命中；
//   - 每个挂载点同一时刻只允许一个扫描任务处于派发窗口内。
type RefreshFileScheduler struct {
	running           bool
	mu                sync.Mutex
	ctx               context.Context
	cancel            context.CancelFunc
	mountPointService mountpoint.Service
	taskEngine        taskengine.TaskEngine
	firstRunSkipped   bool
	startupDelay      time.Duration

	// lastDispatchedAt 记录每个挂载点最近一次派发扫描任务的时间
	// 由同一 goroutine 写入，无需加锁
	lastDispatchedAt map[int64]time.Time
}

func NewRefreshFileScheduler(mountPointService mountpoint.Service, taskEngine taskengine.TaskEngine) Scheduler {
	return &RefreshFileScheduler{
		mountPointService: mountPointService,
		taskEngine:        taskEngine,
		running:           false,
		firstRunSkipped:   false,
		startupDelay:      5 * time.Minute,
		lastDispatchedAt:  make(map[int64]time.Time),
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
		s.loop()

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

// loop 主循环。启动延迟后按 ticker 周期触发 tick。
func (s *RefreshFileScheduler) loop() {
	// 启动延迟
	select {
	case <-s.ctx.Done():
		return
	case <-time.After(s.startupDelay):
	}

	s.firstRunSkipped = true
	s.ctx.Info("文件刷新执行器启动完成", zap.Duration("startup_delay", s.startupDelay))

	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	for {
		s.tick()

		select {
		case <-s.ctx.Done():
			s.ctx.Info("文件刷新执行器停止")

			return
		case <-ticker.C:
		}
	}
}

func (s *RefreshFileScheduler) tick() {
	defer func() {
		if r := recover(); r != nil {
			s.ctx.Error("文件刷新执行器发生异常",
				zap.Any("panic", r),
				zap.String("stack", string(debug.Stack())))
		}
	}()

	if !shared.SettingAddition.EnableStorageAutoRefresh {
		return
	}

	mountPoints, err := s.mountPointService.GetAutoRefreshList(s.ctx, &mountpoint.GetAutoRefreshListRequest{})
	if err != nil {
		s.ctx.Error("查询挂载点失败", zap.Error(err))

		return
	}

	s.ctx.Debug("文件刷新执行器查询到挂载点数量", zap.Int("count", len(mountPoints)))

	now := time.Now()
	activeIds := make(map[int64]struct{}, len(mountPoints))

	for _, mp := range mountPoints {
		activeIds[mp.FileId] = struct{}{}
		s.dispatchIfDue(mp, now)
	}

	// 清理不再需要自动刷新的挂载点的 lastDispatchedAt，避免 map 无限增长
	for id := range s.lastDispatchedAt {
		if _, ok := activeIds[id]; !ok {
			delete(s.lastDispatchedAt, id)
		}
	}
}

// dispatchIfDue 判断指定挂载点是否到了派发时间，如果是就下发扫描任务。
func (s *RefreshFileScheduler) dispatchIfDue(mp *models.MountPoint, now time.Time) {
	if mp == nil || !mp.EnableAutoRefresh {
		return
	}

	interval := mp.RefreshInterval
	if interval < 1 {
		interval = 30 // 最小 30 分钟，兜底 1 分钟
	}

	intervalDuration := time.Duration(interval) * time.Minute

	last, ok := s.lastDispatchedAt[mp.FileId]
	if ok && now.Sub(last) < intervalDuration {
		return
	}

	// 派发扫描任务
	taskReq := &topic.FileScanFileRequest{
		FileId: mp.FileId,
		Deep:   mp.EnableDeepRefresh,
	}

	body, err := json.Marshal(taskReq)
	if err != nil {
		s.ctx.Error("序列化刷新任务失败",
			zap.Int64("mount_point_id", mp.ID),
			zap.Error(err))

		return
	}

	taskCtx := s.ctx.
		WithValue(consts.CtxKeyFullPath, mp.FullPath).
		WithValue(consts.CtxKeyInvokeHandlerName, "定时任务")

	if err := s.taskEngine.PushMessage(taskCtx, taskReq.Topic(), body); err != nil {
		s.ctx.Error("推送文件扫描任务失败",
			zap.Int64("mount_point_id", mp.ID),
			zap.Int64("file_id", mp.FileId),
			zap.String("full_path", mp.FullPath),
			zap.Error(err))

		return
	}

	s.lastDispatchedAt[mp.FileId] = now

	s.ctx.Info("下发文件扫描任务成功",
		zap.Int64("mount_point_id", mp.ID),
		zap.Int64("file_id", mp.FileId),
		zap.String("full_path", mp.FullPath),
		zap.Int("refresh_interval_min", interval),
		zap.Bool("deep", mp.EnableDeepRefresh))
}
