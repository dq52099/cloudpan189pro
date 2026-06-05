package taskcontext

import (
	stdContext "context"
	"errors"
	"strings"
	"testing"

	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

func TestMessageProcessorProcessReturnsNilOnSuccess(t *testing.T) {
	processor := newMessageProcessor(func(ctx *Context) error {
		return nil
	}, zap.NewNop())

	if err := processor.Process(stdContext.Background(), nil); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
}

func TestMessageProcessorProcessReturnsHandlerError(t *testing.T) {
	wantErr := errors.New("handler failed")
	processor := newMessageProcessor(func(ctx *Context) error {
		return wantErr
	}, zap.NewNop())

	err := processor.Process(stdContext.Background(), nil)
	if !errors.Is(err, wantErr) {
		t.Fatalf("expected handler error, got %v", err)
	}
}

func TestMessageProcessorProcessReturnsErrorOnPanic(t *testing.T) {
	processor := newMessageProcessor(func(ctx *Context) error {
		panic("handler panic")
	}, zap.NewNop())

	err := processor.Process(stdContext.Background(), nil)
	if err == nil {
		t.Fatal("expected panic error")
	}

	if !strings.Contains(err.Error(), "task processor panic") {
		t.Fatalf("expected panic error prefix, got %v", err)
	}

	if !strings.Contains(err.Error(), "handler panic") {
		t.Fatalf("expected panic value, got %v", err)
	}
}

func TestMessageProcessorProcessRecoversPanicWithNilLogger(t *testing.T) {
	processor := newMessageProcessor(func(ctx *Context) error {
		panic("handler panic")
	}, nil)

	err := processor.Process(stdContext.Background(), nil)
	if err == nil {
		t.Fatal("expected panic error")
	}

	if !strings.Contains(err.Error(), "task processor panic") {
		t.Fatalf("expected panic error prefix, got %v", err)
	}

	if !strings.Contains(err.Error(), "handler panic") {
		t.Fatalf("expected panic value, got %v", err)
	}
}

func TestMessageProcessorProcessRedactsSensitivePanicValue(t *testing.T) {
	core, logs := observer.New(zap.ErrorLevel)
	logger := zap.New(core)
	processor := newMessageProcessor(func(ctx *Context) error {
		panic(`GET "https://proxy-user:proxy-pass@example.test/api/search?keyword=private-movie&filename=secret.mkv#access_token=secret-fragment": accessCode=abcd`)
	}, logger)

	err := processor.Process(stdContext.Background(), nil)
	if err == nil {
		t.Fatal("expected panic error")
	}

	entries := logs.FilterMessage("task processor panic recovery").All()
	if len(entries) != 1 {
		t.Fatalf("expected one panic log, got %d", len(entries))
	}

	logText := entries[0].Message + entries[0].ContextMap()["panic"].(string)

	combined := err.Error() + "\n" + logText
	for _, leaked := range []string{"proxy-user", "proxy-pass", "private-movie", "secret.mkv", "secret-fragment", "abcd"} {
		if strings.Contains(combined, leaked) {
			t.Fatalf("expected %q to be redacted from %q", leaked, combined)
		}
	}

	if !strings.Contains(combined, "/api/search") {
		t.Fatalf("expected path to remain in %q", combined)
	}
}
