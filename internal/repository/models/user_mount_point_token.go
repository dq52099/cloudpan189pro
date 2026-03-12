package models

import (
	"time"
)

type UserMountPointToken struct {
	ID           int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID       int64     `gorm:"column:user_id;type:bigint;not null;index" json:"userId"`
	MountPointID int64     `gorm:"column:mount_point_id;type:bigint;not null;index" json:"mountPointId"`
	TokenID      int64     `gorm:"column:token_id;type:bigint;not null" json:"tokenId"`
	CreatedAt    time.Time `gorm:"column:created_at;autoCreateTime;type:timestamp" json:"createdAt"`
	UpdatedAt    time.Time `gorm:"column:updated_at;autoUpdateTime;type:timestamp" json:"updatedAt"`
}

func (u *UserMountPointToken) TableName() string {
	return "user_mount_point_tokens"
}
