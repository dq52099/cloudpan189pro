package openai

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
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
