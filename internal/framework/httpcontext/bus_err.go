package httpcontext

import (
	"net/http"

	"github.com/pkg/errors"
	"github.com/xxcheng123/cloudpan189-share/internal/pkgs/utils"
)

type BusinessError interface {
	GetHTTPCode() int
	GetMessage() string
	GetError() error
	GetCode() int
	Error() string

	WithError(err error) BusinessError
	WithHTTPCode(code int) BusinessError
	WithMessage(message string) BusinessError
	WithBusinessCode(businessCode int) BusinessError
}

type businessError struct {
	httpCode     int    // HTTP 状态码
	businessCode int    // 业务码
	message      string // 错误描述
	stackError   error  // 含有堆栈信息的错误
}

func (b *businessError) GetHTTPCode() int {
	if b.httpCode == 0 {
		b.httpCode = http.StatusBadRequest
	}

	return b.httpCode
}

func (b *businessError) GetCode() int {
	return b.businessCode
}

func (b *businessError) GetMessage() string {
	return b.message
}

func (b *businessError) GetError() error {
	return b.stackError
}

func (b *businessError) Error() string {
	return b.message
}

func (b *businessError) WithError(err error) BusinessError {
	b.stackError = errors.WithStack(err)

	return b
}

func (b *businessError) WithHTTPCode(code int) BusinessError {
	b.httpCode = code

	return b
}

func (b *businessError) WithMessage(message string) BusinessError {
	b.message = message

	return b
}

func (b *businessError) WithBusinessCode(businessCode int) BusinessError {
	b.businessCode = businessCode

	return b
}

type BusinessGenerator interface {
	Next(msg string) BusinessError
}

type businessGenerator struct {
	startCode int
	idx       int
}

func (b *businessGenerator) Next(msg string) BusinessError {
	br := &businessError{
		businessCode: b.startCode + b.idx,
		message:      msg,
		httpCode:     http.StatusBadRequest,
	}

	b.idx++

	return br
}

func NewBusinessIota(startCode int) BusinessGenerator {
	return &businessGenerator{startCode: startCode}
}

func NewBusinessGenerator(startCode int) BusinessGenerator {
	return &businessGenerator{startCode: startCode}
}

const (
	defaultUnauthorizedMessage = "Unauthorized"
	defaultForbiddenMessage    = "Forbidden"
	invalidParamsCode          = 99998
)

func unauthorizedBusinessError(messages ...string) BusinessError {
	return &businessError{
		businessCode: http.StatusUnauthorized,
		message:      utils.UseSimplify(defaultUnauthorizedMessage, messages...),
		httpCode:     http.StatusUnauthorized,
	}
}

func forbiddenBusinessError(messages ...string) BusinessError {
	return &businessError{
		businessCode: http.StatusForbidden,
		message:      utils.UseSimplify(defaultForbiddenMessage, messages...),
		httpCode:     http.StatusForbidden,
	}
}

func invalidParamsBusinessError(err error) BusinessError {
	return &businessError{
		businessCode: invalidParamsCode,
		message:      utils.TranslateValidationError(err),
		httpCode:     http.StatusBadRequest,
		stackError:   errors.WithStack(err),
	}
}
