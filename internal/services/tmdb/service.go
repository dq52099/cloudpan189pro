package tmdb

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/xxcheng123/cloudpan189-share/internal/pkgs/utils"
	"go.uber.org/zap"
)

const (
	defaultTMDBBaseURL  = "https://api.themoviedb.org/3"
	maxTMDBResponseSize = 5 << 20
)

var tmdbLogURLPattern = regexp.MustCompile(`(?i)(?:[a-z][a-z0-9+.-]*://|/)[^\s"'<>]+`)

type Service interface {
	GetPopularMovies(page int) ([]Movie, error)
	GetPopularTVs(page int) ([]TV, error)
	GetMoviesByGenre(genreID int, page int) ([]Movie, error)
	GetTVsByGenre(genreID int, page int) ([]TV, error)
	GetGenreList() ([]Genre, error)
	GetConfig() *Config
	SetAPIKey(apiKey string)
}

type Genre struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

type GenreListResponse struct {
	Genres []Genre `json:"genres"`
}

type Config struct {
	APIKey    string
	BaseURL   string
	Language  string
	Region    string
	ProxyURL  string
	ProxyType string
}

type Movie struct {
	ID            int64   `json:"id"`
	Title         string  `json:"title"`
	OriginalTitle string  `json:"original_title"`
	Overview      string  `json:"overview"`
	ReleaseDate   string  `json:"release_date"`
	PosterPath    string  `json:"poster_path"`
	BackdropPath  string  `json:"backdrop_path"`
	VoteAverage   float64 `json:"vote_average"`
	GenreIDs      []int   `json:"genre_ids"`
	Popularity    float64 `json:"popularity"`
}

type TV struct {
	ID           int64   `json:"id"`
	Name         string  `json:"name"`
	OriginalName string  `json:"original_name"`
	Overview     string  `json:"overview"`
	FirstAirDate string  `json:"first_air_date"`
	PosterPath   string  `json:"poster_path"`
	BackdropPath string  `json:"backdrop_path"`
	VoteAverage  float64 `json:"vote_average"`
	GenreIDs     []int   `json:"genre_ids"`
	Popularity   float64 `json:"popularity"`
}

type PopularMoviesResponse struct {
	Page         int     `json:"page"`
	Results      []Movie `json:"results"`
	TotalPages   int     `json:"total_pages"`
	TotalResults int     `json:"total_results"`
}

type PopularTVsResponse struct {
	Page         int  `json:"page"`
	Results      []TV `json:"results"`
	TotalPages   int  `json:"total_pages"`
	TotalResults int  `json:"total_results"`
}

type service struct {
	config  *Config
	logger  *zap.Logger
	client  *http.Client
	baseURL string
}

func sanitizeTMDBError(err error) string {
	if err == nil {
		return ""
	}

	message := err.Error()
	message = tmdbLogURLPattern.ReplaceAllStringFunc(message, utils.RedactURLForLog)

	return utils.RedactSensitiveText(message)
}

func NewService(logger *zap.Logger, apiKey, baseURL, proxyURL, proxyType string) Service {
	if logger == nil {
		logger = zap.NewNop()
	}

	baseURL = normalizeTMDBBaseURL(baseURL)

	s := &service{
		config: &Config{
			APIKey:    apiKey,
			BaseURL:   baseURL,
			Language:  "zh-CN",
			Region:    "CN",
			ProxyURL:  proxyURL,
			ProxyType: proxyType,
		},
		logger: logger,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
		baseURL: baseURL,
	}

	if s.config.APIKey == "" {
		s.config.APIKey = os.Getenv("TMDB_API_KEY")
	}

	s.setupProxy()

	return s
}

func normalizeTMDBBaseURL(baseURL string) string {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if baseURL == "" {
		return defaultTMDBBaseURL
	}

	return baseURL
}

func (s *service) setupProxy() {
	if s.config.ProxyURL == "" {
		s.config.ProxyURL = os.Getenv("TG_PROXY")
		s.config.ProxyType = os.Getenv("TG_PROXY_TYPE")
	}

	if s.config.ProxyURL == "" {
		return
	}

	proxyURL, err := url.Parse(s.config.ProxyURL)
	if err != nil {
		s.logger.Warn("Failed to parse proxy URL", zap.String("proxy", utils.RedactURLForLog(s.config.ProxyURL)), zap.String("error", sanitizeTMDBError(err)))

		return
	}

	transport := proxyHTTPTransport(s.client.Transport)
	transport.Proxy = http.ProxyURL(proxyURL)
	s.client.Transport = transport
	s.logger.Info("TMDB proxy configured", zap.String("proxy", utils.RedactURLForLog(s.config.ProxyURL)))
}

func proxyHTTPTransport(base http.RoundTripper) *http.Transport {
	if transport, ok := base.(*http.Transport); ok && transport != nil {
		return transport.Clone()
	}

	if transport, ok := http.DefaultTransport.(*http.Transport); ok {
		return transport.Clone()
	}

	return &http.Transport{}
}

func (s *service) GetConfig() *Config {
	return s.config
}

func (s *service) SetAPIKey(apiKey string) {
	s.config.APIKey = apiKey
	s.logger.Info("TMDB API key updated", zap.Bool("has_api_key", strings.TrimSpace(apiKey) != ""))
}

func decodeLimitedTMDBResponse(body io.Reader, target interface{}) error {
	data, err := io.ReadAll(io.LimitReader(body, maxTMDBResponseSize+1))
	if err != nil {
		return err
	}

	if len(data) > maxTMDBResponseSize {
		return fmt.Errorf("tmdb 响应体过大，已拒绝")
	}

	return json.Unmarshal(data, target)
}

func buildTMDBAPIURL(baseURL, path string, query url.Values) string {
	return strings.TrimRight(baseURL, "/") + path + "?" + query.Encode()
}

func (s *service) GetPopularMovies(page int) ([]Movie, error) {
	return s.GetPopularMoviesWithContext(context.Background(), page)
}

func (s *service) GetPopularMoviesWithContext(ctx context.Context, page int) ([]Movie, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	if s.config.APIKey == "" {
		return nil, fmt.Errorf("tmdb API key is not set")
	}

	apiURL := buildTMDBAPIURL(s.baseURL, "/movie/popular", url.Values{
		"api_key":  {s.config.APIKey},
		"language": {s.config.Language},
		"page":     {strconv.Itoa(page)},
		"region":   {s.config.Region},
	})

	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("创建 TMDB 请求失败: %s", sanitizeTMDBError(err))
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("TMDB 请求失败: %s", sanitizeTMDBError(err))
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("tmdb API returned status: %d", resp.StatusCode)
	}

	var result PopularMoviesResponse
	if err := decodeLimitedTMDBResponse(resp.Body, &result); err != nil {
		return nil, err
	}

	return result.Results, nil
}

func (s *service) GetPopularTVs(page int) ([]TV, error) {
	return s.GetPopularTVsWithContext(context.Background(), page)
}

func (s *service) GetPopularTVsWithContext(ctx context.Context, page int) ([]TV, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	if s.config.APIKey == "" {
		return nil, fmt.Errorf("tmdb API key is not set")
	}

	apiURL := buildTMDBAPIURL(s.baseURL, "/tv/popular", url.Values{
		"api_key":  {s.config.APIKey},
		"language": {s.config.Language},
		"page":     {strconv.Itoa(page)},
		"region":   {s.config.Region},
	})

	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("创建 TMDB 请求失败: %s", sanitizeTMDBError(err))
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("TMDB 请求失败: %s", sanitizeTMDBError(err))
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("tmdb API returned status: %d", resp.StatusCode)
	}

	var result PopularTVsResponse
	if err := decodeLimitedTMDBResponse(resp.Body, &result); err != nil {
		return nil, err
	}

	return result.Results, nil
}

func (s *service) GetGenreList() ([]Genre, error) {
	return s.GetGenreListWithContext(context.Background())
}

func (s *service) GetGenreListWithContext(ctx context.Context) ([]Genre, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	if s.config.APIKey == "" {
		return nil, fmt.Errorf("tmdb API key is not set")
	}

	apiURL := buildTMDBAPIURL(s.baseURL, "/genre/movie/list", url.Values{
		"api_key":  {s.config.APIKey},
		"language": {s.config.Language},
	})

	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("创建 TMDB 请求失败: %s", sanitizeTMDBError(err))
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("TMDB 请求失败: %s", sanitizeTMDBError(err))
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("tmdb API returned status: %d", resp.StatusCode)
	}

	var result GenreListResponse
	if err := decodeLimitedTMDBResponse(resp.Body, &result); err != nil {
		return nil, err
	}

	return result.Genres, nil
}

func (s *service) GetMoviesByGenre(genreID int, page int) ([]Movie, error) {
	return s.GetMoviesByGenreWithContext(context.Background(), genreID, page)
}

func (s *service) GetMoviesByGenreWithContext(ctx context.Context, genreID int, page int) ([]Movie, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	if s.config.APIKey == "" {
		return nil, fmt.Errorf("tmdb API key is not set")
	}

	apiURL := buildTMDBAPIURL(s.baseURL, "/discover/movie", url.Values{
		"api_key":     {s.config.APIKey},
		"language":    {s.config.Language},
		"page":        {strconv.Itoa(page)},
		"region":      {s.config.Region},
		"sort_by":     {"release_date.desc"},
		"with_genres": {strconv.Itoa(genreID)},
	})

	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("创建 TMDB 请求失败: %s", sanitizeTMDBError(err))
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("TMDB 请求失败: %s", sanitizeTMDBError(err))
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("tmdb API returned status: %d", resp.StatusCode)
	}

	var result PopularMoviesResponse
	if err := decodeLimitedTMDBResponse(resp.Body, &result); err != nil {
		return nil, err
	}

	return result.Results, nil
}

func (s *service) GetTVsByGenre(genreID int, page int) ([]TV, error) {
	return s.GetTVsByGenreWithContext(context.Background(), genreID, page)
}

func (s *service) GetTVsByGenreWithContext(ctx context.Context, genreID int, page int) ([]TV, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	if s.config.APIKey == "" {
		return nil, fmt.Errorf("tmdb API key is not set")
	}

	apiURL := buildTMDBAPIURL(s.baseURL, "/discover/tv", url.Values{
		"api_key":     {s.config.APIKey},
		"language":    {s.config.Language},
		"page":        {strconv.Itoa(page)},
		"region":      {s.config.Region},
		"sort_by":     {"first_air_date.desc"},
		"with_genres": {strconv.Itoa(genreID)},
	})

	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("创建 TMDB 请求失败: %s", sanitizeTMDBError(err))
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("TMDB 请求失败: %s", sanitizeTMDBError(err))
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("tmdb API returned status: %d", resp.StatusCode)
	}

	var result PopularTVsResponse
	if err := decodeLimitedTMDBResponse(resp.Body, &result); err != nil {
		return nil, err
	}

	return result.Results, nil
}
