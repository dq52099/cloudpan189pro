package tmdb

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

	t.Run("创建服务不带代理", func(t *testing.T) {
		svc := NewService(logger, "test-api-key", "", "", "")
		assert.NotNil(t, svc)
		assert.NotNil(t, svc.GetConfig())
		assert.Equal(t, "test-api-key", svc.GetConfig().APIKey)
		assert.Equal(t, defaultTMDBBaseURL, svc.GetConfig().BaseURL)
	})

	t.Run("创建服务使用自定义BaseURL", func(t *testing.T) {
		svc := NewService(logger, "test-api-key", "https://tmdb.example.test/3/", "", "")
		assert.NotNil(t, svc)
		assert.Equal(t, "https://tmdb.example.test/3", svc.GetConfig().BaseURL)
		assert.Equal(t, "https://tmdb.example.test/3", svc.(*service).baseURL)
	})

	t.Run("创建服务带HTTP代理", func(t *testing.T) {
		svc := NewService(logger, "test-api-key", "", "http://192.168.1.1:7890", "http")
		assert.NotNil(t, svc)
		assert.Equal(t, "http://192.168.1.1:7890", svc.GetConfig().ProxyURL)
	})

	t.Run("创建服务带SOCKS5代理", func(t *testing.T) {
		svc := NewService(logger, "test-api-key", "", "socks5://192.168.1.1:1080", "socks5")
		assert.NotNil(t, svc)
		assert.Equal(t, "socks5://192.168.1.1:1080", svc.GetConfig().ProxyURL)
	})
}

func TestGetPopularMovies(t *testing.T) {
	logger := zap.NewNop()

	t.Run("成功获取热门电影", func(t *testing.T) {
		// 创建模拟服务器
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// 验证请求参数
			assert.Contains(t, r.URL.Path, "/movie/popular")
			assert.Contains(t, r.URL.RawQuery, "api_key=test-api-key")
			assert.Contains(t, r.URL.RawQuery, "language=zh-CN")

			// 返回模拟响应
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{
				"page": 1,
				"results": [
					{
						"id": 1,
						"title": "测试电影",
						"original_title": "Test Movie",
						"release_date": "2024-01-01",
						"vote_average": 8.5,
						"popularity": 100.0
					}
				],
				"total_pages": 1,
				"total_results": 1
			}`))
		}))
		defer server.Close()

		svc := NewService(logger, "test-api-key", server.URL, "", "").(*service)

		movies, err := svc.GetPopularMovies(1)
		assert.NoError(t, err)
		assert.Len(t, movies, 1)
		assert.Equal(t, int64(1), movies[0].ID)
		assert.Equal(t, "测试电影", movies[0].Title)
		assert.Equal(t, 8.5, movies[0].VoteAverage)
	})

	t.Run("API密钥为空返回错误", func(t *testing.T) {
		svc := NewService(logger, "", "", "", "")
		movies, err := svc.GetPopularMovies(1)
		assert.Error(t, err)
		assert.Nil(t, movies)
		assert.Contains(t, err.Error(), "API key")
	})

	t.Run("服务器错误返回错误", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer server.Close()

		svc := NewService(logger, "test-api-key", server.URL, "", "").(*service)

		movies, err := svc.GetPopularMovies(1)
		assert.Error(t, err)
		assert.Nil(t, movies)
	})

	t.Run("超大响应返回错误", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(strings.Repeat("x", maxTMDBResponseSize+1)))
		}))
		defer server.Close()

		svc := NewService(logger, "test-api-key", server.URL, "", "")

		movies, err := svc.GetPopularMovies(1)
		assert.Error(t, err)
		assert.Nil(t, movies)
		assert.Contains(t, err.Error(), "响应体过大")
	})
}

func TestGetPopularTVs(t *testing.T) {
	logger := zap.NewNop()

	t.Run("成功获取热门剧集", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Contains(t, r.URL.Path, "/tv/popular")

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{
				"page": 1,
				"results": [
					{
						"id": 1,
						"name": "测试剧集",
						"original_name": "Test TV",
						"first_air_date": "2024-01-01",
						"vote_average": 9.0,
						"popularity": 150.0
					}
				],
				"total_pages": 1,
				"total_results": 1
			}`))
		}))
		defer server.Close()

		svc := NewService(logger, "test-api-key", server.URL, "", "").(*service)

		tvs, err := svc.GetPopularTVs(1)
		assert.NoError(t, err)
		assert.Len(t, tvs, 1)
		assert.Equal(t, int64(1), tvs[0].ID)
		assert.Equal(t, "测试剧集", tvs[0].Name)
		assert.Equal(t, 9.0, tvs[0].VoteAverage)
	})

	t.Run("API密钥为空返回错误", func(t *testing.T) {
		svc := NewService(logger, "", "", "", "")
		tvs, err := svc.GetPopularTVs(1)
		assert.Error(t, err)
		assert.Nil(t, tvs)
	})
}

func TestProxyConfiguration(t *testing.T) {
	logger := zap.NewNop()

	t.Run("HTTP代理配置正确", func(t *testing.T) {
		svc := NewService(logger, "test-key", "", "http://127.0.0.1:8080", "http").(*service)
		assert.NotNil(t, svc.client.Transport)
	})

	t.Run("无效代理URL不崩溃", func(t *testing.T) {
		svc := NewService(logger, "test-key", "", "://invalid-url", "")
		assert.NotNil(t, svc)
	})
}
