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

	"github.com/xxcheng123/cloudpan189-share/internal/pkgs/utils"
	"go.uber.org/zap"
)

const (
	defaultDoubanBaseURL  = "https://movie.douban.com"
	maxDoubanResponseSize = 5 << 20
)

var doubanLogURLPattern = regexp.MustCompile(`(?i)(?:[a-z][a-z0-9+.-]*://|/)[^\s"'<>]+`)

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

func sanitizeDoubanError(err error) string {
	if err == nil {
		return ""
	}

	message := err.Error()
	message = doubanLogURLPattern.ReplaceAllStringFunc(message, utils.RedactURLForLog)

	return utils.RedactSensitiveText(message)
}

func NewService(logger *zap.Logger) Service {
	if logger == nil {
		logger = zap.NewNop()
	}

	return &service{
		logger: logger,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
		baseURL: defaultDoubanBaseURL,
	}
}

func (s *service) GetPopularMovies() ([]Subject, error) {
	return s.GetPopularMoviesWithContext(context.Background())
}

func (s *service) GetPopularMoviesWithContext(ctx context.Context) ([]Subject, error) {
	return s.GetMoviesByTagWithContext(ctx, "热门")
}

func (s *service) GetMoviesByTag(tag string) ([]Subject, error) {
	return s.GetMoviesByTagWithContext(context.Background(), tag)
}

func (s *service) GetMoviesByTagWithContext(ctx context.Context, tag string) ([]Subject, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	if tag == "" {
		tag = "热门"
	}

	apiURL := fmt.Sprintf("%s/j/search_subjects?type=movie&tag=%s&page_limit=50&page_start=0", s.baseURL, url.QueryEscape(tag))

	client := s.client
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}

	firstReq, err := http.NewRequestWithContext(ctx, "GET", s.baseURL+"/", nil)
	if err != nil {
		return nil, fmt.Errorf("创建豆瓣 Cookie 请求失败: %s", sanitizeDoubanError(err))
	}

	firstReq.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	firstReq.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	firstReq.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8")
	resp0, err := client.Do(firstReq)

	var cookies []*http.Cookie
	if err == nil {
		cookies = resp0.Cookies()
		_ = resp0.Body.Close()
	} else {
		s.logger.Warn("failed to fetch douban cookies", zap.String("error", sanitizeDoubanError(err)))
	}

	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("创建豆瓣请求失败: %s", sanitizeDoubanError(err))
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,*/*;q=0.8")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8")
	req.Header.Set("Referer", s.baseURL+"/")
	req.Header.Set("Connection", "keep-alive")
	req.Header.Set("Cache-Control", "no-cache")

	for _, cookie := range cookies {
		req.AddCookie(cookie)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("豆瓣请求失败: %s", sanitizeDoubanError(err))
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("douban API returned status: %d", resp.StatusCode)
	}

	var result SearchResponse
	if err := decodeLimitedDoubanJSON(resp.Body, &result); err != nil {
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
	return s.GetTop250WithContext(context.Background(), start, count)
}

func (s *service) GetTop250WithContext(ctx context.Context, start, count int) ([]Subject, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	apiURL := fmt.Sprintf("%s/top250?start=%d&filter=", s.baseURL, start)

	client := s.client
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}

	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("创建豆瓣 Top250 请求失败: %s", sanitizeDoubanError(err))
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	req.Header.Set("Referer", s.baseURL)

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("豆瓣 Top250 请求失败: %s", sanitizeDoubanError(err))
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("douban API returned status: %d", resp.StatusCode)
	}

	body, err := readLimitedDoubanResponse(resp.Body)
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

func readLimitedDoubanResponse(body io.Reader) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(body, maxDoubanResponseSize+1))
	if err != nil {
		return nil, err
	}

	if len(data) > maxDoubanResponseSize {
		return nil, fmt.Errorf("douban 响应体过大，已拒绝")
	}

	return data, nil
}

func decodeLimitedDoubanJSON(body io.Reader, target interface{}) error {
	data, err := readLimitedDoubanResponse(body)
	if err != nil {
		return err
	}

	return json.Unmarshal(data, target)
}

func (s *service) GetPlaying() ([]Subject, error) {
	return s.GetPlayingWithContext(context.Background())
}

func (s *service) GetPlayingWithContext(ctx context.Context) ([]Subject, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	apiURL := fmt.Sprintf("%s/j/search_subjects?type=movie&tag=%s&page_limit=50", s.baseURL, url.QueryEscape("正在热映"))

	client := s.client
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}

	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("创建豆瓣正在热映请求失败: %s", sanitizeDoubanError(err))
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	req.Header.Set("Referer", s.baseURL)

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("豆瓣正在热映请求失败: %s", sanitizeDoubanError(err))
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("douban API returned status: %d", resp.StatusCode)
	}

	var result SearchResponse
	if err := decodeLimitedDoubanJSON(resp.Body, &result); err != nil {
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
	return s.GetComingWithContext(context.Background())
}

func (s *service) GetComingWithContext(ctx context.Context) ([]Subject, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	apiURL := fmt.Sprintf("%s/j/search_subjects?type=movie&tag=%s&page_limit=50", s.baseURL, url.QueryEscape("即将上映"))

	client := s.client
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}

	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("创建豆瓣即将上映请求失败: %s", sanitizeDoubanError(err))
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	req.Header.Set("Referer", s.baseURL)

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("豆瓣即将上映请求失败: %s", sanitizeDoubanError(err))
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("douban API returned status: %d", resp.StatusCode)
	}

	var result SearchResponse
	if err := decodeLimitedDoubanJSON(resp.Body, &result); err != nil {
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
