package telegram

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
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

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"ok":true,"result":{}}`))
		}))
		defer server.Close()

		svc := NewService("test-token", "123456", "", "", "", logger).(*service)
		svc.apiURL = strings.TrimSuffix(server.URL, "/bottest-token/sendMessage")
		svc.apiURL = server.URL

		// 手动设置 apiURL
		svc.apiURL = server.URL

		err := svc.SendMessage("测试消息")
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
			w.Write([]byte(`{"ok":false,"description":"Bad Request"}`))
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
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"ok":true,"result":{}}`))
		}))
		defer server.Close()

		svc := NewService("test-token", "123456", "", "", "", logger).(*service)
		svc.apiURL = server.URL

		err := svc.SendNotification("测试标题", "测试内容")
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
			w.Write([]byte(`{"ok":true,"result":{}}`))
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
		assert.NotNil(t, svc.client.Transport)
	})

	t.Run("SOCKS5代理正确设置", func(t *testing.T) {
		svc := NewService("test-token", "123456", "socks5://127.0.0.1:1080", "socks5", "", logger).(*service)
		assert.NotNil(t, svc.client.Transport)
	})

	t.Run("无效代理URL不崩溃", func(t *testing.T) {
		svc := NewService("test-token", "123456", "://invalid", "", "", logger)
		assert.NotNil(t, svc)
	})
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
			w.Write([]byte(`{"ok":true}`))
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
}

func TestConcurrentOperations(t *testing.T) {
	logger := zap.NewNop()

	t.Run("并发发送消息", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"ok":true}`))
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
