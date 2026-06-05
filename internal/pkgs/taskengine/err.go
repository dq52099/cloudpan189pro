package taskengine

import "errors"

var (
	ErrEngineAlreadyRunning = errors.New("task engine already running")
	ErrEngineNotRunning     = errors.New("task engine not running")
	ErrBufferFull           = errors.New("task engine buffer full")
	ErrTaskContextMissing   = errors.New("task context is nil")
	ErrProcessorMissing     = errors.New("processor is missing")
	ErrProcessorInvalid     = errors.New("processor is invalid")
	ErrProcessorRegistered  = errors.New("processor already registered")
	ErrProcessorNotFound    = errors.New("no processor found for topic")
	ErrInvalidOptions       = errors.New("invalid options")
)
