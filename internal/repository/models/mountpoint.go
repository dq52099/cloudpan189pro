package models

import (
	"time"
)

type MountPoint struct {
	ID                 int64      `gorm:"primaryKey" json:"id"`
	FileId             int64      `gorm:"column:file_id;not null;uniqueIndex:idx_file_id_name" json:"fileId"`
	OsType             string     `gorm:"column:os_type;type:varchar(1024);not null" json:"osType"`
	TokenId            int64      `gorm:"column:token_id;not null;default:0" json:"tokenId"`                       // 关联的token id
	CreatorUserID      int64      `gorm:"column:creator_user_id;type:bigint;default:1;index" json:"creatorUserId"` // 创建者用户ID
	Name               string     `gorm:"column:name;type:varchar(1024);not null;uniqueIndex:idx_file_id_name" json:"name"`
	FullPath           string     `gorm:"column:full_path;type:text;default:''" json:"fullPath"`
	EnableAutoRefresh  bool       `gorm:"column:enable_auto_refresh;not null;default:false" json:"enableAutoRefresh"`
	AutoRefreshDays    int        `gorm:"column:auto_refresh_days;not null;default:7;comment:'单位:刷新持续天数'" json:"autoRefreshDays"`
	AutoRefreshBeginAt *time.Time `gorm:"column:auto_refresh_begin_at;type:timestamp;default:null" json:"autoRefreshBeginAt"`
	RefreshInterval    int        `gorm:"column:refresh_interval;not null;default:30;comment:'单位:分钟，最小值30'" json:"refreshInterval"`
	EnableDeepRefresh  bool       `gorm:"column:enable_deep_refresh;not null;default:false" json:"enableDeepRefresh"`
	LastState          string     `gorm:"column:last_state;type:varchar(1024);not null;default:'成功'" json:"lastState"`
	CreatedAt          time.Time  `gorm:"column:created_at;autoCreateTime;type:timestamp;default:CURRENT_TIMESTAMP" json:"createdAt"`
	UpdatedAt          time.Time  `gorm:"column:updated_at;autoUpdateTime;type:timestamp;default:CURRENT_TIMESTAMP" json:"updatedAt"`
}

func (m *MountPoint) TableName() string {
	return "mount_points"
}

// 是否还在自动刷新时间范围内
func (m *MountPoint) IsInAutoRefreshPeriod() bool {
	if !m.EnableAutoRefresh || m.AutoRefreshBeginAt == nil {
		return false
	}

	now := time.Now()
	if now.Before(*m.AutoRefreshBeginAt) {
		return false
	}

	endTime := m.AutoRefreshBeginAt.AddDate(0, 0, m.AutoRefreshDays)

	return now.Before(endTime)
}
