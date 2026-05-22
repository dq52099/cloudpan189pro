package scheduler

import (
	"context"
	"errors"
	"testing"
	"time"

	appContext "github.com/xxcheng123/cloudpan189-share/internal/framework/context"
	"github.com/xxcheng123/cloudpan189-share/internal/pkgs/taskengine"
	"github.com/xxcheng123/cloudpan189-share/internal/repository/models"
	"github.com/xxcheng123/cloudpan189-share/internal/shared"
	"go.uber.org/zap"
)

type mockRebuildStrmTaskEngine struct {
	pushErr error
	count   int
}

func (m *mockRebuildStrmTaskEngine) Start() error {
	return nil
}

func (m *mockRebuildStrmTaskEngine) Stop() error {
	return nil
}

func (m *mockRebuildStrmTaskEngine) IsRunning() bool {
	return true
}

func (m *mockRebuildStrmTaskEngine) RegisterProcessor(taskengine.Topic, taskengine.MessageProcessor) error {
	return nil
}

func (m *mockRebuildStrmTaskEngine) PushMessage(context.Context, taskengine.Topic, []byte) error {
	m.count++

	return m.pushErr
}

func (m *mockRebuildStrmTaskEngine) GetStats() taskengine.TaskStats {
	return taskengine.TaskStats{}
}

func (m *mockRebuildStrmTaskEngine) GetRunningTasks() []*taskengine.TaskInfo {
	return nil
}

func (m *mockRebuildStrmTaskEngine) GetPendingTasks() []*taskengine.TaskInfo {
	return nil
}

func (m *mockRebuildStrmTaskEngine) SetWorkerCount(int) error {
	return nil
}

func TestRebuildStrmTickDoesNotAdvanceWhenDispatchFails(t *testing.T) {
	oldConfig := shared.MediaConfig
	defer func() {
		shared.MediaConfig = oldConfig
	}()

	shared.MediaConfig = &models.MediaConfig{
		Enable:            true,
		AutoRebuildEnable: true,
		AutoRebuildCron:   "* * * * *",
	}

	dispatchErr := errors.New("queue unavailable")
	engine := &mockRebuildStrmTaskEngine{pushErr: dispatchErr}
	scheduler := &RebuildStrmScheduler{
		ctx:         appContext.NewContext(context.Background(), appContext.WithLogger(zap.NewNop())),
		taskEngine:  engine,
		currentCron: "* * * * *",
		nextRunAt:   time.Now().Add(-time.Minute),
	}

	scheduler.tick()

	if engine.count != 1 {
		t.Fatalf("expected one dispatch attempt, got %d", engine.count)
	}

	if !scheduler.lastRun.IsZero() {
		t.Fatalf("expected lastRun to stay zero when dispatch fails, got %s", scheduler.lastRun)
	}

	if !shared.MediaConfig.LastRebuildTime.IsZero() {
		t.Fatalf("expected LastRebuildTime to stay zero when dispatch fails, got %s", shared.MediaConfig.LastRebuildTime)
	}

	if time.Until(scheduler.nextRunAt) > 0 {
		t.Fatalf("expected nextRunAt to remain due for retry, got %s", scheduler.nextRunAt)
	}
}
