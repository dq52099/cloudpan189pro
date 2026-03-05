package douban

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"go.uber.org/zap"
)

type Service interface {
	GetPopularMovies() ([]Subject, error)
	GetMoviesByTag(tag string) ([]Subject, error)
	GetTop250(start, count int) ([]Subject, error)
	GetPlaying() ([]Subject, error)
	GetComing() ([]Subject, error)
}

type Subject struct {
	ID            string  `json:"id"`
	Title         string  `json:"title"`
	OriginalTitle string  `json:"original_title"`
	Year          string  `json:"year"`
	Rating        float64 `json:"rating"`
	Cover         string  `json:"cover"`
	URL           string  `json:"url"`
	Episodes      string  `json:"episodes,omitempty"`
	Type          string  `json:"type"`
}

type SearchResponse struct {
	Items []Subject `json:"subjects"`
	Total int       `json:"total"`
}

type service struct {
	logger  *zap.Logger
	client  *http.Client
	baseURL string
}

func NewService(logger *zap.Logger) Service {
	return &service{
		logger: logger,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
		baseURL: "https://movie.douban.com",
	}
}

func (s *service) GetPopularMovies() ([]Subject, error) {
	return s.GetMoviesByTag("热门")
}

func (s *service) GetMoviesByTag(tag string) ([]Subject, error) {
	if tag == "" {
		tag = "热门"
	}
	apiURL := fmt.Sprintf("https://movie.douban.com/j/search_subjects?type=movie&tag=%s&page_limit=50&page_start=0", url.QueryEscape(tag))

	cookieClient := &http.Client{
		Timeout: 30 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return nil
		},
	}

	firstReq, _ := http.NewRequestWithContext(context.Background(), "GET", "https://movie.douban.com/", nil)
	firstReq.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	firstReq.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	firstReq.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8")
	resp0, err := cookieClient.Do(firstReq)
	if err == nil {
		resp0.Body.Close()
	}

	req, err := http.NewRequestWithContext(context.Background(), "GET", apiURL, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,*/*;q=0.8")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8")
	req.Header.Set("Accept-Encoding", "gzip, deflate, br")
	req.Header.Set("Referer", "https://movie.douban.com/")
	req.Header.Set("Connection", "keep-alive")
	req.Header.Set("Cache-Control", "no-cache")

	for _, cookie := range resp0.Cookies() {
		req.AddCookie(cookie)
	}

	resp, err := cookieClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Douban API returned status: %d", resp.StatusCode)
	}

	var result SearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	for i := range result.Items {
		if result.Items[i].Year != "" {
			result.Items[i].Year = extractYear(result.Items[i].Year)
		}
	}

	return result.Items, nil
}

func (s *service) GetTop250(start, count int) ([]Subject, error) {
	apiURL := fmt.Sprintf("https://movie.douban.com/top250?start=%d&filter=", start)

	req, err := http.NewRequestWithContext(context.Background(), "GET", apiURL, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	req.Header.Set("Referer", "https://movie.douban.com")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Douban API returned status: %d", resp.StatusCode)
	}

	// 豆瓣 Top250 返回 HTML，需要解析
	// 这里简化为返回空列表，实际应该解析 HTML
	s.logger.Warn("Douban Top250 HTML parsing not implemented, returning empty list")

	return []Subject{}, nil
}

func (s *service) GetPlaying() ([]Subject, error) {
	apiURL := "https://movie.douban.com/j/search_subjects?type=movie&tag=正在热映&page_limit=50"

	req, err := http.NewRequestWithContext(context.Background(), "GET", apiURL, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	req.Header.Set("Referer", "https://movie.douban.com")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Douban API returned status: %d", resp.StatusCode)
	}

	var result SearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	for i := range result.Items {
		if result.Items[i].Year != "" {
			result.Items[i].Year = extractYear(result.Items[i].Year)
		}
	}

	return result.Items, nil
}

func (s *service) GetComing() ([]Subject, error) {
	apiURL := "https://movie.douban.com/j/search_subjects?type=movie&tag=即将上映&page_limit=50"

	req, err := http.NewRequestWithContext(context.Background(), "GET", apiURL, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	req.Header.Set("Referer", "https://movie.douban.com")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Douban API returned status: %d", resp.StatusCode)
	}

	var result SearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	for i := range result.Items {
		if result.Items[i].Year != "" {
			result.Items[i].Year = extractYear(result.Items[i].Year)
		}
	}

	return result.Items, nil
}

func extractYear(s string) string {
	// 尝试从字符串中提取年份，如 "2024-01-15" -> "2024"
	s = strings.TrimSpace(s)
	if len(s) >= 4 {
		year := s[:4]
		if _, err := strconv.Atoi(year); err == nil {
			return year
		}
	}
	// 处理 "2024" 格式
	if _, err := strconv.Atoi(s); err == nil && len(s) == 4 {
		return s
	}
	return s
}
