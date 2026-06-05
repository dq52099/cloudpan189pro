package models

import (
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"github.com/xxcheng123/cloudpan189-share/internal/pkgs/utils"
	"gorm.io/datatypes"
)

type CloudToken struct {
	ID          int64             `gorm:"primaryKey" json:"id"`
	Name        string            `gorm:"column:name;type:varchar(255);not null" json:"name"`
	AccessToken string            `gorm:"column:access_token;type:varchar(255);not null" json:"-"`
	ExpiresIn   int64             `gorm:"column:expires_in;type:bigint;not null" json:"expiresIn"`
	Status      int8              `gorm:"column:status;type:smallint;default:1" json:"status"`        // 状态 1:正常 2: 登录失败
	LoginType   int8              `gorm:"column:login_type;type:smallint;default:1" json:"loginType"` // 1: 扫码登录 2: 密码登录
	Username    string            `gorm:"column:username;type:varchar(255);not null;default:''" json:"username"`
	Password    string            `gorm:"column:password;type:varchar(255);not null;default:''" json:"-"`
	Addition    datatypes.JSONMap `gorm:"column:addition;type:json" json:"addition" swaggertype:"object"` // 附属参数
	UserID      int64             `gorm:"column:user_id;type:bigint;default:1;index" json:"userId"`       // 所属用户ID
	CreatedAt   time.Time         `gorm:"column:created_at;autoCreateTime;type:timestamp;default:CURRENT_TIMESTAMP" json:"createdAt"`
	UpdatedAt   time.Time         `gorm:"column:updated_at;autoUpdateTime;type:timestamp;default:CURRENT_TIMESTAMP" json:"updatedAt"`
}

func (c *CloudToken) MarshalJSON() ([]byte, error) {
	if c == nil {
		return []byte("null"), nil
	}

	type cloudTokenJSON struct {
		ID        int64             `json:"id"`
		Name      string            `json:"name"`
		ExpiresIn int64             `json:"expiresIn"`
		Status    int8              `json:"status"`
		LoginType int8              `json:"loginType"`
		Username  string            `json:"username"`
		Addition  datatypes.JSONMap `json:"addition"`
		UserID    int64             `json:"userId"`
		CreatedAt time.Time         `json:"createdAt"`
		UpdatedAt time.Time         `json:"updatedAt"`
	}

	return json.Marshal(cloudTokenJSON{
		ID:        c.ID,
		Name:      c.Name,
		ExpiresIn: c.ExpiresIn,
		Status:    c.Status,
		LoginType: c.LoginType,
		Username:  c.Username,
		Addition:  c.SanitizedAddition(),
		UserID:    c.UserID,
		CreatedAt: c.CreatedAt,
		UpdatedAt: c.UpdatedAt,
	})
}

func (c *CloudToken) SanitizedAddition() datatypes.JSONMap {
	if c == nil {
		return nil
	}

	return sanitizeCloudTokenAddition(c.Addition)
}

func sanitizeCloudTokenAddition(addition map[string]interface{}) datatypes.JSONMap {
	if addition == nil {
		return nil
	}

	sanitized := make(datatypes.JSONMap, len(addition))
	for key, value := range addition {
		if utils.IsSensitiveLogKey(key) {
			continue
		}

		sanitized[key] = sanitizeCloudTokenAdditionValue("", value)
	}

	return sanitized
}

func sanitizeCloudTokenAdditionValue(key string, value interface{}) interface{} {
	if key != "" && utils.IsSensitiveLogKey(key) {
		return utils.RedactedSecret
	}

	switch typedValue := value.(type) {
	case datatypes.JSONMap:
		return sanitizeCloudTokenAddition(typedValue)
	case map[string]interface{}:
		sanitized := make(map[string]interface{}, len(typedValue))
		for itemKey, itemValue := range typedValue {
			if utils.IsSensitiveLogKey(itemKey) {
				continue
			}

			sanitized[itemKey] = sanitizeCloudTokenAdditionValue(itemKey, itemValue)
		}

		return sanitized
	case []interface{}:
		sanitized := make([]interface{}, 0, len(typedValue))
		for _, itemValue := range typedValue {
			sanitized = append(sanitized, sanitizeCloudTokenAdditionValue("", itemValue))
		}

		return sanitized
	case string:
		return utils.RedactSensitiveText(utils.RedactURLsInTextForLog(typedValue))
	default:
		return typedValue
	}
}

func (c *CloudToken) TableName() string {
	return "cloud_tokens"
}

const (
	CloudTokenAdditionAutoLoginResultKey = "auto_login_result"
	CloudTokenAdditionAutoLoginTimes     = "auto_login_times"
	CloudTokenAdditionTokenIssuedAt      = "token_issued_at"
)

const (
	LoginTypeScan = iota + 1
	LoginTypePassword
)

const cloudTokenAbsoluteExpireMillisThreshold int64 = 10_000_000_000

func (c *CloudToken) SetIssuedAt(issuedAt time.Time) {
	if c.Addition == nil {
		c.Addition = datatypes.JSONMap{}
	}

	c.Addition[CloudTokenAdditionTokenIssuedAt] = issuedAt.UnixMilli()
}

func (c *CloudToken) IssuedAt() (time.Time, bool) {
	if c == nil || c.Addition == nil {
		return time.Time{}, false
	}

	millis, ok := parseCloudTokenMillis(c.Addition[CloudTokenAdditionTokenIssuedAt])
	if !ok || millis <= 0 {
		return time.Time{}, false
	}

	return time.UnixMilli(millis), true
}

func (c *CloudToken) ExpiresAt(now time.Time) (time.Time, bool) {
	if c == nil || c.ExpiresIn <= 0 {
		return time.Time{}, false
	}

	if c.ExpiresIn > cloudTokenAbsoluteExpireMillisThreshold {
		return time.UnixMilli(c.ExpiresIn), true
	}

	issuedAt, ok := c.IssuedAt()
	if !ok {
		issuedAt = c.CreatedAt
	}

	if issuedAt.IsZero() {
		issuedAt = c.UpdatedAt
	}

	if issuedAt.IsZero() {
		issuedAt = now
	}

	return time.Unix(issuedAt.Unix()+c.ExpiresIn, int64(issuedAt.Nanosecond())), true
}

func (c *CloudToken) AuthExpiresAtMillis(now time.Time) int64 {
	expiresAt, ok := c.ExpiresAt(now)
	if !ok {
		return 0
	}

	return expiresAt.UnixMilli()
}

func (c *CloudToken) RemainingSeconds(now time.Time) int64 {
	expiresAt, ok := c.ExpiresAt(now)
	if !ok {
		return 0
	}

	remaining := expiresAt.Sub(now)
	if remaining <= 0 {
		return 0
	}

	return int64((remaining + time.Second - time.Nanosecond) / time.Second)
}

func (c *CloudToken) IsExpired(now time.Time) bool {
	expiresAt, ok := c.ExpiresAt(now)
	if !ok {
		return true
	}

	return !expiresAt.After(now)
}

func (c *CloudToken) WillExpireWithin(now time.Time, window time.Duration) bool {
	expiresAt, ok := c.ExpiresAt(now)
	if !ok || !expiresAt.After(now) {
		return false
	}

	return !expiresAt.After(now.Add(window))
}

func parseCloudTokenMillis(value interface{}) (int64, bool) {
	switch v := value.(type) {
	case int64:
		return v, true
	case int:
		return int64(v), true
	case int32:
		return int64(v), true
	case float64:
		return int64(v), true
	case float32:
		return int64(v), true
	case json.Number:
		millis, err := v.Int64()
		if err == nil {
			return millis, true
		}

		floatValue, err := v.Float64()
		if err != nil {
			return 0, false
		}

		return int64(floatValue), true
	case string:
		millis, err := strconv.ParseInt(strings.TrimSpace(v), 10, 64)
		if err != nil {
			return 0, false
		}

		return millis, true
	default:
		return 0, false
	}
}
