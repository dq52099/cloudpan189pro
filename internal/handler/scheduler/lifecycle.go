package scheduler

import (
	"sync"

	"github.com/xxcheng123/cloudpan189-share/internal/framework/context"
)

func shouldStartScheduler(running, stopping bool) (bool, error) {
	if !running {
		return true, nil
	}

	if stopping {
		return false, ErrSchedulerRunning
	}

	return false, nil
}

func markSchedulerRunStarted(running, stopping *bool, doneField *chan struct{}) chan struct{} {
	done := make(chan struct{})
	*running = true
	*stopping = false
	*doneField = done

	return done
}

func finishSchedulerRun(
	mu *sync.Mutex,
	running *bool,
	stopping *bool,
	cancel *context.CancelFunc,
	doneField *chan struct{},
	done chan struct{},
) {
	mu.Lock()
	defer mu.Unlock()

	if *doneField == done {
		*running = false
		*stopping = false
		*cancel = nil
		*doneField = nil
	}

	close(done)
}

func beginSchedulerStop(
	mu *sync.Mutex,
	running *bool,
	stopping *bool,
	cancelField *context.CancelFunc,
	doneField *chan struct{},
) (context.CancelFunc, chan struct{}, bool) {
	mu.Lock()
	defer mu.Unlock()

	if !*running {
		return nil, nil, false
	}

	*stopping = true

	return *cancelField, *doneField, true
}

func waitSchedulerStop(cancel context.CancelFunc, done chan struct{}) {
	if cancel != nil {
		cancel()
	}

	if done != nil {
		<-done
	}
}
