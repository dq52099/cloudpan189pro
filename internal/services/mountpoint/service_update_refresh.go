package mountpoint

import (
	"errors"
	"time"

	"github.com/xxcheng123/cloudpan189-share/internal/framework/context"
	"go.uber.org/zap"
)

// RefreshConfig 刷新配置更新结构体
type RefreshConfig struct {
	RefreshInterval    *int       `json:"refreshInterval,omitempty"`    // 刷新间隔（分钟）
	AutoRefreshDays    *int       `json:"autoRefreshDays,omitempty"`    // 自动刷新持续天数
	AutoRefreshBeginAt *time.Time `json:"autoRefreshBeginAt,omitempty"` // 自动刷新开始时间
	EnableDeepRefresh  *bool      `json:"enableDeepRefresh,omitempty"`  // 是否启用深度刷新
}

// UpdateRefreshConfig 更新挂载点刷新配置（合并三个方法为一个）
func (s *service) UpdateRefreshConfig(ctx context.Context, fileId int64, config RefreshConfig) error {
	if fileId <= 0 {
		return errInvalidMountPointFileID
	}

	// 验证参数
	if err := s.validateRefreshConfig(config); err != nil {
		return err
	}

	// 构建更新字段映射
	updates := make(map[string]interface{})

	if config.RefreshInterval != nil {
		updates["refresh_interval"] = *config.RefreshInterval
	}

	if config.AutoRefreshDays != nil {
		updates["auto_refresh_days"] = *config.AutoRefreshDays
	}

	if config.AutoRefreshBeginAt != nil {
		updates["auto_refresh_begin_at"] = *config.AutoRefreshBeginAt
	}

	if config.EnableDeepRefresh != nil {
		updates["enable_deep_refresh"] = *config.EnableDeepRefresh
	}

	// 如果没有要更新的字段，直接返回
	if len(updates) == 0 {
		return nil
	}

	// 执行更新
	result := s.getDB(ctx).Where("file_id = ?", fileId).Updates(updates)
	if result.Error != nil {
		ctx.Error("更新挂载点刷新配置失败",
			zap.Error(result.Error),
			zap.Int64("fileId", fileId),
			zap.Any("config", config))

		return result.Error
	}

	if result.RowsAffected == 0 {
		if err := s.ensureMountPointFileExists(ctx, fileId); err != nil {
			ctx.Error("更新挂载点刷新配置失败，记录不存在",
				zap.Int64("fileId", fileId),
				zap.Any("config", config))

			return err
		}
	}

	ctx.Info("更新挂载点刷新配置成功",
		zap.Int64("fileId", fileId),
		zap.Any("updates", updates))

	return nil
}

// validateRefreshConfig 验证刷新配置参数
func (s *service) validateRefreshConfig(config RefreshConfig) error {
	// 验证刷新间隔，最小值为30分钟
	if config.RefreshInterval != nil {
		interval := *config.RefreshInterval
		if interval > 0 && interval < 30 {
			return errors.New("刷新间隔最小值为30分钟")
		}
	}

	// 验证自动刷新持续天数，最小值为1天，最大值为365天
	if config.AutoRefreshDays != nil {
		days := *config.AutoRefreshDays
		if days > 0 && (days < 1 || days > 365) {
			return errors.New("自动刷新持续天数范围为1-365天")
		}
	}

	return nil
}
