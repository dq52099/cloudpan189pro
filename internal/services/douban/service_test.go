package douban

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

	"github.com/xxcheng123/cloudpan189-share/internal/pkgs/utils"
)

func TestNewService(t *testing.T) {
	logger := zap.NewNop()
	svc := NewService(logger)
	assert.NotNil(t, svc)
}

func TestNewServiceWithNilLoggerDoesNotPanic(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	baseURL := server.URL
	server.Close()

	svc := NewService(nil).(*service)
	svc.baseURL = baseURL

	movies, err := svc.GetMoviesByTag("热门")
	assert.Error(t, err)
	assert.Nil(t, movies)
}

func TestGetPopularMovies(t *testing.T) {
	logger := zap.NewNop()

	t.Run("成功获取热门电影", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/" {
				http.SetCookie(w, &http.Cookie{Name: "bid", Value: "test-cookie"})
				w.WriteHeader(http.StatusOK)

				return
			}

			assert.Contains(t, r.URL.Path, "/j/search_subjects")
			assert.Equal(t, "movie", r.URL.Query().Get("type"))
			assert.Equal(t, "热门", r.URL.Query().Get("tag"))
			assert.Equal(t, "50", r.URL.Query().Get("page_limit"))
			assert.Equal(t, "test-cookie", cookieValue(r, "bid"))
			assert.Contains(t, r.Header.Get("User-Agent"), "Mozilla")

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{
				"subjects": [
					{
						"id": "1234567",
						"title": "测试电影",
						"original_title": "Test Movie",
						"year": "2024",
						"rating": 8.5,
						"cover": "https://example.com/cover.jpg",
						"url": "https://movie.douban.com/subject/1234567/",
						"type": "movie"
					}
				],
				"total": 1
			}`))
		}))
		defer server.Close()

		svc := NewService(logger).(*service)
		svc.baseURL = server.URL

		movies, err := svc.GetPopularMovies()
		assert.NoError(t, err)
		assert.Len(t, movies, 1)
		assert.Equal(t, "1234567", movies[0].ID)
		assert.Equal(t, "测试电影", movies[0].Title)
		assert.Equal(t, "2024", movies[0].Year)
		assert.Equal(t, 8.5, movies[0].Rating)
	})

	t.Run("请求继承调用方 context", func(t *testing.T) {
		transport := &contextCaptureDoubanRoundTripper{}
		svc := NewService(logger).(*service)
		svc.baseURL = "https://movie.douban.test"
		svc.client = &http.Client{Transport: transport}

		ctx := context.WithValue(context.Background(), doubanContextMarkerKey{}, "request-marker")
		movies, err := svc.GetMoviesByTagWithContext(ctx, "热门")
		assert.NoError(t, err)
		assert.Len(t, movies, 1)
		assert.Equal(t, "1234567", movies[0].ID)
		assert.Equal(t, []string{"request-marker", "request-marker"}, transport.contextValues)
		assert.Equal(t, []string{"/", "/j/search_subjects"}, transport.paths)
		assert.Equal(t, "test-cookie", transport.searchCookie)
	})

	t.Run("服务器返回错误", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/" {
				w.WriteHeader(http.StatusOK)

				return
			}

			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer server.Close()

		svc := NewService(logger).(*service)
		svc.baseURL = server.URL

		movies, err := svc.GetPopularMovies()
		assert.Error(t, err)
		assert.Nil(t, movies)
	})

	t.Run("超大响应返回错误", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/" {
				w.WriteHeader(http.StatusOK)

				return
			}

			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(strings.Repeat("x", maxDoubanResponseSize+1)))
		}))
		defer server.Close()

		svc := NewService(logger).(*service)
		svc.baseURL = server.URL

		movies, err := svc.GetPopularMovies()
		assert.Error(t, err)
		assert.Nil(t, movies)
		assert.Contains(t, err.Error(), "响应体过大")
	})
}

func TestGetPlaying(t *testing.T) {
	t.Run("成功获取正在热映", func(t *testing.T) {
		logger := zap.NewNop()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Contains(t, r.URL.Path, "/j/search_subjects")
			assert.Equal(t, "正在热映", r.URL.Query().Get("tag"))

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{
				"subjects": [
					{
						"id": "123",
						"title": "正在热映电影",
						"year": "2024",
						"rating": 7.5
					}
				]
			}`))
		}))
		defer server.Close()

		svc := NewService(logger).(*service)
		svc.baseURL = server.URL

		movies, err := svc.GetPlaying()
		assert.NoError(t, err)
		assert.Len(t, movies, 1)
		assert.Equal(t, "正在热映电影", movies[0].Title)
		assert.Equal(t, "2024", movies[0].Year)
	})

	t.Run("请求继承调用方 context", func(t *testing.T) {
		transport := &contextCaptureDoubanRoundTripper{}
		svc := NewService(zap.NewNop()).(*service)
		svc.baseURL = "https://movie.douban.test"
		svc.client = &http.Client{Transport: transport}

		ctx := context.WithValue(context.Background(), doubanContextMarkerKey{}, "playing-marker")
		movies, err := svc.GetPlayingWithContext(ctx)
		assert.NoError(t, err)
		assert.Len(t, movies, 1)
		assert.Equal(t, "1234567", movies[0].ID)
		assert.Equal(t, []string{"playing-marker"}, transport.contextValues)
		assert.Equal(t, []string{"/j/search_subjects"}, transport.paths)
		assert.Equal(t, []string{"正在热映"}, transport.queryTags)
	})
}

func TestGetComing(t *testing.T) {
	t.Run("成功获取即将上映", func(t *testing.T) {
		logger := zap.NewNop()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Contains(t, r.URL.Path, "/j/search_subjects")
			assert.Equal(t, "即将上映", r.URL.Query().Get("tag"))

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{
				"subjects": [
					{
						"id": "456",
						"title": "即将上映电影",
						"year": "2024",
						"rating": 0
					}
				]
			}`))
		}))
		defer server.Close()

		svc := NewService(logger).(*service)
		svc.baseURL = server.URL

		movies, err := svc.GetComing()
		assert.NoError(t, err)
		assert.Len(t, movies, 1)
		assert.Equal(t, "即将上映电影", movies[0].Title)
		assert.Equal(t, "2024", movies[0].Year)
	})

	t.Run("请求继承调用方 context", func(t *testing.T) {
		transport := &contextCaptureDoubanRoundTripper{}
		svc := NewService(zap.NewNop()).(*service)
		svc.baseURL = "https://movie.douban.test"
		svc.client = &http.Client{Transport: transport}

		ctx := context.WithValue(context.Background(), doubanContextMarkerKey{}, "coming-marker")
		movies, err := svc.GetComingWithContext(ctx)
		assert.NoError(t, err)
		assert.Len(t, movies, 1)
		assert.Equal(t, "1234567", movies[0].ID)
		assert.Equal(t, []string{"coming-marker"}, transport.contextValues)
		assert.Equal(t, []string{"/j/search_subjects"}, transport.paths)
		assert.Equal(t, []string{"即将上映"}, transport.queryTags)
	})
}

func TestGetTop250(t *testing.T) {
	logger := zap.NewNop()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/top250", r.URL.Path)
		assert.Equal(t, "0", r.URL.Query().Get("start"))

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`
			<li>
				<div class="item">
					<div class="pic"><em class="">1</em><a href="https://movie.douban.com/subject/1292052/"><img src="https://img.example.com/cover.jpg" alt="肖申克的救赎"/></a></div>
					<div class="info">
						<div class="hd"><span class="title">肖申克的救赎</span></div>
						<div class="bd"><p class="">导演: Frank Darabont / 1994 / 美国</p><span class="rating_num">9.7</span></div>
					</div>
				</div>
			</li>`))
	}))
	defer server.Close()

	svc := NewService(logger).(*service)
	svc.baseURL = server.URL

	movies, err := svc.GetTop250(0, 1)
	assert.NoError(t, err)
	assert.Len(t, movies, 1)
	assert.Equal(t, "1292052", movies[0].ID)
	assert.Equal(t, "肖申克的救赎", movies[0].Title)
	assert.Equal(t, "1994", movies[0].Year)
	assert.Equal(t, 9.7, movies[0].Rating)
}

func TestGetTop250WithContextUsesCallerContext(t *testing.T) {
	transport := &contextCaptureDoubanRoundTripper{}
	svc := NewService(zap.NewNop()).(*service)
	svc.baseURL = "https://movie.douban.test"
	svc.client = &http.Client{Transport: transport}

	ctx := context.WithValue(context.Background(), doubanContextMarkerKey{}, "top250-marker")
	movies, err := svc.GetTop250WithContext(ctx, 25, 1)
	assert.NoError(t, err)
	assert.Len(t, movies, 1)
	assert.Equal(t, "1292052", movies[0].ID)
	assert.Equal(t, []string{"top250-marker"}, transport.contextValues)
	assert.Equal(t, []string{"/top250"}, transport.paths)
	assert.Equal(t, []string{"25"}, transport.queryStarts)
}

func TestGetTop250RedactsRequestError(t *testing.T) {
	logger := zap.NewNop()
	svc := NewService(logger).(*service)
	svc.baseURL = "https://proxy-user:proxy-pass@example.test"
	svc.client = &http.Client{Transport: failingDoubanRoundTripper{}}

	movies, err := svc.GetTop250(0, 1)
	assert.Error(t, err)
	assert.Nil(t, movies)

	errText := err.Error()
	for _, leaked := range []string{"proxy-user", "proxy-pass", "query-secret", "private-name.mkv", "fragment-secret", "abcd", "secret-token"} {
		assert.NotContains(t, errText, leaked)
	}

	assert.Contains(t, errText, "/top250")
	assert.Contains(t, errText, utils.RedactedSecret)
}

type failingDoubanRoundTripper struct{}

func (failingDoubanRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	return nil, fmt.Errorf("request failed: %s?access_token=query-secret&filename=private-name.mkv#token=fragment-secret accessCode=abcd Authorization: Bearer secret-token", req.URL.String())
}

type doubanContextMarkerKey struct{}

type contextCaptureDoubanRoundTripper struct {
	contextValues []string
	paths         []string
	queryStarts   []string
	queryTags     []string
	searchCookie  string
}

func (rt *contextCaptureDoubanRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	if value, ok := req.Context().Value(doubanContextMarkerKey{}).(string); ok {
		rt.contextValues = append(rt.contextValues, value)
	} else {
		rt.contextValues = append(rt.contextValues, "")
	}

	rt.paths = append(rt.paths, req.URL.Path)

	if req.URL.Path == "/" {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Set-Cookie": []string{"bid=test-cookie"}},
			Body:       io.NopCloser(strings.NewReader("")),
			Request:    req,
		}, nil
	}

	if req.URL.Path == "/top250" {
		rt.queryStarts = append(rt.queryStarts, req.URL.Query().Get("start"))

		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"text/html"}},
			Body: io.NopCloser(strings.NewReader(`
				<li>
					<div class="item">
						<div class="pic"><em class="">1</em><a href="https://movie.douban.com/subject/1292052/"><img src="https://img.example.com/cover.jpg" alt="肖申克的救赎"/></a></div>
						<div class="info">
							<div class="hd"><span class="title">肖申克的救赎</span></div>
							<div class="bd"><p class="">导演: Frank Darabont / 1994 / 美国</p><span class="rating_num">9.7</span></div>
						</div>
					</div>
				</li>`)),
			Request: req,
		}, nil
	}

	rt.queryTags = append(rt.queryTags, req.URL.Query().Get("tag"))
	rt.searchCookie = cookieValue(req, "bid")

	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body: io.NopCloser(strings.NewReader(`{
			"subjects": [{
				"id": "1234567",
				"title": "测试电影",
				"year": "2024",
				"rating": 8.5
			}],
			"total": 1
		}`)),
		Request: req,
	}, nil
}

func cookieValue(r *http.Request, name string) string {
	cookie, err := r.Cookie(name)
	if err != nil {
		return ""
	}

	return cookie.Value
}

func TestExtractYear(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"2024-01-15", "2024"},
		{"2024", "2024"},
		{"2023-12-31", "2023"},
		{"", ""},
		{"invalid", "invalid"},
		{"202", "202"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := extractYear(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}
