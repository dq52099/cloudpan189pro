package taskcontext

import (
	stdContext "context"
	"encoding/json"

	"github.com/xxcheng123/cloudpan189-share/internal/framework/context"
	"go.uber.org/zap"
)

const (
	traceHeaderKey = "Trace-Id"
)

type Context struct {
	stdContext context.Context
	data       []byte
}

func (c *Context) GetContext() context.Context {
	return c.stdContext
}

func (c *Context) Unmarshal(v any) error {
	return json.Unmarshal(c.data, v)
}

func newContext(stdContext stdContext.Context, data []byte, logger *zap.Logger) *Context {
	var traceId string

	if _traceId := stdContext.Value(traceHeaderKey); _traceId != nil {
		traceId, _ = _traceId.(string)
	}

	return &Context{
		data:       data,
		stdContext: context.NewContext(stdContext, context.WithLogger(logger), context.WithTraceId(traceId)),
	}
}
