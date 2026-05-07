package subscription

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/xxcheng123/cloudpan189-share/internal/framework/httpcontext"
	"github.com/xxcheng123/cloudpan189-share/internal/repository/models"
	"github.com/xxcheng123/cloudpan189-share/internal/services/cloudbridge"
	"github.com/xxcheng123/cloudpan189-share/internal/services/douban"
	"github.com/xxcheng123/cloudpan189-share/internal/services/storagefacade"
	"github.com/xxcheng123/cloudpan189-share/internal/services/tmdb"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

var shareCodeRegex = regexp.MustCompile(`/t/([a-zA-Z0-9]+)`)

type Handler struct {
	db                   *gorm.DB
	tmdb                 tmdb.Service
	douban               douban.Service
	logger               *zap.Logger
	storageFacadeService storagefacade.Service
	cloudBridgeService   cloudbridge.Service
	httpClient           *http.Client
	openaiSvc            interface {
		GenerateUpgradeKeyword(title, category string) (string, error)
	}
	tmdbAPIKey string
}

func NewHandler(db *gorm.DB, tmdbSvc tmdb.Service, doubanSvc douban.Service, storageSvc storagefacade.Service, cloudBridgeSvc cloudbridge.Service, logger *zap.Logger, openaiSvc interface {
	GenerateUpgradeKeyword(title, category string) (string, error)
}) *Handler {
	if err := db.AutoMigrate(&Setting{}); err != nil {
		logger.Error("自动迁移订阅 Setting 失败", zap.Error(err))
	}

	logger.Info("Creating subscription handler", zap.Any("storageSvc", storageSvc != nil))

	var tmdbAPIKey string
	if tmdbSvc != nil {
		cfg := tmdbSvc.GetConfig()
		if cfg != nil {
			tmdbAPIKey = cfg.APIKey
		}
	}

	var setting Setting
	if err := db.Where("name = ?", "subscription_config").First(&setting).Error; err == nil && setting.Value.TMDBAPIKey != "" {
		if tmdbSvc != nil {
			tmdbSvc.SetAPIKey(setting.Value.TMDBAPIKey)
		}
		tmdbAPIKey = setting.Value.TMDBAPIKey
	}

	return &Handler{
		db:                   db,
		tmdb:                 tmdbSvc,
		douban:               doubanSvc,
		storageFacadeService: storageSvc,
		cloudBridgeService:   cloudBridgeSvc,
		logger:               logger,
		httpClient:           &http.Client{Timeout: 120 * time.Second},
		openaiSvc:            openaiSvc,
		tmdbAPIKey:           tmdbAPIKey,
	}
}

func invalidParams(err error) httpcontext.BusinessError {
	return &customBusinessError{
		httpCode:     http.StatusBadRequest,
		businessCode: 400,
		message:      err.Error(),
	}
}

type customBusinessError struct {
	httpCode     int
	businessCode int
	message      string
	cause        error
}

func (e *customBusinessError) GetHTTPCode() int   { return e.httpCode }
func (e *customBusinessError) GetCode() int       { return e.businessCode }
func (e *customBusinessError) GetMessage() string { return e.message }
func (e *customBusinessError) GetError() error    { return e.cause }
func (e *customBusinessError) Error() string      { return e.message }
func (e *customBusinessError) WithError(err error) httpcontext.BusinessError {
	e.cause = err
	return e
}
func (e *customBusinessError) WithHTTPCode(code int) httpcontext.BusinessError {
	e.httpCode = code
	return e
}
func (e *customBusinessError) WithMessage(msg string) httpcontext.BusinessError {
	e.message = msg
	return e
}
func (e *customBusinessError) WithBusinessCode(code int) httpcontext.BusinessError {
	e.businessCode = code
	return e
}

type HotMovieItem struct {
	ID            int64   `json:"id"`
	Title         string  `json:"title"`
	OriginalTitle string  `json:"originalTitle"`
	Year          string  `json:"year"`
	Rating        float64 `json:"rating"`
	Cover         string  `json:"cover"`
	PosterPath    string  `json:"posterPath"`
	Type          string  `json:"type"`
	Description   string  `json:"description"`
	Category      string  `json:"category,omitempty"`
}

type HotMoviesResponse struct {
	Movies   []HotMovieItem `json:"movies"`
	Source   string         `json:"source"`
	Category string         `json:"category"`
}

type CategoryOption struct {
	Value    string           `json:"value"`
	Label    string           `json:"label"`
	Children []CategoryOption `json:"children,omitempty"`
}

type CategoriesResponse struct {
	TMDB   []CategoryOption `json:"tmdb"`
	Douban []CategoryOption `json:"douban"`
}

func (h *Handler) GetCategories() httpcontext.HandlerFunc {
	return func(c *httpcontext.Context) {
		tmdbCategories := []CategoryOption{
			{
				Value: "movie",
				Label: "电影",
				Children: []CategoryOption{
					{Value: "movie_popular", Label: "热门电影"},
					{Value: "movie_toprated", Label: "高分电影"},
					{Value: "movie_nowplaying", Label: "正在热映"},
					{Value: "movie_upcoming", Label: "即将上映"},
				},
			},
			{
				Value: "tv",
				Label: "电视剧",
				Children: []CategoryOption{
					{Value: "tv_popular", Label: "热门剧集"},
					{Value: "tv_toprated", Label: "高分剧集"},
					{Value: "tv_airing", Label: "正在播出"},
				},
			},
			{
				Value: "anime",
				Label: "动漫",
				Children: []CategoryOption{
					{Value: "anime_popular", Label: "热门动漫"},
					{Value: "anime_toprated", Label: "高分动漫"},
				},
			},
			{
				Value: "doc",
				Label: "纪录片",
				Children: []CategoryOption{
					{Value: "doc_popular", Label: "热门纪录片"},
					{Value: "doc_toprated", Label: "高分纪录片"},
				},
			},
		}

		doubanCategories := []CategoryOption{
			{
				Value: "all",
				Label: "综合",
				Children: []CategoryOption{
					{Value: "热门", Label: "热门"},
					{Value: "最新", Label: "最新"},
					{Value: "经典", Label: "经典"},
					{Value: "豆瓣高分", Label: "豆瓣高分"},
					{Value: "冷门佳片", Label: "冷门佳片"},
				},
			},
			{
				Value: "chinese",
				Label: "华语",
				Children: []CategoryOption{
					{Value: "华语", Label: "华语热门"},
					{Value: "华语经典", Label: "华语经典"},
					{Value: "华语高分", Label: "华语高分"},
				},
			},
			{
				Value: "western",
				Label: "欧美",
				Children: []CategoryOption{
					{Value: "欧美", Label: "欧美热门"},
					{Value: "欧美经典", Label: "欧美经典"},
					{Value: "欧美高分", Label: "欧美高分"},
				},
			},
			{
				Value: "korean",
				Label: "韩国",
				Children: []CategoryOption{
					{Value: "韩国", Label: "韩国热门"},
					{Value: "韩国经典", Label: "韩国经典"},
					{Value: "韩国高分", Label: "韩国高分"},
				},
			},
			{
				Value: "japanese",
				Label: "日本",
				Children: []CategoryOption{
					{Value: "日本", Label: "日本热门"},
					{Value: "日本经典", Label: "日本经典"},
					{Value: "日本高分", Label: "日本高分"},
				},
			},
			{
				Value: "genre",
				Label: "类型",
				Children: []CategoryOption{
					{Value: "动作", Label: "动作"},
					{Value: "喜剧", Label: "喜剧"},
					{Value: "爱情", Label: "爱情"},
					{Value: "科幻", Label: "科幻"},
					{Value: "动画", Label: "动画"},
					{Value: "悬疑", Label: "悬疑"},
					{Value: "惊悚", Label: "惊悚"},
					{Value: "恐怖", Label: "恐怖"},
				},
			},
		}

		c.Success(CategoriesResponse{
			TMDB:   tmdbCategories,
			Douban: doubanCategories,
		})
	}
}

var tmdbCategoryGenreMap = map[string]struct {
	genreType string
	genreID   int
}{
	"movie_popular":    {genreType: "movie", genreID: 0},
	"movie_toprated":   {genreType: "movie", genreID: 0},
	"movie_nowplaying": {genreType: "movie", genreID: 0},
	"movie_upcoming":   {genreType: "movie", genreID: 0},
	"tv_popular":       {genreType: "tv", genreID: 0},
	"tv_toprated":      {genreType: "tv", genreID: 0},
	"tv_airing":        {genreType: "tv", genreID: 0},
	"anime_popular":    {genreType: "movie", genreID: 16},
	"anime_toprated":   {genreType: "movie", genreID: 16},
	"doc_popular":      {genreType: "movie", genreID: 99},
	"doc_toprated":     {genreType: "movie", genreID: 99},
}

func (h *Handler) GetTMDbMovies() httpcontext.HandlerFunc {
	return func(c *httpcontext.Context) {
		if h.tmdb == nil {
			c.Fail(invalidParams(fmt.Errorf("TMDB 服务未初始化，请检查配置")))
			return
		}

		category := c.DefaultQuery("category", "movie_popular")
		h.logger.Info("[热门数据加载] 正在获取TMDB热门数据", zap.String("category", category))

		var items interface{}
		var err error

		catInfo, ok := tmdbCategoryGenreMap[category]
		if !ok {
			catInfo = tmdbCategoryGenreMap["movie_popular"]
		}

		if catInfo.genreID > 0 {
			items, err = h.tmdb.GetMoviesByGenre(catInfo.genreID, 1)
		} else if category == "movie_popular" || category == "movie_toprated" || category == "movie_nowplaying" || category == "movie_upcoming" {
			items, err = h.tmdb.GetPopularMovies(1)
		} else if category == "tv_popular" || category == "tv_toprated" || category == "tv_airing" {
			items, err = h.tmdb.GetPopularTVs(1)
		} else {
			items, err = h.tmdb.GetPopularMovies(1)
		}

		if err != nil {
			h.logger.Error("failed to get TMDB movies", zap.Error(err))
			c.Fail(invalidParams(fmt.Errorf("获取TMDB热门电影失败: %w", err)))
			return
		}

		movies := make([]HotMovieItem, 0)
		if movieList, ok := items.([]tmdb.Movie); ok {
			for _, m := range movieList {
				year := ""
				if len(m.ReleaseDate) >= 4 {
					year = m.ReleaseDate[:4]
				}
				movies = append(movies, HotMovieItem{
					ID:            m.ID,
					Title:         m.Title,
					OriginalTitle: m.OriginalTitle,
					Year:          year,
					Rating:        m.VoteAverage,
					Cover:         "https://image.tmdb.org/t/p/w500" + m.PosterPath,
					PosterPath:    m.PosterPath,
					Type:          "movie",
					Description:   m.Overview,
					Category:      category,
				})
			}
		} else if tvList, ok := items.([]tmdb.TV); ok {
			for _, m := range tvList {
				year := ""
				if len(m.FirstAirDate) >= 4 {
					year = m.FirstAirDate[:4]
				}
				movies = append(movies, HotMovieItem{
					ID:            m.ID,
					Title:         m.Name,
					OriginalTitle: m.OriginalName,
					Year:          year,
					Rating:        m.VoteAverage,
					Cover:         "https://image.tmdb.org/t/p/w500" + m.PosterPath,
					PosterPath:    m.PosterPath,
					Type:          "tv",
					Description:   m.Overview,
					Category:      category,
				})
			}
		}

		h.logger.Info("[热门数据加载] TMDB热门数据加载完成", zap.Int("count", len(movies)), zap.String("category", category))
		c.Success(HotMoviesResponse{
			Movies:   movies,
			Source:   "tmdb",
			Category: category,
		})
	}
}

func (h *Handler) GetTMDbTVs() httpcontext.HandlerFunc {
	return func(c *httpcontext.Context) {
		if h.tmdb == nil {
			c.Fail(invalidParams(fmt.Errorf("TMDB 服务未初始化，请检查配置")))
			return
		}

		category := c.DefaultQuery("category", "tv_popular")
		h.logger.Info("[热门数据加载] 正在获取TMDB热门电视剧", zap.String("category", category))

		items, err := h.tmdb.GetPopularTVs(1)
		if err != nil {
			h.logger.Error("failed to get TMDB TVs", zap.Error(err))
			c.Fail(invalidParams(fmt.Errorf("获取TMDB热门电视剧失败: %w", err)))
			return
		}

		movies := make([]HotMovieItem, 0, len(items))
		for _, m := range items {
			year := ""
			if len(m.FirstAirDate) >= 4 {
				year = m.FirstAirDate[:4]
			}
			movies = append(movies, HotMovieItem{
				ID:            m.ID,
				Title:         m.Name,
				OriginalTitle: m.OriginalName,
				Year:          year,
				Rating:        m.VoteAverage,
				Cover:         "https://image.tmdb.org/t/p/w500" + m.PosterPath,
				PosterPath:    m.PosterPath,
				Type:          "tv",
				Description:   m.Overview,
				Category:      category,
			})
		}

		h.logger.Info("[热门数据加载] TMDB热门电视剧加载完成", zap.Int("count", len(movies)), zap.String("category", category))
		c.Success(HotMoviesResponse{
			Movies:   movies,
			Source:   "tmdb_tv",
			Category: category,
		})
	}
}

func (h *Handler) GetDoubanMovies() httpcontext.HandlerFunc {
	return func(c *httpcontext.Context) {
		if h.douban == nil {
			c.Fail(invalidParams(fmt.Errorf("豆瓣服务未初始化，请检查配置")))
			return
		}

		tag := c.DefaultQuery("category", "热门")
		h.logger.Info("[热门数据加载] 正在获取豆瓣热门数据", zap.String("category", tag))

		items, err := h.douban.GetMoviesByTag(tag)
		if err != nil {
			h.logger.Error("failed to get Douban movies", zap.Error(err))
			c.Fail(invalidParams(fmt.Errorf("获取豆瓣热门电影失败: %w", err)))
			return
		}

		movies := make([]HotMovieItem, 0, len(items))
		for _, m := range items {
			movies = append(movies, HotMovieItem{
				ID:            0,
				Title:         m.Title,
				OriginalTitle: m.OriginalTitle,
				Year:          m.Year,
				Rating:        m.Rating,
				Cover:         m.Cover,
				PosterPath:    "",
				Type:          "movie",
				Description:   "",
				Category:      tag,
			})
		}

		h.logger.Info("[热门数据加载] 豆瓣热门数据加载完成", zap.Int("count", len(movies)), zap.String("category", tag))
		c.Success(HotMoviesResponse{
			Movies:   movies,
			Source:   "douban",
			Category: tag,
		})
	}
}

type SubscriptionConfig struct {
	EnableTMDB       bool   `json:"enableTMDB"`
	EnableDouban     bool   `json:"enableDouban"`
	PanSearchURL     string `json:"panSearchURL"`
	DefaultMountPath string `json:"defaultMountPath"`
	AutoMount        bool   `json:"autoMount"`
	CronExpression   string `json:"cronExpression"`
	TMDBAPIKey       string `json:"tmdbAPIKey"`
	OpenAIAPIKey     string `json:"openaiAPIKey"`
	OpenAIBaseURL    string `json:"openaiBaseURL"`
	OpenAIModel      string `json:"openaiModel"`
}

type Setting struct {
	ID    int64                     `gorm:"primaryKey" json:"id"`
	Name  string                    `gorm:"column:name;type:varchar(255)" json:"name"`
	Value models.SubscriptionConfig `gorm:"column:value;type:json" json:"value"`
}

func (s Setting) TableName() string {
	return "system_settings"
}

func (h *Handler) GetConfig() httpcontext.HandlerFunc {
	return func(c *httpcontext.Context) {
		var setting Setting
		result := h.db.Where("name = ?", "subscription_config").First(&setting)
		if result.Error != nil && !errors.Is(result.Error, gorm.ErrRecordNotFound) {
			c.Fail(invalidParams(result.Error))
			return
		}

		if setting.ID == 0 {
			setting = Setting{
				Name: "subscription_config",
				Value: models.SubscriptionConfig{
					Enabled:          true,
					CronExpression:   "0 2 * * *",
					PanSearchURL:     "https://so.252035.xyz/api/search",
					EnableTMDB:       true,
					EnableDouban:     true,
					DefaultMountPath: "/热门订阅",
					AutoMount:        false,
					TMDBAPIKey:       h.tmdbAPIKey,
					OpenAIBaseURL:    "https://api.openai.com",
					OpenAIModel:      "gpt-4o-mini",
				},
			}
			h.db.Create(&setting)
			config := SubscriptionConfig{
				EnableTMDB:       setting.Value.EnableTMDB,
				EnableDouban:     setting.Value.EnableDouban,
				PanSearchURL:     setting.Value.PanSearchURL,
				DefaultMountPath: setting.Value.DefaultMountPath,
				AutoMount:        setting.Value.AutoMount,
				CronExpression:   setting.Value.CronExpression,
				TMDBAPIKey:       setting.Value.TMDBAPIKey,
				OpenAIAPIKey:     setting.Value.OpenAIAPIKey,
				OpenAIBaseURL:    setting.Value.OpenAIBaseURL,
				OpenAIModel:      setting.Value.OpenAIModel,
			}
			if config.TMDBAPIKey == "" {
				config.TMDBAPIKey = h.tmdbAPIKey
			}
			c.Success(config)
			return
		}

		config := SubscriptionConfig{
			EnableTMDB:       setting.Value.EnableTMDB,
			EnableDouban:     setting.Value.EnableDouban,
			PanSearchURL:     setting.Value.PanSearchURL,
			DefaultMountPath: setting.Value.DefaultMountPath,
			AutoMount:        setting.Value.AutoMount,
			CronExpression:   setting.Value.CronExpression,
			TMDBAPIKey:       setting.Value.TMDBAPIKey,
			OpenAIAPIKey:     setting.Value.OpenAIAPIKey,
			OpenAIBaseURL:    setting.Value.OpenAIBaseURL,
			OpenAIModel:      setting.Value.OpenAIModel,
		}
		if config.TMDBAPIKey == "" {
			config.TMDBAPIKey = h.tmdbAPIKey
		}
		c.Success(config)
	}
}

type UpdateConfigReq struct {
	EnableTMDB       bool   `json:"enableTMDB"`
	EnableDouban     bool   `json:"enableDouban"`
	PanSearchURL     string `json:"panSearchURL"`
	DefaultMountPath string `json:"defaultMountPath"`
	AutoMount        bool   `json:"autoMount"`
	CronExpression   string `json:"cronExpression"`
	TMDBAPIKey       string `json:"tmdbAPIKey"`
	OpenAIAPIKey     string `json:"openaiAPIKey"`
	OpenAIBaseURL    string `json:"openaiBaseURL"`
	OpenAIModel      string `json:"openaiModel"`
}

func (h *Handler) UpdateConfig() httpcontext.HandlerFunc {
	return func(c *httpcontext.Context) {
		var req UpdateConfigReq
		if err := c.ShouldBindJSON(&req); err != nil {
			c.Fail(invalidParams(err))
			return
		}

		var setting Setting
		result := h.db.Where("name = ?", "subscription_config").First(&setting)
		if result.Error != nil && !errors.Is(result.Error, gorm.ErrRecordNotFound) {
			c.Fail(invalidParams(result.Error))
			return
		}

		setting.Name = "subscription_config"
		setting.Value = models.SubscriptionConfig{
			EnableTMDB:       req.EnableTMDB,
			EnableDouban:     req.EnableDouban,
			PanSearchURL:     req.PanSearchURL,
			CronExpression:   req.CronExpression,
			DefaultMountPath: req.DefaultMountPath,
			AutoMount:        req.AutoMount,
			TMDBAPIKey:       req.TMDBAPIKey,
			OpenAIAPIKey:     req.OpenAIAPIKey,
			OpenAIBaseURL:    req.OpenAIBaseURL,
			OpenAIModel:      req.OpenAIModel,
		}

		if setting.ID == 0 {
			result = h.db.Create(&setting)
		} else {
			result = h.db.Save(&setting)
		}

		if result.Error != nil {
			c.Fail(invalidParams(result.Error))
			return
		}

		if req.TMDBAPIKey != "" && h.tmdb != nil {
			h.tmdb.SetAPIKey(req.TMDBAPIKey)
			h.tmdbAPIKey = req.TMDBAPIKey
			h.logger.Info("Updated TMDB API Key from subscription config")
		}

		c.Success(req)
	}
}

type SearchResult struct {
	ShareURL   string `json:"shareUrl"`
	ShareCode  string `json:"shareCode"`
	Name       string `json:"name"`
	Size       string `json:"size"`
	UploadTime string `json:"uploadTime"`
	Source     string `json:"source"`
	Cover      string `json:"cover,omitempty"`
	Note       string `json:"note,omitempty"`
}

func (h *Handler) SearchPan() httpcontext.HandlerFunc {
	return func(c *httpcontext.Context) {
		keyword := c.Query("keyword")
		if keyword == "" {
			c.Fail(invalidParams(fmt.Errorf("关键词不能为空")))
			return
		}

		panSearchURL := "https://so.252035.xyz/api/search"
		var setting Setting
		if err := h.db.Where("name = ?", "subscription_config").First(&setting).Error; err == nil {
			if setting.Value.PanSearchURL != "" {
				panSearchURL = setting.Value.PanSearchURL
			}
		}

		searchURL := panSearchURL
		if !strings.Contains(panSearchURL, "/api/search") {
			searchURL = strings.TrimRight(panSearchURL, "/") + "/api/search"
		}
		searchURL = fmt.Sprintf("%s?kw=%s&cloud_types=tianyi", searchURL, url.QueryEscape(keyword))

		h.logger.Info("Searching pan", zap.String("url", searchURL))

		req, err := http.NewRequestWithContext(c.Request.Context(), "GET", searchURL, nil)
		if err != nil {
			c.Fail(invalidParams(fmt.Errorf("请求失败: %w", err)))
			return
		}

		req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
		req.Header.Set("Accept", "application/json, text/plain, */*")
		req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8")
		req.Header.Set("Referer", "https://so.252035.xyz/")

		// 使用代理（如果配置了）
		client := h.httpClient
		proxyURL := os.Getenv("TG_PROXY")
		if proxyURL != "" {
			if proxyURL_, err := url.Parse(proxyURL); err == nil {
				client = &http.Client{
					Timeout:   h.httpClient.Timeout,
					Transport: &http.Transport{Proxy: http.ProxyURL(proxyURL_)},
				}
				h.logger.Info("Using proxy for pan search", zap.String("proxy", proxyURL))
			}
		}

		resp, err := client.Do(req)
		if err != nil {
			h.logger.Error("Pan search request failed", zap.Error(err))
			if strings.Contains(err.Error(), "context canceled") {
				c.Fail(invalidParams(fmt.Errorf("请求超时，请稍后重试")))
			} else {
				c.Fail(invalidParams(fmt.Errorf("搜索服务不可用: %v", err)))
			}
			return
		}
		defer resp.Body.Close()

		// 限制响应体最大 5MB，避免异常大返回耗尽内存
		const maxResponseSize = 5 << 20
		bodyBytes, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseSize+1))
		if err != nil {
			h.logger.Error("Failed to read response body", zap.Error(err))
			c.Fail(invalidParams(fmt.Errorf("读取响应失败: %v", err)))
			return
		}
		if len(bodyBytes) > maxResponseSize {
			c.Fail(invalidParams(fmt.Errorf("盘搜返回体过大，已拒绝")))
			return
		}

		h.logger.Info("Pan search response", zap.Int("status", resp.StatusCode), zap.Int("body_len", len(bodyBytes)))

		if resp.StatusCode != http.StatusOK {
			h.logger.Warn("Pan search returned non-OK status", zap.Int("status", resp.StatusCode), zap.String("url", searchURL))
			c.Fail(invalidParams(fmt.Errorf("搜索接口返回状态: %d，请检查盘搜 API 地址是否正确", resp.StatusCode)))
			return
		}

		type PanSearchResponseV2 struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
			Data    struct {
				Total        int `json:"total"`
				MergedByType map[string][]struct {
					URL      string   `json:"url"`
					Password string   `json:"password"`
					Note     string   `json:"note"`
					Datetime string   `json:"datetime"`
					Source   string   `json:"source"`
					Images   []string `json:"images"`
				} `json:"merged_by_type"`
			} `json:"data"`
		}

		var result PanSearchResponseV2
		if err := json.Unmarshal(bodyBytes, &result); err != nil {
			h.logger.Error("Failed to parse pan search response", zap.Error(err))
			c.Fail(invalidParams(fmt.Errorf("解析响应失败: %w", err)))
			return
		}

		if result.Code != 0 {
			c.Fail(invalidParams(fmt.Errorf("%s", result.Message)))
			return
		}

		var results []SearchResult
		if tianyiData, ok := result.Data.MergedByType["tianyi"]; ok {
			for _, item := range tianyiData {
				matches := shareCodeRegex.FindStringSubmatch(item.URL)
				shareCode := ""
				if len(matches) > 1 {
					shareCode = matches[1]
				}
				cover := ""
				if len(item.Images) > 0 {
					cover = item.Images[0]
				}
				results = append(results, SearchResult{
					ShareURL:   item.URL,
					ShareCode:  shareCode,
					Name:       item.Note,
					UploadTime: item.Datetime,
					Source:     item.Source,
					Cover:      cover,
					Note:       item.Note,
				})
			}
		}

		c.Success(results)
	}
}

func (h *Handler) MountSubscription() httpcontext.HandlerFunc {
	return func(c *httpcontext.Context) {
		var req struct {
			Title     string `json:"title" binding:"required"`
			ShareURL  string `json:"shareUrl" binding:"required"`
			ShareCode string `json:"shareCode"`
			Cover     string `json:"cover"`
			MountPath string `json:"mountPath"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.Fail(invalidParams(err))
			return
		}

		shareCode := req.ShareCode
		if shareCode == "" {
			matches := shareCodeRegex.FindStringSubmatch(req.ShareURL)
			if len(matches) >= 2 {
				shareCode = matches[1]
			}
		}

		if shareCode == "" {
			c.Fail(invalidParams(fmt.Errorf("无法从分享链接中提取分享码")))
			return
		}

		defaultMountPath := "/热门订阅"
		// 从 settings 中读取用户自定义默认挂载路径
		var cfgSetting Setting
		if err := h.db.Where("name = ?", "subscription_config").First(&cfgSetting).Error; err == nil {
			if cfgSetting.Value.DefaultMountPath != "" {
				defaultMountPath = cfgSetting.Value.DefaultMountPath
			}
		}

		mountPath := req.MountPath
		if mountPath == "" {
			mountPath = defaultMountPath + "/" + req.Title
		}

		if h.storageFacadeService == nil {
			c.Fail(invalidParams(fmt.Errorf("storage service is nil, please restart the application")))
			return
		}

		ctx := c.GetContext()

		if h.cloudBridgeService == nil {
			c.Fail(invalidParams(fmt.Errorf("cloud bridge service is nil, please restart the application")))
			return
		}

		shareInfo, err := h.cloudBridgeService.GetShareInfo(ctx, shareCode, "")
		if err != nil {
			c.Fail(invalidParams(fmt.Errorf("获取分享信息失败: %v", err)))
			return
		}

		storageReq := &storagefacade.CreateStorageRequest{
			LocalPath:     mountPath,
			OsType:        "subscribe_share_folder",
			CloudToken:    0,
			FileId:        shareInfo.ID,
			AllowExisting: true,
		}
		id, err := h.storageFacadeService.CreateStorage(ctx, storageReq)
		if err != nil {
			h.logger.Error("Failed to create storage", zap.Error(err), zap.String("path", mountPath))
			c.Fail(invalidParams(fmt.Errorf("创建挂载点失败: %v", err)))
			return
		}
		c.Success(gin.H{
			"message":   "挂载成功",
			"mountPath": mountPath,
			"shareURL":  req.ShareURL,
			"shareCode": shareCode,
			"fileId":    id,
			"name":      shareInfo.Name,
		})
	}
}

func (h *Handler) SearchPanWithAI() httpcontext.HandlerFunc {
	return func(c *httpcontext.Context) {
		keyword := c.Query("keyword")
		if keyword == "" {
			c.Fail(invalidParams(fmt.Errorf("关键词不能为空")))
			return
		}

		panSearchURL := "https://so.252035.xyz/api/search"
		var setting Setting
		if err := h.db.Where("name = ?", "subscription_config").First(&setting).Error; err == nil {
			if setting.Value.PanSearchURL != "" {
				panSearchURL = setting.Value.PanSearchURL
			}
		}

		searchURL := panSearchURL
		if !strings.Contains(panSearchURL, "/api/search") {
			searchURL = strings.TrimRight(panSearchURL, "/") + "/api/search"
		}
		searchURL = fmt.Sprintf("%s?kw=%s&cloud_types=tianyi", searchURL, url.QueryEscape(keyword))

		h.logger.Info("Searching pan with AI", zap.String("url", searchURL))

		req, err := http.NewRequestWithContext(c.Request.Context(), "GET", searchURL, nil)
		if err != nil {
			c.Fail(invalidParams(fmt.Errorf("请求失败: %w", err)))
			return
		}

		req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
		req.Header.Set("Accept", "application/json, text/plain, */*")
		req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8")
		req.Header.Set("Referer", "https://so.252035.xyz/")

		resp, err := h.httpClient.Do(req)
		if err != nil {
			h.logger.Error("Pan search request failed", zap.Error(err))
			if strings.Contains(err.Error(), "context canceled") {
				c.Fail(invalidParams(fmt.Errorf("请求超时，请稍后重试")))
			} else {
				c.Fail(invalidParams(fmt.Errorf("搜索服务不可用: %v", err)))
			}
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			h.logger.Warn("Pan search returned non-OK status", zap.Int("status", resp.StatusCode))
			c.Fail(invalidParams(fmt.Errorf("搜索接口返回状态: %d", resp.StatusCode)))
			return
		}

		type PanSearchResponseV2 struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
			Data    struct {
				Total        int `json:"total"`
				MergedByType map[string][]struct {
					URL      string   `json:"url"`
					Password string   `json:"password"`
					Note     string   `json:"note"`
					Datetime string   `json:"datetime"`
					Source   string   `json:"source"`
					Images   []string `json:"images"`
				} `json:"merged_by_type"`
			} `json:"data"`
		}

		var searchResult PanSearchResponseV2
		if err := json.NewDecoder(resp.Body).Decode(&searchResult); err != nil {
			h.logger.Error("Failed to parse pan search response", zap.Error(err))
			c.Fail(invalidParams(fmt.Errorf("解析响应失败: %w", err)))
			return
		}

		if searchResult.Code != 0 {
			c.Fail(invalidParams(fmt.Errorf("%s", searchResult.Message)))
			return
		}

		if h.openaiSvc == nil {
			c.Fail(invalidParams(fmt.Errorf("AI服务未配置")))
			return
		}

		var results []SearchResult
		if tianyiData, ok := searchResult.Data.MergedByType["tianyi"]; ok {
			for _, item := range tianyiData {
				matches := shareCodeRegex.FindStringSubmatch(item.URL)
				shareCode := ""
				if len(matches) > 1 {
					shareCode = matches[1]
				}
				cover := ""
				if len(item.Images) > 0 {
					cover = item.Images[0]
				}
				results = append(results, SearchResult{
					ShareURL:   item.URL,
					ShareCode:  shareCode,
					Name:       item.Note,
					UploadTime: item.Datetime,
					Source:     item.Source,
					Cover:      cover,
					Note:       item.Note,
				})
			}
		}

		if len(results) == 0 {
			c.Success(gin.H{
				"message": "未找到相关资源",
				"result":  nil,
			})
			return
		}

		bestKeyword, err := h.openaiSvc.GenerateUpgradeKeyword(keyword, "movie")
		if err != nil {
			h.logger.Warn("Failed to generate AI keyword", zap.Error(err))
			bestKeyword = keyword
		}

		h.logger.Info("AI recommended keyword", zap.String("keyword", bestKeyword))

		var bestResult *SearchResult
		bestScore := 0

		for i := range results {
			score := 0
			name := strings.ToLower(results[i].Name)
			keywordLower := strings.ToLower(bestKeyword)

			if strings.Contains(name, keywordLower) {
				score += 50
			}

			// 名字已经转小写，只匹配小写关键字
			if strings.Contains(name, "4k") {
				score += 20
			}
			if strings.Contains(name, "2160p") {
				score += 20
			}
			if strings.Contains(name, "1080p") {
				score += 15
			}

			if strings.Contains(name, "杜比") || strings.Contains(name, "dolby") {
				score += 10
			}
			if strings.Contains(name, "纯净") || strings.Contains(name, "无广") {
				score += 10
			}
			if strings.Contains(name, "全集") || strings.Contains(name, "完结") {
				score += 10
			}

			if score > bestScore {
				bestScore = score
				bestResult = &results[i]
			}
		}

		if bestResult == nil && len(results) > 0 {
			bestResult = &results[0]
		}

		c.Success(gin.H{
			"message":       "AI推荐成功",
			"keyword":       bestKeyword,
			"aiDescription": "基于资源质量、清晰度、完整性等因素智能推荐",
			"result":        bestResult,
			"allResults":    results,
		})
	}
}
