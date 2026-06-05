package tmdb

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"

	"github.com/xxcheng123/cloudpan189-share/internal/pkgs/utils"
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

func TestSetAPIKeyDoesNotLogSecret(t *testing.T) {
	core, logs := observer.New(zap.InfoLevel)
	logger := zap.New(core)
	svc := NewService(logger, "", "", "", "").(*service)

	secret := "1234567890abcdef-secret"
	svc.SetAPIKey(secret)

	assert.Equal(t, secret, svc.GetConfig().APIKey)

	entries := logs.All()
	require.Len(t, entries, 1)

	entry := entries[0]
	fields := entry.ContextMap()
	logText := entry.Message + fmt.Sprint(entry.Context)
	assert.Equal(t, "TMDB API key updated", entry.Message)
	assert.Equal(t, true, fields["has_api_key"])
	assert.NotContains(t, logText, secret)
	assert.NotContains(t, logText, "1234567890")
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

	t.Run("API密钥特殊字符会编码", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			query := r.URL.Query()
			assert.Equal(t, "secret&language=en-US", query.Get("api_key"))
			assert.Equal(t, []string{"zh-CN"}, query["language"])

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"page":1,"results":[],"total_pages":1,"total_results":0}`))
		}))
		defer server.Close()

		svc := NewService(logger, "secret&language=en-US", server.URL, "", "").(*service)

		movies, err := svc.GetPopularMovies(1)
		assert.NoError(t, err)
		assert.Empty(t, movies)
	})

	t.Run("请求继承调用方 context", func(t *testing.T) {
		transport := &contextCaptureTMDBRoundTripper{}
		svc := NewService(logger, "test-api-key", "https://api.tmdb.test/3", "", "").(*service)
		svc.client = &http.Client{Transport: transport}

		ctx := context.WithValue(context.Background(), tmdbContextMarkerKey{}, "request-marker")
		movies, err := svc.GetPopularMoviesWithContext(ctx, 1)
		assert.NoError(t, err)
		assert.Len(t, movies, 1)
		assert.Equal(t, int64(1), movies[0].ID)
		assert.Equal(t, "request-marker", transport.contextValue)
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

	t.Run("请求错误脱敏", func(t *testing.T) {
		svc := NewService(logger, "secret-tmdb-key", "https://proxy-user:proxy-pass@example.test/3", "", "").(*service)
		svc.client = &http.Client{Transport: failingTMDBRoundTripper{}}

		movies, err := svc.GetPopularMovies(1)
		assert.Error(t, err)
		assert.Nil(t, movies)

		errText := err.Error()
		for _, leaked := range []string{"secret-tmdb-key", "proxy-user", "proxy-pass", "abcd"} {
			assert.NotContains(t, errText, leaked)
		}

		assert.Contains(t, errText, "/movie/popular")
		assert.Contains(t, errText, utils.RedactedSecret)
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

type failingTMDBRoundTripper struct{}

func (failingTMDBRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	return nil, fmt.Errorf("request failed: %s accessCode=abcd", req.URL.String())
}

type tmdbContextMarkerKey struct{}

type contextCaptureTMDBRoundTripper struct {
	contextValue string
	path         string
}

func (rt *contextCaptureTMDBRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	if value, ok := req.Context().Value(tmdbContextMarkerKey{}).(string); ok {
		rt.contextValue = value
	}

	rt.path = req.URL.Path

	body := `{"page":1,"results":[{"id":1,"title":"测试电影","vote_average":8.5}],"total_pages":1,"total_results":1}`

	switch req.URL.Path {
	case "/3/genre/movie/list":
		body = `{"genres":[{"id":16,"name":"动画"}]}`
	case "/3/discover/tv":
		body = `{"page":1,"results":[{"id":2,"name":"测试剧集","vote_average":9.0}],"total_pages":1,"total_results":1}`
	}

	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
		Request:    req,
	}, nil
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

func TestGetGenreListWithContextUsesRequestContext(t *testing.T) {
	logger := zap.NewNop()
	transport := &contextCaptureTMDBRoundTripper{}
	svc := NewService(logger, "test-api-key", "https://api.tmdb.test/3", "", "").(*service)
	svc.client = &http.Client{Transport: transport}

	ctx := context.WithValue(context.Background(), tmdbContextMarkerKey{}, "request-marker")
	genres, err := svc.GetGenreListWithContext(ctx)
	assert.NoError(t, err)
	assert.Len(t, genres, 1)
	assert.Equal(t, 16, genres[0].ID)
	assert.Equal(t, "动画", genres[0].Name)
	assert.Equal(t, "request-marker", transport.contextValue)
	assert.Equal(t, "/3/genre/movie/list", transport.path)
}

func TestGetTVsByGenreWithContextUsesRequestContext(t *testing.T) {
	logger := zap.NewNop()
	transport := &contextCaptureTMDBRoundTripper{}
	svc := NewService(logger, "test-api-key", "https://api.tmdb.test/3", "", "").(*service)
	svc.client = &http.Client{Transport: transport}

	ctx := context.WithValue(context.Background(), tmdbContextMarkerKey{}, "request-marker")
	tvs, err := svc.GetTVsByGenreWithContext(ctx, 18, 1)
	assert.NoError(t, err)
	assert.Len(t, tvs, 1)
	assert.Equal(t, int64(2), tvs[0].ID)
	assert.Equal(t, "测试剧集", tvs[0].Name)
	assert.Equal(t, "request-marker", transport.contextValue)
	assert.Equal(t, "/3/discover/tv", transport.path)
}

func TestProxyConfiguration(t *testing.T) {
	logger := zap.NewNop()

	t.Run("HTTP代理配置正确", func(t *testing.T) {
		svc := NewService(logger, "test-key", "", "http://127.0.0.1:8080", "http").(*service)
		transport, ok := svc.client.Transport.(*http.Transport)
		require.True(t, ok)
		require.NotNil(t, transport.Proxy)
		assertProxyTransportClonesDefault(t, transport)
	})

	t.Run("无效代理URL不崩溃", func(t *testing.T) {
		svc := NewService(logger, "test-key", "", "://invalid-url", "")
		assert.NotNil(t, svc)
	})

	t.Run("nil logger 代理日志不崩溃", func(t *testing.T) {
		svc := NewService(nil, "test-key", "", "://invalid-url", "")
		assert.NotNil(t, svc)
	})

	t.Run("代理日志不泄露凭据", func(t *testing.T) {
		core, logs := observer.New(zap.InfoLevel)
		logger := zap.New(core)

		svc := NewService(logger, "test-key", "", "http://proxy-user:proxy-pass@127.0.0.1:8080", "http")
		assert.NotNil(t, svc)

		entries := logs.FilterMessage("TMDB proxy configured").All()
		require.Len(t, entries, 1)

		logText, err := url.QueryUnescape(entries[0].Message + fmt.Sprint(entries[0].Context))
		require.NoError(t, err)
		assert.NotContains(t, logText, "proxy-user")
		assert.NotContains(t, logText, "proxy-pass")
		assert.Contains(t, logText, "[REDACTED]")
	})
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
