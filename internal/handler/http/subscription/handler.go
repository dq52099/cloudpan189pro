package subscription

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/xxcheng123/cloudpan189-share/internal/framework/httpcontext"
	"github.com/xxcheng123/cloudpan189-share/internal/repository/models"
	"github.com/xxcheng123/cloudpan189-share/internal/services/douban"
	"github.com/xxcheng123/cloudpan189-share/internal/services/tmdb"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type Handler struct {
	db        *gorm.DB
	tmdb      tmdb.Service
	douban    douban.Service
	logger    *zap.Logger
	openaiSvc interface {
		GenerateUpgradeKeyword(title, category string) (string, error)
	}
	tmdbAPIKey string
}

func NewHandler(db *gorm.DB, tmdbSvc tmdb.Service, doubanSvc douban.Service, logger *zap.Logger, openaiSvc interface {
	GenerateUpgradeKeyword(title, category string) (string, error)
}) *Handler {
	db.AutoMigrate(&Setting{})

	var tmdbAPIKey string
	cfg := tmdbSvc.GetConfig()
	if cfg != nil {
		tmdbAPIKey = cfg.APIKey
	}

	var setting Setting
	if err := db.Where("name = ?", "subscription_config").First(&setting).Error; err == nil && setting.Value.TMDBAPIKey != "" {
		tmdbSvc.SetAPIKey(setting.Value.TMDBAPIKey)
		tmdbAPIKey = setting.Value.TMDBAPIKey
	}

	return &Handler{
		db:         db,
		tmdb:       tmdbSvc,
		douban:     doubanSvc,
		logger:     logger,
		openaiSvc:  openaiSvc,
		tmdbAPIKey: tmdbAPIKey,
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
}

func (e *customBusinessError) GetHTTPCode() int   { return e.httpCode }
func (e *customBusinessError) GetCode() int       { return e.businessCode }
func (e *customBusinessError) GetMessage() string { return e.message }
func (e *customBusinessError) GetError() error    { return nil }
func (e *customBusinessError) Error() string      { return e.message }
func (e *customBusinessError) WithError(err error) httpcontext.BusinessError {
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
}

type HotMoviesResponse struct {
	Movies []HotMovieItem `json:"movies"`
	Source string         `json:"source"`
}

func (h *Handler) GetTMDbMovies() httpcontext.HandlerFunc {
	return func(c *httpcontext.Context) {
		items, err := h.tmdb.GetPopularMovies(1)
		if err != nil {
			h.logger.Error("failed to get TMDB movies", zap.Error(err))
			c.Fail(invalidParams(fmt.Errorf("获取TMDB热门电影失败: %w", err)))
			return
		}

		movies := make([]HotMovieItem, 0, len(items))
		for _, m := range items {
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
			})
		}

		c.Success(HotMoviesResponse{
			Movies: movies,
			Source: "tmdb",
		})
	}
}

func (h *Handler) GetTMDbTVs() httpcontext.HandlerFunc {
	return func(c *httpcontext.Context) {
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
			})
		}

		c.Success(HotMoviesResponse{
			Movies: movies,
			Source: "tmdb_tv",
		})
	}
}

func (h *Handler) GetDoubanMovies() httpcontext.HandlerFunc {
	return func(c *httpcontext.Context) {
		items, err := h.douban.GetPopularMovies()
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
			})
		}

		c.Success(HotMoviesResponse{
			Movies: movies,
			Source: "douban",
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
		if result.Error != nil && result.Error.Error() != "record not found" {
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
		if result.Error != nil && result.Error.Error() != "record not found" {
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

		client := &http.Client{Timeout: 60 * time.Second}
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
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			h.logger.Error("Failed to parse pan search response", zap.Error(err))
			c.Fail(invalidParams(fmt.Errorf("解析响应失败: %w", err)))
			return
		}

		if result.Code != 0 {
			c.Fail(invalidParams(fmt.Errorf(result.Message)))
			return
		}

		var results []SearchResult
		if tianyiData, ok := result.Data.MergedByType["tianyi"]; ok {
			for _, item := range tianyiData {
				re := regexp.MustCompile(`/t/([a-zA-Z0-9]+)`)
				matches := re.FindStringSubmatch(item.URL)
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
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.Fail(invalidParams(err))
			return
		}

		defaultMountPath := "/热门订阅"

		c.Success(gin.H{
			"message":   "挂载功能需要通过自动入库计划实现",
			"mountPath": defaultMountPath + "/" + req.Title,
			"shareURL":  req.ShareURL,
			"shareCode": req.ShareCode,
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

		client := &http.Client{Timeout: 60 * time.Second}
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
			c.Fail(invalidParams(fmt.Errorf(searchResult.Message)))
			return
		}

		if h.openaiSvc == nil {
			c.Fail(invalidParams(fmt.Errorf("AI服务未配置")))
			return
		}

		var results []SearchResult
		if tianyiData, ok := searchResult.Data.MergedByType["tianyi"]; ok {
			re := regexp.MustCompile(`/t/([a-zA-Z0-9]+)`)
			for _, item := range tianyiData {
				matches := re.FindStringSubmatch(item.URL)
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

			if strings.Contains(name, "4k") || strings.Contains(name, "4K") {
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
