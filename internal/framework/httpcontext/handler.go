package httpcontext

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"runtime/debug"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/xxcheng123/cloudpan189-share/internal/framework/context"
	"github.com/xxcheng123/cloudpan189-share/internal/pkgs/utils"
)

type HandlerFunc func(ctx *Context)

type HandlerFuncWrapper struct {
	logger *zap.Logger
}

var loggedURLPattern = regexp.MustCompile(`(?i)(?:[a-z][a-z0-9+.-]*://|/)[^\s"'<>]+`)

func NewHandlerFuncWrapper(logger *zap.Logger) *HandlerFuncWrapper {
	return &HandlerFuncWrapper{logger: logger}
}

const httpContextKey = "__request__context__"
const (
	maxLoggedJSONRequestBodySize = 4 * 1024
	maxLoggedResponseBodySize    = 4 * 1024
	loggedBodyTruncatedSuffix    = "...[truncated]"
)

func (w *HandlerFuncWrapper) Wrap(handler HandlerFunc) gin.HandlerFunc {
	return func(c *gin.Context) {
		if v, ok := c.Get(httpContextKey); ok {
			if ctx, ok2 := v.(*Context); ok2 {
				handler(ctx)

				return
			}
		}

		ctx := newContext(c, w.logger)

		c.Set(httpContextKey, ctx)

		handler(ctx)
	}
}

func (w *HandlerFuncWrapper) Wraps(handlers ...HandlerFunc) []gin.HandlerFunc {
	hds := make([]gin.HandlerFunc, 0, len(handlers))
	for _, h := range handlers {
		hds = append(hds, w.Wrap(h))
	}

	return hds
}

func loggerHandler() HandlerFunc {
	return func(reqContext *Context) {
		var (
			ts      = time.Now()
			ctx     = reqContext.GetContext()
			traceId = ctx.ID()

			fields = make([]zap.Field, 0)
		)

		reqContext.Writer.Header().Set(traceHeaderKey, traceId)

		var (
			decodedURL, _ = url.QueryUnescape(reqContext.Request.URL.RequestURI())
			safeURL       = sanitizeLoggedURL(decodedURL)
			requestInfo   = &context.Request{
				TTL:        "un-limit",
				Method:     reqContext.Request.Method,
				DecodedURL: safeURL,
				Header:     sanitizeLoggedHeaders(reqContext.Request.Header),
			}
		)

		// 只记录 json 请求体，且限制日志读取大小，避免大请求体被日志层完整读入内存。
		if strings.Contains(reqContext.Request.Header.Get("Content-Type"), "application/json") {
			if body, err := readRequestBodyForLog(reqContext.Request, maxLoggedJSONRequestBodySize); err == nil && body != "" {
				requestInfo.Body = sanitizeLoggedText(body)
			}
		}

		writer := &responseWriter{
			ResponseWriter: reqContext.Writer,
			b:              &bytes.Buffer{},
			maxBodySize:    maxLoggedResponseBodySize,
		}
		reqContext.Writer = writer

		defer func() {
			// region 发生 Panic 异常发送告警提醒
			if err := recover(); err != nil {
				stackInfo := string(debug.Stack())
				fields = append(fields,
					zap.String("panic", sanitizeLoggedPanicValue(err)),
					zap.String("stack", stackInfo),
					zap.String("trace_id", traceId),
				)
				reqContext.GetContext().Error("HTTP panic recovery", fields...)
				_ = reqContext.AbortWithError(http.StatusInternalServerError, errors.New("内部服务器错误"))
			}

			cost := time.Since(ts).Seconds()
			success := reqContext.Writer.Status() == http.StatusOK

			respInfo := &context.Response{
				Header:      sanitizeLoggedHeaders(reqContext.Writer.Header()),
				HttpCode:    reqContext.Writer.Status(),
				HttpCodeMsg: http.StatusText(reqContext.Writer.Status()),
				CostSeconds: cost,
			}
			if strings.Contains(reqContext.Writer.Header().Get("Content-Type"), "application/json") {
				respInfo.Body = sanitizeLoggedText(writer.loggedBody())
			}

			ctx.WithRequest(requestInfo)
			ctx.WithResponse(respInfo)

			fields = append(fields,
				zap.String("method", reqContext.Request.Method),
				zap.String("path", safeURL),
				zap.Int("http_code", reqContext.Writer.Status()),
				zap.Bool("success", success),
				zap.Float64("cost_seconds", cost),
				zap.Any("trace_info", ctx.Trace),
				zap.Strings("errors", sanitizeLoggedErrors(reqContext.errors)),
				zap.String("client_ip", reqContext.ClientIP()),
			)

			ctx.Info("http-log", fields...)
		}()

		reqContext.Next()
	}
}

func LoggerHandler(logger *zap.Logger) gin.HandlerFunc {
	return NewHandlerFuncWrapper(logger).Wrap(loggerHandler())
}

type requestBodyReplayReadCloser struct {
	io.Reader
	io.Closer
}

func readRequestBodyForLog(req *http.Request, maxSize int) (string, error) {
	if req == nil || req.Body == nil || maxSize <= 0 {
		return "", nil
	}

	body := req.Body
	data, err := io.ReadAll(io.LimitReader(body, int64(maxSize)+1))
	req.Body = &requestBodyReplayReadCloser{
		Reader: io.MultiReader(bytes.NewReader(data), body),
		Closer: body,
	}

	if err != nil {
		return "", err
	}

	if len(data) == 0 {
		return "", nil
	}

	if len(data) > maxSize {
		return string(data[:maxSize]) + loggedBodyTruncatedSuffix, nil
	}

	return string(data), nil
}

func sanitizeLoggedURL(rawURL string) string {
	return utils.RedactURLForLog(rawURL)
}

func sanitizeLoggedText(text string) string {
	message := loggedURLPattern.ReplaceAllStringFunc(text, sanitizeLoggedURL)

	return utils.RedactSensitiveText(message)
}

func sanitizeLoggedErrors(errs []error) []string {
	if len(errs) == 0 {
		return nil
	}

	sanitized := make([]string, 0, len(errs))
	for _, err := range errs {
		if err == nil {
			continue
		}

		sanitized = append(sanitized, sanitizeLoggedError(err))
	}

	return sanitized
}

func sanitizeLoggedError(err error) string {
	if err == nil {
		return ""
	}

	return sanitizeLoggedText(err.Error())
}

func sanitizeLoggedPanicValue(value interface{}) string {
	return sanitizeLoggedText(fmt.Sprint(value))
}

func sanitizeLoggedHeaders(headers http.Header) map[string][]string {
	sanitized := make(map[string][]string, len(headers))
	for key, values := range headers {
		if utils.IsSensitiveLogKey(key) {
			sanitized[key] = []string{utils.RedactedSecret}

			continue
		}

		redactedValues := make([]string, 0, len(values))
		for _, value := range values {
			redactedValues = append(redactedValues, sanitizeLoggedText(value))
		}

		sanitized[key] = redactedValues
	}

	return sanitized
}

type responseWriter struct {
	gin.ResponseWriter
	b           *bytes.Buffer
	maxBodySize int
	truncated   bool
}

func (w *responseWriter) Write(data []byte) (int, error) {
	w.captureBodyBytes(data)

	return w.ResponseWriter.Write(data)
}

func (w *responseWriter) WriteString(data string) (int, error) {
	w.captureBodyString(data)

	return w.ResponseWriter.WriteString(data)
}

func (w *responseWriter) captureBodyBytes(data []byte) {
	limit, ok := w.captureLimit(len(data))
	if !ok {
		return
	}

	_, _ = w.b.Write(data[:limit])
}

func (w *responseWriter) captureBodyString(data string) {
	limit, ok := w.captureLimit(len(data))
	if !ok {
		return
	}

	_, _ = w.b.WriteString(data[:limit])
}

func (w *responseWriter) captureLimit(dataSize int) (int, bool) {
	if w == nil || w.b == nil || w.maxBodySize <= 0 || dataSize == 0 {
		return 0, false
	}

	remaining := w.maxBodySize - w.b.Len()
	if remaining <= 0 {
		w.truncated = true

		return 0, false
	}

	if dataSize > remaining {
		w.truncated = true

		return remaining, true
	}

	return dataSize, true
}

func (w *responseWriter) loggedBody() string {
	if w == nil || w.b == nil {
		return ""
	}

	body := w.b.String()
	if w.truncated {
		return body + loggedBodyTruncatedSuffix
	}

	return body
}
