package filetasklog

import (
	"fmt"
	"time"

	"github.com/xxcheng123/cloudpan189-share/internal/framework/context"
	"github.com/xxcheng123/cloudpan189-share/internal/repository/models"
	"go.uber.org/zap"
)

// Clear 清空全部任务日志
func (s *service) Clear(ctx context.Context) (int64, error) {
	// 使用 ORM 删除整表，避免硬编码表名，也保留 GORM 钩子
	result := s.getDB(ctx).Unscoped().Where("1 = 1").Delete(&models.FileTaskLog{})
	if result.Error != nil {
		ctx.Error("清空任务日志失败", zap.Error(result.Error))

		return result.RowsAffected, result.Error
	}

	ctx.Info("清空任务日志成功", zap.Int64("deleted", result.RowsAffected))

	return result.RowsAffected, nil
}

// ClearByDuration 按保留时长清理任务日志。
// 支持常见的简写（1h/1d/7d/30d/90d），也支持 time.ParseDuration 能解析的格式。
func (s *service) ClearByDuration(ctx context.Context, duration string) (int64, error) {
	cutoff, err := resolveCutoff(duration)
	if err != nil {
		ctx.Warn("无法识别的清理时长", zap.String("duration", duration), zap.Error(err))

		return 0, err
	}

	result := s.getDB(ctx).Unscoped().
		Where("created_at < ?", cutoff).
		Delete(&models.FileTaskLog{})
	if result.Error != nil {
		ctx.Error("按时长清理任务日志失败", zap.Error(result.Error))

		return result.RowsAffected, result.Error
	}

	ctx.Info("按时长清理任务日志成功", zap.String("duration", duration), zap.Int64("deleted", result.RowsAffected))

	return result.RowsAffected, nil
}

// resolveCutoff 根据 duration 字符串计算清理截止时间。
func resolveCutoff(duration string) (time.Time, error) {
	now := time.Now()

	switch duration {
	case "1h":
		return now.Add(-1 * time.Hour), nil
	case "1d":
		return now.AddDate(0, 0, -1), nil
	case "7d":
		return now.AddDate(0, 0, -7), nil
	case "30d":
		return now.AddDate(0, 0, -30), nil
	case "90d":
		return now.AddDate(0, 0, -90), nil
	case "":
		return time.Time{}, fmt.Errorf("duration 不能为空")
	}

	// 兜底：尝试 Go 标准库格式（如 "24h"、"2160h"）
	d, err := time.ParseDuration(duration)
	if err != nil {
		return time.Time{}, fmt.Errorf("不支持的时长格式: %s", duration)
	}

	if d <= 0 {
		return time.Time{}, fmt.Errorf("duration 必须大于 0: %s", duration)
	}

	return now.Add(-d), nil
}
