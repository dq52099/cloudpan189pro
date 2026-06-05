package scheduler

import (
	"context"
	"errors"
	"testing"
	"time"

	appContext "github.com/xxcheng123/cloudpan189-share/internal/framework/context"
	"github.com/xxcheng123/cloudpan189-share/internal/pkgs/taskengine"
	"github.com/xxcheng123/cloudpan189-share/internal/pkgs/utils"
	"github.com/xxcheng123/cloudpan189-share/internal/repository/models"
	mediaconfigSvi "github.com/xxcheng123/cloudpan189-share/internal/services/mediaconfig"
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

type mockRebuildStrmMediaConfigService struct {
	updateErr   error
	updateCount int
	fields      []utils.Field
}

func (m *mockRebuildStrmMediaConfigService) Query(appContext.Context) (*models.MediaConfig, error) {
	return nil, nil
}

func (m *mockRebuildStrmMediaConfigService) Update(_ appContext.Context, fields ...utils.Field) error {
	m.updateCount++
	m.fields = append(m.fields, fields...)

	return m.updateErr
}

func (m *mockRebuildStrmMediaConfigService) Init(appContext.Context, *mediaconfigSvi.InitRequest) error {
	return nil
}

func (m *mockRebuildStrmMediaConfigService) Toggle(appContext.Context, bool) error {
	return nil
}

func restoreRebuildStrmSharedMediaConfig(t *testing.T) {
	t.Helper()

	oldConfig := shared.GetMediaConfig()

	t.Cleanup(func() {
		shared.SetMediaConfig(oldConfig)
	})
}

func TestRebuildStrmSchedulerStartRejectsMissingTaskEngine(t *testing.T) {
	scheduler := NewRebuildStrmScheduler(nil, &mockRebuildStrmMediaConfigService{})
	ctx := appContext.NewContext(context.Background(), appContext.WithLogger(zap.NewNop()))

	if err := scheduler.Start(ctx); !errors.Is(err, ErrSchedulerTaskEngineMissing) {
		t.Fatalf("expected missing task engine error, got %v", err)
	}
}

func TestRebuildStrmTickDoesNotAdvanceWhenDispatchFails(t *testing.T) {
	restoreRebuildStrmSharedMediaConfig(t)

	shared.SetMediaConfig(&models.MediaConfig{
		Enable:            true,
		AutoRebuildEnable: true,
		AutoRebuildCron:   "* * * * *",
	})

	dispatchErr := errors.New("queue unavailable")
	engine := &mockRebuildStrmTaskEngine{pushErr: dispatchErr}
	mediaConfig := &mockRebuildStrmMediaConfigService{}
	scheduler := &RebuildStrmScheduler{
		ctx:         appContext.NewContext(context.Background(), appContext.WithLogger(zap.NewNop())),
		taskEngine:  engine,
		mediaConfig: mediaConfig,
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

	if mediaConfig.updateCount != 0 {
		t.Fatalf("expected media config not to be updated when dispatch fails, got %d updates", mediaConfig.updateCount)
	}

	if cfg := shared.GetMediaConfig(); cfg != nil && !cfg.LastRebuildTime.IsZero() {
		t.Fatalf("expected LastRebuildTime to stay zero when dispatch fails, got %s", cfg.LastRebuildTime)
	}

	if time.Until(scheduler.nextRunAt) > 0 {
		t.Fatalf("expected nextRunAt to remain due for retry, got %s", scheduler.nextRunAt)
	}
}

func TestRebuildStrmTickPersistsLastRebuildTimeAfterDispatch(t *testing.T) {
	restoreRebuildStrmSharedMediaConfig(t)

	shared.SetMediaConfig(&models.MediaConfig{
		Enable:            true,
		AutoRebuildEnable: true,
		AutoRebuildCron:   "* * * * *",
	})

	engine := &mockRebuildStrmTaskEngine{}
	mediaConfig := &mockRebuildStrmMediaConfigService{}
	scheduler := &RebuildStrmScheduler{
		ctx:         appContext.NewContext(context.Background(), appContext.WithLogger(zap.NewNop())),
		taskEngine:  engine,
		mediaConfig: mediaConfig,
		currentCron: "* * * * *",
		nextRunAt:   time.Now().Add(-time.Minute),
	}

	scheduler.tick()

	if engine.count != 1 {
		t.Fatalf("expected one dispatch attempt, got %d", engine.count)
	}

	if scheduler.lastRun.IsZero() {
		t.Fatal("expected lastRun to be updated after dispatch")
	}

	if mediaConfig.updateCount != 1 {
		t.Fatalf("expected one media config update, got %d", mediaConfig.updateCount)
	}

	if len(mediaConfig.fields) != 1 || mediaConfig.fields[0].Key != "last_rebuild_time" {
		t.Fatalf("expected last_rebuild_time update field, got %#v", mediaConfig.fields)
	}

	fieldTime, ok := mediaConfig.fields[0].Value.(time.Time)
	if !ok || fieldTime.IsZero() {
		t.Fatalf("expected last_rebuild_time value to be non-zero time, got %#v", mediaConfig.fields[0].Value)
	}

	if cfg := shared.GetMediaConfig(); cfg == nil || cfg.LastRebuildTime.IsZero() {
		t.Fatalf("expected shared LastRebuildTime to be updated, got %#v", cfg)
	}
}
