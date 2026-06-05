package telegram

import (
	stdCtx "context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"

	"github.com/xxcheng123/cloudpan189-share/internal/pkgs/utils"
)

func TestNewService(t *testing.T) {
	logger := zap.NewNop()

	t.Run("创建服务不带代理", func(t *testing.T) {
		svc := NewService("test-token", "123456", "", "", "", logger)
		assert.NotNil(t, svc)
		assert.True(t, svc.IsEnabled())
		assert.Equal(t, "test-token", svc.GetBotToken())
	})

	t.Run("创建服务带HTTP代理", func(t *testing.T) {
		svc := NewService("test-token", "123456", "http://127.0.0.1:7890", "http", "", logger)
		assert.NotNil(t, svc)
		assert.Equal(t, "http://127.0.0.1:7890", svc.GetProxyURL())
	})

	t.Run("创建服务带SOCKS5代理", func(t *testing.T) {
		svc := NewService("test-token", "123456", "socks5://127.0.0.1:1080", "socks5", "", logger)
		assert.NotNil(t, svc)
		assert.Equal(t, "socks5://127.0.0.1:1080", svc.GetProxyURL())
	})

	t.Run("空Token创建禁用服务", func(t *testing.T) {
		svc := NewService("", "", "", "", "", logger)
		assert.NotNil(t, svc)
		assert.False(t, svc.IsEnabled())
	})
}

func TestSendMessage(t *testing.T) {
	logger := zap.NewNop()

	t.Run("成功发送消息", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// 验证请求
			assert.Equal(t, "POST", r.Method)
			assert.Contains(t, r.URL.Path, "/bottest-token/sendMessage")

			var payload telegramMessage
			require.NoError(t, json.NewDecoder(r.Body).Decode(&payload))
			assert.Equal(t, "测试 <b>消息</b> & 文本", payload.Text)
			assert.Empty(t, payload.ParseMode)

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"ok":true,"result":{}}`))
		}))
		defer server.Close()

		svc := NewService("test-token", "123456", "", "", "", logger).(*service)
		svc.apiURL = strings.TrimSuffix(server.URL, "/bottest-token/sendMessage")
		svc.apiURL = server.URL

		// 手动设置 apiURL
		svc.apiURL = server.URL

		err := svc.SendMessage("测试 <b>消息</b> & 文本")
		assert.NoError(t, err)
	})

	t.Run("服务禁用时不发送消息", func(t *testing.T) {
		svc := NewService("", "", "", "", "", logger)
		err := svc.SendMessage("测试消息")
		assert.NoError(t, err) // 不应该报错，只是不发送
	})

	t.Run("ChatID为空时不发送消息", func(t *testing.T) {
		svc := NewService("test-token", "", "", "", "", logger)
		err := svc.SendMessage("测试消息")
		assert.NoError(t, err)
	})

	t.Run("API返回错误", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"ok":false,"description":"Bad Request"}`))
		}))
		defer server.Close()

		svc := NewService("test-token", "123456", "", "", "", logger).(*service)
		svc.apiURL = server.URL

		err := svc.SendMessage("测试消息")
		assert.Error(t, err)
	})
}

func TestSendNotification(t *testing.T) {
	logger := zap.NewNop()

	t.Run("成功发送通知", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var payload telegramMessage
			require.NoError(t, json.NewDecoder(r.Body).Decode(&payload))
			assert.Equal(t, "HTML", payload.ParseMode)
			assert.Equal(t, "<b>测试&lt;标题&gt;&amp;</b>\n\n测试&lt;内容&gt;&amp;", payload.Text)

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"ok":true,"result":{}}`))
		}))
		defer server.Close()

		svc := NewService("test-token", "123456", "", "", "", logger).(*service)
		svc.apiURL = server.URL

		err := svc.SendNotification("测试<标题>&", "测试<内容>&")
		assert.NoError(t, err)
	})
}

func TestTestConnection(t *testing.T) {
	logger := zap.NewNop()

	t.Run("连接测试成功", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Contains(t, r.URL.Path, "/sendMessage")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"ok":true,"result":{}}`))
		}))
		defer server.Close()

		svc := NewService("test-token", "123456", "", "", "", logger).(*service)
		svc.apiURL = server.URL

		err := svc.TestConnection()
		assert.NoError(t, err)
	})

	t.Run("服务禁用时测试失败", func(t *testing.T) {
		svc := NewService("", "", "", "", "", logger)
		err := svc.TestConnection()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not enabled")
	})
}

func TestProxySetup(t *testing.T) {
	logger := zap.NewNop()

	t.Run("HTTP代理正确设置", func(t *testing.T) {
		svc := NewService("test-token", "123456", "http://127.0.0.1:8080", "http", "", logger).(*service)
		transport, ok := svc.client.Transport.(*http.Transport)
		require.True(t, ok)
		require.NotNil(t, transport.Proxy)
		assertProxyTransportClonesDefault(t, transport)
	})

	t.Run("SOCKS5代理正确设置", func(t *testing.T) {
		svc := NewService("test-token", "123456", "socks5://127.0.0.1:1080", "socks5", "", logger).(*service)
		transport, ok := svc.client.Transport.(*http.Transport)
		require.True(t, ok)
		require.NotNil(t, transport.Proxy)
		assertProxyTransportClonesDefault(t, transport)
	})

	t.Run("无效代理URL不崩溃", func(t *testing.T) {
		svc := NewService("test-token", "123456", "://invalid", "", "", logger)
		assert.NotNil(t, svc)
	})

	t.Run("代理日志不泄露凭据", func(t *testing.T) {
		core, logs := observer.New(zap.InfoLevel)
		logger := zap.New(core)

		svc := NewService("test-token", "123456", "http://proxy-user:proxy-pass@127.0.0.1:8080", "http", "", logger)
		assert.NotNil(t, svc)

		entries := logs.FilterMessage("Telegram proxy configured").All()
		require.Len(t, entries, 1)

		proxyLog, ok := entries[0].ContextMap()["proxy"].(string)
		require.True(t, ok)

		logText, err := url.QueryUnescape(entries[0].Message + proxyLog)
		require.NoError(t, err)
		assert.NotContains(t, logText, "proxy-user")
		assert.NotContains(t, logText, "proxy-pass")
		assert.Contains(t, logText, "[REDACTED]")
	})

	t.Run("无效代理URL日志不泄露凭据", func(t *testing.T) {
		core, logs := observer.New(zap.WarnLevel)
		logger := zap.New(core)

		svc := NewService("test-token", "123456", "http://proxy-user:proxy-pass@[::1", "http", "", logger)
		assert.NotNil(t, svc)

		entries := logs.FilterMessage("Failed to parse proxy URL").All()
		require.Len(t, entries, 1)

		fields := entries[0].ContextMap()
		proxyLog, ok := fields["proxy"].(string)
		require.True(t, ok)
		errorLog, ok := fields["error"].(string)
		require.True(t, ok)

		logText, err := url.QueryUnescape(entries[0].Message + proxyLog + errorLog)
		require.NoError(t, err)

		assert.NotContains(t, logText, "proxy-user")
		assert.NotContains(t, logText, "proxy-pass")
		assert.Contains(t, logText, utils.RedactedSecret)
	})
}

func TestFetchUpdatesRedactsTelegramAPIErrorResultInLogs(t *testing.T) {
	core, logs := observer.New(zap.ErrorLevel)
	logger := zap.New(core)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":false,"description":"request failed: /botrequest-token/getUpdates?access_token=query-secret#token=fragment-secret accessCode=abcd via https://proxy-user:proxy-pass@example.test/path","parameters":{"botToken":"nested-token","retry_after":1}}`))
	}))
	defer server.Close()

	svc := NewService("request-token", "chat", "", "", "", logger).(*service)
	svc.apiURL = server.URL

	svc.fetchUpdates(stdCtx.Background())

	entries := logs.FilterMessage("Telegram API returned error").All()
	require.Len(t, entries, 1)

	loggedResult, ok := entries[0].ContextMap()["result"].(string)
	require.True(t, ok)

	for _, leaked := range []string{"request-token", "query-secret", "fragment-secret", "abcd", "proxy-user", "proxy-pass", "nested-token"} {
		assert.NotContains(t, loggedResult, leaked)
	}

	assert.Contains(t, loggedResult, "/getUpdates")
	assert.Contains(t, loggedResult, utils.RedactedSecret)
}

func TestFetchUpdatesRedactsRequestErrorInLogs(t *testing.T) {
	core, logs := observer.New(zap.ErrorLevel)
	logger := zap.New(core)

	svc := NewService("request-token", "chat", "", "", "", logger).(*service)
	svc.client = &http.Client{Transport: failingTelegramRoundTripper{}}

	svc.fetchUpdates(stdCtx.Background())

	entries := logs.FilterMessage("Failed to fetch updates").All()
	require.Len(t, entries, 1)

	loggedError, ok := entries[0].ContextMap()["error"].(string)
	require.True(t, ok)

	assert.NotContains(t, loggedError, "request-token")
	assert.NotContains(t, loggedError, "abcd")
	assert.Contains(t, loggedError, "/getUpdates")
	assert.Contains(t, loggedError, utils.RedactedSecret)
}

func TestDoRequestWithDataRedactsReturnedRequestError(t *testing.T) {
	logger := zap.NewNop()
	svc := NewService("request-token", "chat", "", "", "", logger).(*service)
	svc.client = &http.Client{Transport: failingTelegramRoundTripper{}}

	result, err := svc.doRequestWithData("sendMessage", telegramMessage{
		ChatID: "chat",
		Text:   "test",
	})
	assert.Error(t, err)
	assert.Nil(t, result)

	errText := err.Error()
	assert.NotContains(t, errText, "request-token")
	assert.NotContains(t, errText, "abcd")
	assert.Contains(t, errText, "/sendMessage")
	assert.Contains(t, errText, utils.RedactedSecret)
}

func TestHandleMessageRedactsIncomingMessageLog(t *testing.T) {
	core, logs := observer.New(zap.InfoLevel)
	logger := zap.New(core)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true,"result":{}}`))
	}))
	defer server.Close()

	svc := NewService("request-token", "chat", "", "", "", logger).(*service)
	svc.apiURL = server.URL

	rawText := `/unknown HTTPS://WWW.CLOUD.189.CN/t/abcDEF?filename=private-name.mkv#token=fragment-secret shareCode:qWe123 accessCode=wxyz Authorization: Bearer bearer-secret`
	svc.handleMessage(stdCtx.Background(), map[string]interface{}{
		"text": rawText,
		"from": map[string]interface{}{
			"id":         float64(1001),
			"username":   "telegram-user",
			"first_name": "Private Name",
		},
		"chat": map[string]interface{}{
			"id": float64(2002),
		},
	})

	entries := logs.FilterMessage("Received message").All()
	require.Len(t, entries, 1)

	logText := entries[0].Message + fmt.Sprint(entries[0].Context)
	for _, leaked := range []string{
		"telegram-user",
		"Private Name",
		"abcDEF",
		"qWe123",
		"private-name.mkv",
		"fragment-secret",
		"wxyz",
		"bearer-secret",
	} {
		assert.NotContains(t, logText, leaked)
	}

	assert.Contains(t, logText, utils.MaskSecret("telegram-user"))
	assert.Contains(t, logText, utils.MaskSecret("Private Name"))
	assert.Contains(t, logText, utils.RedactedSecret)
}

type failingTelegramRoundTripper struct{}

func (failingTelegramRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	return nil, fmt.Errorf("request failed: %s accessCode=abcd", req.URL.String())
}

func TestNewServiceWithNilLoggerDoesNotPanic(t *testing.T) {
	svc := NewService("test-token", "123456", "://invalid", "", "", nil)
	assert.NotNil(t, svc)
	assert.True(t, svc.IsEnabled())
}

func TestBuildURL(t *testing.T) {
	logger := zap.NewNop()
	svc := NewService("test-token", "123456", "", "", "", logger).(*service)

	url := svc.buildURL("sendMessage")
	assert.Contains(t, url, "test-token")
	assert.Contains(t, url, "sendMessage")
}

func TestDoRequestWithRetry(t *testing.T) {
	logger := zap.NewNop()

	t.Run("第一次成功", func(t *testing.T) {
		callCount := 0

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			callCount++

			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"ok":true}`))
		}))
		defer server.Close()

		svc := NewService("test-token", "123456", "", "", "", logger).(*service)
		svc.apiURL = server.URL

		err := svc.SendMessage("test")
		assert.NoError(t, err)
		assert.Equal(t, 1, callCount) // 只调用一次
	})

	t.Run("重试3次后失败", func(t *testing.T) {
		callCount := 0

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			callCount++

			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer server.Close()

		svc := NewService("test-token", "123456", "", "", "", logger).(*service)
		svc.apiURL = server.URL

		err := svc.SendMessage("test")
		assert.Error(t, err)
		// SendMessage 内部调用 doRequestWithRetry，最多重试3次
	})
}

func TestDoRequestWithDataRejectsOversizedResponse(t *testing.T) {
	logger := zap.NewNop()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(strings.Repeat("x", maxTelegramResponseSize+1)))
	}))
	defer server.Close()

	svc := NewService("test-token", "123456", "", "", "", logger).(*service)
	svc.apiURL = server.URL

	result, err := svc.doRequestWithData("sendMessage", map[string]interface{}{
		"chat_id": "123456",
		"text":    "test",
	})
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "响应体过大")
}

func TestIsEnabled(t *testing.T) {
	logger := zap.NewNop()

	t.Run("有Token时启用", func(t *testing.T) {
		svc := NewService("token", "chat", "", "", "", logger)
		assert.True(t, svc.IsEnabled())
	})

	t.Run("无Token时禁用", func(t *testing.T) {
		svc := NewService("", "chat", "", "", "", logger)
		assert.False(t, svc.IsEnabled())
	})
}

func TestGetBotToken(t *testing.T) {
	logger := zap.NewNop()
	svc := NewService("my-token", "chat", "", "", "", logger)
	assert.Equal(t, "my-token", svc.GetBotToken())
}

func TestGetProxyURL(t *testing.T) {
	logger := zap.NewNop()

	t.Run("有代理", func(t *testing.T) {
		svc := NewService("token", "chat", "http://proxy:8080", "http", "", logger)
		assert.Equal(t, "http://proxy:8080", svc.GetProxyURL())
	})

	t.Run("无代理", func(t *testing.T) {
		svc := NewService("token", "chat", "", "", "", logger)
		assert.Equal(t, "", svc.GetProxyURL())
	})
}

func TestStopBot(t *testing.T) {
	logger := zap.NewNop()
	svc := NewService("token", "chat", "", "", "", logger)

	// StopBot 不应该 panic
	svc.StopBot()
	svc.StopBot()
}

func TestNewServiceWithConfigHonorsEnableFlag(t *testing.T) {
	logger := zap.NewNop()
	svc := NewServiceWithConfig(Config{
		BotToken: "token",
		ChatID:   "chat",
		Enable:   false,
	}, logger)

	assert.Equal(t, "token", svc.GetBotToken())
	assert.False(t, svc.IsEnabled())
}

func TestUpdateConfigSynchronizesRuntimeSettings(t *testing.T) {
	logger := zap.NewNop()
	svc := NewServiceWithConfig(Config{
		BotToken: "old-token",
		ChatID:   "old-chat",
		Enable:   true,
	}, logger)

	err := svc.UpdateConfig(Config{
		BotToken:  "new-token",
		ChatID:    "new-chat",
		ProxyURL:  "http://127.0.0.1:7890",
		ProxyType: "http",
		Enable:    false,
	})
	assert.NoError(t, err)
	assert.Equal(t, "new-token", svc.GetBotToken())
	assert.Equal(t, "http://127.0.0.1:7890", svc.GetProxyURL())
	assert.False(t, svc.IsEnabled())
}

func TestUpdateConfigResetsOffsetWhenBotTokenChanges(t *testing.T) {
	logger := zap.NewNop()
	svc := NewServiceWithConfig(Config{
		BotToken: "old-token",
		ChatID:   "chat",
		Enable:   true,
	}, logger).(*service)
	svc.lastOffset = 991

	err := svc.UpdateConfig(Config{
		BotToken: "new-token",
		ChatID:   "chat",
		Enable:   false,
	})
	assert.NoError(t, err)
	assert.Equal(t, int64(0), svc.lastOffset)
}

func TestUpdateConfigKeepsOffsetWhenBotTokenUnchanged(t *testing.T) {
	logger := zap.NewNop()
	svc := NewServiceWithConfig(Config{
		BotToken: "token",
		ChatID:   "chat",
		Enable:   true,
	}, logger).(*service)
	svc.lastOffset = 991

	err := svc.UpdateConfig(Config{
		BotToken: "token",
		ChatID:   "new-chat",
		Enable:   false,
	})
	assert.NoError(t, err)
	assert.Equal(t, int64(991), svc.lastOffset)
}

func TestUpdateConfigWaitsForOldPollingBeforeResettingOffset(t *testing.T) {
	logger := zap.NewNop()

	requestSeen := make(chan struct{})
	releaseResponse := make(chan struct{})

	var requestCount int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "/botold-token/getUpdates") {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"ok":true,"result":[]}`))

			return
		}

		if atomic.AddInt32(&requestCount, 1) == 1 {
			close(requestSeen)
		}

		select {
		case <-releaseResponse:
		case <-r.Context().Done():
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true,"result":[{"update_id":999,"message":{"text":"/start","from":{"id":1},"chat":{"id":2}}}]}`))
	}))
	defer server.Close()

	svc := NewServiceWithConfig(Config{
		BotToken: "old-token",
		ChatID:   "chat",
		APIURL:   server.URL,
		Enable:   true,
	}, logger).(*service)

	botCtx, cancel := stdCtx.WithCancel(svc.ctx)
	svc.mu.Lock()
	svc.running = true
	svc.botCancel = cancel
	svc.mu.Unlock()

	svc.wg.Add(1)
	go func() {
		defer svc.wg.Done()

		svc.fetchUpdates(botCtx)
	}()

	select {
	case <-requestSeen:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for old polling request")
	}

	done := make(chan error, 1)
	go func() {
		done <- svc.UpdateConfig(Config{
			BotToken: "new-token",
			ChatID:   "chat",
			APIURL:   server.URL,
			Enable:   false,
		})
	}()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("update config: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for config update")
	}

	close(releaseResponse)

	svc.mu.RLock()
	lastOffset := svc.lastOffset
	running := svc.running
	svc.mu.RUnlock()

	if running {
		t.Fatal("expected bot polling to be stopped")
	}

	if lastOffset != 0 {
		t.Fatalf("expected offset reset to survive stale polling response, got %d", lastOffset)
	}
}

func assertProxyTransportClonesDefault(t *testing.T, transport *http.Transport) {
	t.Helper()

	defaultTransport, ok := http.DefaultTransport.(*http.Transport)
	require.True(t, ok)

	if transport == defaultTransport {
		t.Fatal("expected proxy transport to clone default transport")
	}

	assert.Equal(t, defaultTransport.MaxIdleConns, transport.MaxIdleConns)
	assert.Equal(t, defaultTransport.IdleConnTimeout, transport.IdleConnTimeout)
	assert.Equal(t, defaultTransport.TLSHandshakeTimeout, transport.TLSHandshakeTimeout)
	assert.Equal(t, defaultTransport.ExpectContinueTimeout, transport.ExpectContinueTimeout)
	assert.Equal(t, defaultTransport.ForceAttemptHTTP2, transport.ForceAttemptHTTP2)
}

func TestSocks5ProxyUsesConfigSnapshot(t *testing.T) {
	logger := zap.NewNop()
	svc := NewServiceWithConfig(Config{
		BotToken:  "token",
		ChatID:    "chat",
		ProxyURL:  "socks5://127.0.0.1:1080",
		ProxyType: "socks5",
		Enable:    true,
	}, logger).(*service)

	transport, ok := svc.GetClient().Transport.(*http.Transport)
	assert.True(t, ok)
	assert.NotNil(t, transport.Proxy)

	err := svc.UpdateConfig(Config{
		BotToken:  "token",
		ChatID:    "chat",
		ProxyURL:  "socks5://127.0.0.1:2080",
		ProxyType: "socks5",
		Enable:    false,
	})
	assert.NoError(t, err)

	proxyURL, err := transport.Proxy(httptest.NewRequest(http.MethodGet, "https://example.com", nil))
	assert.NoError(t, err)
	assert.Equal(t, "socks5://127.0.0.1:1080", proxyURL.String())
}

func TestUpdateConfigStartsAndStopsPolling(t *testing.T) {
	logger := zap.NewNop()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true,"result":[]}`))
	}))
	defer server.Close()

	svc := NewServiceWithConfig(Config{
		BotToken: "token",
		ChatID:   "chat",
		APIURL:   server.URL,
		Enable:   false,
	}, logger).(*service)

	err := svc.UpdateConfig(Config{
		BotToken: "token",
		ChatID:   "chat",
		APIURL:   server.URL,
		Enable:   true,
	})
	assert.NoError(t, err)

	svc.mu.RLock()
	running := svc.running
	svc.mu.RUnlock()
	assert.True(t, running)

	err = svc.UpdateConfig(Config{
		BotToken: "token",
		ChatID:   "chat",
		APIURL:   server.URL,
		Enable:   false,
	})
	assert.NoError(t, err)

	svc.mu.RLock()
	running = svc.running
	svc.mu.RUnlock()
	assert.False(t, running)
	assert.False(t, svc.IsEnabled())
}

func TestParseBatchAddMountResultHandlesJSONDecodedNumberCode(t *testing.T) {
	mountID, errMsg, ok := parseBatchAddMountResult(map[string]interface{}{
		"code": float64(200),
		"data": map[string]interface{}{
			"results": []interface{}{
				map[string]interface{}{
					"success": true,
					"id":      float64(42),
				},
			},
		},
	})

	assert.True(t, ok)
	assert.Empty(t, errMsg)
	assert.Equal(t, int64(42), mountID)
}

func TestParseBatchAddMountResultRedactsFailureMessage(t *testing.T) {
	rawError := "request failed: https://proxy-user:proxy-pass@storage.example.test/path?access_token=query-secret&filename=private-name.mkv#token=fragment-secret accessCode=abcd Authorization: Bearer bearer-secret"

	for name, batchResult := range map[string]map[string]interface{}{
		"top-level msg": {
			"code": 500,
			"msg":  rawError,
		},
		"item error": {
			"code": 200,
			"data": map[string]interface{}{
				"results": []interface{}{
					map[string]interface{}{
						"success": false,
						"error":   rawError,
					},
				},
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			mountID, errMsg, ok := parseBatchAddMountResult(batchResult)

			assert.False(t, ok)
			assert.Zero(t, mountID)
			assertTelegramMountMessageRedacted(t, errMsg)
		})
	}
}

func TestParseAndMountShareLinkHandlesMalformedBatchAddResponse(t *testing.T) {
	logger := zap.NewNop()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch r.URL.Path {
		case "/api/public/share_info":
			_, _ = w.Write([]byte(`{"data":{"name":"Shared","shareId":123,"id":"file-id"}}`))
		case "/api/storage/batch_add":
			_, _ = w.Write([]byte(`{"code":200,"data":null}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	svc := NewService("token", "chat", "", "", "", logger).(*service)
	svc.backendURL = server.URL

	result, err := svc.ParseAndMountShareLink("https://cloud.189.cn/t/abcDEF", "", true)
	assert.NoError(t, err)
	assert.False(t, result.Success)
	assert.Contains(t, result.Message, "响应结构异常")
}

func TestParseAndMountShareLinkParsesWWWUppercaseLinkWithAccessCodeDirect(t *testing.T) {
	logger := zap.NewNop()
	fetcher := &telegramShareFetcherStub{
		info: &ShareInfo{Name: "Shared", ShareId: 123, FileId: "file-id", IsFolder: true},
	}
	mounter := &telegramStorageMounterStub{}
	svc := NewService("token", "chat", "", "", "", logger).(*service)
	svc.SetMountDependencies(fetcher, mounter)

	result, err := svc.ParseAndMountShareLink("HTTPS://WWW.CLOUD.189.CN/t/AbC123?shareId=1 提取码：wxyz", "", true)
	require.NoError(t, err)
	require.True(t, result.Success)
	assert.Equal(t, []string{"AbC123"}, fetcher.shareCodes)
	assert.Equal(t, []string{"wxyz"}, fetcher.accessCodes)
	require.Len(t, mounter.requests, 1)
	assert.Equal(t, "AbC123", mounter.requests[0].ShareCode)
	assert.Equal(t, "wxyz", mounter.requests[0].AccessCode)
}

func TestParseAndMountShareLinkWithContextDirectUsesCallerContext(t *testing.T) {
	logger := zap.NewNop()
	fetcher := &telegramShareFetcherStub{
		info: &ShareInfo{Name: "Shared", ShareId: 123, FileId: "file-id", IsFolder: true},
	}
	mounter := &telegramStorageMounterStub{}
	svc := NewService("token", "chat", "", "", "", logger).(*service)
	svc.SetMountDependencies(fetcher, mounter)

	ctx := stdCtx.WithValue(stdCtx.Background(), telegramContextMarkerKey{}, "direct-marker")
	result, err := svc.ParseAndMountShareLinkWithContext(ctx, "https://cloud.189.cn/t/abcDEF", "", true)
	require.NoError(t, err)
	require.True(t, result.Success)
	assert.Equal(t, []string{"direct-marker"}, fetcher.contextValues)
	assert.Equal(t, []string{"direct-marker"}, mounter.contextValues)
}

func TestParseAndMountShareLinkRejectsMalformedCloudShareLinkBeforeLookup(t *testing.T) {
	logger := zap.NewNop()

	for _, shareURL := range []string{
		"https://cloud.189.cn/t/?token=secret-token",
		"https://cloud.189.cn/t/!!!?token=secret-token",
		"note https://cloud.189.cn.evil.test/t/abcDEF",
		"note https://evil-cloud.189.cn/t/xyZ123",
		"note https://evilcloud.189.cn/t/qWe123",
	} {
		t.Run(shareURL, func(t *testing.T) {
			fetcher := &telegramShareFetcherStub{}
			mounter := &telegramStorageMounterStub{}
			svc := NewService("token", "chat", "", "", "", logger).(*service)
			svc.SetMountDependencies(fetcher, mounter)

			result, err := svc.ParseAndMountShareLink(shareURL, "", true)
			require.NoError(t, err)
			require.NotNil(t, result)
			assert.False(t, result.Success)
			assert.Contains(t, result.Message, "无法识别分享链接")
			assert.NotContains(t, result.Message, "secret-token")
			assert.Empty(t, fetcher.shareCodes)
			assert.Empty(t, fetcher.accessCodes)
			assert.Empty(t, mounter.requests)
		})
	}
}

func TestParseAndMountShareLinkRejectsMalformedAccessCodeBeforeLookup(t *testing.T) {
	logger := zap.NewNop()

	for _, shareURL := range []string{
		"https://cloud.189.cn/t/abcDEF 提取码：bad-code",
		"https://cloud.189.cn/t/abcDEF 提取码：https://example.com/share?token=secret-token",
	} {
		t.Run(shareURL, func(t *testing.T) {
			fetcher := &telegramShareFetcherStub{}
			mounter := &telegramStorageMounterStub{}
			svc := NewService("token", "chat", "", "", "", logger).(*service)
			svc.SetMountDependencies(fetcher, mounter)

			result, err := svc.ParseAndMountShareLink(shareURL, "", true)
			require.NoError(t, err)
			require.NotNil(t, result)
			assert.False(t, result.Success)
			assert.Contains(t, result.Message, "无法识别分享链接")
			assert.NotContains(t, result.Message, "secret-token")
			assert.Empty(t, fetcher.shareCodes)
			assert.Empty(t, fetcher.accessCodes)
			assert.Empty(t, mounter.requests)
		})
	}
}

func TestHandleShareLinkDirectEscapesHTMLDynamicFields(t *testing.T) {
	logger := zap.NewNop()
	messages := make([]telegramMessage, 0)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload telegramMessage
		require.NoError(t, json.NewDecoder(r.Body).Decode(&payload))
		messages = append(messages, payload)

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true,"result":{}}`))
	}))
	defer server.Close()

	svc := NewService("token", "chat", "", "", "", logger).(*service)
	svc.apiURL = server.URL
	svc.SetMountDependencies(
		&telegramShareFetcherStub{
			info: &ShareInfo{
				Name:     `Shared <b>& "Movie"`,
				ShareId:  123,
				FileId:   `file<id>&`,
				IsFolder: true,
			},
		},
		&telegramStorageMounterStub{},
	)

	svc.handleShareLink("https://cloud.189.cn/t/abcDEF", 2002, 1001)

	require.Len(t, messages, 3)

	infoText := messages[1].Text
	assert.Equal(t, "HTML", messages[1].ParseMode)
	assert.Contains(t, infoText, "Shared &lt;b&gt;&amp; &#34;Movie&#34;")
	assert.Contains(t, infoText, "file&lt;id&gt;&amp;")
	assert.NotContains(t, infoText, `Shared <b>& "Movie"`)
	assert.NotContains(t, infoText, `file<id>&`)

	successText := messages[2].Text
	assert.Equal(t, "HTML", messages[2].ParseMode)
	assert.Contains(t, successText, "✅ <b>挂载成功！</b>")
	assert.Contains(t, successText, `/Telegram/Shared _b_&amp; _Movie_`)
	assert.NotContains(t, successText, `/Telegram/Shared <b>& "Movie"`)
}

func TestHandleShareLinkWithContextDirectUsesCallerContext(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true,"result":{}}`))
	}))
	defer server.Close()

	fetcher := &telegramShareFetcherStub{
		info: &ShareInfo{Name: "Shared", ShareId: 123, FileId: "file-id", IsFolder: true},
	}
	mounter := &telegramStorageMounterStub{}
	svc := NewService("token", "chat", "", "", "", zap.NewNop()).(*service)
	svc.apiURL = server.URL
	svc.SetMountDependencies(fetcher, mounter)

	ctx := stdCtx.WithValue(stdCtx.Background(), telegramContextMarkerKey{}, "bot-direct-marker")
	svc.handleShareLinkWithContext(ctx, "https://cloud.189.cn/t/abcDEF", 2002, 1001)

	assert.Equal(t, []string{"bot-direct-marker"}, fetcher.contextValues)
	assert.Equal(t, []string{"bot-direct-marker"}, mounter.contextValues)
}

func TestHandleShareLinkWithContextFallbackUsesCallerContext(t *testing.T) {
	transport := &telegramBotShareContextRoundTripper{}
	svc := NewService("token", "chat", "", "", "", zap.NewNop()).(*service)
	svc.backendURL = "http://backend.test"
	svc.client = &http.Client{Transport: transport}

	ctx := stdCtx.WithValue(stdCtx.Background(), telegramContextMarkerKey{}, "bot-fallback-marker")
	svc.handleShareLinkWithContext(ctx, "https://cloud.189.cn/t/abcDEF", 2002, 1001)

	assert.Equal(t, []string{"bot-fallback-marker", "bot-fallback-marker"}, transport.backendContextValues)
	assert.Equal(t, []string{"/api/storage/advance/share_info", "/api/storage/batch_add"}, transport.backendPaths)
}

func TestParseAndMountShareLinkDirectEscapesLiteralPercentMountPath(t *testing.T) {
	logger := zap.NewNop()
	fetcher := &telegramShareFetcherStub{
		info: &ShareInfo{Name: "a%2Fb", ShareId: 123, FileId: "file-id", IsFolder: true},
	}
	mounter := &telegramStorageMounterStub{}
	svc := NewService("token", "chat", "", "", "", logger).(*service)
	svc.SetMountDependencies(fetcher, mounter)

	result, err := svc.ParseAndMountShareLink("https://cloud.189.cn/t/abcDEF", "", true)
	require.NoError(t, err)
	require.True(t, result.Success)

	wantPath := "/Telegram/a%252Fb"
	assert.Equal(t, wantPath, result.MountPath)
	require.Len(t, mounter.requests, 1)
	assert.Equal(t, wantPath, mounter.requests[0].LocalPath)
}

func TestParseAndMountShareLinkDirectNormalizesExplicitMountPath(t *testing.T) {
	logger := zap.NewNop()
	fetcher := &telegramShareFetcherStub{
		info: &ShareInfo{Name: "Shared", ShareId: 123, FileId: "file-id", IsFolder: true},
	}
	mounter := &telegramStorageMounterStub{}
	svc := NewService("token", "chat", "", "", "", logger).(*service)
	svc.SetMountDependencies(fetcher, mounter)

	result, err := svc.ParseAndMountShareLink(
		"https://cloud.189.cn/t/abcDEF",
		"/Telegram//%E4%B8%AD%E6%96%87%20/a%3ab",
		true,
	)
	require.NoError(t, err)
	require.True(t, result.Success)

	wantPath := "/Telegram/中文/a_b"
	assert.Equal(t, wantPath, result.MountPath)
	require.Len(t, mounter.requests, 1)
	assert.Equal(t, wantPath, mounter.requests[0].LocalPath)
}

func TestParseAndMountShareLinkFallbackPassesAccessCode(t *testing.T) {
	logger := zap.NewNop()

	var (
		sawShareInfo bool
		sawBatchAdd  bool
	)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch r.URL.Path {
		case "/api/public/share_info":
			sawShareInfo = true

			assert.Equal(t, "AbC123", r.URL.Query().Get("shareCode"))
			assert.Equal(t, "wxyz", r.URL.Query().Get("shareAccessCode"))

			_, _ = w.Write([]byte(`{"data":{"name":"Shared","shareId":123,"id":"file-id"}}`))
		case "/api/storage/batch_add":
			sawBatchAdd = true

			var payload struct {
				Items []struct {
					ShareCode       string `json:"shareCode"`
					ShareAccessCode string `json:"shareAccessCode"`
				} `json:"items"`
			}

			require.NoError(t, json.NewDecoder(r.Body).Decode(&payload))
			require.Len(t, payload.Items, 1)
			assert.Equal(t, "AbC123", payload.Items[0].ShareCode)
			assert.Equal(t, "wxyz", payload.Items[0].ShareAccessCode)

			_, _ = w.Write([]byte(`{"code":200,"data":{"results":[{"success":true,"id":42}]}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	svc := NewService("token", "chat", "", "", "", logger).(*service)
	svc.backendURL = server.URL

	result, err := svc.ParseAndMountShareLink("HTTPS://WWW.CLOUD.189.CN/t/AbC123?shareId=1 提取码：wxyz", "", true)
	require.NoError(t, err)
	require.True(t, result.Success)
	assert.True(t, sawShareInfo)
	assert.True(t, sawBatchAdd)
}

func TestParseAndMountShareLinkWithContextFallbackUsesCallerContext(t *testing.T) {
	transport := &telegramContextCaptureRoundTripper{}
	svc := NewService("token", "chat", "", "", "", zap.NewNop()).(*service)
	svc.backendURL = "http://backend.test"
	svc.client = &http.Client{Transport: transport}

	ctx := stdCtx.WithValue(stdCtx.Background(), telegramContextMarkerKey{}, "fallback-marker")
	result, err := svc.ParseAndMountShareLinkWithContext(ctx, "https://cloud.189.cn/t/abcDEF", "", true)
	require.NoError(t, err)
	require.True(t, result.Success)
	assert.Equal(t, []string{"fallback-marker", "fallback-marker"}, transport.contextValues)
	assert.Equal(t, []string{"/api/public/share_info", "/api/storage/batch_add"}, transport.paths)
}

func TestParseAndMountShareLinkRedactsFallbackBatchAddFailureMessage(t *testing.T) {
	logger := zap.NewNop()
	rawError := "request failed: https://proxy-user:proxy-pass@storage.example.test/path?access_token=query-secret&filename=private-name.mkv#token=fragment-secret accessCode=abcd Authorization: Bearer bearer-secret"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch r.URL.Path {
		case "/api/public/share_info":
			_, _ = w.Write([]byte(`{"data":{"name":"Shared","shareId":123,"id":"file-id"}}`))
		case "/api/storage/batch_add":
			_, _ = w.Write([]byte(`{"code":500,"msg":"` + rawError + `"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	svc := NewService("token", "chat", "", "", "", logger).(*service)
	svc.backendURL = server.URL

	result, err := svc.ParseAndMountShareLink("https://cloud.189.cn/t/abcDEF", "", true)
	assert.NoError(t, err)
	assert.False(t, result.Success)
	assertTelegramMountMessageRedacted(t, result.Message)
}

func TestParseAndMountShareLinkRedactsDirectFailureMessages(t *testing.T) {
	logger := zap.NewNop()
	rawError := "request failed: https://proxy-user:proxy-pass@cloud.example.test/path?access_token=query-secret&filename=private-name.mkv#token=fragment-secret accessCode=abcd Authorization: Bearer bearer-secret"

	t.Run("share info error", func(t *testing.T) {
		svc := NewService("token", "chat", "", "", "", logger).(*service)
		svc.SetMountDependencies(&telegramShareFetcherStub{err: fmt.Errorf("%s", rawError)}, nil)

		result, err := svc.ParseAndMountShareLink("https://cloud.189.cn/t/abcDEF", "", false)
		assert.NoError(t, err)
		assert.False(t, result.Success)
		assertTelegramMountMessageRedacted(t, result.Message)
	})

	t.Run("mount error", func(t *testing.T) {
		svc := NewService("token", "chat", "", "", "", logger).(*service)
		svc.SetMountDependencies(
			&telegramShareFetcherStub{info: &ShareInfo{Name: "Shared", ShareId: 123, FileId: "file-id", IsFolder: true}},
			&telegramStorageMounterStub{err: fmt.Errorf("%s", rawError)},
		)

		result, err := svc.ParseAndMountShareLink("https://cloud.189.cn/t/abcDEF", "", true)
		assert.NoError(t, err)
		assert.False(t, result.Success)
		assertTelegramMountMessageRedacted(t, result.Message)
	})
}

func TestParseAndMountShareLinkRejectsOversizedShareInfoResponse(t *testing.T) {
	logger := zap.NewNop()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(strings.Repeat("x", maxTelegramResponseSize+1)))
	}))
	defer server.Close()

	svc := NewService("token", "chat", "", "", "", logger).(*service)
	svc.backendURL = server.URL

	result, err := svc.ParseAndMountShareLink("https://cloud.189.cn/t/abcDEF", "", false)
	assert.NoError(t, err)
	assert.False(t, result.Success)
	assert.Contains(t, result.Message, "响应体过大")
}

func assertTelegramMountMessageRedacted(t *testing.T, text string) {
	t.Helper()

	for _, leaked := range []string{"proxy-user", "proxy-pass", "query-secret", "private-name.mkv", "fragment-secret", "abcd", "bearer-secret"} {
		assert.NotContains(t, text, leaked)
	}

	assert.Contains(t, text, utils.RedactedSecret)
}

type telegramShareFetcherStub struct {
	info          *ShareInfo
	err           error
	shareCodes    []string
	accessCodes   []string
	contextValues []string
}

func (s *telegramShareFetcherStub) GetShareInfo(ctx stdCtx.Context, shareCode string, accessCode string) (*ShareInfo, error) {
	s.shareCodes = append(s.shareCodes, shareCode)
	s.accessCodes = append(s.accessCodes, accessCode)
	s.contextValues = append(s.contextValues, telegramContextValue(ctx))

	if s.err != nil {
		return nil, s.err
	}

	if s.info != nil {
		return s.info, nil
	}

	return &ShareInfo{Name: "Shared", ShareId: 123, FileId: "file-id", IsFolder: true}, nil
}

type telegramStorageMounterStub struct {
	err           error
	requests      []*MountRequest
	contextValues []string
}

func (s *telegramStorageMounterStub) CreateMountPoint(ctx stdCtx.Context, req *MountRequest) (int64, error) {
	s.requests = append(s.requests, req)
	s.contextValues = append(s.contextValues, telegramContextValue(ctx))

	if s.err != nil {
		return 0, s.err
	}

	return 42, nil
}

type telegramContextMarkerKey struct{}

func telegramContextValue(ctx stdCtx.Context) string {
	if ctx == nil {
		return ""
	}

	value, _ := ctx.Value(telegramContextMarkerKey{}).(string)

	return value
}

type telegramContextCaptureRoundTripper struct {
	contextValues []string
	paths         []string
}

func (rt *telegramContextCaptureRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	rt.contextValues = append(rt.contextValues, telegramContextValue(req.Context()))
	rt.paths = append(rt.paths, req.URL.Path)

	switch req.URL.Path {
	case "/api/public/share_info":
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(`{"data":{"name":"Shared","shareId":123,"id":"file-id"}}`)),
			Request:    req,
		}, nil
	case "/api/storage/batch_add":
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(`{"code":200,"data":{"results":[{"success":true,"id":42}]}}`)),
			Request:    req,
		}, nil
	default:
		return &http.Response{
			StatusCode: http.StatusNotFound,
			Body:       io.NopCloser(strings.NewReader(`{}`)),
			Request:    req,
		}, nil
	}
}

type telegramBotShareContextRoundTripper struct {
	backendContextValues []string
	backendPaths         []string
}

func (rt *telegramBotShareContextRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	if strings.Contains(req.URL.Path, "/sendMessage") {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(`{"ok":true,"result":{}}`)),
			Request:    req,
		}, nil
	}

	rt.backendContextValues = append(rt.backendContextValues, telegramContextValue(req.Context()))
	rt.backendPaths = append(rt.backendPaths, req.URL.Path)

	switch req.URL.Path {
	case "/api/storage/advance/share_info":
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(`{"data":{"name":"Shared","shareId":123,"id":"file-id"}}`)),
			Request:    req,
		}, nil
	case "/api/storage/batch_add":
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(`{"code":200,"data":{"results":[{"success":true,"id":42}]}}`)),
			Request:    req,
		}, nil
	default:
		return &http.Response{
			StatusCode: http.StatusNotFound,
			Body:       io.NopCloser(strings.NewReader(`{}`)),
			Request:    req,
		}, nil
	}
}

func TestConcurrentOperations(t *testing.T) {
	logger := zap.NewNop()

	t.Run("并发发送消息", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"ok":true}`))
		}))
		defer server.Close()

		svc := NewService("test-token", "123456", "", "", "", logger).(*service)
		svc.apiURL = server.URL

		// 并发发送
		done := make(chan bool, 10)

		for i := 0; i < 10; i++ {
			go func() {
				err := svc.SendMessage("test")
				assert.NoError(t, err)

				done <- true
			}()
		}

		// 等待所有完成
		for i := 0; i < 10; i++ {
			select {
			case <-done:
			case <-time.After(5 * time.Second):
				t.Fatal("Timeout waiting for concurrent operations")
			}
		}
	})
}
