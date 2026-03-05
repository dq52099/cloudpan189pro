package tmdb

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"time"

	"go.uber.org/zap"
)

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

func NewService(logger *zap.Logger, apiKey, baseURL, proxyURL, proxyType string) Service {
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
		baseURL: "https://api.themoviedb.org/3",
	}

	if s.config.APIKey == "" {
		s.config.APIKey = os.Getenv("TMDB_API_KEY")
	}

	s.setupProxy()

	return s
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
		s.logger.Warn("Failed to parse proxy URL", zap.String("proxy", s.config.ProxyURL), zap.Error(err))
		return
	}

	s.client.Transport = &http.Transport{
		Proxy: http.ProxyURL(proxyURL),
	}
	s.logger.Info("TMDB proxy configured", zap.String("proxy", s.config.ProxyURL))
}

func (s *service) GetConfig() *Config {
	return s.config
}

func (s *service) SetAPIKey(apiKey string) {
	s.config.APIKey = apiKey
	s.logger.Info("TMDB API key updated", zap.String("apiKey", apiKey[:min(10, len(apiKey))]+"..."))
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func (s *service) GetPopularMovies(page int) ([]Movie, error) {
	if s.config.APIKey == "" {
		return nil, fmt.Errorf("TMDB API key is not set")
	}

	apiURL := fmt.Sprintf("%s/movie/popular?api_key=%s&language=%s&region=%s&page=%d",
		s.baseURL, s.config.APIKey, s.config.Language, s.config.Region, page)

	req, err := http.NewRequestWithContext(context.Background(), "GET", apiURL, nil)
	if err != nil {
		return nil, err
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("TMDB API returned status: %d", resp.StatusCode)
	}

	var result PopularMoviesResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return result.Results, nil
}

func (s *service) GetPopularTVs(page int) ([]TV, error) {
	if s.config.APIKey == "" {
		return nil, fmt.Errorf("TMDB API key is not set")
	}

	apiURL := fmt.Sprintf("%s/tv/popular?api_key=%s&language=%s&region=%s&page=%d",
		s.baseURL, s.config.APIKey, s.config.Language, s.config.Region, page)

	req, err := http.NewRequestWithContext(context.Background(), "GET", apiURL, nil)
	if err != nil {
		return nil, err
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("TMDB API returned status: %d", resp.StatusCode)
	}

	var result PopularTVsResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return result.Results, nil
}

func (s *service) GetGenreList() ([]Genre, error) {
	if s.config.APIKey == "" {
		return nil, fmt.Errorf("TMDB API key is not set")
	}

	apiURL := fmt.Sprintf("%s/genre/movie/list?api_key=%s&language=%s",
		s.baseURL, s.config.APIKey, s.config.Language)

	req, err := http.NewRequestWithContext(context.Background(), "GET", apiURL, nil)
	if err != nil {
		return nil, err
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("TMDB API returned status: %d", resp.StatusCode)
	}

	var result GenreListResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return result.Genres, nil
}

var genreIDMap = map[string]int{
	"movie_28":    28,
	"movie_12":    12,
	"movie_16":    16,
	"movie_35":    35,
	"movie_80":    80,
	"movie_99":    99,
	"movie_18":    18,
	"movie_10751": 10751,
	"movie_14":    14,
	"movie_36":    36,
	"movie_27":    27,
	"movie_10402": 10402,
	"movie_9648":  9648,
	"movie_878":   878,
	"movie_10770": 10770,
	"movie_53":    53,
	"movie_10752": 10752,
	"movie_37":    37,
	"tv_10759":    10759,
	"tv_16":       16,
	"tv_35":       35,
	"tv_80":       80,
	"tv_99":       99,
	"tv_18":       18,
	"tv_10751":    10751,
	"tv_10762":    10762,
	"tv_9648":     9648,
	"tv_10763":    10763,
	"tv_10764":    10764,
	"tv_10765":    10765,
	"tv_10766":    10766,
	"tv_10767":    10767,
	"tv_10768":    10768,
	"tv_37":       37,
}

func (s *service) GetMoviesByGenre(genreID int, page int) ([]Movie, error) {
	if s.config.APIKey == "" {
		return nil, fmt.Errorf("TMDB API key is not set")
	}

	apiURL := fmt.Sprintf("%s/discover/movie?api_key=%s&language=%s&region=%s&with_genres=%d&sort_by=release_date.desc&page=%d",
		s.baseURL, s.config.APIKey, s.config.Language, s.config.Region, genreID, page)

	req, err := http.NewRequestWithContext(context.Background(), "GET", apiURL, nil)
	if err != nil {
		return nil, err
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("TMDB API returned status: %d", resp.StatusCode)
	}

	var result PopularMoviesResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return result.Results, nil
}

func (s *service) GetTVsByGenre(genreID int, page int) ([]TV, error) {
	if s.config.APIKey == "" {
		return nil, fmt.Errorf("TMDB API key is not set")
	}

	apiURL := fmt.Sprintf("%s/discover/tv?api_key=%s&language=%s&region=%s&with_genres=%d&sort_by=first_air_date.desc&page=%d",
		s.baseURL, s.config.APIKey, s.config.Language, s.config.Region, genreID, page)

	req, err := http.NewRequestWithContext(context.Background(), "GET", apiURL, nil)
	if err != nil {
		return nil, err
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("TMDB API returned status: %d", resp.StatusCode)
	}

	var result PopularTVsResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return result.Results, nil
}
