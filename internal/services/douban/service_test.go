package douban

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
	svc := NewService(logger)
	assert.NotNil(t, svc)
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
