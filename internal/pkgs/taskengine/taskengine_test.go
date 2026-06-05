package taskengine

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/xxcheng123/cloudpan189-share/internal/pkgs/utils"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

type testProcessor struct {
	id      string
	process func(ctx context.Context, message []byte) error
}

type nilableTestProcessor struct {
	id string
}

type panicIDProcessor struct{}

type nilableContext struct{}

func (p testProcessor) Process(ctx context.Context, message []byte) error {
	return p.process(ctx, message)
}

func (p testProcessor) ProcessorID() string {
	return p.id
}

func (p *nilableTestProcessor) Process(ctx context.Context, message []byte) error {
	return nil
}

func (p *nilableTestProcessor) ProcessorID() string {
	return p.id
}

func (p panicIDProcessor) Process(ctx context.Context, message []byte) error {
	return nil
}

func (p panicIDProcessor) ProcessorID() string {
	panic(`GET "https://proxy-user:proxy-pass@example.test/api/search?keyword=private-movie&filename=secret.mkv#access_token=secret-fragment": accessCode=abcd`)
}

func (*nilableContext) Deadline() (time.Time, bool) {
	return time.Time{}, false
}

func (*nilableContext) Done() <-chan struct{} {
	return nil
}

func (*nilableContext) Err() error {
	return nil
}

func (*nilableContext) Value(any) any {
	return nil
}

func TestRegisterProcessorRejectsNilProcessor(t *testing.T) {
	engine := NewTaskEngine(EngineOption{
		Logger: zap.NewNop(),
	})
	impl := engine.(*taskEngine)

	var processor MessageProcessor

	err := engine.RegisterProcessor(Topic("nil"), processor)
	if !errors.Is(err, ErrProcessorMissing) {
		t.Fatalf("expected processor missing error, got %v", err)
	}

	if got := len(impl.topicProcessors[Topic("nil")]); got != 0 {
		t.Fatalf("expected nil processor not to be registered, got %d processors", got)
	}
}

func TestRegisterProcessorRejectsTypedNilProcessor(t *testing.T) {
	engine := NewTaskEngine(EngineOption{
		Logger: zap.NewNop(),
	})
	impl := engine.(*taskEngine)

	var processor *nilableTestProcessor

	err := engine.RegisterProcessor(Topic("typed-nil"), processor)
	if !errors.Is(err, ErrProcessorMissing) {
		t.Fatalf("expected processor missing error, got %v", err)
	}

	if got := len(impl.topicProcessors[Topic("typed-nil")]); got != 0 {
		t.Fatalf("expected typed-nil processor not to be registered, got %d processors", got)
	}
}

func TestRegisterProcessorRejectsProcessorIDPanicWithoutRegistering(t *testing.T) {
	engine := NewTaskEngine(EngineOption{
		Logger: zap.NewNop(),
	})
	impl := engine.(*taskEngine)

	err := engine.RegisterProcessor(Topic("panic-id"), panicIDProcessor{})
	if !errors.Is(err, ErrProcessorInvalid) {
		t.Fatalf("expected invalid processor error, got %v", err)
	}

	if got := len(impl.topicProcessors[Topic("panic-id")]); got != 0 {
		t.Fatalf("expected panicking processor not to be registered, got %d processors", got)
	}

	for _, leaked := range []string{"proxy-user", "proxy-pass", "private-movie", "secret.mkv", "secret-fragment", "abcd"} {
		if strings.Contains(err.Error(), leaked) {
			t.Fatalf("expected %q to be redacted from %q", leaked, err.Error())
		}
	}
}

func TestRegisterProcessorRejectsEmptyProcessorID(t *testing.T) {
	engine := NewTaskEngine(EngineOption{
		Logger: zap.NewNop(),
	})
	impl := engine.(*taskEngine)

	err := engine.RegisterProcessor(Topic("empty-id"), testProcessor{
		id:      " \t",
		process: func(ctx context.Context, message []byte) error { return nil },
	})
	if !errors.Is(err, ErrProcessorInvalid) {
		t.Fatalf("expected invalid processor error, got %v", err)
	}

	if got := len(impl.topicProcessors[Topic("empty-id")]); got != 0 {
		t.Fatalf("expected empty-id processor not to be registered, got %d processors", got)
	}
}

func TestRegisterProcessorRejectsDuplicateProcessorIDForSameTopic(t *testing.T) {
	engine := NewTaskEngine(EngineOption{
		Logger: zap.NewNop(),
	})
	impl := engine.(*taskEngine)
	topic := Topic("duplicate-id")

	if err := engine.RegisterProcessor(topic, testProcessor{
		id:      "same",
		process: func(ctx context.Context, message []byte) error { return nil },
	}); err != nil {
		t.Fatalf("register first processor: %v", err)
	}

	err := engine.RegisterProcessor(topic, testProcessor{
		id:      "same",
		process: func(ctx context.Context, message []byte) error { return nil },
	})
	if !errors.Is(err, ErrProcessorRegistered) {
		t.Fatalf("expected duplicate processor error, got %v", err)
	}

	assertProcessorIDs(t, impl.topicProcessors[topic], []string{"same"})
}

func TestRegisterProcessorAllowsSameProcessorIDAcrossDifferentTopics(t *testing.T) {
	engine := NewTaskEngine(EngineOption{
		Logger: zap.NewNop(),
	})
	impl := engine.(*taskEngine)

	for _, topic := range []Topic{"topic-a", "topic-b"} {
		if err := engine.RegisterProcessor(topic, testProcessor{
			id:      "shared",
			process: func(ctx context.Context, message []byte) error { return nil },
		}); err != nil {
			t.Fatalf("register processor for %s: %v", topic, err)
		}
	}

	assertProcessorIDs(t, impl.topicProcessors[Topic("topic-a")], []string{"shared"})
	assertProcessorIDs(t, impl.topicProcessors[Topic("topic-b")], []string{"shared"})
}

func TestUnregisterProcessorRemovesOnlyMatchingProcessor(t *testing.T) {
	engine := NewTaskEngine(EngineOption{
		Logger: zap.NewNop(),
	})
	impl := engine.(*taskEngine)

	first := testProcessor{id: "first", process: func(ctx context.Context, message []byte) error { return nil }}
	second := testProcessor{id: "second", process: func(ctx context.Context, message []byte) error { return nil }}

	if err := engine.RegisterProcessor(Topic("topic"), first); err != nil {
		t.Fatalf("register first processor: %v", err)
	}

	if err := engine.RegisterProcessor(Topic("topic"), second); err != nil {
		t.Fatalf("register second processor: %v", err)
	}

	if err := impl.UnregisterProcessor(Topic("topic"), "first"); err != nil {
		t.Fatalf("unregister first processor: %v", err)
	}

	processors := impl.topicProcessors[Topic("topic")]
	if len(processors) != 1 {
		t.Fatalf("expected one processor after unregister, got %d", len(processors))
	}

	if got := processors[0].ProcessorID(); got != "second" {
		t.Fatalf("expected second processor to remain, got %q", got)
	}
}

func TestUnregisterProcessorDoesNotMutateExistingProcessorSlices(t *testing.T) {
	engine := NewTaskEngine(EngineOption{
		Logger: zap.NewNop(),
	})
	impl := engine.(*taskEngine)
	topic := Topic("stable-slice")

	for _, id := range []string{"first", "second", "third"} {
		if err := engine.RegisterProcessor(topic, testProcessor{
			id:      id,
			process: func(ctx context.Context, message []byte) error { return nil },
		}); err != nil {
			t.Fatalf("register %s processor: %v", id, err)
		}
	}

	previous := impl.topicProcessors[topic]
	if err := impl.UnregisterProcessor(topic, "second"); err != nil {
		t.Fatalf("unregister second processor: %v", err)
	}

	assertProcessorIDs(t, previous, []string{"first", "second", "third"})
	assertProcessorIDs(t, impl.topicProcessors[topic], []string{"first", "third"})
}

func TestUnregisterProcessorDeletesEmptyTopicAndRejectsMissingProcessor(t *testing.T) {
	engine := NewTaskEngine(EngineOption{
		Logger: zap.NewNop(),
	})
	impl := engine.(*taskEngine)

	if err := engine.RegisterProcessor(Topic("topic"), testProcessor{
		id:      "only",
		process: func(ctx context.Context, message []byte) error { return nil },
	}); err != nil {
		t.Fatalf("register processor: %v", err)
	}

	if err := impl.UnregisterProcessor(Topic("topic"), "only"); err != nil {
		t.Fatalf("unregister processor: %v", err)
	}

	if _, ok := impl.topicProcessors[Topic("topic")]; ok {
		t.Fatal("expected empty topic to be deleted")
	}

	if err := impl.UnregisterProcessor(Topic("topic"), "only"); !errors.Is(err, ErrProcessorNotFound) {
		t.Fatalf("expected missing processor error, got %v", err)
	}
}

func TestTaskEnginePushMessageRejectsNilContext(t *testing.T) {
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

	var nilCtx context.Context
	if err := engine.PushMessage(nilCtx, Topic("nil-context"), []byte(`{}`)); !errors.Is(err, ErrTaskContextMissing) {
		t.Fatalf("expected missing task context error, got %v", err)
	}

	stats := engine.GetStats()
	if stats.TotalTasks != 0 || stats.PendingTasks != 0 {
		t.Fatalf("expected nil context message not to be queued, got stats %+v", stats)
	}

	if pending := engine.GetPendingTasks(); len(pending) != 0 {
		t.Fatalf("expected no pending tasks, got %d", len(pending))
	}
}

func TestTaskEnginePushMessageRejectsTypedNilContext(t *testing.T) {
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

	var (
		typedContext *nilableContext
		ctx          context.Context = typedContext
	)
	if err := engine.PushMessage(ctx, Topic("typed-nil-context"), []byte(`{}`)); !errors.Is(err, ErrTaskContextMissing) {
		t.Fatalf("expected missing task context error, got %v", err)
	}

	stats := engine.GetStats()
	if stats.TotalTasks != 0 || stats.PendingTasks != 0 {
		t.Fatalf("expected typed-nil context message not to be queued, got stats %+v", stats)
	}
}

func TestTaskEngineRunningTaskUsesProcessorSnapshotDuringUnregister(t *testing.T) {
	engine := NewTaskEngine(EngineOption{
		Logger: zap.NewNop(),
		Options: []OptionFunc{
			WithWorkerCount(1),
			WithBufferSize(1),
		},
	})
	impl := engine.(*taskEngine)
	topic := Topic("processor-snapshot")

	firstStarted := make(chan struct{})
	releaseFirst := make(chan struct{})
	releaseSecond := make(chan struct{})
	secondRan := make(chan struct{})

	releaseAll := func() {
		select {
		case <-releaseFirst:
		default:
			close(releaseFirst)
		}

		select {
		case <-releaseSecond:
		default:
			close(releaseSecond)
		}
	}

	err := engine.RegisterProcessor(topic, testProcessor{
		id: "first",
		process: func(ctx context.Context, message []byte) error {
			close(firstStarted)
			<-releaseFirst

			return nil
		},
	})
	if err != nil {
		t.Fatalf("register first processor: %v", err)
	}

	err = engine.RegisterProcessor(topic, testProcessor{
		id: "second",
		process: func(ctx context.Context, message []byte) error {
			<-releaseSecond
			close(secondRan)

			return nil
		},
	})
	if err != nil {
		t.Fatalf("register second processor: %v", err)
	}

	if err := engine.Start(); err != nil {
		t.Fatalf("start engine: %v", err)
	}
	defer func() {
		if err := engine.Stop(); err != nil {
			t.Fatalf("stop engine: %v", err)
		}
	}()
	defer releaseAll()

	if err := engine.PushMessage(context.Background(), topic, []byte(`{}`)); err != nil {
		t.Fatalf("push message: %v", err)
	}

	select {
	case <-firstStarted:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for first processor")
	}

	if err := impl.UnregisterProcessor(topic, "second"); err != nil {
		t.Fatalf("unregister second processor: %v", err)
	}

	releaseAll()

	select {
	case <-secondRan:
	case <-time.After(time.Second):
		t.Fatal("expected second processor from running task snapshot to run")
	}

	stats := waitTaskStats(t, engine, func(stats TaskStats) bool {
		return stats.TotalTasks == 1 &&
			stats.PendingTasks == 0 &&
			stats.RunningTasks == 0 &&
			stats.CompletedTasks == 1
	})

	if stats.FailedTasks != 0 || stats.CancelledTasks != 0 {
		t.Fatalf("expected snapshot task to complete cleanly, got %+v", stats)
	}
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
			WithMaxRetry(0),
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

func TestTaskEngineRetriesOnlyFailedProcessors(t *testing.T) {
	engine := NewTaskEngine(EngineOption{
		Logger: zap.NewNop(),
		Options: []OptionFunc{
			WithWorkerCount(1),
			WithBufferSize(1),
			WithMaxRetry(1),
			WithRetryDelay(0),
		},
	})

	var (
		stableCalls atomic.Int64
		flakyCalls  atomic.Int64
	)

	if err := engine.RegisterProcessor(Topic("retry-success"), testProcessor{
		id: "stable",
		process: func(ctx context.Context, message []byte) error {
			stableCalls.Add(1)

			return nil
		},
	}); err != nil {
		t.Fatalf("register stable processor: %v", err)
	}

	if err := engine.RegisterProcessor(Topic("retry-success"), testProcessor{
		id: "flaky",
		process: func(ctx context.Context, message []byte) error {
			if flakyCalls.Add(1) == 1 {
				return errors.New("temporary failure")
			}

			return nil
		},
	}); err != nil {
		t.Fatalf("register flaky processor: %v", err)
	}

	if err := engine.Start(); err != nil {
		t.Fatalf("start engine: %v", err)
	}

	if err := engine.PushMessage(context.Background(), Topic("retry-success"), []byte(`{}`)); err != nil {
		t.Fatalf("push message: %v", err)
	}

	stats := waitTaskStats(t, engine, func(stats TaskStats) bool {
		return stats.TotalTasks == 1 &&
			stats.PendingTasks == 0 &&
			stats.RunningTasks == 0 &&
			stats.CompletedTasks == 1
	})

	if stats.FailedTasks != 0 || stats.CancelledTasks != 0 {
		t.Fatalf("expected retry task to complete cleanly, got %+v", stats)
	}

	if got := stableCalls.Load(); got != 1 {
		t.Fatalf("expected stable processor to run once, got %d", got)
	}

	if got := flakyCalls.Load(); got != 2 {
		t.Fatalf("expected flaky processor to run twice, got %d", got)
	}

	if err := engine.Stop(); err != nil {
		t.Fatalf("stop engine: %v", err)
	}
}

func TestTaskEngineFailsAfterMaxRetry(t *testing.T) {
	engine := NewTaskEngine(EngineOption{
		Logger: zap.NewNop(),
		Options: []OptionFunc{
			WithWorkerCount(1),
			WithBufferSize(1),
			WithMaxRetry(1),
			WithRetryDelay(0),
		},
	})

	var calls atomic.Int64

	if err := engine.RegisterProcessor(Topic("retry-failed"), testProcessor{
		id: "failing",
		process: func(ctx context.Context, message []byte) error {
			calls.Add(1)

			return errors.New("still failing")
		},
	}); err != nil {
		t.Fatalf("register failing processor: %v", err)
	}

	if err := engine.Start(); err != nil {
		t.Fatalf("start engine: %v", err)
	}

	if err := engine.PushMessage(context.Background(), Topic("retry-failed"), []byte(`{}`)); err != nil {
		t.Fatalf("push message: %v", err)
	}

	stats := waitTaskStats(t, engine, func(stats TaskStats) bool {
		return stats.TotalTasks == 1 &&
			stats.PendingTasks == 0 &&
			stats.RunningTasks == 0 &&
			stats.FailedTasks == 1
	})

	if stats.CompletedTasks != 0 || stats.CancelledTasks != 0 {
		t.Fatalf("expected retry task to fail after max retry, got %+v", stats)
	}

	if got := calls.Load(); got != 2 {
		t.Fatalf("expected initial attempt plus one retry, got %d calls", got)
	}

	if err := engine.Stop(); err != nil {
		t.Fatalf("stop engine: %v", err)
	}
}

func TestTaskEngineDoesNotRetryCancelledTask(t *testing.T) {
	engine := NewTaskEngine(EngineOption{
		Logger: zap.NewNop(),
		Options: []OptionFunc{
			WithWorkerCount(1),
			WithBufferSize(1),
			WithMaxRetry(3),
			WithRetryDelay(0),
			WithProcessTimeout(10 * time.Millisecond),
		},
	})

	var calls atomic.Int64

	if err := engine.RegisterProcessor(Topic("retry-cancelled"), testProcessor{
		id: "cancelled",
		process: func(ctx context.Context, message []byte) error {
			calls.Add(1)
			<-ctx.Done()

			return ctx.Err()
		},
	}); err != nil {
		t.Fatalf("register cancelled processor: %v", err)
	}

	if err := engine.Start(); err != nil {
		t.Fatalf("start engine: %v", err)
	}

	if err := engine.PushMessage(context.Background(), Topic("retry-cancelled"), []byte(`{}`)); err != nil {
		t.Fatalf("push message: %v", err)
	}

	stats := waitTaskStats(t, engine, func(stats TaskStats) bool {
		return stats.TotalTasks == 1 &&
			stats.PendingTasks == 0 &&
			stats.RunningTasks == 0 &&
			stats.CancelledTasks == 1
	})

	if stats.CompletedTasks != 0 || stats.FailedTasks != 0 {
		t.Fatalf("expected cancelled task not to be retried as failure, got %+v", stats)
	}

	if got := calls.Load(); got != 1 {
		t.Fatalf("expected cancelled task not to retry, got %d calls", got)
	}

	if err := engine.Stop(); err != nil {
		t.Fatalf("stop engine: %v", err)
	}
}

func TestTaskEngineMarksTimedOutTaskCancelledEvenWhenProcessorReturnsNil(t *testing.T) {
	engine := NewTaskEngine(EngineOption{
		Logger: zap.NewNop(),
		Options: []OptionFunc{
			WithWorkerCount(1),
			WithBufferSize(1),
			WithMaxRetry(3),
			WithRetryDelay(0),
			WithProcessTimeout(10 * time.Millisecond),
		},
	})

	var calls atomic.Int64

	if err := engine.RegisterProcessor(Topic("timeout-ignored"), testProcessor{
		id: "timeout-ignored",
		process: func(ctx context.Context, message []byte) error {
			calls.Add(1)
			time.Sleep(30 * time.Millisecond)

			return nil
		},
	}); err != nil {
		t.Fatalf("register timeout processor: %v", err)
	}

	if err := engine.Start(); err != nil {
		t.Fatalf("start engine: %v", err)
	}

	if err := engine.PushMessage(context.Background(), Topic("timeout-ignored"), []byte(`{}`)); err != nil {
		t.Fatalf("push message: %v", err)
	}

	stats := waitTaskStats(t, engine, func(stats TaskStats) bool {
		return stats.TotalTasks == 1 &&
			stats.PendingTasks == 0 &&
			stats.RunningTasks == 0 &&
			stats.CancelledTasks == 1
	})

	if stats.CompletedTasks != 0 || stats.FailedTasks != 0 {
		t.Fatalf("expected timed out task to be cancelled, got %+v", stats)
	}

	if got := calls.Load(); got != 1 {
		t.Fatalf("expected timed out task not to retry after nil return, got %d calls", got)
	}

	if err := engine.Stop(); err != nil {
		t.Fatalf("stop engine: %v", err)
	}
}

func TestTaskEngineTimeoutReleasesWorkerWhenProcessorIgnoresContext(t *testing.T) {
	engine := NewTaskEngine(EngineOption{
		Logger: zap.NewNop(),
		Options: []OptionFunc{
			WithWorkerCount(1),
			WithBufferSize(2),
			WithMaxRetry(0),
			WithProcessTimeout(10 * time.Millisecond),
		},
	})

	started := make(chan struct{})

	release := make(chan struct{})
	defer close(release)

	if err := engine.RegisterProcessor(Topic("stubborn"), testProcessor{
		id: "stubborn",
		process: func(ctx context.Context, message []byte) error {
			close(started)
			<-release

			return nil
		},
	}); err != nil {
		t.Fatalf("register stubborn processor: %v", err)
	}

	fastDone := make(chan struct{})

	if err := engine.RegisterProcessor(Topic("fast"), testProcessor{
		id: "fast",
		process: func(ctx context.Context, message []byte) error {
			close(fastDone)

			return nil
		},
	}); err != nil {
		t.Fatalf("register fast processor: %v", err)
	}

	if err := engine.Start(); err != nil {
		t.Fatalf("start engine: %v", err)
	}

	if err := engine.PushMessage(context.Background(), Topic("stubborn"), []byte(`{}`)); err != nil {
		t.Fatalf("push stubborn message: %v", err)
	}

	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for stubborn processor")
	}

	waitTaskStats(t, engine, func(stats TaskStats) bool {
		return stats.TotalTasks == 1 &&
			stats.PendingTasks == 0 &&
			stats.RunningTasks == 0 &&
			stats.CancelledTasks == 1
	})

	if err := engine.PushMessage(context.Background(), Topic("fast"), []byte(`{}`)); err != nil {
		t.Fatalf("push fast message after timeout: %v", err)
	}

	select {
	case <-fastDone:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for worker to process fast message after timeout")
	}

	waitTaskStats(t, engine, func(stats TaskStats) bool {
		return stats.TotalTasks == 2 &&
			stats.PendingTasks == 0 &&
			stats.RunningTasks == 0 &&
			stats.CancelledTasks == 1 &&
			stats.CompletedTasks == 1
	})

	stopped := make(chan error, 1)
	go func() {
		stopped <- engine.Stop()
	}()

	select {
	case err := <-stopped:
		if err != nil {
			t.Fatalf("stop engine: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for stop after ignored processor timeout")
	}
}

func TestSanitizeProcessorPanicValueRedactsSensitiveText(t *testing.T) {
	got := sanitizeProcessorPanicValue(`GET "https://proxy-user:proxy-pass@example.test/api/search?keyword=private-movie&filename=secret.mkv#access_token=secret-fragment": accessCode=abcd`)

	for _, leaked := range []string{"proxy-user", "proxy-pass", "private-movie", "secret.mkv", "secret-fragment", "abcd"} {
		if strings.Contains(got, leaked) {
			t.Fatalf("expected %q to be redacted from %q", leaked, got)
		}
	}

	if !strings.Contains(got, "/api/search") {
		t.Fatalf("expected path to remain in %q", got)
	}

	if !strings.Contains(got, utils.RedactedSecret) {
		t.Fatalf("expected redacted marker in %q", got)
	}
}

func TestSanitizeProcessorPanicValueOmitsOversizeText(t *testing.T) {
	got := sanitizeProcessorPanicValue(`panic with token=secret-token ` + strings.Repeat("x", maxProcessorLogTextSize+1))

	for _, leaked := range []string{"secret-token", strings.Repeat("x", 32)} {
		if strings.Contains(got, leaked) {
			t.Fatalf("expected oversized panic value %q to be omitted, got %q", leaked, got)
		}
	}

	if !strings.Contains(got, oversizeProcessorLogText) {
		t.Fatalf("expected oversized panic marker, got %q", got)
	}

	if !strings.Contains(got, "[truncated]") {
		t.Fatalf("expected oversized panic to be marked truncated, got %q", got)
	}
}

func TestSanitizeProcessorErrorOmitsOversizeText(t *testing.T) {
	got := sanitizeProcessorError(errors.New(`processor failed with accessToken=secret-access ` + strings.Repeat("x", maxProcessorLogTextSize+1)))

	for _, leaked := range []string{"secret-access", strings.Repeat("x", 32)} {
		if strings.Contains(got, leaked) {
			t.Fatalf("expected oversized processor error value %q to be omitted, got %q", leaked, got)
		}
	}

	if !strings.Contains(got, oversizeProcessorLogText) {
		t.Fatalf("expected oversized processor error marker, got %q", got)
	}

	if !strings.Contains(got, "[truncated]") {
		t.Fatalf("expected oversized processor error to be marked truncated, got %q", got)
	}
}

func TestTaskEngineRedactsProcessorErrorResultAndLog(t *testing.T) {
	core, logs := observer.New(zap.ErrorLevel)
	release := make(chan struct{})
	failed := make(chan struct{})
	rawErr := errors.New(`GET "https://proxy-user:proxy-pass@example.test/api/search?keyword=private-movie&filename=secret.mkv#access_token=secret-fragment": accessCode=abcd Authorization: Bearer bearer-secret`)
	engine := NewTaskEngine(EngineOption{
		Logger: zap.New(core),
		Options: []OptionFunc{
			WithWorkerCount(1),
			WithBufferSize(1),
			WithMaxRetry(0),
		},
	})

	defer func() {
		select {
		case <-release:
		default:
			close(release)
		}
	}()

	err := engine.RegisterProcessor(Topic("redact-error"), testProcessor{
		id: "failing",
		process: func(ctx context.Context, message []byte) error {
			close(failed)

			return rawErr
		},
	})
	if err != nil {
		t.Fatalf("register failing processor: %v", err)
	}

	err = engine.RegisterProcessor(Topic("redact-error"), testProcessor{
		id: "blocking",
		process: func(ctx context.Context, message []byte) error {
			<-failed
			<-release

			return nil
		},
	})
	if err != nil {
		t.Fatalf("register blocking processor: %v", err)
	}

	if err := engine.Start(); err != nil {
		t.Fatalf("start engine: %v", err)
	}

	if err := engine.PushMessage(context.Background(), Topic("redact-error"), []byte(`{}`)); err != nil {
		t.Fatalf("push message: %v", err)
	}

	result := waitProcessorResult(t, engine, "failing")
	assertNoSensitiveTaskEngineText(t, result.Error)

	entries := logs.FilterMessage("processor failed").All()
	if len(entries) != 1 {
		t.Fatalf("expected one processor failed log, got %d", len(entries))
	}

	loggedError, ok := entries[0].ContextMap()["error"].(string)
	if !ok {
		t.Fatalf("expected string error field, got %#v", entries[0].ContextMap())
	}

	assertNoSensitiveTaskEngineText(t, loggedError)

	close(release)

	waitTaskStats(t, engine, func(stats TaskStats) bool {
		return stats.TotalTasks == 1 &&
			stats.PendingTasks == 0 &&
			stats.RunningTasks == 0 &&
			stats.FailedTasks == 1
	})

	if err := engine.Stop(); err != nil {
		t.Fatalf("stop engine: %v", err)
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

func TestTaskEngineStartDuringStopIsSafe(t *testing.T) {
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
	releaseClosed := false

	closeRelease := func() {
		if !releaseClosed {
			close(release)

			releaseClosed = true
		}
	}
	defer closeRelease()

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

	startErr := engine.Start()
	if startErr != nil && !errors.Is(startErr, ErrEngineAlreadyRunning) {
		t.Fatalf("expected start during stop to either succeed after stop or return running, got %v", startErr)
	}

	closeRelease()

	select {
	case err := <-stopped:
		if err != nil {
			t.Fatalf("stop engine: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for stop")
	}

	if startErr == nil {
		if err := engine.Stop(); err != nil {
			t.Fatalf("stop restarted engine: %v", err)
		}

		return
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
	if stats.PendingTasks != 0 || stats.RunningTasks != 0 || stats.CancelledTasks != 2 {
		t.Fatalf("expected no pending/running tasks and two cancelled tasks after stop, got %+v", stats)
	}

	if stats.TotalTasks != stats.CompletedTasks+stats.FailedTasks+stats.CancelledTasks+stats.RunningTasks+stats.PendingTasks {
		t.Fatalf("expected terminal stats to reconcile with total after stop, got %+v", stats)
	}

	if pending := engine.GetPendingTasks(); len(pending) != 0 {
		t.Fatalf("expected no pending tasks after stop, got %d", len(pending))
	}

	if running := engine.GetRunningTasks(); len(running) != 0 {
		t.Fatalf("expected no running tasks after stop, got %d", len(running))
	}
}

func TestTaskEngineTaskSnapshotsAreDetached(t *testing.T) {
	engine := NewTaskEngine(EngineOption{
		Logger: zap.NewNop(),
		Options: []OptionFunc{
			WithWorkerCount(1),
			WithBufferSize(2),
		},
	})

	started := make(chan struct{})
	release := make(chan struct{})

	err := engine.RegisterProcessor(Topic("blocking"), testProcessor{
		id: "blocking",
		process: func(ctx context.Context, message []byte) error {
			select {
			case <-started:
			default:
				close(started)
			}

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

	if err := engine.PushMessage(context.Background(), Topic("blocking"), []byte(`{"id":1}`)); err != nil {
		t.Fatalf("push running message: %v", err)
	}

	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for processor")
	}

	if err := engine.PushMessage(context.Background(), Topic("blocking"), []byte(`{"id":2}`)); err != nil {
		t.Fatalf("push pending message: %v", err)
	}

	running := waitTaskList(t, engine.GetRunningTasks, 1)
	pending := waitTaskList(t, engine.GetPendingTasks, 1)

	running[0].Status = "tampered"
	running[0].Payload[0] = 'x'
	running[0].AddResult(ProcessorResult{ProcessorID: "external"})

	pending[0].Status = "tampered"
	pending[0].Payload[0] = 'x'

	running = waitTaskList(t, engine.GetRunningTasks, 1)
	if running[0].Status != TaskStatusRunning {
		t.Fatalf("expected running task status to stay %q, got %q", TaskStatusRunning, running[0].Status)
	}

	if string(running[0].Payload) != `{"id":1}` {
		t.Fatalf("expected running payload snapshot to be detached, got %q", string(running[0].Payload))
	}

	if len(running[0].Results) != 0 {
		t.Fatalf("expected external result mutation not to affect task, got %#v", running[0].Results)
	}

	pending = waitTaskList(t, engine.GetPendingTasks, 1)
	if pending[0].Status != TaskStatusPending {
		t.Fatalf("expected pending task status to stay %q, got %q", TaskStatusPending, pending[0].Status)
	}

	if string(pending[0].Payload) != `{"id":2}` {
		t.Fatalf("expected pending payload snapshot to be detached, got %q", string(pending[0].Payload))
	}

	close(release)

	waitTaskStats(t, engine, func(stats TaskStats) bool {
		return stats.TotalTasks == 2 &&
			stats.PendingTasks == 0 &&
			stats.RunningTasks == 0 &&
			stats.CompletedTasks == 2
	})

	if err := engine.Stop(); err != nil {
		t.Fatalf("stop engine: %v", err)
	}
}

func TestTaskEngineSetWorkerCountShrinksIdleWorkers(t *testing.T) {
	engine := NewTaskEngine(EngineOption{
		Logger: zap.NewNop(),
		Options: []OptionFunc{
			WithWorkerCount(3),
			WithBufferSize(1),
		},
	}).(*taskEngine)

	if err := engine.Start(); err != nil {
		t.Fatalf("start engine: %v", err)
	}

	waitWorkerCount(t, engine, 3)

	if err := engine.SetWorkerCount(1); err != nil {
		t.Fatalf("set worker count: %v", err)
	}

	waitWorkerCount(t, engine, 1)

	if err := engine.Stop(); err != nil {
		t.Fatalf("stop engine: %v", err)
	}
}

func TestTaskEngineSetWorkerCountDoesNotCancelRunningTask(t *testing.T) {
	engine := NewTaskEngine(EngineOption{
		Logger: zap.NewNop(),
		Options: []OptionFunc{
			WithWorkerCount(2),
			WithBufferSize(1),
		},
	})

	started := make(chan struct{})
	release := make(chan struct{})
	cancelled := make(chan struct{}, 1)

	err := engine.RegisterProcessor(Topic("blocking"), testProcessor{
		id: "blocking",
		process: func(ctx context.Context, message []byte) error {
			close(started)

			select {
			case <-ctx.Done():
				cancelled <- struct{}{}

				return ctx.Err()
			case <-release:
				return nil
			}
		},
	})
	if err != nil {
		t.Fatalf("register processor: %v", err)
	}

	if err := engine.Start(); err != nil {
		t.Fatalf("start engine: %v", err)
	}

	if err := engine.PushMessage(context.Background(), Topic("blocking"), []byte(`{}`)); err != nil {
		t.Fatalf("push running message: %v", err)
	}

	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for processor")
	}

	if err := engine.SetWorkerCount(1); err != nil {
		t.Fatalf("set worker count: %v", err)
	}

	select {
	case <-cancelled:
		t.Fatal("worker shrink should not cancel the running task")
	case <-time.After(50 * time.Millisecond):
	}

	close(release)

	stats := waitTaskStats(t, engine, func(stats TaskStats) bool {
		return stats.TotalTasks == 1 &&
			stats.PendingTasks == 0 &&
			stats.RunningTasks == 0 &&
			stats.CompletedTasks == 1
	})

	if stats.FailedTasks != 0 {
		t.Fatalf("expected no failed tasks, got %+v", stats)
	}

	if err := engine.Stop(); err != nil {
		t.Fatalf("stop engine: %v", err)
	}
}

func assertProcessorIDs(t *testing.T, processors []MessageProcessor, want []string) {
	t.Helper()

	if len(processors) != len(want) {
		t.Fatalf("expected %d processors, got %d", len(want), len(processors))
	}

	for idx, processor := range processors {
		if got := processor.ProcessorID(); got != want[idx] {
			t.Fatalf("expected processor %d to be %q, got %q", idx, want[idx], got)
		}
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

func waitProcessorResult(t *testing.T, engine TaskEngine, processorID string) ProcessorResult {
	t.Helper()

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		for _, task := range engine.GetRunningTasks() {
			for _, result := range task.Results {
				if result.ProcessorID == processorID {
					return result
				}
			}
		}

		time.Sleep(10 * time.Millisecond)
	}

	t.Fatalf("timed out waiting for processor result %q", processorID)

	return ProcessorResult{}
}

func assertNoSensitiveTaskEngineText(t *testing.T, text string) {
	t.Helper()

	for _, leaked := range []string{"proxy-user", "proxy-pass", "private-movie", "secret.mkv", "secret-fragment", "abcd", "bearer-secret"} {
		if strings.Contains(text, leaked) {
			t.Fatalf("expected %q to be redacted from %q", leaked, text)
		}
	}

	if !strings.Contains(text, "/api/search") {
		t.Fatalf("expected path to remain in %q", text)
	}

	if !strings.Contains(text, utils.RedactedSecret) {
		t.Fatalf("expected redacted marker in %q", text)
	}
}

func waitTaskList(t *testing.T, get func() []*TaskInfo, want int) []*TaskInfo {
	t.Helper()

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		tasks := get()
		if len(tasks) == want {
			return tasks
		}

		time.Sleep(10 * time.Millisecond)
	}

	tasks := get()
	t.Fatalf("timed out waiting for %d tasks, got %d", want, len(tasks))

	return tasks
}

func waitWorkerCount(t *testing.T, engine *taskEngine, want int) {
	t.Helper()

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		engine.mu.RLock()
		got := len(engine.workerCancels)
		active := engine.activeWorkerCountLocked()
		engine.mu.RUnlock()

		if got == want && active == want {
			return
		}

		time.Sleep(10 * time.Millisecond)
	}

	engine.mu.RLock()
	got := len(engine.workerCancels)
	active := engine.activeWorkerCountLocked()
	engine.mu.RUnlock()

	t.Fatalf("timed out waiting for %d workers, got total=%d active=%d", want, got, active)
}
