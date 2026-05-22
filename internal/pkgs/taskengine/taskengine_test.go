package taskengine

import (
	"context"
	"errors"
	"testing"
	"time"

	"go.uber.org/zap"
)

type testProcessor struct {
	id      string
	process func(ctx context.Context, message []byte) error
}

func (p testProcessor) Process(ctx context.Context, message []byte) error {
	return p.process(ctx, message)
}

func (p testProcessor) ProcessorID() string {
	return p.id
}

func TestTaskEngineRemovesCompletedTaskFromPending(t *testing.T) {
	engine := NewTaskEngine(EngineOption{
		Logger: zap.NewNop(),
		Options: []OptionFunc{
			WithWorkerCount(1),
			WithBufferSize(1),
		},
	})

	processed := make(chan struct{})

	err := engine.RegisterProcessor(Topic("registered"), testProcessor{
		id: "test",
		process: func(ctx context.Context, message []byte) error {
			close(processed)

			return nil
		},
	})
	if err != nil {
		t.Fatalf("register processor: %v", err)
	}

	if err := engine.Start(); err != nil {
		t.Fatalf("start engine: %v", err)
	}

	defer func() {
		if err := engine.Stop(); err != nil {
			t.Fatalf("stop engine: %v", err)
		}
	}()

	if err := engine.PushMessage(context.Background(), Topic("registered"), []byte(`{}`)); err != nil {
		t.Fatalf("push message: %v", err)
	}

	select {
	case <-processed:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for processor")
	}

	stats := waitTaskStats(t, engine, func(stats TaskStats) bool {
		return stats.TotalTasks == 1 &&
			stats.PendingTasks == 0 &&
			stats.RunningTasks == 0 &&
			stats.CompletedTasks == 1
	})

	if stats.FailedTasks != 0 {
		t.Fatalf("expected no failed tasks, got %+v", stats)
	}

	if pending := engine.GetPendingTasks(); len(pending) != 0 {
		t.Fatalf("expected no pending tasks, got %d", len(pending))
	}

	if running := engine.GetRunningTasks(); len(running) != 0 {
		t.Fatalf("expected no running tasks, got %d", len(running))
	}
}

func TestTaskEngineMarksMissingProcessorAsFailed(t *testing.T) {
	engine := NewTaskEngine(EngineOption{
		Logger: zap.NewNop(),
		Options: []OptionFunc{
			WithWorkerCount(1),
			WithBufferSize(1),
		},
	})

	if err := engine.Start(); err != nil {
		t.Fatalf("start engine: %v", err)
	}

	defer func() {
		if err := engine.Stop(); err != nil {
			t.Fatalf("stop engine: %v", err)
		}
	}()

	if err := engine.PushMessage(context.Background(), Topic("missing"), []byte(`{}`)); err != nil {
		t.Fatalf("push message: %v", err)
	}

	stats := waitTaskStats(t, engine, func(stats TaskStats) bool {
		return stats.TotalTasks == 1 &&
			stats.PendingTasks == 0 &&
			stats.RunningTasks == 0 &&
			stats.FailedTasks == 1
	})

	if stats.CompletedTasks != 0 {
		t.Fatalf("expected no completed tasks, got %+v", stats)
	}

	if pending := engine.GetPendingTasks(); len(pending) != 0 {
		t.Fatalf("expected no pending tasks, got %d", len(pending))
	}

	if running := engine.GetRunningTasks(); len(running) != 0 {
		t.Fatalf("expected no running tasks, got %d", len(running))
	}
}

func TestTaskEngineRecoversProcessorPanic(t *testing.T) {
	engine := NewTaskEngine(EngineOption{
		Logger: zap.NewNop(),
		Options: []OptionFunc{
			WithWorkerCount(1),
			WithBufferSize(2),
		},
	})

	err := engine.RegisterProcessor(Topic("panic"), testProcessor{
		id: "panic",
		process: func(ctx context.Context, message []byte) error {
			panic("processor panic")
		},
	})
	if err != nil {
		t.Fatalf("register panic processor: %v", err)
	}

	err = engine.RegisterProcessor(Topic("success"), testProcessor{
		id: "success",
		process: func(ctx context.Context, message []byte) error {
			return nil
		},
	})
	if err != nil {
		t.Fatalf("register success processor: %v", err)
	}

	if err := engine.Start(); err != nil {
		t.Fatalf("start engine: %v", err)
	}

	defer func() {
		if err := engine.Stop(); err != nil {
			t.Fatalf("stop engine: %v", err)
		}
	}()

	if err := engine.PushMessage(context.Background(), Topic("panic"), []byte(`{}`)); err != nil {
		t.Fatalf("push panic message: %v", err)
	}

	waitTaskStats(t, engine, func(stats TaskStats) bool {
		return stats.TotalTasks == 1 &&
			stats.PendingTasks == 0 &&
			stats.RunningTasks == 0 &&
			stats.FailedTasks == 1
	})

	if err := engine.PushMessage(context.Background(), Topic("success"), []byte(`{}`)); err != nil {
		t.Fatalf("push success message: %v", err)
	}

	stats := waitTaskStats(t, engine, func(stats TaskStats) bool {
		return stats.TotalTasks == 2 &&
			stats.PendingTasks == 0 &&
			stats.RunningTasks == 0 &&
			stats.FailedTasks == 1 &&
			stats.CompletedTasks == 1
	})

	if stats.CompletedTasks != 1 {
		t.Fatalf("expected engine to keep processing after panic, got %+v", stats)
	}
}

func TestTaskEngineStopDoesNotDeadlockWhenProcessorPushesOnCancel(t *testing.T) {
	engine := NewTaskEngine(EngineOption{
		Logger: zap.NewNop(),
		Options: []OptionFunc{
			WithWorkerCount(1),
			WithBufferSize(2),
		},
	})

	started := make(chan struct{})
	pushErr := make(chan error, 1)

	err := engine.RegisterProcessor(Topic("running"), testProcessor{
		id: "test",
		process: func(ctx context.Context, message []byte) error {
			close(started)
			<-ctx.Done()

			pushErr <- engine.PushMessage(context.Background(), Topic("running"), []byte(`{}`))

			return nil
		},
	})
	if err != nil {
		t.Fatalf("register processor: %v", err)
	}

	if err := engine.Start(); err != nil {
		t.Fatalf("start engine: %v", err)
	}

	if err := engine.PushMessage(context.Background(), Topic("running"), []byte(`{}`)); err != nil {
		t.Fatalf("push message: %v", err)
	}

	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for processor")
	}

	stopped := make(chan error, 1)
	go func() {
		stopped <- engine.Stop()
	}()

	select {
	case err := <-pushErr:
		if !errors.Is(err, ErrEngineNotRunning) {
			t.Fatalf("expected engine not running error, got %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for push during stop")
	}

	select {
	case err := <-stopped:
		if err != nil {
			t.Fatalf("stop engine: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for stop")
	}
}

func TestTaskEngineRejectsStartWhileStopping(t *testing.T) {
	engine := NewTaskEngine(EngineOption{
		Logger: zap.NewNop(),
		Options: []OptionFunc{
			WithWorkerCount(1),
			WithBufferSize(1),
		},
	})

	started := make(chan struct{})
	cancelled := make(chan struct{})
	release := make(chan struct{})

	err := engine.RegisterProcessor(Topic("blocking"), testProcessor{
		id: "blocking",
		process: func(ctx context.Context, message []byte) error {
			close(started)
			<-ctx.Done()
			close(cancelled)
			<-release

			return nil
		},
	})
	if err != nil {
		t.Fatalf("register processor: %v", err)
	}

	if err := engine.Start(); err != nil {
		t.Fatalf("start engine: %v", err)
	}

	if err := engine.PushMessage(context.Background(), Topic("blocking"), []byte(`{}`)); err != nil {
		t.Fatalf("push message: %v", err)
	}

	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for processor")
	}

	stopped := make(chan error, 1)
	go func() {
		stopped <- engine.Stop()
	}()

	select {
	case <-cancelled:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for processor cancel")
	}

	if err := engine.Start(); !errors.Is(err, ErrEngineAlreadyRunning) {
		t.Fatalf("expected engine already running while stopping, got %v", err)
	}

	close(release)

	select {
	case err := <-stopped:
		if err != nil {
			t.Fatalf("stop engine: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for stop")
	}

	if err := engine.Start(); err != nil {
		t.Fatalf("restart engine after stop: %v", err)
	}

	if err := engine.Stop(); err != nil {
		t.Fatalf("stop restarted engine: %v", err)
	}
}

func TestTaskEngineStopClearsQueuedPendingTasks(t *testing.T) {
	engine := NewTaskEngine(EngineOption{
		Logger: zap.NewNop(),
		Options: []OptionFunc{
			WithWorkerCount(1),
			WithBufferSize(2),
		},
	})

	started := make(chan struct{})

	err := engine.RegisterProcessor(Topic("blocking"), testProcessor{
		id: "blocking",
		process: func(ctx context.Context, message []byte) error {
			select {
			case <-started:
			default:
				close(started)
			}

			<-ctx.Done()

			return nil
		},
	})
	if err != nil {
		t.Fatalf("register processor: %v", err)
	}

	if err := engine.Start(); err != nil {
		t.Fatalf("start engine: %v", err)
	}

	if err := engine.PushMessage(context.Background(), Topic("blocking"), []byte(`{"id":1}`)); err != nil {
		t.Fatalf("push running message: %v", err)
	}

	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for processor")
	}

	if err := engine.PushMessage(context.Background(), Topic("blocking"), []byte(`{"id":2}`)); err != nil {
		t.Fatalf("push queued message: %v", err)
	}

	if pending := engine.GetPendingTasks(); len(pending) != 1 {
		t.Fatalf("expected one queued pending task, got %d", len(pending))
	}

	if err := engine.Stop(); err != nil {
		t.Fatalf("stop engine: %v", err)
	}

	stats := engine.GetStats()
	if stats.PendingTasks != 0 || stats.RunningTasks != 0 {
		t.Fatalf("expected no pending or running tasks after stop, got %+v", stats)
	}

	if pending := engine.GetPendingTasks(); len(pending) != 0 {
		t.Fatalf("expected no pending tasks after stop, got %d", len(pending))
	}

	if running := engine.GetRunningTasks(); len(running) != 0 {
		t.Fatalf("expected no running tasks after stop, got %d", len(running))
	}
}

func waitTaskStats(t *testing.T, engine TaskEngine, match func(TaskStats) bool) TaskStats {
	t.Helper()

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		stats := engine.GetStats()
		if match(stats) {
			return stats
		}

		time.Sleep(10 * time.Millisecond)
	}

	stats := engine.GetStats()
	t.Fatalf("timed out waiting for stats, got %+v", stats)

	return stats
}
