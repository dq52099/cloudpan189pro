package taskcontext

import (
	stdContext "context"
	"fmt"
	"regexp"
	"runtime/debug"

	"github.com/google/uuid"
	"github.com/xxcheng123/cloudpan189-share/internal/pkgs/taskengine"
	"github.com/xxcheng123/cloudpan189-share/internal/pkgs/utils"
	"go.uber.org/zap"
)

type HandlerFunc func(ctx *Context) error

var panicURLPattern = regexp.MustCompile(`(?i)(?:[a-z][a-z0-9+.-]*://|/)[^\s"'<>]+`)

type messageProcessor struct {
	handlerFunc HandlerFunc
	logger      *zap.Logger
	processorId string
}

func (p *messageProcessor) Process(ctx stdContext.Context, message []byte) (err error) {
	defer func() {
		// 捕获panic并记录日志
		if recovered := recover(); recovered != nil {
			stackInfo := string(debug.Stack())
			panicValue := sanitizePanicValue(recovered)

			logger := p.logger
			if logger == nil {
				logger = zap.NewNop()
			}

			logger.Error("task processor panic recovery",
				zap.String("panic", panicValue),
				zap.String("stack", stackInfo),
				zap.String("processor_id", p.processorId),
			)

			err = fmt.Errorf("task processor panic: %s", panicValue)
		}
	}()

	return p.handlerFunc(newContext(ctx, message, p.logger))
}

func (p *messageProcessor) ProcessorID() string {
	return p.processorId
}

func newMessageProcessor(handlerFunc HandlerFunc, logger *zap.Logger) *messageProcessor {
	if logger == nil {
		logger = zap.NewNop()
	}

	return &messageProcessor{
		handlerFunc: handlerFunc,
		logger:      logger,
		processorId: uuid.NewString(),
	}
}

func sanitizePanicValue(recovered interface{}) string {
	message := fmt.Sprint(recovered)
	message = panicURLPattern.ReplaceAllStringFunc(message, utils.RedactURLForLog)

	return utils.RedactSensitiveText(message)
}

type HandlerFuncWrapper struct {
	logger *zap.Logger
}

func (h *HandlerFuncWrapper) Wrap(fn HandlerFunc) taskengine.MessageProcessor {
	var logger *zap.Logger
	if h != nil {
		logger = h.logger
	}

	return newMessageProcessor(fn, logger)
}

func NewHandlerFuncWrapper(logger *zap.Logger) *HandlerFuncWrapper {
	if logger == nil {
		logger = zap.NewNop()
	}

	return &HandlerFuncWrapper{logger: logger}
}
