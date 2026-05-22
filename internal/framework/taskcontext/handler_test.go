package taskcontext

import (
	stdContext "context"
	"errors"
	"strings"
	"testing"

	"go.uber.org/zap"
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
