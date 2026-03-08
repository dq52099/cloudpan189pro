package models

import (
	"time"

	"github.com/xxcheng123/cloudpan189-share/internal/pkgs/datatypes"
	"github.com/xxcheng123/cloudpan189-share/internal/types/media"
)

type MediaFile struct {
	ID        int64      `gorm:"primaryKey" json:"id"`
	FID       int64      `gorm:"column:fid;type:bigint;not null" json:"fid"` // 与之关联的文件ID
	Name      string     `gorm:"column:name;type:varchar(255);not null" json:"name"`
	Path      string     `gorm:"column:path;type:varchar(255);not null;uniqueIndex" json:"path"` // 包括文件名
	Size      int64      `gorm:"column:size;type:bigint;not null" json:"size"`
	MediaType media.Type `gorm:"column:media_type;type:varchar(20);not null;default:'strm'" json:"mediaType"`
	Hash      string     `gorm:"column:hash;type:varchar(255);not null" json:"hash"`
	CreatedAt time.Time  `gorm:"column:created_at;autoCreateTime;type:timestamp;default:CURRENT_TIMESTAMP" json:"createdAt"`
	UpdatedAt time.Time  `gorm:"column:updated_at;autoUpdateTime;type:timestamp;default:CURRENT_TIMESTAMP;on update:CURRENT_TIMESTAMP" json:"updatedAt"`
}

func (m *MediaFile) TableName() string {
	return "media_files"
}

type MediaConfig struct {
	ID     int64 `gorm:"primaryKey" json:"id"`
	Enable bool  `gorm:"column:enable;type:boolean;not null;default:false" json:"enable"`
	// StoragePath StoragePath 落盘根路径
	StoragePath string `gorm:"column:storage_path;type:varchar(255);not null" json:"storagePath"`
	// AutoClean 自动清理空文件夹 文件删除后自动检查是否为空文件夹
	AutoClean bool `gorm:"column:auto_clean;type:boolean;not null;default:false" json:"autoClean"`
	// ConflictPolicy 冲突策略 跳过/替换
	ConflictPolicy media.FileConflictPolicy `gorm:"column:conflict_policy;type:varchar(20);not null;default:'skip'" json:"conflictPolicy"`
	// IncludedSuffixes 包括的后缀格式 不包括的将过滤 如果为空则表示不过滤
	IncludedSuffixes datatypes.JSONSlice[string] `gorm:"column:included_suffixes;type:json;not null" json:"includedSuffixes"`
	BaseURL          string                      `gorm:"column:base_url;type:varchar(255);not null" json:"baseURL"`
	// AutoRebuildEnable 定时重建strm开关
	AutoRebuildEnable bool `gorm:"column:auto_rebuild_enable;type:boolean;not null;default:false" json:"autoRebuildEnable"`
	// AutoRebuildInterval 定时重建间隔（小时）- 已废弃，使用Cron表达式
	AutoRebuildInterval int `gorm:"column:auto_rebuild_interval;type:int;not null;default:24" json:"autoRebuildInterval"`
	// AutoRebuildCron 定时重建cron表达式
	AutoRebuildCron string `gorm:"column:auto_rebuild_cron;type:varchar(50);not null;default:'0 2 * * *'" json:"autoRebuildCron"`
	// LastRebuildTime 上次重建时间
	LastRebuildTime time.Time `gorm:"column:last_rebuild_time;type:timestamp" json:"lastRebuildTime"`
	CreatedAt       time.Time `gorm:"column:created_at;autoCreateTime;type:timestamp;default:CURRENT_TIMESTAMP" json:"createdAt"`
	UpdatedAt       time.Time `gorm:"column:updated_at;autoUpdateTime;type:timestamp;default:CURRENT_TIMESTAMP;on update:CURRENT_TIMESTAMP" json:"updatedAt"`
}

func (m *MediaConfig) TableName() string {
	return "media_config"
}

func (m *MediaConfig) GetCar(parentPath string, subPath string) media.WriterCar {
	return media.NewWriterCar(m.StoragePath, m.ConflictPolicy, m.BaseURL).
		NewSubCar(parentPath).
		NewSubCar(subPath)
}
