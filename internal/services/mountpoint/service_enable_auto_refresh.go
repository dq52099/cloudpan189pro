package mountpoint

import (
	"time"

	"github.com/xxcheng123/cloudpan189-share/internal/framework/context"
	"github.com/xxcheng123/cloudpan189-share/internal/repository/models"
	"go.uber.org/zap"
)

func (s *service) EnableAutoRefresh(ctx context.Context, fileId int64, enable bool) error {
	updates := map[string]interface{}{
		"enable_auto_refresh": enable,
	}

	if enable {
		// 检查 auto_refresh_begin_at 是否为空，如果为空则设置为当前时间
		var mp *models.MountPoint
		if err := s.getDB(ctx).Where("file_id = ?", fileId).First(&mp).Error; err == nil {
			if mp.AutoRefreshBeginAt.IsZero() {
				updates["auto_refresh_begin_at"] = time.Now()
			}
		}
	}

	if err := s.getDB(ctx).Where("file_id = ?", fileId).Updates(updates).Error; err != nil {
		ctx.Error("更新挂载点自动刷新状态失败", zap.Error(err), zap.Int64("fileId", fileId), zap.Bool("enable", enable))

		return err
	}

	return nil
}
