package models

import (
	"time"

	"github.com/xxcheng123/cloudpan189-share/internal/pkgs/datatypes"
	"github.com/xxcheng123/cloudpan189-share/internal/types/loginlog"
)

const (
	StatusPending   = "pending"
	StatusRunning   = "running"
	StatusCompleted = "completed"
	StatusFailed    = "failed"
)

type FileTaskLog struct {
	ID int64 `gorm:"primaryKey" json:"id"`

	Title string `gorm:"column:title;type:varchar(255);not null" json:"title"`     // 任务标题
	Type  string `gorm:"column:type;type:varchar(100);not null;index" json:"type"` // 任务类型（如：file_refresh, file_scan等）
	Desc  string `gorm:"column:desc;type:text" json:"desc"`

	BeginAt time.Time  `gorm:"column:begin_at;type:timestamp;not null" json:"beginAt"` // 开始时间
	EndAt   *time.Time `gorm:"column:end_at;type:timestamp" json:"endAt"`              // 结束时间

	Status   string            `gorm:"column:status;type:varchar(50);not null;default:'pending';index" json:"status"` // 状态：pending, running, completed, failed
	Result   string            `gorm:"column:result;type:varchar(1024)" json:"result"`                                // 执行结果描述
	ErrorMsg string            `gorm:"column:error_msg;type:text" json:"errorMsg"`                                    // 错误信息
	Addition datatypes.JSONMap `gorm:"column:addition;type:json" json:"addition"`                                     // 附加信息
	Duration int64             `gorm:"column:duration;type:bigint;default:0" json:"duration"`                         // 执行耗时（毫秒）

	// 关联信息
	FileId int64 `gorm:"column:file_id;type:bigint;index;default:0" json:"fileId"` // 关联的文件ID（如果适用）
	UserID int64 `gorm:"column:user_id;type:bigint;index;default:0" json:"userId"` // 触发用户ID（如果适用）

	// 进度显示
	Completed int64 `gorm:"column:completed;type:bigint;default:0" json:"completed"` // 已完成数量
	Total     int64 `gorm:"column:total;type:bigint;default:0" json:"total"`         // 总数量

	CreatedAt time.Time `gorm:"column:created_at;autoCreateTime;type:timestamp;default:CURRENT_TIMESTAMP" json:"createdAt"`
	UpdatedAt time.Time `gorm:"column:updated_at;autoUpdateTime;type:timestamp;default:CURRENT_TIMESTAMP" json:"updatedAt"`
}

func (l *FileTaskLog) TableName() string {
	return "file_task_logs"
}

type LoginLog struct {
	ID        int64           `gorm:"primaryKey" json:"id"`
	UserId    int64           `gorm:"column:user_id;type:bigint;index;index:idx_user_time,priority:1;default:0" json:"userId"`
	Username  string          `gorm:"column:username;type:varchar(255);not null;default:'';comment:用户名" json:"username"`
	Addr      string          `gorm:"column:addr;type:varchar(45);not null;default:'';comment:客户端地址或IP(IPv4/IPv6);index:idx_addr_time,priority:1" json:"addr"`
	Location  string          `gorm:"column:location;type:varchar(255);not null;default:'';comment:地理信息(可脱敏)" json:"location"`
	Method    loginlog.Method `gorm:"column:method;type:varchar(64);not null;default:'web';comment:事件来源(web/api/app/cli);index:idx_method_time,priority:1" json:"method"`
	Event     loginlog.Event  `gorm:"column:event;type:varchar(32);not null;default:'login';comment:事件类型(login/refresh_token);index:idx_event_time,priority:1" json:"event"`
	Status    loginlog.Status `gorm:"column:status;type:varchar(16);not null;default:'failed';comment:success/failed/blocked;index:idx_status_time,priority:1" json:"status"`
	Reason    string          `gorm:"column:reason;type:varchar(255);default:'';comment:失败或拦截原因" json:"reason"`
	UserAgent string          `gorm:"column:user_agent;type:varchar(512);default:'';comment:客户端UA" json:"userAgent"`
	TraceId   string          `gorm:"column:trace_id;type:varchar(255);not null;default:''" json:"traceId"`
	CreatedAt time.Time       `gorm:"column:created_at;autoCreateTime;type:timestamp;default:CURRENT_TIMESTAMP;index:idx_user_time,priority:2;index:idx_status_time,priority:2;index:idx_event_time,priority:2;index:idx_method_time,priority:2;index:idx_addr_time,priority:2" json:"createdAt"`
	UpdatedAt time.Time       `gorm:"column:updated_at;autoUpdateTime;type:timestamp;default:CURRENT_TIMESTAMP" json:"updatedAt"`
}

func (l *LoginLog) TableName() string {
	return "login_logs"
}
