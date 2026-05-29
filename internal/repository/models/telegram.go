package models

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"time"
)

type TelegramSetting struct {
	ID                int64     `gorm:"primaryKey;autoIncrement:false" json:"id"`
	BotTokenEncrypted string    `gorm:"column:bot_token_encrypted;type:varchar(512);not null" json:"-"`
	BotToken          string    `gorm:"-" json:"botToken"`
	ProxyURL          string    `gorm:"column:proxy_url;type:varchar(255);default:''" json:"proxyURL"`
	ProxyType         string    `gorm:"column:proxy_type;type:varchar(20);default:''" json:"proxyType"` // http, https, socks5
	APIURL            string    `gorm:"column:api_url;type:varchar(255);default:'https://api.telegram.org'" json:"apiURL"`
	ChatID            string    `gorm:"column:chat_id;type:varchar(64);default:''" json:"chatID"`
	DefaultMountPath  string    `gorm:"column:default_mount_path;type:varchar(255);default:'/转存'" json:"defaultMountPath"`
	EnableNotify      bool      `gorm:"column:enable_notify;type:boolean;default:true" json:"enableNotify"`
	Enable            bool      `gorm:"column:enable;type:boolean;default:false" json:"enable"`
	CreatedAt         time.Time `gorm:"column:created_at;autoCreateTime;type:timestamp;default:CURRENT_TIMESTAMP" json:"createdAt"`
	UpdatedAt         time.Time `gorm:"column:updated_at;autoUpdateTime;type:timestamp;default:CURRENT_TIMESTAMP" json:"updatedAt"`
}

func (t *TelegramSetting) TableName() string {
	return "telegram_settings"
}

type TelegramUser struct {
	ID         int64     `gorm:"primaryKey" json:"id"`
	UserID     int64     `gorm:"column:user_id;not null;uniqueIndex" json:"userID"`
	Username   string    `gorm:"column:username;type:varchar(255)" json:"username"`
	FirstName  string    `gorm:"column:first_name;type:varchar(255)" json:"firstName"`
	LastName   string    `gorm:"column:last_name;type:varchar(255)" json:"lastName"`
	MountPath  string    `gorm:"column:mount_path;type:varchar(255);default:''" json:"mountPath"`
	IsAdmin    bool      `gorm:"column:is_admin;type:boolean;default:false" json:"isAdmin"`
	LastSeenAt time.Time `gorm:"column:last_seen_at;type:timestamp" json:"lastSeenAt"`
	CreatedAt  time.Time `gorm:"column:created_at;autoCreateTime;type:timestamp;default:CURRENT_TIMESTAMP" json:"createdAt"`
	UpdatedAt  time.Time `gorm:"column:updated_at;autoUpdateTime;type:timestamp;default:CURRENT_TIMESTAMP" json:"updatedAt"`
}

func (t *TelegramUser) TableName() string {
	return "telegram_users"
}

type SubscriptionSource string

const (
	SubscriptionSourceTMDB   SubscriptionSource = "tmdb"
	SubscriptionSourceDouban SubscriptionSource = "douban"
)

type SubscriptionCategory string

const (
	SubscriptionCategoryMovie SubscriptionCategory = "movie"
	SubscriptionCategoryTV    SubscriptionCategory = "tv"
	SubscriptionCategoryAnime SubscriptionCategory = "anime"
	SubscriptionCategoryDoc   SubscriptionCategory = "doc"
)

type Subscription struct {
	ID                int64                `gorm:"primaryKey" json:"id"`
	Name              string               `gorm:"column:name;type:varchar(255);not null" json:"name"`
	Source            SubscriptionSource   `gorm:"column:source;type:varchar(20);not null" json:"source"`        // tmdb, douban
	Category          SubscriptionCategory `gorm:"column:category;type:varchar(20);not null" json:"category"`    // movie, tv, anime, doc
	Keywords          string               `gorm:"column:keywords;type:varchar(500);default:''" json:"keywords"` // 关键词订阅，用逗号分隔
	ListType          string               `gorm:"column:list_type;type:varchar(50);default:''" json:"listType"` // popular, top250, coming, playing 等
	MountPath         string               `gorm:"column:mount_path;type:varchar(255);default:''" json:"mountPath"`
	Enable            bool                 `gorm:"column:enable;type:boolean;default:true" json:"enable"`
	EnableAutoUpgrade bool                 `gorm:"column:enable_auto_upgrade;type:boolean;default:false" json:"enableAutoUpgrade"` // AI 洗版
	MatchCount        int                  `gorm:"column:match_count;not null;default:0" json:"matchCount"`
	SuccessCount      int                  `gorm:"column:success_count;not null;default:0" json:"successCount"`
	LastRunAt         *time.Time           `gorm:"column:last_run_at;type:timestamp" json:"lastRunAt"`
	LastMatchAt       *time.Time           `gorm:"column:last_match_at;type:timestamp" json:"lastMatchAt"`
	CreatedAt         time.Time            `gorm:"column:created_at;autoCreateTime;type:timestamp;default:CURRENT_TIMESTAMP" json:"createdAt"`
	UpdatedAt         time.Time            `gorm:"column:updated_at;autoUpdateTime;type:timestamp;default:CURRENT_TIMESTAMP" json:"updatedAt"`
}

func (s *Subscription) TableName() string {
	return "subscriptions"
}

type MatchStatus string

const (
	MatchStatusPending  MatchStatus = "pending"
	MatchStatusMatched  MatchStatus = "matched"
	MatchStatusFailed   MatchStatus = "failed"
	MatchStatusUpgraded MatchStatus = "upgraded"
)

type MatchHistory struct {
	ID             int64                `gorm:"primaryKey" json:"id"`
	SubscriptionID int64                `gorm:"column:subscription_id;not null;index" json:"subscriptionID"`
	Title          string               `gorm:"column:title;type:varchar(500);not null" json:"title"`
	Year           string               `gorm:"column:year;type:varchar(10)" json:"year"`
	Category       SubscriptionCategory `gorm:"column:category;type:varchar(20)" json:"category"`
	SearchKeyword  string               `gorm:"column:search_keyword;type:varchar(500)" json:"searchKeyword"`
	UpgradeKeyword string               `gorm:"column:upgrade_keyword;type:varchar(500)" json:"upgradeKeyword"`
	ShareURL       string               `gorm:"column:share_url;type:varchar(1000)" json:"shareURL"`
	STrmPath       string               `gorm:"column:strm_path;type:varchar(1000)" json:"strmPath"`
	Status         MatchStatus          `gorm:"column:status;type:varchar(20);not null" json:"status"`
	RetryCount     int                  `gorm:"column:retry_count;not null;default:0" json:"retryCount"`
	ErrorMessage   string               `gorm:"column:error_message;type:text" json:"errorMessage"`
	MatchedAt      *time.Time           `gorm:"column:matched_at;type:timestamp" json:"matchedAt"`
	CreatedAt      time.Time            `gorm:"column:created_at;autoCreateTime;type:timestamp;default:CURRENT_TIMESTAMP" json:"createdAt"`
	UpdatedAt      time.Time            `gorm:"column:updated_at;autoUpdateTime;type:timestamp;default:CURRENT_TIMESTAMP" json:"updatedAt"`
}

func (m *MatchHistory) TableName() string {
	return "match_history"
}

type DailyHotHistory struct {
	ID          int64     `gorm:"primaryKey" json:"id"`
	Source      string    `gorm:"column:source;type:varchar(20);not null" json:"source"` // tmdb, douban
	ContentID   string    `gorm:"column:content_id;type:varchar(100);not null" json:"contentID"`
	Title       string    `gorm:"column:title;type:varchar(500);not null" json:"title"`
	Year        string    `gorm:"column:year;type:varchar(10)" json:"year"`
	ProcessedAt time.Time `gorm:"column:processed_at;type:timestamp;not null" json:"processedAt"`
	CreatedAt   time.Time `gorm:"column:created_at;autoCreateTime;type:timestamp;default:CURRENT_TIMESTAMP" json:"createdAt"`
}

func (d *DailyHotHistory) TableName() string {
	return "daily_hot_history"
}

type SubscriptionConfig struct {
	Enabled          bool   `json:"enabled"`
	CronExpression   string `json:"cronExpression"`
	PanSearchURL     string `json:"panSearchURL"`
	EnableTMDB       bool   `json:"enableTMDB"`
	EnableDouban     bool   `json:"enableDouban"`
	DefaultMountPath string `json:"defaultMountPath"`
	AutoMount        bool   `json:"autoMount"`
	TMDBAPIKey       string `json:"tmdbAPIKey"`
	OpenAIAPIKey     string `json:"openaiAPIKey"`
	OpenAIBaseURL    string `json:"openaiBaseURL"`
	OpenAIModel      string `json:"openaiModel"`
}

func (sc SubscriptionConfig) Value() (driver.Value, error) {
	if sc == (SubscriptionConfig{}) {
		return nil, nil
	}

	return json.Marshal(sc)
}

func (sc *SubscriptionConfig) Scan(value interface{}) error {
	if value == nil {
		*sc = SubscriptionConfig{}

		return nil
	}

	var bytes []byte

	switch v := value.(type) {
	case []byte:
		bytes = v
	case string:
		bytes = []byte(v)
	default:
		return errors.New("cannot scan SubscriptionConfig from non-string/[]byte value")
	}

	return json.Unmarshal(bytes, sc)
}
