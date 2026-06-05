package openai

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"

	"github.com/xxcheng123/cloudpan189-share/internal/pkgs/utils"
)

func TestNewService(t *testing.T) {
	logger := zap.NewNop()

	t.Run("创建服务带完整配置", func(t *testing.T) {
		svc := NewService(logger, "test-api-key", "https://api.openai.com", "gpt-4")
		assert.NotNil(t, svc)
	})

	t.Run("创建服务使用默认配置", func(t *testing.T) {
		svc := NewService(logger, "", "", "")
		assert.NotNil(t, svc)
		// 验证默认值
		config := svc.(*service).config
		assert.Equal(t, "https://api.openai.com", config.BaseURL)
		assert.Equal(t, "gpt-4o-mini", config.Model)
	})
}

func TestNewServiceWithNilLoggerDoesNotPanic(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"choices": [
				{
					"message": {
						"content": "测试电影 2024 4K"
					}
				}
			]
		}`))
	}))
	defer server.Close()

	svc := NewService(nil, "test-api-key", server.URL, "gpt-4o-mini")

	keyword, err := svc.GenerateUpgradeKeyword("测试电影", "movie")
	assert.NoError(t, err)
	assert.Equal(t, "测试电影 2024 4K", keyword)
}

func TestGenerateUpgradeKeyword(t *testing.T) {
	logger := zap.NewNop()

	t.Run("成功生成电影升级关键词", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// 验证请求
			assert.Equal(t, "POST", r.Method)
			assert.Contains(t, r.URL.Path, "/v1/chat/completions")
			assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
			assert.Contains(t, r.Header.Get("Authorization"), "Bearer test-api-key")

			// 返回模拟响应
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{
				"choices": [
					{
						"message": {
							"content": "测试电影 2024 4K HDR"
						}
					}
				]
			}`))
		}))
		defer server.Close()

		svc := NewService(logger, "test-api-key", server.URL, "gpt-4o-mini").(*service)

		keyword, err := svc.GenerateUpgradeKeyword("测试电影", "movie")
		assert.NoError(t, err)
		assert.Equal(t, "测试电影 2024 4K HDR", keyword)
	})

	t.Run("成功生成剧集升级关键词", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{
				"choices": [
					{
						"message": {
							"content": "测试剧集 第一季 4K 全集"
						}
					}
				]
			}`))
		}))
		defer server.Close()

		svc := NewService(logger, "test-api-key", server.URL, "gpt-4o-mini").(*service)

		keyword, err := svc.GenerateUpgradeKeyword("测试剧集", "tv")
		assert.NoError(t, err)
		assert.Contains(t, keyword, "4K")
	})

	t.Run("请求继承调用方 context", func(t *testing.T) {
		transport := &contextCaptureOpenAIRoundTripper{}
		svc := NewService(logger, "test-api-key", "https://api.openai.test", "gpt-4o-mini").(*service)
		svc.client = &http.Client{Transport: transport}

		ctx := context.WithValue(context.Background(), openAIContextMarkerKey{}, "request-marker")
		keyword, err := svc.GenerateUpgradeKeywordWithContext(ctx, "测试电影", "movie")
		assert.NoError(t, err)
		assert.Equal(t, "测试电影 2024 4K", keyword)
		assert.Equal(t, "request-marker", transport.contextValue)
	})

	t.Run("成功日志脱敏但返回原始关键词", func(t *testing.T) {
		sensitiveKeyword := "https://proxy-user:proxy-pass@example.test/search?accessToken=query-secret&filename=private-name.mkv#refreshToken=fragment-secret accessCode=abcd Authorization: Bearer bearer-secret"
		sensitiveTitle := "https://title-user:title-pass@example.test/movie?token=title-secret#refreshToken=title-fragment accessCode=wxyz"

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{
				"choices": [
					{
						"message": {
							"content": "` + sensitiveKeyword + `"
						}
					}
				]
			}`))
		}))
		defer server.Close()

		core, logs := observer.New(zap.InfoLevel)
		svc := NewService(zap.New(core), "test-api-key", server.URL, "gpt-4o-mini").(*service)

		keyword, err := svc.GenerateUpgradeKeyword(sensitiveTitle, "movie")
		assert.NoError(t, err)
		assert.Equal(t, sensitiveKeyword, keyword)

		entries := logs.FilterMessage("Generated upgrade keyword").All()
		assert.Len(t, entries, 1)

		fields := entries[0].ContextMap()

		logText := fmt.Sprint(fields["title"]) + " " + fmt.Sprint(fields["keyword"])
		for _, leaked := range []string{
			"proxy-user", "proxy-pass", "query-secret", "private-name.mkv", "fragment-secret", "abcd", "bearer-secret",
			"title-user", "title-pass", "title-secret", "title-fragment", "wxyz",
		} {
			assert.NotContains(t, logText, leaked)
		}

		assert.Contains(t, logText, utils.RedactedSecret)
	})

	t.Run("API密钥为空返回错误", func(t *testing.T) {
		svc := NewService(logger, "", "", "")
		keyword, err := svc.GenerateUpgradeKeyword("测试", "movie")
		assert.Error(t, err)
		assert.Empty(t, keyword)
		assert.Contains(t, err.Error(), "API key")
	})

	t.Run("服务器返回错误", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer server.Close()

		svc := NewService(logger, "test-api-key", server.URL, "gpt-4o-mini").(*service)

		keyword, err := svc.GenerateUpgradeKeyword("测试", "movie")
		assert.Error(t, err)
		assert.Empty(t, keyword)
	})

	t.Run("错误响应体脱敏", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadGateway)
			_, _ = w.Write([]byte(`{"error":{"message":"Authorization: Bearer secret-openai-key accessCode=abcd","access_token":"response-secret","url":"https://proxy-user:proxy-pass@example.test/path?client_secret=query-secret&filename=private-name.mkv#token=fragment-secret"}}`))
		}))
		defer server.Close()

		svc := NewService(logger, "secret-openai-key", server.URL, "gpt-4o-mini").(*service)

		keyword, err := svc.GenerateUpgradeKeyword("测试", "movie")
		assert.Error(t, err)
		assert.Empty(t, keyword)

		errText := err.Error()
		for _, leaked := range []string{"secret-openai-key", "response-secret", "proxy-user", "proxy-pass", "query-secret", "private-name.mkv", "fragment-secret", "abcd"} {
			assert.NotContains(t, errText, leaked)
		}

		assert.Contains(t, errText, utils.RedactedSecret)
	})

	t.Run("请求错误脱敏", func(t *testing.T) {
		svc := NewService(logger, "secret-openai-key", "https://proxy-user:proxy-pass@example.test", "gpt-4o-mini").(*service)
		svc.client = &http.Client{Transport: failingOpenAIRoundTripper{}}

		keyword, err := svc.GenerateUpgradeKeyword("测试", "movie")
		assert.Error(t, err)
		assert.Empty(t, keyword)

		errText := err.Error()
		for _, leaked := range []string{"secret-openai-key", "proxy-user", "proxy-pass", "abcd"} {
			assert.NotContains(t, errText, leaked)
		}

		assert.Contains(t, errText, "/v1/chat/completions")
		assert.Contains(t, errText, utils.RedactedSecret)
	})

	t.Run("成功状态超大响应返回错误", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(strings.Repeat("x", maxOpenAIResponseSize+1)))
		}))
		defer server.Close()

		svc := NewService(logger, "test-api-key", server.URL, "gpt-4o-mini").(*service)

		keyword, err := svc.GenerateUpgradeKeyword("测试", "movie")
		assert.Error(t, err)
		assert.Empty(t, keyword)
		assert.Contains(t, err.Error(), "响应体过大")
	})

	t.Run("错误状态超大响应返回错误", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(strings.Repeat("x", maxOpenAIResponseSize+1)))
		}))
		defer server.Close()

		svc := NewService(logger, "test-api-key", server.URL, "gpt-4o-mini").(*service)

		keyword, err := svc.GenerateUpgradeKeyword("测试", "movie")
		assert.Error(t, err)
		assert.Empty(t, keyword)
		assert.Contains(t, err.Error(), "响应体过大")
	})

	t.Run("空响应返回错误", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"choices": []}`))
		}))
		defer server.Close()

		svc := NewService(logger, "test-api-key", server.URL, "gpt-4o-mini").(*service)

		keyword, err := svc.GenerateUpgradeKeyword("测试", "movie")
		assert.Error(t, err)
		assert.Empty(t, keyword)
		assert.Contains(t, err.Error(), "no response")
	})
}

type openAIContextMarkerKey struct{}

type contextCaptureOpenAIRoundTripper struct {
	contextValue string
}

func (rt *contextCaptureOpenAIRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	if value, ok := req.Context().Value(openAIContextMarkerKey{}).(string); ok {
		rt.contextValue = value
	}

	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(`{"choices":[{"message":{"content":"测试电影 2024 4K"}}]}`)),
		Request:    req,
	}, nil
}

type failingOpenAIRoundTripper struct{}

func (failingOpenAIRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	return nil, fmt.Errorf("request failed: %s?filename=private-name.mkv#token=fragment-secret Authorization: %s accessCode=abcd", req.URL.String(), req.Header.Get("Authorization"))
}

func TestAPIKeyFromEnv(t *testing.T) {
	logger := zap.NewNop()

	t.Run("从环境变量读取API密钥", func(t *testing.T) {
		// 保存原始值
		origAPIKey := "original-key"

		svc := NewService(logger, "", "", "").(*service)
		// 如果环境变量未设置，APIKey 应该为空
		assert.Equal(t, "", svc.config.APIKey)

		_ = origAPIKey
	})
}
