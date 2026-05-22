package scheduler

import (
	"context"
	"testing"
	"time"

	appContext "github.com/xxcheng123/cloudpan189-share/internal/framework/context"
	"github.com/xxcheng123/cloudpan189-share/internal/repository/models"
	"go.uber.org/zap"
)

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
