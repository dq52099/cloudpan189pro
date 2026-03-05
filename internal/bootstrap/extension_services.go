package bootstrap

import (
	"os"

	"github.com/xxcheng123/cloudpan189-share/internal/configs"
	"github.com/xxcheng123/cloudpan189-share/internal/repository/models"
	"github.com/xxcheng123/cloudpan189-share/internal/services/douban"
	"github.com/xxcheng123/cloudpan189-share/internal/services/openai"
	"github.com/xxcheng123/cloudpan189-share/internal/services/subscription"
	"github.com/xxcheng123/cloudpan189-share/internal/services/telegram"
	"github.com/xxcheng123/cloudpan189-share/internal/services/tmdb"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// ExtensionServices 扩展服务集合
type ExtensionServices struct {
	Telegram        telegram.Service
	TMDB            tmdb.Service
	Douban          douban.Service
	OpenAI          openai.Service
	Subscription    subscription.Service
	TelegramSetting *models.TelegramSetting
}

// InitExtensionServices 初始化扩展服务
func InitExtensionServices(db *gorm.DB, logger *zap.Logger, cfg *configs.Config) (*ExtensionServices, error) {
	ext := &ExtensionServices{}

	// 1. 从数据库加载 Telegram 设置
	var telegramSetting models.TelegramSetting
	_ = db.First(&telegramSetting)

	ext.TelegramSetting = &telegramSetting

	// 2. 初始化 Telegram 服务
	// 优先级：环境变量 > 数据库配置
	telegramSetting.BotToken = telegramSetting.BotTokenEncrypted
	botToken := telegramSetting.BotToken
	chatID := telegramSetting.ChatID
	proxyURL := telegramSetting.ProxyURL
	proxyType := telegramSetting.ProxyType
	apiURL := telegramSetting.APIURL

	// 环境变量优先级
	if envToken := os.Getenv("TG_BOT_TOKEN"); envToken != "" {
		botToken = envToken
	}
	if envChatID := os.Getenv("TG_CHAT_ID"); envChatID != "" {
		chatID = envChatID
	}
	if envProxy := os.Getenv("TG_PROXY"); envProxy != "" {
		proxyURL = envProxy
	}
	if envProxyType := os.Getenv("TG_PROXY_TYPE"); envProxyType != "" {
		proxyType = envProxyType
	}

	ext.Telegram = telegram.NewService(botToken, chatID, proxyURL, proxyType, apiURL, logger.Named("telegram"))

	// 3. 初始化 TMDB 服务
	tmdbAPIKey := os.Getenv("TMDB_API_KEY")
	tmdbProxyURL := os.Getenv("TMDB_PROXY")
	tmdbProxyType := os.Getenv("TMDB_PROXY_TYPE")
	ext.TMDB = tmdb.NewService(logger.Named("tmdb"), tmdbAPIKey, "", tmdbProxyURL, tmdbProxyType)

	// 4. 初始化 Douban 服务
	ext.Douban = douban.NewService(logger.Named("douban"))

	// 5. 初始化 OpenAI 服务
	// 优先级：环境变量 > 配置文件
	openaiAPIKey := cfg.OpenAI.APIKey
	openaiBaseURL := cfg.OpenAI.BaseURL
	openaiModel := cfg.OpenAI.Model
	if envToken := os.Getenv("OPENAI_API_KEY"); envToken != "" {
		openaiAPIKey = envToken
	}
	if envBaseURL := os.Getenv("OPENAI_BASE_URL"); envBaseURL != "" {
		openaiBaseURL = envBaseURL
	}
	if envModel := os.Getenv("OPENAI_MODEL"); envModel != "" {
		openaiModel = envModel
	}
	ext.OpenAI = openai.NewService(logger.Named("openai"), openaiAPIKey, openaiBaseURL, openaiModel)

	// 6. 初始化 Subscription 服务
	// 优先级：环境变量 > 配置文件
	panSearchURL := "https://tg.252035.xyz"
	enableTMDB := true
	enableDouban := true
	if cfg.Subscription != nil {
		panSearchURL = cfg.Subscription.PanSearchURL
		enableTMDB = cfg.Subscription.EnableTMDB
		enableDouban = cfg.Subscription.EnableDouban
	}
	if envURL := os.Getenv("PAN_SEARCH_URL"); envURL != "" {
		panSearchURL = envURL
	}
	if envTMDB := os.Getenv("SUBSCRIPTION_ENABLE_TMDB"); envTMDB != "" {
		enableTMDB = envTMDB == "true"
	}
	if envDouban := os.Getenv("SUBSCRIPTION_ENABLE_DOUBAN"); envDouban != "" {
		enableDouban = envDouban == "true"
	}
	subscriptionConfig := &subscription.SubscriptionConfig{
		PanSearchURL: panSearchURL,
		EnableTMDB:   enableTMDB,
		EnableDouban: enableDouban,
	}
	ext.Subscription = subscription.NewService(db, logger.Named("subscription"), subscriptionConfig)

	// 7. 设置依赖关系
	ext.Subscription.SetTelegramService(ext.Telegram)
	ext.Subscription.SetTMDBService(ext.TMDB)
	ext.Subscription.SetDoubanService(ext.Douban)
	ext.Subscription.SetOpenAIService(ext.OpenAI)

	return ext, nil
}

// StartTelegramBot 启动 Telegram Bot 轮询
func (ext *ExtensionServices) StartTelegramBot() error {
	if ext.Telegram != nil && ext.Telegram.IsEnabled() {
		return ext.Telegram.StartBot()
	}
	return nil
}

// StopTelegramBot 停止 Telegram Bot 轮询
func (ext *ExtensionServices) StopTelegramBot() {
	if ext.Telegram != nil && ext.Telegram.IsEnabled() {
		ext.Telegram.StopBot()
	}
}
