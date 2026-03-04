package subscription

import (
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/xxcheng123/cloudpan189-share/internal/repository/models"
	"github.com/xxcheng123/cloudpan189-share/internal/services/douban"
	"github.com/xxcheng123/cloudpan189-share/internal/services/tmdb"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type Service interface {
	RunSubscriptionJob() error
	GetDailyHotMovies() ([]HotResource, error)
	GetDailyHotTVs() ([]HotResource, error)
	SearchPan(keyword string) ([]SearchResult, error)
	MatchAndMount(sub *models.Subscription, title, year, category string) (*MatchResult, error)
	GetSubscriptions() ([]models.Subscription, error)
	CreateSubscription(sub *models.Subscription) error
	UpdateSubscription(sub *models.Subscription) error
	DeleteSubscription(id int64) error
	GetMatchHistory(subscriptionID int64) ([]models.MatchHistory, error)
	SetTelegramService(TelegramService)
	SetTMDBService(TMDBService)
	SetDoubanService(DoubanService)
	SetOpenAIService(OpenAIService)
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
	db              *gorm.DB
	logger          *zap.Logger
	tmdbService     TMDBService
	doubanService   DoubanService
	openaiService   OpenAIService
	telegramService TelegramService
	config          *SubscriptionConfig
	mu              sync.RWMutex
}

type SubscriptionConfig struct {
	PanSearchURL string
	EnableTMDB   bool
	EnableDouban bool
}

func NewService(db *gorm.DB, logger *zap.Logger, config *SubscriptionConfig) Service {
	return &service{
		db:     db,
		logger: logger,
		config: config,
	}
}

func (s *service) SetTelegramService(svc TelegramService) {
	s.telegramService = svc
}

func (s *service) SetTMDBService(svc TMDBService) {
	s.tmdbService = svc
}

func (s *service) SetDoubanService(svc DoubanService) {
	s.doubanService = svc
}

func (s *service) SetOpenAIService(svc OpenAIService) {
	s.openaiService = svc
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

	// 根据订阅源获取热门资源
	switch sub.Source {
	case "tmdb":
		if sub.Category == "movie" && s.config.EnableTMDB {
			items = s.getTMDbMovies()
		} else if sub.Category == "tv" && s.config.EnableTMDB {
			items = s.getTMDbTVs()
		}
	case "douban":
		if s.config.EnableDouban {
			items = s.getDoubanMovies()
		}
	}

	// 处理关键词订阅
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
		return
	}

	// 处理热门资源
	for _, item := range items {
		// 检查今天是否已处理
		var exists models.DailyHotHistory
		err := s.db.Where("source = ? AND content_id = ? AND processed_at = ?",
			item.Source, item.Title, time.Now().Format("2006-01-02")).First(&exists).Error
		if err == nil {
			s.logger.Debug("Already processed today", zap.String("title", item.Title))
			continue
		}

		// 记录处理历史
		s.db.Create(&models.DailyHotHistory{
			Source:      item.Source,
			ContentID:   item.Title,
			Title:       item.Title,
			Year:        item.Year,
			ProcessedAt: time.Now(),
		})

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
	s.db.Save(sub)
}

func (s *service) getTMDbMovies() []HotResource {
	var results []HotResource
	s.logger.Info("Fetching TMDB popular movies")
	return results
}

func (s *service) getTMDbTVs() []HotResource {
	var results []HotResource
	s.logger.Info("Fetching TMDB popular TVs")
	return results
}

func (s *service) getDoubanMovies() []HotResource {
	var results []HotResource
	s.logger.Info("Fetching Douban popular movies")
	return results
}

func (s *service) processSearchResult(sub *models.Subscription, result SearchResult) {
	matchResult, err := s.MatchAndMount(sub, result.Title, "", string(sub.Category))
	if err != nil {
		s.logger.Error("Match and mount failed", zap.Error(err))
		return
	}

	if matchResult.Success {
		sub.SuccessCount++
		sub.LastMatchAt = new(time.Time)
		*sub.LastMatchAt = time.Now()
		s.db.Save(sub)

		// 发送通知
		if s.telegramService != nil {
			s.telegramService.SendNotification(
				"资源已挂载",
				fmt.Sprintf("%s\n\nSTRM 路径：%s\n分享链接：%s", result.Title, matchResult.STrmPath, matchResult.ShareURL),
			)
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
	s.db.Create(&models.MatchHistory{
		SubscriptionID: sub.ID,
		Title:          item.Title,
		Year:           item.Year,
		Category:       models.SubscriptionCategory(item.Category),
		SearchKeyword:  item.Title,
		UpgradeKeyword: keyword,
		Status:         models.MatchStatusUpgraded,
		MatchedAt:      &matchedAt,
	})
}

func (s *service) GetDailyHotMovies() ([]HotResource, error) {
	var results []HotResource

	if s.config.EnableTMDB && s.tmdbService != nil {
		movies, err := s.tmdbService.GetPopularMovies(1)
		if err == nil {
			for _, _ = range movies {
				results = append(results, HotResource{
					Source:   "tmdb",
					Category: "movie",
				})
			}
		}
	}

	if s.config.EnableDouban && s.doubanService != nil {
		movies, err := s.doubanService.GetPopularMovies()
		if err == nil {
			for _, _ = range movies {
				results = append(results, HotResource{
					Source:   "douban",
					Category: "movie",
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
			for _, _ = range tvs {
				results = append(results, HotResource{
					Source:   "tmdb",
					Category: "tv",
				})
			}
		}
	}

	return results, nil
}

func (s *service) SearchPan(keyword string) ([]SearchResult, error) {
	if s.config.PanSearchURL == "" {
		s.config.PanSearchURL = "https://tg.252035.xyz"
	}

	apiURL := fmt.Sprintf("%s/api.php?mod=php_search&q=%s", s.config.PanSearchURL, keyword)

	resp, err := http.Get(apiURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// 简化解析，实际应根据盘搜接口返回格式解析
	var results []SearchResult
	// results = parseResponse(resp.Body)

	return results, nil
}

func (s *service) MatchAndMount(sub *models.Subscription, title, year, category string) (*MatchResult, error) {
	// 检查路径是否已存在
	mountPath := sub.MountPath
	if mountPath == "" {
		mountPath = "/热门"
	}

	// 简化实现：实际应该调用挂载服务
	result := &MatchResult{
		Success:  true,
		ShareURL: "",
		STrmPath: fmt.Sprintf("%s/%s.strm", mountPath, title),
		Message:  "Mount successful",
	}

	// 记录匹配历史
	var matchedAt = time.Now()
	s.db.Create(&models.MatchHistory{
		SubscriptionID: sub.ID,
		Title:          title,
		Year:           year,
		Category:       models.SubscriptionCategory(category),
		Status:         models.MatchStatusMatched,
		MatchedAt:      &matchedAt,
	})

	return result, nil
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

func (s *service) UpdateSubscription(sub *models.Subscription) error {
	sub.UpdatedAt = time.Now()
	return s.db.Save(sub).Error
}

func (s *service) DeleteSubscription(id int64) error {
	return s.db.Delete(&models.Subscription{}, id).Error
}

func (s *service) GetMatchHistory(subscriptionID int64) ([]models.MatchHistory, error) {
	var history []models.MatchHistory
	result := s.db.Where("subscription_id = ?", subscriptionID).Order("created_at desc").Find(&history)
	return history, result.Error
}

func parseShareURL(text string) string {
	// 匹配 189 分享链接
	regex := regexp.MustCompile(`https?://[^\s]+189[^\s]*`)
	match := regex.FindString(text)
	return match
}
