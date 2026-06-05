package context

import (
	"fmt"
	"io"
	"net/http"
	"regexp"
	"runtime"
	"strings"
	"time"

	"go.uber.org/zap"
	"resty.dev/v3"

	"github.com/xxcheng123/cloudpan189-share/internal/consts"
	"github.com/xxcheng123/cloudpan189-share/internal/pkgs/utils"
)

var httpLogURLPattern = regexp.MustCompile(`(?i)(?:[a-z][a-z0-9+.-]*://|/)[^\s"'<>]+`)

const (
	truncatedHTTPLogBodySuffix = "...[truncated]"
	oversizeHTTPLogBodyMessage = "[body omitted: exceeds log limit]"
)

// HTTPLogConfig HTTP日志配置
type HTTPLogConfig struct {
	EnableRequestLog  bool     // 是否启用请求日志
	EnableResponseLog bool     // 是否启用响应日志
	EnableTraceLog    bool     // 是否启用 trace 记录（默认启用）
	LogLevel          string   // 日志级别: debug, info, warn, error
	LogHeaders        bool     // 是否记录请求头
	LogBody           bool     // 是否记录请求体
	MaxBodySize       int      // 最大记录的请求体大小（字节）
	SensitiveHeaders  []string // 敏感请求头列表（将被脱敏）
}

// DefaultHTTPLogConfig 默认HTTP日志配置
func DefaultHTTPLogConfig() *HTTPLogConfig {
	return &HTTPLogConfig{
		EnableRequestLog:  true,
		EnableResponseLog: true,
		EnableTraceLog:    true, // 默认启用 trace 记录
		LogLevel:          "info",
		LogHeaders:        true,
		LogBody:           true,
		MaxBodySize:       1024, // 1KB
		SensitiveHeaders: []string{
			"authorization", "cookie", "x-auth-token",
			"x-api-key", "token", "password",
		},
	}
}

// HTTPClientOption HTTP客户端选项
type HTTPClientOption struct {
	LogConfig *HTTPLogConfig
	Timeout   time.Duration
}

// HTTPClientOptionFunc HTTP客户端选项函数
type HTTPClientOptionFunc func(*HTTPClientOption)

// WithHTTPLogConfig 设置HTTP日志配置
func WithHTTPLogConfig(config *HTTPLogConfig) HTTPClientOptionFunc {
	return func(opt *HTTPClientOption) {
		if config != nil {
			opt.LogConfig = config
		}
	}
}

// WithHTTPTimeout 设置HTTP超时时间
func WithHTTPTimeout(timeout time.Duration) HTTPClientOptionFunc {
	return func(opt *HTTPClientOption) {
		opt.Timeout = timeout
	}
}

// HTTPClient 创建带有日志钩子的HTTP客户端
func (c Context) HTTPClient(opts ...HTTPClientOptionFunc) *resty.Client {
	option := &HTTPClientOption{
		LogConfig: DefaultHTTPLogConfig(),
		Timeout:   30 * time.Second,
	}

	for _, opt := range opts {
		opt(option)
	}

	client := resty.New().
		SetTimeout(option.Timeout)

	// 设置请求中间件
	if option.LogConfig.EnableRequestLog {
		client.AddRequestMiddleware(c.createRequestMiddleware(option.LogConfig))
	}

	// 设置响应中间件 - 如果启用了 trace 记录或响应日志，都需要添加
	if option.LogConfig.EnableResponseLog || option.LogConfig.EnableTraceLog {
		client.AddResponseMiddleware(c.createResponseMiddleware(option.LogConfig))
	}

	// 设置错误钩子
	client.OnError(c.createErrorHook(option.LogConfig))

	return client
}

// createRequestMiddleware 创建请求中间件
func (c Context) createRequestMiddleware(config *HTTPLogConfig) resty.RequestMiddleware {
	return func(client *resty.Client, req *resty.Request) error {
		startTime := time.Now()

		// 将开始时间存储到请求上下文中，供响应中间件使用
		req.SetContext(req.Context())

		fields := []zap.Field{
			zap.String("method", req.Method),
			zap.String("url", redactHTTPLogURL(req.URL)),
			zap.Time("start_time", startTime),
		}

		// 记录请求头
		if config.LogHeaders && req.Header != nil {
			headers := c.sanitizeHeaders(req.Header, config.SensitiveHeaders)
			fields = append(fields, zap.Any("headers", headers))
		}

		// 记录请求体
		if config.LogBody && req.Body != nil {
			bodyStr := c.formatRequestBody(req.Body, config.MaxBodySize)
			if bodyStr != "" {
				fields = append(fields, zap.String("body", bodyStr))
			}
		}

		// 根据配置的日志级别记录
		c.logWithLevel(config.LogLevel, "HTTP Request", fields...)

		return nil
	}
}

// createResponseMiddleware 创建响应中间件
func (c Context) createResponseMiddleware(config *HTTPLogConfig) resty.ResponseMiddleware {
	return func(client *resty.Client, resp *resty.Response) error {
		duration := resp.Duration()

		// 如果启用了 trace 记录，添加到 trace 中
		if config.EnableTraceLog {
			thirdPartyReq := &ThirdPartyRequest{
				Time:        time.Now().Format(consts.TimeFormat),
				Stack:       c.getCallerStack(),
				Method:      resp.Request.Method,
				URL:         redactHTTPLogURL(resp.Request.URL),
				StatusCode:  resp.StatusCode(),
				StatusMsg:   resp.Status(),
				CostSeconds: duration.Seconds(),
			}

			// 记录请求体
			if config.LogBody && resp.Request.Body != nil {
				thirdPartyReq.RequestBody = c.formatRequestBody(resp.Request.Body, config.MaxBodySize)
			}

			// 记录响应体
			if config.LogBody && resp.Size() > 0 {
				thirdPartyReq.ResponseBody = c.formatResponseBody(resp.Bytes(), config.MaxBodySize)
			}

			// 添加到 trace 中
			c.AppendThirdPartyRequest(thirdPartyReq)
		}

		// 如果启用了响应日志，输出日志
		if config.EnableResponseLog {
			fields := []zap.Field{
				zap.String("method", resp.Request.Method),
				zap.String("url", redactHTTPLogURL(resp.Request.URL)),
				zap.Int("status_code", resp.StatusCode()),
				zap.String("status", resp.Status()),
				zap.Duration("duration", duration),
				zap.Int64("response_size", resp.Size()),
			}

			// 记录响应头
			if config.LogHeaders && resp.Header() != nil {
				headers := c.sanitizeHeaders(resp.Header(), config.SensitiveHeaders)
				fields = append(fields, zap.Any("response_headers", headers))
			}

			// 记录响应体
			if config.LogBody && resp.Size() > 0 {
				bodyStr := c.formatResponseBody(resp.Bytes(), config.MaxBodySize)
				if bodyStr != "" {
					fields = append(fields, zap.String("response_body", bodyStr))
				}
			}

			// 根据状态码决定日志级别
			logLevel := config.LogLevel
			if resp.StatusCode() >= 400 {
				logLevel = "error"
			} else if resp.StatusCode() >= 300 {
				logLevel = "warn"
			}

			c.logWithLevel(logLevel, "HTTP Response", fields...)
		}

		return nil
	}
}

// createErrorHook 创建错误钩子
func (c Context) createErrorHook(config *HTTPLogConfig) resty.ErrorHook {
	return func(req *resty.Request, err error) {
		fields := []zap.Field{
			zap.String("method", req.Method),
			zap.String("url", redactHTTPLogURL(req.URL)),
			zap.String("error", redactHTTPLogError(req.URL, err)),
		}

		c.logWithLevel("error", "HTTP Request Error", fields...)
	}
}

func redactHTTPLogURL(rawURL string) string {
	return utils.RedactURLForLog(rawURL)
}

func redactHTTPLogError(rawURL string, err error) string {
	if err == nil {
		return ""
	}

	message := err.Error()
	if rawURL != "" {
		message = strings.ReplaceAll(message, rawURL, redactHTTPLogURL(rawURL))
	}

	return sanitizeHTTPLogText(message)
}

func sanitizeHTTPLogText(text string) string {
	if text == "" {
		return ""
	}

	text = httpLogURLPattern.ReplaceAllStringFunc(text, utils.RedactURLForLog)

	return utils.RedactSensitiveText(text)
}

// sanitizeHeaders 脱敏敏感请求头
func (c Context) sanitizeHeaders(headers http.Header, sensitiveHeaders []string) map[string][]string {
	sanitized := make(map[string][]string)

	for key, values := range headers {
		lowerKey := strings.ToLower(key)
		isSensitive := false

		for _, sensitive := range sensitiveHeaders {
			if strings.ToLower(sensitive) == lowerKey {
				isSensitive = true

				break
			}
		}

		if isSensitive || utils.IsSensitiveLogKey(key) {
			sanitized[key] = []string{utils.RedactedSecret}
		} else {
			redactedValues := make([]string, 0, len(values))
			for _, value := range values {
				redactedValues = append(redactedValues, sanitizeHTTPLogText(value))
			}

			sanitized[key] = redactedValues
		}
	}

	return sanitized
}

// formatRequestBody 格式化请求体
func (c Context) formatRequestBody(body interface{}, maxSize int) string {
	if body == nil {
		return ""
	}

	var bodyStr string

	switch v := body.(type) {
	case string:
		if isHTTPLogBodyOverLimit(len(v), maxSize) {
			return oversizeHTTPLogBodyMessage + truncatedHTTPLogBodySuffix
		}

		bodyStr = v
	case []byte:
		if isHTTPLogBodyOverLimit(len(v), maxSize) {
			return oversizeHTTPLogBodyMessage + truncatedHTTPLogBodySuffix
		}

		bodyStr = string(v)
	case io.Reader:
		// 对于 io.Reader，我们不读取内容以避免消耗流
		bodyStr = "[Reader - content not logged]"
	default:
		bodyStr = fmt.Sprintf("%v", v)
	}

	redactedBody := sanitizeHTTPLogText(bodyStr)
	if maxSize > 0 && len(redactedBody) > maxSize {
		return redactedBody[:maxSize] + truncatedHTTPLogBodySuffix
	}

	return redactedBody
}

// formatResponseBody 格式化响应体
func (c Context) formatResponseBody(body []byte, maxSize int) string {
	if len(body) == 0 {
		return ""
	}

	if isHTTPLogBodyOverLimit(len(body), maxSize) {
		return oversizeHTTPLogBodyMessage + truncatedHTTPLogBodySuffix
	}

	redactedBody := sanitizeHTTPLogText(string(body))
	if maxSize > 0 && len(redactedBody) > maxSize {
		return redactedBody[:maxSize] + truncatedHTTPLogBodySuffix
	}

	return redactedBody
}

func isHTTPLogBodyOverLimit(size int, maxSize int) bool {
	return maxSize > 0 && size > maxSize
}

// getCallerStack 获取调用栈信息
func (c Context) getCallerStack() string {
	_, file, line, ok := runtime.Caller(4) // 跳过中间件调用层级
	if !ok {
		return "unknown"
	}

	return fmt.Sprintf("%s:%d", file, line)
}

// logWithLevel 根据级别记录日志
func (c Context) logWithLevel(level string, msg string, fields ...zap.Field) {
	switch strings.ToLower(level) {
	case "debug":
		c.Debug(msg, fields...)
	case "info":
		c.Info(msg, fields...)
	case "warn":
		c.Warn(msg, fields...)
	case "error":
		c.Error(msg, fields...)
	default:
		c.Info(msg, fields...)
	}
}
