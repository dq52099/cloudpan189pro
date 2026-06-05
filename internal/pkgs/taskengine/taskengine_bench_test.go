package taskengine

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"sync/atomic"
	"testing"
	"time"

	"go.uber.org/zap"
)

func BenchmarkTaskEngineProcessMessages(b *testing.B) {
	for _, workerCount := range []int{1, 4, 16} {
		b.Run(fmt.Sprintf("workers_%d", workerCount), func(b *testing.B) {
			benchmarkTaskEngineProcessMessages(b, workerCount)
		})
	}
}

func benchmarkTaskEngineProcessMessages(b *testing.B, workerCount int) {
	b.Helper()

	const topic = Topic("benchmark")

	var processed atomic.Int64

	done := make(chan struct{})
	target := int64(b.N)
	engine := NewTaskEngine(EngineOption{
		Logger: zap.NewNop(),
		Options: []OptionFunc{
			WithWorkerCount(workerCount),
			WithBufferSize(4096),
			WithProcessTimeout(time.Minute),
			WithMaxRetry(0),
		},
	})

	err := engine.RegisterProcessor(topic, testProcessor{
		id: "noop",
		process: func(context.Context, []byte) error {
			if processed.Add(1) == target {
				close(done)
			}

			return nil
		},
	})
	if err != nil {
		b.Fatalf("register processor: %v", err)
	}

	if err := engine.Start(); err != nil {
		b.Fatalf("start engine: %v", err)
	}

	payload := []byte(`{"id":1}`)
	ctx := context.Background()

	b.ReportAllocs()
	b.SetBytes(int64(len(payload)))
	b.ResetTimer()

	for idx := 0; idx < b.N; idx++ {
		pushBenchmarkMessage(b, engine, ctx, topic, payload)
	}

	select {
	case <-done:
	case <-time.After(30 * time.Second):
		b.Fatalf("timed out waiting for %d processed messages, got %d", b.N, processed.Load())
	}

	b.StopTimer()

	if err := engine.Stop(); err != nil {
		b.Fatalf("stop engine: %v", err)
	}
}

func pushBenchmarkMessage(
	b *testing.B,
	engine TaskEngine,
	ctx context.Context,
	topic Topic,
	payload []byte,
) {
	b.Helper()

	for {
		err := engine.PushMessage(ctx, topic, payload)
		if err == nil {
			return
		}

		if errors.Is(err, ErrBufferFull) {
			runtime.Gosched()

			continue
		}

		b.Fatalf("push message: %v", err)
	}
}
