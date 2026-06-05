package scheduler

import (
	"context"
	"errors"
	"testing"
	"time"

	appContext "github.com/xxcheng123/cloudpan189-share/internal/framework/context"
	"github.com/xxcheng123/cloudpan189-share/internal/repository/models"
	"github.com/xxcheng123/cloudpan189-share/internal/services/mountpoint"
	"github.com/xxcheng123/cloudpan189-share/internal/shared"
	"go.uber.org/zap"
)

type mockRefreshFileMountPointService struct {
	mountpoint.Service
	list []*models.MountPoint
	err  error
}

func (m *mockRefreshFileMountPointService) GetAutoRefreshList(
	appContext.Context,
	*mountpoint.GetAutoRefreshListRequest,
) ([]*models.MountPoint, error) {
	return m.list, m.err
}

func restoreRefreshFileSharedSetting(t *testing.T) {
	t.Helper()

	oldSaltKey := shared.GetSaltKey()
	oldBaseURL := shared.GetBaseURL()
	oldEnableAuth := shared.IsAuthEnabled()
	oldAddition := shared.GetSettingAddition()

	t.Cleanup(func() {
		shared.SetSetting(oldSaltKey, oldBaseURL, oldEnableAuth, oldAddition)
	})
}

func TestRefreshFileSchedulerStartRejectsMissingMountPointService(t *testing.T) {
	scheduler := NewRefreshFileScheduler(nil, &mockRebuildStrmTaskEngine{})
	ctx := appContext.NewContext(context.Background(), appContext.WithLogger(zap.NewNop()))

	if err := scheduler.Start(ctx); !errors.Is(err, ErrSchedulerMountPointServiceMissing) {
		t.Fatalf("expected missing mount point service error, got %v", err)
	}
}

func TestRefreshFileSchedulerStartRejectsMissingTaskEngine(t *testing.T) {
	scheduler := NewRefreshFileScheduler(&mockRefreshFileMountPointService{}, nil)
	ctx := appContext.NewContext(context.Background(), appContext.WithLogger(zap.NewNop()))

	if err := scheduler.Start(ctx); !errors.Is(err, ErrSchedulerTaskEngineMissing) {
		t.Fatalf("expected missing task engine error, got %v", err)
	}
}

func TestRefreshFileSchedulerSkipsInvalidMountPointFileID(t *testing.T) {
	engine := &mockRebuildStrmTaskEngine{}
	scheduler := &RefreshFileScheduler{
		ctx:              appContext.NewContext(context.Background(), appContext.WithLogger(zap.NewNop())),
		taskEngine:       engine,
		lastDispatchedAt: make(map[int64]time.Time),
	}

	scheduler.dispatchIfDue(&models.MountPoint{
		ID:                7,
		FileId:            0,
		FullPath:          "/invalid",
		EnableAutoRefresh: true,
		RefreshInterval:   30,
	}, time.Now())

	if engine.count != 0 {
		t.Fatalf("expected invalid file id not to be queued, got %d", engine.count)
	}

	if len(scheduler.lastDispatchedAt) != 0 {
		t.Fatalf("expected invalid file id not to update dispatch cache, got %v", scheduler.lastDispatchedAt)
	}
}

func TestRefreshFileSchedulerTickSkipsNilMountPoint(t *testing.T) {
	restoreRefreshFileSharedSetting(t)

	addition := shared.GetSettingAddition()
	addition.EnableStorageAutoRefresh = true
	shared.SetSetting(shared.GetSaltKey(), shared.GetBaseURL(), shared.IsAuthEnabled(), addition)

	engine := &mockRebuildStrmTaskEngine{}
	validMountPoint := &models.MountPoint{
		ID:                7,
		FileId:            42,
		FullPath:          "/auto-refresh",
		EnableAutoRefresh: true,
		RefreshInterval:   30,
	}
	scheduler := &RefreshFileScheduler{
		ctx:               appContext.NewContext(context.Background(), appContext.WithLogger(zap.NewNop())),
		mountPointService: &mockRefreshFileMountPointService{list: []*models.MountPoint{nil, validMountPoint}},
		taskEngine:        engine,
		lastDispatchedAt:  map[int64]time.Time{99: time.Now()},
	}

	scheduler.tick()

	if engine.count != 1 {
		t.Fatalf("expected nil mount point to be skipped and valid mount point queued, got %d", engine.count)
	}

	if _, ok := scheduler.lastDispatchedAt[validMountPoint.FileId]; !ok {
		t.Fatalf("expected valid mount point to update dispatch cache, got %v", scheduler.lastDispatchedAt)
	}

	if _, ok := scheduler.lastDispatchedAt[99]; ok {
		t.Fatalf("expected stale dispatch cache entry to be removed, got %v", scheduler.lastDispatchedAt)
	}

	if len(scheduler.lastDispatchedAt) != 1 {
		t.Fatalf("expected dispatch cache to contain only valid mount point, got %v", scheduler.lastDispatchedAt)
	}
}

func TestRefreshFileSchedulerDoesNotThrottleFailedDispatchByInterval(t *testing.T) {
	engine := &mockRebuildStrmTaskEngine{pushErr: errors.New("queue unavailable")}
	scheduler := &RefreshFileScheduler{
		ctx:              appContext.NewContext(context.Background(), appContext.WithLogger(zap.NewNop())),
		taskEngine:       engine,
		lastDispatchedAt: make(map[int64]time.Time),
	}

	mountPoint := &models.MountPoint{
		ID:                7,
		FileId:            42,
		FullPath:          "/auto-refresh",
		EnableAutoRefresh: true,
		RefreshInterval:   30,
	}
	now := time.Now()

	scheduler.dispatchIfDue(mountPoint, now)
	scheduler.dispatchIfDue(mountPoint, now.Add(time.Minute))

	if engine.count != 2 {
		t.Fatalf("expected failed dispatch to be retried on next tick, got %d enqueue attempts", engine.count)
	}

	if _, ok := scheduler.lastDispatchedAt[mountPoint.FileId]; ok {
		t.Fatalf("expected failed dispatch not to update dispatch cache, got %v", scheduler.lastDispatchedAt)
	}
}
