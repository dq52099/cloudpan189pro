package subscription

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/robfig/cron/v3"
	"github.com/xxcheng123/cloudpan189-share/internal/consts"
	"github.com/xxcheng123/cloudpan189-share/internal/framework/httpcontext"
	"github.com/xxcheng123/cloudpan189-share/internal/repository/models"
	"github.com/xxcheng123/cloudpan189-share/internal/services/cloudbridge"
	"github.com/xxcheng123/cloudpan189-share/internal/services/douban"
	"github.com/xxcheng123/cloudpan189-share/internal/services/storagefacade"
	"github.com/xxcheng123/cloudpan189-share/internal/services/tmdb"
	"go.uber.org/zap"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var shareCodeRegex = regexp.MustCompile(`/t/([a-zA-Z0-9]+)`)

const (
	defaultPanSearchURL          = "https://so.252035.xyz/api/search"
	defaultSubscriptionMountPath = "/热门订阅"
	maxPanSearchResponseSize     = 5 << 20
	subscriptionConfigName       = "subscription_config"
)

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
	tmdbAPIKey           string
	subscriptionConfigMu sync.Mutex
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
	if err := db.Where("name = ?", subscriptionConfigName).First(&setting).Error; err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		logger.Warn("读取订阅配置失败，将使用默认配置", zap.Error(err))
	} else if setting.Value.TMDBAPIKey != "" {
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

func notFound(msg string) httpcontext.BusinessError {
	return &customBusinessError{
		httpCode:     http.StatusNotFound,
		businessCode: 404,
		message:      msg,
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

// GetCategories 获取订阅分类
// @Summary 获取订阅分类
// @Description 获取 TMDB 和豆瓣热门订阅分类配置
// @Tags 订阅管理
// @Accept json
// @Produce json
// @Param Authorization header string true "Bearer token"
// @Success 200 {object} httpcontext.Response{data=CategoriesResponse} "获取成功"
// @Failure 401 {object} httpcontext.Response "未授权访问"
// @Failure 403 {object} httpcontext.Response "权限不足"
// @Router /api/subscription/categories [get]
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

// GetTMDbMovies 获取 TMDB 热门影视
// @Summary 获取 TMDB 热门影视
// @Description 按分类获取 TMDB 热门电影、剧集、动漫或纪录片
// @Tags 订阅管理
// @Accept json
// @Produce json
// @Param Authorization header string true "Bearer token"
// @Param category query string false "分类" default(movie_popular)
// @Success 200 {object} httpcontext.Response{data=HotMoviesResponse} "获取成功"
// @Failure 400 {object} httpcontext.Response "TMDB 服务未初始化或获取失败"
// @Failure 401 {object} httpcontext.Response "未授权访问"
// @Failure 403 {object} httpcontext.Response "权限不足"
// @Router /api/subscription/tmdb/movies [get]
func (h *Handler) GetTMDbMovies() httpcontext.HandlerFunc {
	return func(c *httpcontext.Context) {
		if h.tmdb == nil {
			c.Fail(invalidParams(fmt.Errorf("TMDB 服务未初始化，请检查配置")))

			return
		}

		category := c.DefaultQuery("category", "movie_popular")
		h.logger.Info("[热门数据加载] 正在获取TMDB热门数据", zap.String("category", category))

		var (
			items interface{}
			err   error
		)

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

// GetTMDbTVs 获取 TMDB 热门电视剧
// @Summary 获取 TMDB 热门电视剧
// @Description 获取 TMDB 热门电视剧列表
// @Tags 订阅管理
// @Accept json
// @Produce json
// @Param Authorization header string true "Bearer token"
// @Param category query string false "分类" default(tv_popular)
// @Success 200 {object} httpcontext.Response{data=HotMoviesResponse} "获取成功"
// @Failure 400 {object} httpcontext.Response "TMDB 服务未初始化或获取失败"
// @Failure 401 {object} httpcontext.Response "未授权访问"
// @Failure 403 {object} httpcontext.Response "权限不足"
// @Router /api/subscription/tmdb/tvs [get]
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

// GetDoubanMovies 获取豆瓣热门影视
// @Summary 获取豆瓣热门影视
// @Description 按分类获取豆瓣热门影视列表
// @Tags 订阅管理
// @Accept json
// @Produce json
// @Param Authorization header string true "Bearer token"
// @Param category query string false "分类" default(热门)
// @Success 200 {object} httpcontext.Response{data=HotMoviesResponse} "获取成功"
// @Failure 400 {object} httpcontext.Response "豆瓣服务未初始化或获取失败"
// @Failure 401 {object} httpcontext.Response "未授权访问"
// @Failure 403 {object} httpcontext.Response "权限不足"
// @Router /api/subscription/douban/movies [get]
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
	Name  string                    `gorm:"column:name;type:varchar(255);uniqueIndex" json:"name"`
	Value models.SubscriptionConfig `gorm:"column:value;type:json" json:"value"`
}

func (s Setting) TableName() string {
	return "system_settings"
}

// GetConfig 获取订阅配置
// @Summary 获取订阅配置
// @Description 获取热门订阅功能配置；未保存配置时返回默认配置
// @Tags 订阅管理
// @Accept json
// @Produce json
// @Param Authorization header string true "Bearer token"
// @Success 200 {object} httpcontext.Response{data=SubscriptionConfig} "获取成功"
// @Failure 400 {object} httpcontext.Response "获取订阅配置失败"
// @Failure 401 {object} httpcontext.Response "未授权访问"
// @Failure 403 {object} httpcontext.Response "权限不足"
// @Router /api/subscription/config [get]
func (h *Handler) GetConfig() httpcontext.HandlerFunc {
	return func(c *httpcontext.Context) {
		var setting Setting

		result := h.db.Where("name = ?", subscriptionConfigName).First(&setting)
		if result.Error != nil && !errors.Is(result.Error, gorm.ErrRecordNotFound) {
			c.Fail(invalidParams(result.Error))

			return
		}

		if setting.ID == 0 {
			c.Success(h.defaultSubscriptionConfig())

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

func (h *Handler) defaultSubscriptionConfig() SubscriptionConfig {
	return SubscriptionConfig{
		EnableTMDB:       true,
		EnableDouban:     true,
		PanSearchURL:     defaultPanSearchURL,
		DefaultMountPath: defaultSubscriptionMountPath,
		AutoMount:        false,
		CronExpression:   "0 2 * * *",
		TMDBAPIKey:       h.tmdbAPIKey,
		OpenAIBaseURL:    "https://api.openai.com",
		OpenAIModel:      "gpt-4o-mini",
	}
}

func (h *Handler) loadSubscriptionConfigValue() (models.SubscriptionConfig, error) {
	var setting Setting
	if err := h.db.Where("name = ?", subscriptionConfigName).First(&setting).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return models.SubscriptionConfig{}, nil
		}

		return models.SubscriptionConfig{}, err
	}

	return setting.Value, nil
}

func (h *Handler) panSearchURLFromConfig() (string, error) {
	config, err := h.loadSubscriptionConfigValue()
	if err != nil {
		return "", err
	}

	if config.PanSearchURL != "" {
		return config.PanSearchURL, nil
	}

	return defaultPanSearchURL, nil
}

func (h *Handler) defaultMountPathFromConfig() (string, error) {
	config, err := h.loadSubscriptionConfigValue()
	if err != nil {
		return "", err
	}

	if config.DefaultMountPath != "" {
		return config.DefaultMountPath, nil
	}

	return defaultSubscriptionMountPath, nil
}

type UpdateConfigReq struct {
	EnableTMDB       *bool   `json:"enableTMDB"`
	EnableDouban     *bool   `json:"enableDouban"`
	PanSearchURL     *string `json:"panSearchURL"`
	DefaultMountPath *string `json:"defaultMountPath"`
	AutoMount        *bool   `json:"autoMount"`
	CronExpression   *string `json:"cronExpression"`
	TMDBAPIKey       *string `json:"tmdbAPIKey"`
	OpenAIAPIKey     *string `json:"openaiAPIKey"`
	OpenAIBaseURL    *string `json:"openaiBaseURL"`
	OpenAIModel      *string `json:"openaiModel"`
}

// UpdateConfig 更新订阅配置
// @Summary 更新订阅配置
// @Description 创建或更新热门订阅功能配置，支持只提交需要修改的字段
// @Tags 订阅管理
// @Accept json
// @Produce json
// @Param Authorization header string true "Bearer token"
// @Param request body UpdateConfigReq true "订阅配置"
// @Success 200 {object} httpcontext.Response{data=models.SubscriptionConfig} "更新成功"
// @Failure 400 {object} httpcontext.Response "参数验证失败或更新失败"
// @Failure 401 {object} httpcontext.Response "未授权访问"
// @Failure 403 {object} httpcontext.Response "权限不足"
// @Failure 404 {object} httpcontext.Response "订阅配置不存在"
// @Router /api/subscription/config [post]
func (h *Handler) UpdateConfig() httpcontext.HandlerFunc {
	return func(c *httpcontext.Context) {
		var req UpdateConfigReq
		if err := c.ShouldBindJSON(&req); err != nil {
			c.Fail(invalidParams(err))

			return
		}

		if err := normalizeSubscriptionConfigUpdate(&req); err != nil {
			c.Fail(invalidParams(err))

			return
		}

		config, err := h.updateSubscriptionConfig(c.Request.Context(), &req)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				c.Fail(notFound("订阅配置不存在").WithError(err))
			} else {
				c.Fail(invalidParams(err))
			}

			return
		}

		c.Success(config)
	}
}

func (h *Handler) updateSubscriptionConfig(ctx context.Context, req *UpdateConfigReq) (models.SubscriptionConfig, error) {
	var updatedConfig models.SubscriptionConfig

	err := h.withSubscriptionConfigWriteLock(func() error {
		if err := h.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			if err := h.ensureSubscriptionConfigSetting(tx); err != nil {
				return err
			}

			setting, err := lockSubscriptionConfigSetting(tx)
			if err != nil {
				return err
			}

			config := setting.Value
			applySubscriptionConfigUpdate(&config, req)

			result := tx.Model(&Setting{}).
				Where("id = ?", setting.ID).
				Update("value", config)
			if err := h.checkSubscriptionSettingUpdateResultInDB(tx, result, setting.ID); err != nil {
				return err
			}

			updatedConfig = config

			return nil
		}); err != nil {
			return err
		}

		if h.tmdb != nil && req.TMDBAPIKey != nil {
			h.tmdb.SetAPIKey(updatedConfig.TMDBAPIKey)
			h.tmdbAPIKey = updatedConfig.TMDBAPIKey
			h.logger.Info("Updated TMDB API Key from subscription config")
		}

		return nil
	})

	return updatedConfig, err
}

func (h *Handler) ensureSubscriptionConfigSetting(tx *gorm.DB) error {
	return tx.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "name"}},
		DoNothing: true,
	}).Create(&Setting{
		Name:  subscriptionConfigName,
		Value: h.defaultSubscriptionConfigValue(),
	}).Error
}

func (h *Handler) defaultSubscriptionConfigValue() models.SubscriptionConfig {
	defaultConfig := h.defaultSubscriptionConfig()

	return models.SubscriptionConfig{
		EnableTMDB:       defaultConfig.EnableTMDB,
		EnableDouban:     defaultConfig.EnableDouban,
		PanSearchURL:     defaultConfig.PanSearchURL,
		CronExpression:   defaultConfig.CronExpression,
		DefaultMountPath: defaultConfig.DefaultMountPath,
		AutoMount:        defaultConfig.AutoMount,
		TMDBAPIKey:       defaultConfig.TMDBAPIKey,
		OpenAIAPIKey:     defaultConfig.OpenAIAPIKey,
		OpenAIBaseURL:    defaultConfig.OpenAIBaseURL,
		OpenAIModel:      defaultConfig.OpenAIModel,
	}
}

func lockSubscriptionConfigSetting(tx *gorm.DB) (*Setting, error) {
	var setting Setting

	query := tx.Where("name = ?", subscriptionConfigName)
	if subscriptionConfigSupportsRowLock(tx) {
		query = query.Clauses(clause.Locking{Strength: "UPDATE"})
	}

	if err := query.First(&setting).Error; err != nil {
		return nil, err
	}

	return &setting, nil
}

func (h *Handler) withSubscriptionConfigWriteLock(fn func() error) error {
	h.subscriptionConfigMu.Lock()
	defer h.subscriptionConfigMu.Unlock()

	return fn()
}

func subscriptionConfigSupportsRowLock(db *gorm.DB) bool {
	if db == nil || db.Dialector == nil {
		return true
	}

	switch db.Name() {
	case "sqlite", "sqlite3":
		return false
	default:
		return true
	}
}

func normalizeSubscriptionConfigUpdate(req *UpdateConfigReq) error {
	if req.PanSearchURL != nil {
		value := strings.TrimSpace(*req.PanSearchURL)
		if err := validateOptionalHTTPURL("盘搜 API 地址", value); err != nil {
			return err
		}

		*req.PanSearchURL = value
	}

	if req.DefaultMountPath != nil {
		value := strings.TrimSpace(*req.DefaultMountPath)
		if value != "" && !strings.HasPrefix(value, "/") {
			return fmt.Errorf("默认挂载路径必须以 / 开头")
		}

		*req.DefaultMountPath = value
	}

	if req.CronExpression != nil {
		value := strings.TrimSpace(*req.CronExpression)
		if value != "" {
			if _, err := cron.ParseStandard(value); err != nil {
				return fmt.Errorf("订阅 cron 表达式无效: %w", err)
			}
		}

		*req.CronExpression = value
	}

	if req.OpenAIBaseURL != nil {
		value := strings.TrimSpace(*req.OpenAIBaseURL)
		if err := validateOptionalHTTPURL("OpenAI API 地址", value); err != nil {
			return err
		}

		*req.OpenAIBaseURL = value
	}

	return nil
}

func validateOptionalHTTPURL(field string, value string) error {
	if value == "" {
		return nil
	}

	parsedURL, err := url.ParseRequestURI(value)
	if err != nil {
		return fmt.Errorf("%s无效: %w", field, err)
	}

	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return fmt.Errorf("%s必须使用 http 或 https", field)
	}

	if parsedURL.Host == "" {
		return fmt.Errorf("%s必须包含主机名", field)
	}

	return nil
}

func applySubscriptionConfigUpdate(config *models.SubscriptionConfig, req *UpdateConfigReq) {
	if req.EnableTMDB != nil {
		config.EnableTMDB = *req.EnableTMDB
	}

	if req.EnableDouban != nil {
		config.EnableDouban = *req.EnableDouban
	}

	if req.PanSearchURL != nil {
		config.PanSearchURL = *req.PanSearchURL
	}

	if req.DefaultMountPath != nil {
		config.DefaultMountPath = *req.DefaultMountPath
	}

	if req.AutoMount != nil {
		config.AutoMount = *req.AutoMount
	}

	if req.CronExpression != nil {
		config.CronExpression = *req.CronExpression
	}

	if req.TMDBAPIKey != nil {
		config.TMDBAPIKey = *req.TMDBAPIKey
	}

	if req.OpenAIAPIKey != nil {
		config.OpenAIAPIKey = *req.OpenAIAPIKey
	}

	if req.OpenAIBaseURL != nil {
		config.OpenAIBaseURL = *req.OpenAIBaseURL
	}

	if req.OpenAIModel != nil {
		config.OpenAIModel = *req.OpenAIModel
	}
}

func (h *Handler) checkSubscriptionSettingUpdateResult(result *gorm.DB, id int64) error {
	return h.checkSubscriptionSettingUpdateResultInDB(h.db, result, id)
}

func (h *Handler) checkSubscriptionSettingUpdateResultInDB(db *gorm.DB, result *gorm.DB, id int64) error {
	if result.Error != nil {
		return result.Error
	}

	if result.RowsAffected != 0 {
		return nil
	}

	var count int64
	if err := db.Model(&Setting{}).Where("id = ?", id).Count(&count).Error; err != nil {
		return err
	}

	if count == 0 {
		return gorm.ErrRecordNotFound
	}

	return nil
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

type mountSubscriptionReq struct {
	Title     string `json:"title" binding:"required"`
	ShareURL  string `json:"shareUrl" binding:"required"`
	ShareCode string `json:"shareCode"`
	Cover     string `json:"cover"`
	MountPath string `json:"mountPath"`
}

type panSearchResponseV2 struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    struct {
		Total        int                        `json:"total"`
		MergedByType map[string][]panSearchItem `json:"merged_by_type"`
	} `json:"data"`
}

type panSearchItem struct {
	URL      string   `json:"url"`
	Password string   `json:"password"`
	Note     string   `json:"note"`
	Datetime string   `json:"datetime"`
	Source   string   `json:"source"`
	Images   []string `json:"images"`
}

// SearchPan 搜索云盘资源
// @Summary 搜索云盘资源
// @Description 按关键词从配置的盘搜接口搜索天翼云盘资源
// @Tags 订阅管理
// @Accept json
// @Produce json
// @Param Authorization header string true "Bearer token"
// @Param keyword query string true "搜索关键词"
// @Success 200 {object} httpcontext.Response{data=[]SearchResult} "搜索成功"
// @Failure 400 {object} httpcontext.Response "参数验证失败或搜索失败"
// @Failure 401 {object} httpcontext.Response "未授权访问"
// @Failure 403 {object} httpcontext.Response "权限不足"
// @Router /api/subscription/search [get]
func (h *Handler) SearchPan() httpcontext.HandlerFunc {
	return func(c *httpcontext.Context) {
		keyword := strings.TrimSpace(c.Query("keyword"))
		if keyword == "" {
			c.Fail(invalidParams(fmt.Errorf("关键词不能为空")))

			return
		}

		results, err := h.searchPanResults(c, keyword)
		if err != nil {
			c.Fail(invalidParams(err))

			return
		}

		c.Success(results)
	}
}

func (h *Handler) searchPanResults(c *httpcontext.Context, keyword string) ([]SearchResult, error) {
	searchURL, err := h.panSearchRequestURL(keyword)
	if err != nil {
		return nil, err
	}

	h.logger.Info("Searching pan", zap.String("url", searchURL))

	req, err := http.NewRequestWithContext(c.Request.Context(), "GET", searchURL, nil)
	if err != nil {
		return nil, fmt.Errorf("请求失败: %w", err)
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "application/json, text/plain, */*")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8")
	req.Header.Set("Referer", "https://so.252035.xyz/")

	resp, err := h.panSearchHTTPClient().Do(req)
	if err != nil {
		h.logger.Error("Pan search request failed", zap.Error(err))

		if isPanSearchTimeoutError(err) {
			return nil, fmt.Errorf("请求超时，请稍后重试")
		}

		return nil, fmt.Errorf("搜索服务不可用: %v", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	bodyBytes, err := io.ReadAll(io.LimitReader(resp.Body, maxPanSearchResponseSize+1))
	if err != nil {
		h.logger.Error("Failed to read response body", zap.Error(err))

		return nil, fmt.Errorf("读取响应失败: %v", err)
	}

	if len(bodyBytes) > maxPanSearchResponseSize {
		return nil, fmt.Errorf("盘搜返回体过大，已拒绝")
	}

	h.logger.Info("Pan search response", zap.Int("status", resp.StatusCode), zap.Int("body_len", len(bodyBytes)))

	if resp.StatusCode != http.StatusOK {
		h.logger.Warn("Pan search returned non-OK status", zap.Int("status", resp.StatusCode), zap.String("url", searchURL))

		return nil, fmt.Errorf("搜索接口返回状态: %d，请检查盘搜 API 地址是否正确", resp.StatusCode)
	}

	var result panSearchResponseV2
	if err := json.Unmarshal(bodyBytes, &result); err != nil {
		h.logger.Error("Failed to parse pan search response", zap.Error(err))

		return nil, fmt.Errorf("解析响应失败: %w", err)
	}

	if result.Code != 0 {
		return nil, fmt.Errorf("%s", result.Message)
	}

	return panSearchResultsFromResponse(result), nil
}

func (h *Handler) panSearchRequestURL(keyword string) (string, error) {
	panSearchURL, err := h.panSearchURLFromConfig()
	if err != nil {
		return "", fmt.Errorf("读取订阅配置失败: %w", err)
	}

	searchURL := panSearchURL
	if !strings.Contains(panSearchURL, "/api/search") {
		searchURL = strings.TrimRight(panSearchURL, "/") + "/api/search"
	}

	return fmt.Sprintf("%s?kw=%s&cloud_types=tianyi", searchURL, url.QueryEscape(keyword)), nil
}

func (h *Handler) panSearchHTTPClient() *http.Client {
	client := h.httpClient
	if client == nil {
		client = &http.Client{Timeout: 120 * time.Second}
	}

	proxyURL := os.Getenv("TG_PROXY")
	if proxyURL == "" {
		return client
	}

	parsedProxyURL, err := url.Parse(proxyURL)
	if err != nil {
		h.logger.Warn("Invalid TG_PROXY for pan search", zap.String("proxy", proxyURL), zap.Error(err))

		return client
	}

	h.logger.Info("Using proxy for pan search", zap.String("proxy", proxyURL))

	return &http.Client{
		Timeout:   client.Timeout,
		Transport: &http.Transport{Proxy: http.ProxyURL(parsedProxyURL)},
	}
}

func panSearchResultsFromResponse(result panSearchResponseV2) []SearchResult {
	results := make([]SearchResult, 0)

	tianyiData, ok := result.Data.MergedByType["tianyi"]
	if !ok {
		return results
	}

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

	return results
}

// MountSubscription 挂载订阅资源
// @Summary 挂载订阅资源
// @Description 根据分享链接创建订阅挂载点，未传挂载路径时使用订阅默认挂载路径
// @Tags 订阅管理
// @Accept json
// @Produce json
// @Param Authorization header string true "Bearer token"
// @Param request body mountSubscriptionReq true "挂载参数"
// @Success 200 {object} httpcontext.Response "挂载成功"
// @Failure 400 {object} httpcontext.Response "参数验证失败、分享信息获取失败或挂载失败"
// @Failure 401 {object} httpcontext.Response "未授权访问"
// @Failure 403 {object} httpcontext.Response "权限不足"
// @Router /api/subscription/mount [post]
func (h *Handler) MountSubscription() httpcontext.HandlerFunc {
	return func(c *httpcontext.Context) {
		var req mountSubscriptionReq
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

		mountPath := strings.TrimSpace(req.MountPath)
		if mountPath == "" {
			defaultMountPath, err := h.defaultMountPathFromConfig()
			if err != nil {
				c.Fail(invalidParams(fmt.Errorf("读取订阅配置失败: %w", err)))

				return
			}

			mountPath = strings.TrimRight(defaultMountPath, "/") + "/" + strings.TrimLeft(req.Title, "/")
		}

		if !strings.HasPrefix(mountPath, "/") {
			c.Fail(invalidParams(fmt.Errorf("挂载路径必须以 / 开头")))

			return
		}

		if h.storageFacadeService == nil {
			c.Fail(invalidParams(fmt.Errorf("storage service is nil, please restart the application")))

			return
		}

		ctx := c.GetContext()
		userID := c.GetInt64(consts.CtxKeyUserId)
		isAdmin := c.GetBool(consts.CtxKeyIsAdmin)

		var existingMountPoint models.MountPoint
		if err := h.db.Where("full_path = ?", mountPath).First(&existingMountPoint).Error; err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			c.Fail(invalidParams(fmt.Errorf("查询挂载路径失败: %v", err)))

			return
		} else if err == nil && !isAdmin && (userID <= 0 || existingMountPoint.CreatorUserID != userID) {
			c.Forbidden("路径已被其他用户挂载")

			return
		}

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
			CreatorUserID: userID,
			IsAdmin:       isAdmin,
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

// SearchPanWithAI AI 推荐云盘资源
// @Summary AI 推荐云盘资源
// @Description 搜索云盘资源后使用 AI 生成优化关键词并推荐更合适的结果
// @Tags 订阅管理
// @Accept json
// @Produce json
// @Param Authorization header string true "Bearer token"
// @Param keyword query string true "搜索关键词"
// @Success 200 {object} httpcontext.Response "推荐成功"
// @Failure 400 {object} httpcontext.Response "参数验证失败或搜索失败"
// @Failure 401 {object} httpcontext.Response "未授权访问"
// @Failure 403 {object} httpcontext.Response "权限不足"
// @Router /api/subscription/search/ai [get]
func (h *Handler) SearchPanWithAI() httpcontext.HandlerFunc {
	return func(c *httpcontext.Context) {
		keyword := strings.TrimSpace(c.Query("keyword"))
		if keyword == "" {
			c.Fail(invalidParams(fmt.Errorf("关键词不能为空")))

			return
		}

		results, err := h.searchPanResults(c, keyword)
		if err != nil {
			c.Fail(invalidParams(err))

			return
		}

		if len(results) == 0 {
			c.Success(gin.H{
				"message":       "未找到相关资源",
				"keyword":       keyword,
				"aiDescription": "",
				"result":        nil,
				"allResults":    []SearchResult{},
			})

			return
		}

		if h.openaiSvc == nil {
			c.Success(gin.H{
				"message":       "AI服务未配置，返回全部搜索结果",
				"keyword":       keyword,
				"aiDescription": "",
				"result":        nil,
				"allResults":    results,
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

func isPanSearchTimeoutError(err error) bool {
	return errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)
}
