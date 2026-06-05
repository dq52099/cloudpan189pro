package scheduler

import (
	"errors"
	"sync"
	"testing"
	"time"

	appContext "github.com/xxcheng123/cloudpan189-share/internal/framework/context"
)

func TestSchedulerLifecycleStartStateTransitions(t *testing.T) {
	shouldStart, err := shouldStartScheduler(false, false)
	if err != nil || !shouldStart {
		t.Fatalf("expected stopped scheduler to start, shouldStart=%v err=%v", shouldStart, err)
	}

	var doneField chan struct{}

	running := false
	stopping := false
	done := markSchedulerRunStarted(&running, &stopping, &doneField)

	if !running || stopping || doneField != done {
		t.Fatalf("expected scheduler to be marked running, running=%v stopping=%v doneField=%p done=%p", running, stopping, doneField, done)
	}

	shouldStart, err = shouldStartScheduler(running, stopping)
	if err != nil || shouldStart {
		t.Fatalf("expected running scheduler start to be idempotent, shouldStart=%v err=%v", shouldStart, err)
	}

	stopping = true

	shouldStart, err = shouldStartScheduler(running, stopping)
	if !errors.Is(err, ErrSchedulerRunning) || shouldStart {
		t.Fatalf("expected stopping scheduler start to be rejected, shouldStart=%v err=%v", shouldStart, err)
	}
}

func TestSchedulerLifecycleStopWaitsForFinishAndCleansState(t *testing.T) {
	var (
		mu        sync.Mutex
		cancel    appContext.CancelFunc
		doneField chan struct{}
		running   bool
		stopping  bool
	)

	cancelled := make(chan struct{})
	cancel = func() {
		close(cancelled)
	}

	done := markSchedulerRunStarted(&running, &stopping, &doneField)

	stopDone := make(chan struct{})

	go func() {
		cancel, done, ok := beginSchedulerStop(&mu, &running, &stopping, &cancel, &doneField)
		if !ok {
			t.Errorf("expected stop to begin for running scheduler")

			return
		}

		waitSchedulerStop(cancel, done)
		close(stopDone)
	}()

	select {
	case <-cancelled:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for scheduler cancellation")
	}

	select {
	case <-stopDone:
		t.Fatal("expected stop to wait until worker finishes")
	case <-time.After(20 * time.Millisecond):
	}

	finishSchedulerRun(&mu, &running, &stopping, &cancel, &doneField, done)

	select {
	case <-stopDone:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for stop to finish")
	}

	if running || stopping || cancel != nil || doneField != nil {
		t.Fatalf("expected lifecycle state cleanup, running=%v stopping=%v cancel=%v doneField=%v", running, stopping, cancel, doneField)
	}
}
