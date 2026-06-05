package models

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"gorm.io/gorm"
)

type Setting struct {
	ID          int64           `gorm:"primaryKey;autoIncrement:false" json:"id"`
	Title       string          `gorm:"column:title;type:varchar(255);not null" json:"title"`
	EnableAuth  bool            `gorm:"column:enable_auth;type:boolean;default:true" json:"enableAuth"` // 是否启用鉴权 1 启用 0 不启用
	SaltKey     string          `gorm:"column:salt_key;type:varchar(255);not null" json:"-"`
	BaseURL     string          `gorm:"column:base_url;type:varchar(255);not null;default:''" json:"baseURL"` // base url
	Initialized bool            `gorm:"column:initialized;type:boolean;default:false" json:"initialized"`     // 是否初始化完成
	Addition    SettingAddition `gorm:"column:addition;type:json" json:"addition" swaggertype:"object"`
	CreatedAt   time.Time       `gorm:"column:created_at;autoCreateTime;type:timestamp;default:CURRENT_TIMESTAMP" json:"createdAt"`
	UpdatedAt   time.Time       `gorm:"column:updated_at;autoUpdateTime;type:timestamp;default:CURRENT_TIMESTAMP" json:"updatedAt"`
}

func (s *Setting) TableName() string {
	return "setting"
}

// AfterFind GORM hook: 在查询完成后注入默认值
func (s *Setting) AfterFind(tx *gorm.DB) (err error) {
	s.Addition.applyDefaults()

	return nil
}

// SettingAddition 系统附加配置
type SettingAddition struct {
	Keep                      string   `json:"-"`
	LocalProxy                bool     `json:"localProxy"`
	LocalProxyURL             string   `json:"localProxyURL"` // HTTP 代理地址，如 http://192.168.31.51:7890
	MultipleStream            bool     `json:"multipleStream"`
	MultipleStreamThreadCount int      `json:"multipleStreamThreadCount"`
	MultipleStreamChunkSize   int64    `json:"multipleStreamChunkSize"`
	TaskThreadCount           int      `json:"taskThreadCount"`
	WorkerCount               int      `json:"workerCount"`
	EnableStorageAutoRefresh  bool     `json:"enableStorageAutoRefresh"`
	WebDAVUserStrmOnly        bool     `json:"webdavUserStrmOnly"`
	WebDAVAllowedSuffixes     []string `json:"webdavAllowedSuffixes"`
}

var DefaultWebDAVAllowedSuffixes = []string{
	".mp4", ".mkv", ".avi", ".mov", ".wmv", ".flv", ".webm", ".m3u8", ".m4v", ".mpg", ".mpeg",
	".m2v", ".m4p", ".m4b", ".ts", ".mts", ".m2ts", ".m2t", ".mxf", ".dv", ".dvr-ms",
	".asf", ".3gp", ".3g2", ".f4v", ".f4p", ".f4a", ".f4b", ".vob", ".ogv", ".ogg",
	".divx", ".xvid", ".rm", ".rmvb", ".dat", ".nsv", ".qt", ".amv", ".mpv", ".m1v",
	".svi", ".viv", ".fli", ".flc",
}

func NormalizeSuffixes(items []string) []string {
	if len(items) == 0 {
		return nil
	}

	result := make([]string, 0, len(items))

	seen := make(map[string]struct{}, len(items))
	for _, item := range items {
		suffix := strings.ToLower(strings.TrimSpace(item))
		if suffix == "" {
			continue
		}

		if !strings.HasPrefix(suffix, ".") {
			suffix = "." + suffix
		}

		if _, ok := seen[suffix]; ok {
			continue
		}

		seen[suffix] = struct{}{}
		result = append(result, suffix)
	}

	return result
}

// applyDefaults 统一填充默认值，确保零值时也能获得期望配置
func (sa *SettingAddition) applyDefaults() {
	if sa.MultipleStreamThreadCount <= 0 {
		sa.MultipleStreamThreadCount = 4
	}

	if sa.MultipleStreamChunkSize <= 0 {
		sa.MultipleStreamChunkSize = 4 * 1024 * 1024 // 4MiB
	}

	if sa.TaskThreadCount <= 0 {
		sa.TaskThreadCount = 1
	}

	if sa.WorkerCount <= 0 {
		sa.WorkerCount = 5
	}

	sa.WebDAVAllowedSuffixes = NormalizeSuffixes(sa.WebDAVAllowedSuffixes)
	if len(sa.WebDAVAllowedSuffixes) == 0 {
		sa.WebDAVAllowedSuffixes = append([]string(nil), DefaultWebDAVAllowedSuffixes...)
	}

	// 不再强制设置 EnableStorageAutoRefresh 默认为 true，保持用户设置的值
}

func (sa *SettingAddition) ApplyDefaultsForWrite() {
	sa.applyDefaults()
}

// Value 实现 driver.Valuer 接口 - 将结构体转换为数据库值
func (sa SettingAddition) Value() (driver.Value, error) {
	normalized := sa

	normalized.WebDAVAllowedSuffixes = NormalizeSuffixes(normalized.WebDAVAllowedSuffixes)
	if normalized.Keep == "" &&
		!normalized.LocalProxy &&
		normalized.LocalProxyURL == "" &&
		!normalized.MultipleStream &&
		normalized.MultipleStreamThreadCount == 0 &&
		normalized.MultipleStreamChunkSize == 0 &&
		normalized.TaskThreadCount == 0 &&
		normalized.WorkerCount == 0 &&
		!normalized.EnableStorageAutoRefresh &&
		!normalized.WebDAVUserStrmOnly &&
		len(normalized.WebDAVAllowedSuffixes) == 0 {
		return nil, nil
	}

	return json.Marshal(normalized)
}

// Scan 实现 sql.Scanner 接口 - 从数据库值转换为结构体
func (sa *SettingAddition) Scan(value interface{}) error {
	if value == nil {
		*sa = SettingAddition{}

		return nil
	}

	var bytes []byte

	switch v := value.(type) {
	case []byte:
		bytes = v
	case string:
		bytes = []byte(v)
	default:
		return errors.New("cannot scan SettingAddition from non-string/[]byte value")
	}

	return json.Unmarshal(bytes, sa)
}
