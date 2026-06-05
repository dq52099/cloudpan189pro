package scheduler

import (
	"context"
	"errors"
	"testing"

	appContext "github.com/xxcheng123/cloudpan189-share/internal/framework/context"
	"github.com/xxcheng123/cloudpan189-share/internal/pkgs/utils"
	"github.com/xxcheng123/cloudpan189-share/internal/repository/models"
	filetasklogSvi "github.com/xxcheng123/cloudpan189-share/internal/services/filetasklog"
	"go.uber.org/zap"
)

type mockFileTaskLogCheckService struct {
	filetasklogSvi.Service
	failedIDs []int64
	err       error
}

func (m *mockFileTaskLogCheckService) Failed(
	_ appContext.Context,
	key filetasklogSvi.LogKey,
	_ ...utils.Field,
) error {
	m.failedIDs = append(m.failedIDs, key.GetID())

	return m.err
}

func TestFileTaskLogCheckSchedulerReclaimStaleTasksSkipsNilTask(t *testing.T) {
	service := &mockFileTaskLogCheckService{}
	scheduler := &FileTaskLogCheckScheduler{fileTaskLogService: service}
	ctx := appContext.NewContext(context.Background(), appContext.WithLogger(zap.NewNop()))

	scheduler.reclaimStaleTasks(ctx, []*models.FileTaskLog{
		nil,
		{
			ID:     12,
			Status: models.StatusRunning,
		},
	})

	if len(service.failedIDs) != 1 || service.failedIDs[0] != 12 {
		t.Fatalf("expected only valid stale task to be reclaimed, got %v", service.failedIDs)
	}
}

func TestFileTaskLogCheckSchedulerStartRejectsMissingService(t *testing.T) {
	scheduler := NewFileTaskLogCheckScheduler(nil)
	ctx := appContext.NewContext(context.Background(), appContext.WithLogger(zap.NewNop()))

	if err := scheduler.Start(ctx); !errors.Is(err, ErrSchedulerFileTaskLogServiceMissing) {
		t.Fatalf("expected missing file task log service error, got %v", err)
	}
}
