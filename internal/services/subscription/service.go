package subscription

import (
	stdContext "context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/xxcheng123/cloudpan189-share/internal/consts"
	"github.com/xxcheng123/cloudpan189-share/internal/framework/context"
	"github.com/xxcheng123/cloudpan189-share/internal/pkgs/datatypes"
	"github.com/xxcheng123/cloudpan189-share/internal/pkgs/utils"
	"github.com/xxcheng123/cloudpan189-share/internal/repository/models"
	"github.com/xxcheng123/cloudpan189-share/internal/services/douban"
	"github.com/xxcheng123/cloudpan189-share/internal/services/tmdb"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

const (
	DefaultPanSearchURL      = "https://so.252035.xyz/api/search"
	maxPanSearchResponseSize = 5 << 20
)

var subscriptionLogURLPattern = regexp.MustCompile(`(?i)(?:[a-z][a-z0-9+.-]*://|/)[^\s"'<>]+`)

// ShareInfo 订阅模块需要的分享元数据最小子集。
type ShareInfo struct {
	Name       string
	IsFolder   bool
	ShareId    int64
	ShareMode  int
	ID         string
	AccessCode string
}

// MountStorageRequest 订阅模块需要的挂载参数。
type MountStorageRequest struct {
	LocalPath         string
	OsType            string
	CloudToken        int64
	FileId            string
	Addition          datatypes.JSONMap
	EnableAutoRefresh bool
	AutoRefreshDays   int
	RefreshInterval   int
	EnableDeepRefresh bool
	CreatorUserID     int64
	IsAdmin           bool
}

// MountService 挂载服务接口（storagefacade 子集），避免直接依赖具体实现。
type MountService interface {
	CreateStorage(ctx context.Context, req *MountStorageRequest) (int64, error)
}

// ShareInfoFetcher 分享信息获取接口（cloudbridge 子集）。
type ShareInfoFetcher interface {
	GetShareInfo(ctx context.Context, shareCode string, accessCode string) (*ShareInfo, error)
}

// Service 订阅服务接口
type Service interface {
	RunSubscriptionJob() error
	GetDailyHotMovies() ([]HotResource, error)
	GetDailyHotTVs() ([]HotResource, error)
	SearchPan(keyword string) ([]SearchResult, error)
	MatchAndMount(sub *models.Subscription, result SearchResult, title, year, category string) (*MatchResult, error)
	GetSubscriptions() ([]models.Subscription, error)
	CreateSubscription(sub *models.Subscription) error
	UpdateSubscription(sub *models.Subscription) error
	DeleteSubscription(id int64) error
	GetMatchHistory(subscriptionID int64) ([]models.MatchHistory, error)
	SetTelegramService(TelegramService)
	SetTMDBService(TMDBService)
	SetDoubanService(DoubanService)
	SetOpenAIService(OpenAIService)
	SetMountService(MountService)
	SetShareInfoFetcher(ShareInfoFetcher)
	UpdateConfig(SubscriptionConfig)
}

// TelegramService Telegram 服务接口
type TelegramService interface {
	SendNotification(title, content string) error
}

// TMDBService TMDB 服务接口
type TMDBService interface {
	GetPopularMovies(page int) ([]tmdb.Movie, error)
	GetPopularTVs(page int) ([]tmdb.TV, error)
}

// DoubanService 豆瓣服务接口
type DoubanService interface {
	GetPopularMovies() ([]douban.Subject, error)
}

// OpenAIService OpenAI 服务接口
type OpenAIService interface {
	GenerateUpgradeKeyword(title, category string) (string, error)
}

type HotResource struct {
	Title    string
	Year     string
	Category string
	Rating   float64
	Cover    string
	Source   string // tmdb, douban
}

type SearchResult struct {
	Title           string
	ShareURL        string
	ShareAccessCode string
	FileID          int64
	Size            string
}

type MatchResult struct {
	Success  bool
	ShareURL string
	STrmPath string
	Message  string
}

type service struct {
	db               *gorm.DB
	logger           *zap.Logger
	tmdbService      TMDBService
	doubanService    DoubanService
	openaiService    OpenAIService
	telegramService  TelegramService
	mountService     MountService
	shareInfoFetcher ShareInfoFetcher
	config           *SubscriptionConfig
	mu               sync.RWMutex
}

type SubscriptionConfig struct {
	Enabled        bool
	PanSearchURL   string
	EnableTMDB     bool
	EnableDouban   bool
	CronExpression string
}

func NewService(db *gorm.DB, logger *zap.Logger, config *SubscriptionConfig) Service {
	if logger == nil {
		logger = zap.NewNop()
	}

	resolvedConfig := defaultSubscriptionServiceConfig()
	if config == nil {
		config = &resolvedConfig
	} else {
		resolvedConfig = normalizeSubscriptionServiceConfig(*config)
		config = &resolvedConfig
	}

	return &service{
		db:     db,
		logger: logger,
		config: config,
	}
}

func defaultSubscriptionServiceConfig() SubscriptionConfig {
	return SubscriptionConfig{
		Enabled:        false,
		PanSearchURL:   DefaultPanSearchURL,
		EnableTMDB:     true,
		EnableDouban:   true,
		CronExpression: consts.DefaultSubscriptionCronExpression,
	}
}

func normalizeSubscriptionServiceConfig(config SubscriptionConfig) SubscriptionConfig {
	config.PanSearchURL = strings.TrimSpace(config.PanSearchURL)
	config.CronExpression = strings.TrimSpace(config.CronExpression)

	if config.CronExpression == "" {
		config.CronExpression = consts.DefaultSubscriptionCronExpression
	}

	return config
}

func (s *service) UpdateConfig(config SubscriptionConfig) {
	normalized := normalizeSubscriptionServiceConfig(config)

	s.mu.Lock()
	defer s.mu.Unlock()

	s.config = &normalized
}

func (s *service) SetTelegramService(svc TelegramService) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.telegramService = svc
}

func (s *service) SetTMDBService(svc TMDBService) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.tmdbService = svc
}

func (s *service) SetDoubanService(svc DoubanService) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.doubanService = svc
}

func (s *service) SetOpenAIService(svc OpenAIService) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.openaiService = svc
}

func (s *service) SetMountService(svc MountService) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.mountService = svc
}

func (s *service) SetShareInfoFetcher(svc ShareInfoFetcher) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.shareInfoFetcher = svc
}

// snapshotDeps 在持锁时拍一份当前依赖的快照，供无锁调用。
func (s *service) snapshotDeps() (TMDBService, DoubanService, OpenAIService, TelegramService, MountService, ShareInfoFetcher) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.tmdbService, s.doubanService, s.openaiService, s.telegramService, s.mountService, s.shareInfoFetcher
}

func (s *service) configSnapshot() SubscriptionConfig {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.config == nil {
		return defaultSubscriptionServiceConfig()
	}

	return *s.config
}

func (s *service) RunSubscriptionJob() error {
	s.logger.Info("Starting subscription job...")

	var subscriptions []models.Subscription

	result := s.db.Where("enable = ?", true).Find(&subscriptions)
	if result.Error != nil {
		return result.Error
	}

	for _, sub := range subscriptions {
		s.logger.Info("Processing subscription", zap.String("name", sub.Name))
		s.processSubscription(&sub)
	}

	s.logger.Info("Subscription job completed")

	return nil
}

func (s *service) processSubscription(sub *models.Subscription) {
	var items []HotResource

	tmdbSvc, doubanSvc, _, _, _, _ := s.snapshotDeps()
	config := s.configSnapshot()

	// 根据订阅源获取热门资源
	switch sub.Source {
	case "tmdb":
		if sub.Category == "movie" && config.EnableTMDB && !isNilDependency(tmdbSvc) {
			items = s.getTMDbMovies(tmdbSvc)
		} else if sub.Category == "tv" && config.EnableTMDB && !isNilDependency(tmdbSvc) {
			items = s.getTMDbTVs(tmdbSvc)
		}
	case "douban":
		if config.EnableDouban && !isNilDependency(doubanSvc) {
			items = s.getDoubanMovies(doubanSvc)
		}
	}

	// 处理关键词订阅（关键词分支结束后也需要更新 LastRunAt）
	if sub.Keywords != "" {
		keywords := strings.Split(sub.Keywords, ",")
		for _, keyword := range keywords {
			keyword = strings.TrimSpace(keyword)
			if keyword == "" {
				continue
			}

			results, err := s.SearchPan(keyword)
			if err != nil {
				s.logger.Error("Search failed",
					zap.String("keyword", sanitizeSubscriptionErrorText(keyword)),
					zap.String("error", sanitizeSubscriptionErrorText(err.Error())),
				)

				continue
			}

			for _, r := range results {
				s.processSearchResult(sub, r)
			}
		}

		// 更新订阅状态（原实现未更新，导致下次运行时看不出进度）
		now := time.Now()

		sub.LastRunAt = &now
		if err := s.updateSubscriptionRunProgress(sub); err != nil {
			s.logger.Error("更新订阅状态失败", zap.Int64("sub_id", sub.ID), zap.Error(err))
		}

		return
	}

	// 处理热门资源
	for _, item := range items {
		// 使用 Title + Year 去重，避免同名不同年份被误判为重复
		contentID := item.Title
		if item.Year != "" {
			contentID = item.Title + "|" + item.Year
		}

		// 检查今天是否已处理
		var exists models.DailyHotHistory

		now := time.Now()
		todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
		todayEnd := todayStart.AddDate(0, 0, 1)

		err := s.db.Where("source = ? AND content_id = ? AND processed_at >= ? AND processed_at < ?",
			item.Source, contentID, todayStart, todayEnd).First(&exists).Error
		if err == nil {
			s.logger.Debug("Already processed today", zap.String("title", item.Title))

			continue
		}

		if !errors.Is(err, gorm.ErrRecordNotFound) {
			s.logger.Error("查询每日热门历史失败", zap.String("title", item.Title), zap.Error(err))

			continue
		}

		// 搜索并挂载
		results, err := s.SearchPan(item.Title + " " + item.Year)
		if err != nil {
			s.logger.Error("Search failed", zap.Error(err))

			continue
		}

		matched := false

		for _, r := range results {
			if s.processSearchResult(sub, r) {
				matched = true
			}
		}

		// 如果启用 AI 洗版
		if sub.EnableAutoUpgrade {
			s.processAutoUpgrade(sub, item)
		}

		if !matched {
			s.logger.Debug("热门资源未匹配成功，保留后续重试机会", zap.String("title", item.Title))

			continue
		}

		// 只有搜索并挂载成功后才记录处理历史，避免失败资源当天被永久跳过。
		if err := s.db.Create(&models.DailyHotHistory{
			Source:      item.Source,
			ContentID:   contentID,
			Title:       item.Title,
			Year:        item.Year,
			ProcessedAt: time.Now(),
		}).Error; err != nil {
			s.logger.Error("记录每日热门历史失败", zap.String("title", item.Title), zap.Error(err))
		}
	}

	// 更新订阅状态
	sub.LastRunAt = new(time.Time)

	*sub.LastRunAt = time.Now()
	if err := s.updateSubscriptionRunProgress(sub); err != nil {
		s.logger.Error("更新订阅状态失败", zap.Int64("sub_id", sub.ID), zap.Error(err))
	}
}

func (s *service) getTMDbMovies(tmdbSvc TMDBService) []HotResource {
	s.logger.Info("Fetching TMDB popular movies")

	if isNilDependency(tmdbSvc) {
		s.logger.Warn("TMDB service not set, skip fetch")

		return nil
	}

	movies, err := tmdbSvc.GetPopularMovies(1)
	if err != nil {
		s.logger.Error("获取 TMDB 热门电影失败", zap.Error(err))

		return nil
	}

	results := make([]HotResource, 0, len(movies))
	for _, m := range movies {
		year := ""
		if len(m.ReleaseDate) >= 4 {
			year = m.ReleaseDate[:4]
		}

		results = append(results, HotResource{
			Title:    m.Title,
			Year:     year,
			Category: "movie",
			Rating:   m.VoteAverage,
			Cover:    m.PosterPath,
			Source:   "tmdb",
		})
	}

	return results
}

func (s *service) getTMDbTVs(tmdbSvc TMDBService) []HotResource {
	s.logger.Info("Fetching TMDB popular TVs")

	if isNilDependency(tmdbSvc) {
		s.logger.Warn("TMDB service not set, skip fetch")

		return nil
	}

	tvs, err := tmdbSvc.GetPopularTVs(1)
	if err != nil {
		s.logger.Error("获取 TMDB 热门剧集失败", zap.Error(err))

		return nil
	}

	results := make([]HotResource, 0, len(tvs))
	for _, t := range tvs {
		year := ""
		if len(t.FirstAirDate) >= 4 {
			year = t.FirstAirDate[:4]
		}

		results = append(results, HotResource{
			Title:    t.Name,
			Year:     year,
			Category: "tv",
			Rating:   t.VoteAverage,
			Cover:    t.PosterPath,
			Source:   "tmdb",
		})
	}

	return results
}

func (s *service) getDoubanMovies(doubanSvc DoubanService) []HotResource {
	s.logger.Info("Fetching Douban popular movies")

	if isNilDependency(doubanSvc) {
		s.logger.Warn("Douban service not set, skip fetch")

		return nil
	}

	subjects, err := doubanSvc.GetPopularMovies()
	if err != nil {
		s.logger.Error("获取豆瓣热门电影失败", zap.Error(err))

		return nil
	}

	results := make([]HotResource, 0, len(subjects))
	for _, sub := range subjects {
		category := "movie"
		if sub.Type == "tv" {
			category = "tv"
		}

		results = append(results, HotResource{
			Title:    sub.Title,
			Year:     sub.Year,
			Category: category,
			Rating:   sub.Rating,
			Cover:    sub.Cover,
			Source:   "douban",
		})
	}

	return results
}

func (s *service) processSearchResult(sub *models.Subscription, result SearchResult) bool {
	title := result.Title
	if title == "" {
		title = sub.Name
	}

	matchResult, err := s.MatchAndMount(sub, result, title, "", string(sub.Category))
	if err != nil {
		s.logger.Error("Match and mount failed", zap.Error(err))

		return false
	}

	if matchResult.Success {
		sub.SuccessCount++
		sub.LastMatchAt = new(time.Time)

		*sub.LastMatchAt = time.Now()
		if err := s.updateSubscriptionMatchProgress(sub); err != nil {
			s.logger.Error("更新订阅匹配状态失败", zap.Int64("sub_id", sub.ID), zap.Error(err))
		}

		// 发送通知
		_, _, _, telegramSvc, _, _ := s.snapshotDeps()
		if !isNilDependency(telegramSvc) {
			if err := telegramSvc.SendNotification(
				"资源已挂载",
				fmt.Sprintf("%s\n\nSTRM 路径：%s\n分享链接：%s", title, matchResult.STrmPath, matchResult.ShareURL),
			); err != nil {
				s.logger.Warn("发送订阅挂载通知失败", zap.Error(err))
			}
		}

		return true
	}

	return false
}

func (s *service) processAutoUpgrade(sub *models.Subscription, item HotResource) {
	_, _, openaiSvc, _, _, _ := s.snapshotDeps()
	if isNilDependency(openaiSvc) {
		return
	}

	// 生成优化关键词
	keyword, err := openaiSvc.GenerateUpgradeKeyword(item.Title, item.Category)
	if err != nil {
		s.logger.Error("Generate upgrade keyword failed", zap.Error(err))

		return
	}

	// 搜索更高品质资源
	results, err := s.SearchPan(keyword)
	if err != nil {
		s.logger.Error("Search upgrade resource failed", zap.Error(err))

		return
	}

	// 处理升级结果
	for _, r := range results {
		s.processSearchResult(sub, r)
	}

	// 记录升级历史
	var matchedAt = time.Now()
	s.recordMatchHistory(&models.MatchHistory{
		SubscriptionID: sub.ID,
		Title:          item.Title,
		Year:           item.Year,
		Category:       models.SubscriptionCategory(item.Category),
		SearchKeyword:  item.Title,
		UpgradeKeyword: keyword,
		Status:         models.MatchStatusUpgraded,
		MatchedAt:      &matchedAt,
	}, "记录升级历史失败")
}

func (s *service) GetDailyHotMovies() ([]HotResource, error) {
	var results []HotResource

	tmdbSvc, doubanSvc, _, _, _, _ := s.snapshotDeps()
	config := s.configSnapshot()

	if config.EnableTMDB && !isNilDependency(tmdbSvc) {
		movies, err := tmdbSvc.GetPopularMovies(1)
		if err == nil {
			for _, movie := range movies {
				year := ""
				if len(movie.ReleaseDate) >= 4 {
					year = movie.ReleaseDate[:4]
				}

				results = append(results, HotResource{
					Title:    movie.Title,
					Year:     year,
					Category: "movie",
					Rating:   movie.VoteAverage,
					Cover:    "https://image.tmdb.org/t/p/w500" + movie.PosterPath,
					Source:   "tmdb",
				})
			}
		}
	}

	if config.EnableDouban && !isNilDependency(doubanSvc) {
		movies, err := doubanSvc.GetPopularMovies()
		if err == nil {
			for _, movie := range movies {
				results = append(results, HotResource{
					Title:    movie.Title,
					Year:     movie.Year,
					Category: "movie",
					Rating:   movie.Rating,
					Cover:    movie.Cover,
					Source:   "douban",
				})
			}
		}
	}

	return results, nil
}

func (s *service) GetDailyHotTVs() ([]HotResource, error) {
	var results []HotResource

	tmdbSvc, _, _, _, _, _ := s.snapshotDeps()
	config := s.configSnapshot()

	if config.EnableTMDB && !isNilDependency(tmdbSvc) {
		tvs, err := tmdbSvc.GetPopularTVs(1)
		if err == nil {
			for _, tv := range tvs {
				year := ""
				if len(tv.FirstAirDate) >= 4 {
					year = tv.FirstAirDate[:4]
				}

				results = append(results, HotResource{
					Title:    tv.Name,
					Year:     year,
					Category: "tv",
					Rating:   tv.VoteAverage,
					Cover:    "https://image.tmdb.org/t/p/w500" + tv.PosterPath,
					Source:   "tmdb",
				})
			}
		}
	}

	return results, nil
}

func (s *service) SearchPan(keyword string) ([]SearchResult, error) {
	return s.searchPanWithContext(stdContext.Background(), keyword)
}

// BuildPanSearchRequestURL 构建盘搜请求 URL，保留配置中已有查询参数。
func BuildPanSearchRequestURL(panSearchURL string, keyword string) (string, error) {
	searchURL := strings.TrimSpace(panSearchURL)
	if searchURL == "" {
		searchURL = DefaultPanSearchURL
	}

	parsedURL, err := url.Parse(searchURL)
	if err != nil {
		return "", fmt.Errorf("盘搜 API 地址无效: %w", err)
	}

	if !isPanSearchAPIPath(parsedURL.Path) {
		parsedURL.Path = strings.TrimRight(parsedURL.Path, "/") + "/api/search"
	}

	query := parsedURL.Query()
	query.Set("kw", keyword)
	query.Set("cloud_types", "tianyi")
	parsedURL.RawQuery = query.Encode()

	return parsedURL.String(), nil
}

func isPanSearchAPIPath(path string) bool {
	path = strings.TrimRight(path, "/")

	return path == "/api/search" || strings.HasSuffix(path, "/api/search")
}

// RedactPanSearchError removes request URL query details from pan-search errors.
func RedactPanSearchError(searchURL string, err error) string {
	if err == nil {
		return ""
	}

	message := err.Error()
	if searchURL != "" {
		message = strings.ReplaceAll(message, searchURL, utils.RedactURLForLog(searchURL))
	}

	return sanitizeSubscriptionErrorText(message)
}

func sanitizeSubscriptionErrorText(text string) string {
	text = subscriptionLogURLPattern.ReplaceAllStringFunc(text, utils.RedactURLForLog)

	return utils.RedactSensitiveText(text)
}

// searchPanWithContext 带 ctx 的盘搜，复用给订阅定时任务。
func (s *service) searchPanWithContext(ctx stdContext.Context, keyword string) ([]SearchResult, error) {
	config := s.configSnapshot()

	searchURL, err := BuildPanSearchRequestURL(config.PanSearchURL, keyword)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, searchURL, nil)
	if err != nil {
		return nil, fmt.Errorf("构建请求失败: %s", RedactPanSearchError(searchURL, err))
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; cloudpan189-share/1.0)")
	req.Header.Set("Accept", "application/json, text/plain, */*")

	resp, err := panSearchClient.Do(req)
	if err != nil {
		return nil, errors.New(RedactPanSearchError(searchURL, err))
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("盘搜接口返回状态: %d", resp.StatusCode)
	}

	type panSearchResponse struct {
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

	var result panSearchResponse

	bodyBytes, err := io.ReadAll(io.LimitReader(resp.Body, maxPanSearchResponseSize+1))
	if err != nil {
		return nil, fmt.Errorf("读取盘搜响应失败: %s", RedactPanSearchError(searchURL, err))
	}

	if len(bodyBytes) > maxPanSearchResponseSize {
		return nil, fmt.Errorf("盘搜返回体过大，已拒绝")
	}

	if err := json.Unmarshal(bodyBytes, &result); err != nil {
		return nil, fmt.Errorf("解析盘搜响应失败: %w", err)
	}

	if result.Code != 0 {
		return nil, fmt.Errorf("盘搜接口错误: %s", sanitizeSubscriptionErrorText(result.Message))
	}

	results := make([]SearchResult, 0)

	if tianyiData, ok := result.Data.MergedByType["tianyi"]; ok {
		for _, item := range tianyiData {
			_, accessCode := utils.ParseCloud189ShareCode(item.URL, item.Password)

			results = append(results, SearchResult{
				Title:           item.Note,
				ShareURL:        item.URL,
				ShareAccessCode: accessCode,
			})
		}
	}

	return results, nil
}

// panSearchClient 复用连接池，避免每次 SearchPan 都新建 Transport。
var panSearchClient = &http.Client{
	Timeout: 60 * time.Second,
	Transport: &http.Transport{
		MaxIdleConns:        20,
		MaxIdleConnsPerHost: 10,
		IdleConnTimeout:     90 * time.Second,
	},
}

// MatchAndMount 将搜索到的资源挂载到对应路径，并记录匹配历史。
// result 来自 SearchPan 的结果；title/year/category 为热门资源元信息。
func (s *service) MatchAndMount(sub *models.Subscription, result SearchResult, title, year, category string) (*MatchResult, error) {
	mountPath, mountPathErr := buildSubscriptionMountPath(sub.MountPath, title, year)

	matchedAt := time.Now()
	history := &models.MatchHistory{
		SubscriptionID: sub.ID,
		Title:          title,
		Year:           year,
		Category:       models.SubscriptionCategory(category),
		SearchKeyword:  title,
		ShareURL:       result.ShareURL,
		STrmPath:       mountPath,
		Status:         models.MatchStatusPending,
		MatchedAt:      &matchedAt,
	}

	recordFailure := func(msg string, err error) (*MatchResult, error) {
		history.Status = models.MatchStatusFailed

		var returnErr error

		history.ErrorMessage = sanitizeSubscriptionErrorText(msg)
		if err != nil {
			history.ErrorMessage = sanitizeSubscriptionErrorText(fmt.Sprintf("%s: %v", msg, err))
			returnErr = errors.New(history.ErrorMessage)
		}

		s.recordMatchHistory(history, "记录失败匹配历史出错")

		return &MatchResult{
			Success: false,
			Message: history.ErrorMessage,
		}, returnErr
	}

	if mountPathErr != nil {
		return recordFailure("生成挂载路径失败", mountPathErr)
	}

	// 如果没有可挂载的 ShareURL，则直接失败
	if result.ShareURL == "" {
		return recordFailure("搜索结果缺少分享链接", nil)
	}

	_, _, _, _, mountSvc, shareInfoFetcher := s.snapshotDeps()
	if isNilDependency(mountSvc) || isNilDependency(shareInfoFetcher) {
		return recordFailure("挂载或分享信息服务未初始化", nil)
	}

	// 从 ShareURL 提取分享码和访问码。
	shareCode, accessCode := utils.ParseCloud189ShareCode(result.ShareURL, result.ShareAccessCode)
	if !utils.IsCloud189ShareCode(shareCode) {
		return recordFailure("无法从分享链接中提取分享码", nil)
	}

	if accessCode != "" && !utils.IsCloud189AccessCode(accessCode) {
		return recordFailure("访问码格式无效", nil)
	}

	// 获取分享元数据
	bgCtx := context.NewContext(stdContext.Background(), context.WithLogger(s.logger))

	shareInfo, err := shareInfoFetcher.GetShareInfo(bgCtx, shareCode, accessCode)
	if err != nil {
		return recordFailure("获取分享信息失败", err)
	}

	// 创建挂载
	storageReq := &MountStorageRequest{
		LocalPath:         mountPath,
		OsType:            string(models.OsTypeSubscribeShareFolder),
		CloudToken:        0,
		FileId:            shareInfo.ID,
		Addition:          subscriptionShareMountAdditionWithAccessCode(shareInfo, accessCode),
		EnableDeepRefresh: true,
		CreatorUserID:     1,
		IsAdmin:           true,
	}

	mountID, err := mountSvc.CreateStorage(bgCtx, storageReq)
	if err != nil {
		return recordFailure("创建挂载点失败", err)
	}

	// 记录成功
	history.Status = models.MatchStatusMatched
	s.recordMatchHistory(history, "记录匹配历史失败")

	s.logger.Info("订阅资源已挂载",
		zap.String("title", title),
		zap.String("path", mountPath),
		zap.String("share_code", utils.MaskShareCodeForLog(shareCode)),
		zap.Int64("mount_id", mountID),
	)

	return &MatchResult{
		Success:  true,
		ShareURL: result.ShareURL,
		STrmPath: mountPath,
		Message:  "挂载成功",
	}, nil
}

func buildSubscriptionMountPath(basePath, title, year string) (string, error) {
	basePath = strings.TrimSpace(basePath)
	if basePath == "" {
		basePath = "/热门订阅"
	}

	name := utils.SanitizeFileName(title)
	if name == "" {
		return "", fmt.Errorf("资源名称不能为空")
	}

	year = utils.SanitizeFileName(strings.TrimSpace(year))
	if year != "" {
		name = fmt.Sprintf("%s (%s)", name, year)
	}

	return utils.JoinStoragePath(basePath, name)
}

func (s *service) recordMatchHistory(history *models.MatchHistory, message string) {
	if history == nil {
		return
	}

	if err := s.db.Create(history).Error; err != nil {
		s.logger.Error(message,
			zap.String("error", sanitizeSubscriptionErrorText(err.Error())),
			zap.Int64("subscription_id", history.SubscriptionID),
			zap.String("title", history.Title),
			zap.String("category", string(history.Category)),
			zap.String("status", string(history.Status)),
			zap.String("share_url", utils.MaskShareCodeForLog(history.ShareURL)),
			zap.String("strm_path", history.STrmPath),
			zap.String("error_message", sanitizeSubscriptionErrorText(history.ErrorMessage)),
		)
	}
}

func subscriptionShareMountAdditionWithAccessCode(shareInfo *ShareInfo, fallbackAccessCode string) datatypes.JSONMap {
	if shareInfo == nil {
		return datatypes.JSONMap{}
	}

	shareMode := shareInfo.ShareMode
	if shareMode <= 0 {
		shareMode = 1
	}

	accessCode := shareInfo.AccessCode
	if accessCode == "" {
		accessCode = fallbackAccessCode
	}

	return datatypes.JSONMap{
		consts.FileAdditionKeyShareId:    shareInfo.ShareId,
		consts.FileAdditionKeyIsFolder:   shareInfo.IsFolder,
		consts.FileAdditionKeyAccessCode: accessCode,
		consts.FileAdditionKeyShareMode:  shareMode,
	}
}

func (s *service) GetSubscriptions() ([]models.Subscription, error) {
	var subs []models.Subscription

	result := s.db.Order("created_at desc").Find(&subs)

	return subs, result.Error
}

func (s *service) CreateSubscription(sub *models.Subscription) error {
	sub.CreatedAt = time.Now()
	sub.UpdatedAt = time.Now()

	keywords := sub.Keywords
	listType := sub.ListType
	mountPath := sub.MountPath
	enable := sub.Enable
	enableAutoUpgrade := sub.EnableAutoUpgrade
	matchCount := sub.MatchCount
	successCount := sub.SuccessCount
	lastRunAt := sub.LastRunAt
	lastMatchAt := sub.LastMatchAt
	updatedAt := sub.UpdatedAt

	return s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(sub).Error; err != nil {
			return err
		}

		result := tx.Model(&models.Subscription{}).
			Where("id = ?", sub.ID).
			Updates(map[string]interface{}{
				"keywords":            keywords,
				"list_type":           listType,
				"mount_path":          mountPath,
				"enable":              enable,
				"enable_auto_upgrade": enableAutoUpgrade,
				"match_count":         matchCount,
				"success_count":       successCount,
				"last_run_at":         lastRunAt,
				"last_match_at":       lastMatchAt,
				"updated_at":          updatedAt,
			})
		if result.Error != nil {
			return result.Error
		}

		sub.Keywords = keywords
		sub.ListType = listType
		sub.MountPath = mountPath
		sub.Enable = enable
		sub.EnableAutoUpgrade = enableAutoUpgrade
		sub.MatchCount = matchCount
		sub.SuccessCount = successCount
		sub.LastRunAt = lastRunAt
		sub.LastMatchAt = lastMatchAt
		sub.UpdatedAt = updatedAt

		return nil
	})
}

func (s *service) updateSubscriptionRunProgress(sub *models.Subscription) error {
	result := s.db.Model(&models.Subscription{}).
		Where("id = ?", sub.ID).
		Updates(map[string]interface{}{
			"last_run_at": sub.LastRunAt,
			"updated_at":  time.Now(),
		})

	return s.checkSubscriptionUpdateResult(result, sub.ID)
}

func (s *service) updateSubscriptionMatchProgress(sub *models.Subscription) error {
	result := s.db.Model(&models.Subscription{}).
		Where("id = ?", sub.ID).
		Updates(map[string]interface{}{
			"success_count": sub.SuccessCount,
			"last_match_at": sub.LastMatchAt,
			"updated_at":    time.Now(),
		})

	return s.checkSubscriptionUpdateResult(result, sub.ID)
}

func (s *service) UpdateSubscription(sub *models.Subscription) error {
	sub.UpdatedAt = time.Now()

	result := s.db.Model(&models.Subscription{}).
		Where("id = ?", sub.ID).
		Select(
			"name",
			"source",
			"category",
			"keywords",
			"list_type",
			"mount_path",
			"enable",
			"enable_auto_upgrade",
			"match_count",
			"success_count",
			"last_run_at",
			"last_match_at",
			"updated_at",
		).
		Updates(sub)

	return s.checkSubscriptionUpdateResult(result, sub.ID)
}

func (s *service) checkSubscriptionUpdateResult(result *gorm.DB, id int64) error {
	if result.Error != nil {
		return result.Error
	}

	if result.RowsAffected != 0 {
		return nil
	}

	var count int64
	if err := s.db.Model(&models.Subscription{}).Where("id = ?", id).Count(&count).Error; err != nil {
		return err
	}

	if count == 0 {
		return gorm.ErrRecordNotFound
	}

	return nil
}

func (s *service) DeleteSubscription(id int64) error {
	result := s.db.Delete(&models.Subscription{}, id)
	if result.Error != nil {
		return result.Error
	}

	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}

	return nil
}

func (s *service) GetMatchHistory(subscriptionID int64) ([]models.MatchHistory, error) {
	var history []models.MatchHistory

	result := s.db.Where("subscription_id = ?", subscriptionID).Order("created_at desc").Find(&history)

	return history, result.Error
}
