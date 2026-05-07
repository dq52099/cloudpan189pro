package douban

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
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

// top250ItemRegex 解析豆瓣 Top250 页面的条目块。
// 页面每部电影以 <li> 内 <div class="item"> 包裹，包含：
//   - <div class="pic"><em class="">rank</em><a href="subjectURL"><img src="cover" alt="title"/>
//   - <div class="info">...<span class="title">Title</span>...<span class="rating_num">9.7</span>
//   - <div class="bd"><p class="">Director / Year-country</p></div>
var (
	top250ItemRegex   = regexp.MustCompile(`(?s)<div class="item">(.*?)</li>`)
	top250IDRegex     = regexp.MustCompile(`/subject/(\d+)/`)
	top250URLRegex    = regexp.MustCompile(`<a href="([^"]+)"[^>]*>\s*<img`)
	top250CoverRegex  = regexp.MustCompile(`<img[^>]+src="([^"]+)"`)
	top250TitleRegex  = regexp.MustCompile(`<span class="title">([^<]+)</span>`)
	top250RatingRegex = regexp.MustCompile(`<span class="rating_num"[^>]*>([\d.]+)</span>`)
	top250YearRegex   = regexp.MustCompile(`(\d{4})`)
	top250InfoRegex   = regexp.MustCompile(`(?s)<div class="bd">\s*<p[^>]*>(.*?)</p>`)
)

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

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取豆瓣响应失败: %w", err)
	}

	subjects := parseDoubanTop250HTML(string(body))
	if count > 0 && len(subjects) > count {
		subjects = subjects[:count]
	}

	return subjects, nil
}

// parseDoubanTop250HTML 通过正则解析 Top250 HTML。
// 豆瓣页面结构长期稳定，对 HTML 解析来说正则足以覆盖，且无需额外依赖。
func parseDoubanTop250HTML(body string) []Subject {
	items := top250ItemRegex.FindAllStringSubmatch(body, -1)
	subjects := make([]Subject, 0, len(items))

	for _, m := range items {
		if len(m) < 2 {
			continue
		}

		block := m[1]
		subject := Subject{Type: "movie"}

		if urlMatches := top250URLRegex.FindStringSubmatch(block); len(urlMatches) > 1 {
			subject.URL = urlMatches[1]
		}
		if idMatches := top250IDRegex.FindStringSubmatch(block); len(idMatches) > 1 {
			subject.ID = idMatches[1]
		}
		if coverMatches := top250CoverRegex.FindStringSubmatch(block); len(coverMatches) > 1 {
			subject.Cover = coverMatches[1]
		}
		if titleMatches := top250TitleRegex.FindStringSubmatch(block); len(titleMatches) > 1 {
			subject.Title = strings.TrimSpace(titleMatches[1])
		}
		if ratingMatches := top250RatingRegex.FindStringSubmatch(block); len(ratingMatches) > 1 {
			if rating, err := strconv.ParseFloat(ratingMatches[1], 64); err == nil {
				subject.Rating = rating
			}
		}
		if infoMatches := top250InfoRegex.FindStringSubmatch(block); len(infoMatches) > 1 {
			if yearMatches := top250YearRegex.FindStringSubmatch(infoMatches[1]); len(yearMatches) > 1 {
				subject.Year = yearMatches[1]
			}
		}

		if subject.Title == "" {
			continue
		}

		subjects = append(subjects, subject)
	}

	return subjects
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
