package taskcontext

import (
	stdContext "context"
	"testing"

	"go.uber.org/zap"
)

type traceIDContext struct {
	stdContext.Context
	value any
}

func (c traceIDContext) Value(key any) any {
	if key == traceHeaderKey {
		return c.value
	}

	return c.Context.Value(key)
}

func TestNewContextIgnoresNonStringTraceID(t *testing.T) {
	parent := traceIDContext{
		Context: stdContext.Background(),
		value:   123,
	}

	ctx := newContext(parent, []byte(`{"ok":true}`), zap.NewNop())
	if ctx == nil {
		t.Fatal("expected task context")
	}

	if ctx.GetContext().ID() == "" {
		t.Fatal("expected generated trace id")
	}
}

func TestNewContextUsesStringTraceID(t *testing.T) {
	parent := traceIDContext{
		Context: stdContext.Background(),
		value:   "trace-test",
	}

	ctx := newContext(parent, nil, zap.NewNop())
	if ctx.GetContext().ID() != "trace-test" {
		t.Fatalf("expected trace id trace-test, got %q", ctx.GetContext().ID())
	}
}
