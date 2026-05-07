package loginlog

import (
	"time"

	"github.com/xxcheng123/cloudpan189-share/internal/framework/context"
	"github.com/xxcheng123/cloudpan189-share/internal/repository/models"
	"go.uber.org/zap"
)

// ClearAll 清空全部登录日志。
func (s *service) ClearAll(ctx context.Context) (int64, error) {
	result := s.getDB(ctx).Unscoped().Where("1 = 1").Delete(&models.LoginLog{})
	if result.Error != nil {
		ctx.Error("清空登录日志失败", zap.Error(result.Error))
	}

	return result.RowsAffected, result.Error
}

// ClearBefore 清理指定时间之前的登录日志。
func (s *service) ClearBefore(ctx context.Context, before time.Time) (int64, error) {
	if before.IsZero() {
		return 0, nil
	}

	result := s.getDB(ctx).Unscoped().Where("created_at < ?", before).Delete(&models.LoginLog{})
	if result.Error != nil {
		ctx.Error("按时间清理登录日志失败", zap.Time("before", before), zap.Error(result.Error))
	}

	return result.RowsAffected, result.Error
}
