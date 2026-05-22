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

	"github.com/xxcheng123/cloudpan189-share/internal/framework/context"
	"github.com/xxcheng123/cloudpan189-share/internal/pkgs/datatypes"
	"github.com/xxcheng123/cloudpan189-share/internal/pkgs/utils"
	"github.com/xxcheng123/cloudpan189-share/internal/repository/models"
	"github.com/xxcheng123/cloudpan189-share/internal/services/douban"
	"github.com/xxcheng123/cloudpan189-share/internal/services/tmdb"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

const maxPanSearchResponseSize = 5 << 20

// ShareInfo 订阅模块需要的分享元数据最小子集。
type ShareInfo struct {
	Name       string
	IsFolder   bool
	ShareId    int64
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
	Title    string
	ShareURL string
	FileID   int64
	Size     string
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
	PanSearchURL string
	EnableTMDB   bool
	EnableDouban bool
}

func NewService(db *gorm.DB, logger *zap.Logger, config *SubscriptionConfig) Service {
	if config == nil {
		config = &SubscriptionConfig{
			PanSearchURL: "https://so.252035.xyz/api/search",
			EnableTMDB:   true,
			EnableDouban: true,
		}
	}

	return &service{
		db:     db,
		logger: logger,
		config: config,
	}
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

	// 根据订阅源获取热门资源
	switch sub.Source {
	case "tmdb":
		if sub.Category == "movie" && s.config.EnableTMDB && tmdbSvc != nil {
			items = s.getTMDbMovies()
		} else if sub.Category == "tv" && s.config.EnableTMDB && tmdbSvc != nil {
			items = s.getTMDbTVs()
		}
	case "douban":
		if s.config.EnableDouban && doubanSvc != nil {
			items = s.getDoubanMovies()
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
				s.logger.Error("Search failed", zap.String("keyword", keyword), zap.Error(err))

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

		// 记录处理历史
		if err := s.db.Create(&models.DailyHotHistory{
			Source:      item.Source,
			ContentID:   contentID,
			Title:       item.Title,
			Year:        item.Year,
			ProcessedAt: time.Now(),
		}).Error; err != nil {
			s.logger.Error("记录每日热门历史失败", zap.String("title", item.Title), zap.Error(err))

			continue
		}

		// 搜索并挂载
		results, err := s.SearchPan(item.Title + " " + item.Year)
		if err != nil {
			s.logger.Error("Search failed", zap.Error(err))

			continue
		}

		for _, r := range results {
			s.processSearchResult(sub, r)
		}

		// 如果启用 AI 洗版
		if sub.EnableAutoUpgrade {
			s.processAutoUpgrade(sub, item)
		}
	}

	// 更新订阅状态
	sub.LastRunAt = new(time.Time)

	*sub.LastRunAt = time.Now()
	if err := s.updateSubscriptionRunProgress(sub); err != nil {
		s.logger.Error("更新订阅状态失败", zap.Int64("sub_id", sub.ID), zap.Error(err))
	}
}

func (s *service) getTMDbMovies() []HotResource {
	s.logger.Info("Fetching TMDB popular movies")

	if s.tmdbService == nil {
		s.logger.Warn("TMDB service not set, skip fetch")

		return nil
	}

	movies, err := s.tmdbService.GetPopularMovies(1)
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

func (s *service) getTMDbTVs() []HotResource {
	s.logger.Info("Fetching TMDB popular TVs")

	if s.tmdbService == nil {
		s.logger.Warn("TMDB service not set, skip fetch")

		return nil
	}

	tvs, err := s.tmdbService.GetPopularTVs(1)
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

func (s *service) getDoubanMovies() []HotResource {
	s.logger.Info("Fetching Douban popular movies")

	if s.doubanService == nil {
		s.logger.Warn("Douban service not set, skip fetch")

		return nil
	}

	subjects, err := s.doubanService.GetPopularMovies()
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

func (s *service) processSearchResult(sub *models.Subscription, result SearchResult) {
	title := result.Title
	if title == "" {
		title = sub.Name
	}

	matchResult, err := s.MatchAndMount(sub, result, title, "", string(sub.Category))
	if err != nil {
		s.logger.Error("Match and mount failed", zap.Error(err))

		return
	}

	if matchResult.Success {
		sub.SuccessCount++
		sub.LastMatchAt = new(time.Time)

		*sub.LastMatchAt = time.Now()
		if err := s.updateSubscriptionMatchProgress(sub); err != nil {
			s.logger.Error("更新订阅匹配状态失败", zap.Int64("sub_id", sub.ID), zap.Error(err))
		}

		// 发送通知
		if s.telegramService != nil {
			if err := s.telegramService.SendNotification(
				"资源已挂载",
				fmt.Sprintf("%s\n\nSTRM 路径：%s\n分享链接：%s", title, matchResult.STrmPath, matchResult.ShareURL),
			); err != nil {
				s.logger.Warn("发送订阅挂载通知失败", zap.Error(err))
			}
		}
	}
}

func (s *service) processAutoUpgrade(sub *models.Subscription, item HotResource) {
	if s.openaiService == nil {
		return
	}

	// 生成优化关键词
	keyword, err := s.openaiService.GenerateUpgradeKeyword(item.Title, item.Category)
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
	if err := s.db.Create(&models.MatchHistory{
		SubscriptionID: sub.ID,
		Title:          item.Title,
		Year:           item.Year,
		Category:       models.SubscriptionCategory(item.Category),
		SearchKeyword:  item.Title,
		UpgradeKeyword: keyword,
		Status:         models.MatchStatusUpgraded,
		MatchedAt:      &matchedAt,
	}).Error; err != nil {
		s.logger.Error("记录升级历史失败", zap.String("title", item.Title), zap.Error(err))
	}
}

func (s *service) GetDailyHotMovies() ([]HotResource, error) {
	var results []HotResource

	if s.config.EnableTMDB && s.tmdbService != nil {
		movies, err := s.tmdbService.GetPopularMovies(1)
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

	if s.config.EnableDouban && s.doubanService != nil {
		movies, err := s.doubanService.GetPopularMovies()
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

	if s.config.EnableTMDB && s.tmdbService != nil {
		tvs, err := s.tmdbService.GetPopularTVs(1)
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

// searchPanWithContext 带 ctx 的盘搜，复用给订阅定时任务。
func (s *service) searchPanWithContext(ctx stdContext.Context, keyword string) ([]SearchResult, error) {
	if s.config.PanSearchURL == "" {
		s.config.PanSearchURL = "https://so.252035.xyz/api/search"
	}

	searchURL := s.config.PanSearchURL
	if !strings.Contains(searchURL, "/api/search") {
		searchURL = strings.TrimRight(searchURL, "/") + "/api/search"
	}

	searchURL = fmt.Sprintf("%s?kw=%s&cloud_types=tianyi", searchURL, url.QueryEscape(keyword))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, searchURL, nil)
	if err != nil {
		return nil, fmt.Errorf("构建请求失败: %w", err)
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; cloudpan189-share/1.0)")
	req.Header.Set("Accept", "application/json, text/plain, */*")

	resp, err := panSearchClient.Do(req)
	if err != nil {
		return nil, err
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
		return nil, fmt.Errorf("读取盘搜响应失败: %w", err)
	}

	if len(bodyBytes) > maxPanSearchResponseSize {
		return nil, fmt.Errorf("盘搜返回体过大，已拒绝")
	}

	if err := json.Unmarshal(bodyBytes, &result); err != nil {
		return nil, fmt.Errorf("解析盘搜响应失败: %w", err)
	}

	if result.Code != 0 {
		return nil, fmt.Errorf("盘搜接口错误: %s", result.Message)
	}

	results := make([]SearchResult, 0)

	if tianyiData, ok := result.Data.MergedByType["tianyi"]; ok {
		for _, item := range tianyiData {
			results = append(results, SearchResult{
				Title:    item.Note,
				ShareURL: item.URL,
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

// shareCodeRegex 用于从分享链接中提取 189 分享码。
var matchShareCodeRegex = regexp.MustCompile(`/t/([a-zA-Z0-9]+)`)

// MatchAndMount 将搜索到的资源挂载到对应路径，并记录匹配历史。
// result 来自 SearchPan 的结果；title/year/category 为热门资源元信息。
func (s *service) MatchAndMount(sub *models.Subscription, result SearchResult, title, year, category string) (*MatchResult, error) {
	// 确定挂载路径
	basePath := sub.MountPath
	if basePath == "" {
		basePath = "/热门订阅"
	}

	safeName := utils.SanitizeFileName(title)
	if safeName == "" {
		safeName = title
	}

	mountPath := basePath + "/" + safeName
	if year != "" {
		mountPath = fmt.Sprintf("%s (%s)", mountPath, year)
	}

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

		history.ErrorMessage = msg
		if err != nil {
			history.ErrorMessage = fmt.Sprintf("%s: %v", msg, err)
		}

		if createErr := s.db.Create(history).Error; createErr != nil {
			s.logger.Error("记录失败匹配历史出错", zap.Error(createErr))
		}

		return &MatchResult{
			Success: false,
			Message: history.ErrorMessage,
		}, err
	}

	// 如果没有可挂载的 ShareURL，则直接失败
	if result.ShareURL == "" {
		return recordFailure("搜索结果缺少分享链接", nil)
	}

	if s.mountService == nil || s.shareInfoFetcher == nil {
		return recordFailure("挂载或分享信息服务未初始化", nil)
	}

	// 从 ShareURL 提取分享码
	matches := matchShareCodeRegex.FindStringSubmatch(result.ShareURL)
	if len(matches) < 2 {
		return recordFailure("无法从分享链接中提取分享码", nil)
	}

	shareCode := matches[1]

	// 获取分享元数据
	bgCtx := context.NewContext(stdContext.Background(), context.WithLogger(s.logger))

	shareInfo, err := s.shareInfoFetcher.GetShareInfo(bgCtx, shareCode, "")
	if err != nil {
		return recordFailure("获取分享信息失败", err)
	}

	// 创建挂载
	storageReq := &MountStorageRequest{
		LocalPath:         mountPath,
		OsType:            string(models.OsTypeSubscribeShareFolder),
		CloudToken:        0,
		FileId:            shareInfo.ID,
		EnableDeepRefresh: true,
	}

	mountID, err := s.mountService.CreateStorage(bgCtx, storageReq)
	if err != nil {
		return recordFailure("创建挂载点失败", err)
	}

	// 记录成功
	history.Status = models.MatchStatusMatched
	if err := s.db.Create(history).Error; err != nil {
		s.logger.Error("记录匹配历史失败", zap.String("title", title), zap.Error(err))
	}

	s.logger.Info("订阅资源已挂载",
		zap.String("title", title),
		zap.String("path", mountPath),
		zap.String("share_code", shareCode),
		zap.Int64("mount_id", mountID),
	)

	return &MatchResult{
		Success:  true,
		ShareURL: result.ShareURL,
		STrmPath: mountPath,
		Message:  "挂载成功",
	}, nil
}

func (s *service) GetSubscriptions() ([]models.Subscription, error) {
	var subs []models.Subscription

	result := s.db.Order("created_at desc").Find(&subs)

	return subs, result.Error
}

func (s *service) CreateSubscription(sub *models.Subscription) error {
	sub.CreatedAt = time.Now()
	sub.UpdatedAt = time.Now()

	return s.db.Create(sub).Error
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
