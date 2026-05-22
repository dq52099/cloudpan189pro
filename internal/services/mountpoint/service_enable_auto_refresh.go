package mountpoint

import (
	"errors"
	"time"

	"github.com/xxcheng123/cloudpan189-share/internal/framework/context"
	"github.com/xxcheng123/cloudpan189-share/internal/repository/models"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

func (s *service) EnableAutoRefresh(ctx context.Context, fileId int64, enable bool) error {
	if fileId <= 0 {
		return errInvalidMountPointFileID
	}

	updates := map[string]interface{}{
		"enable_auto_refresh": enable,
	}

	if enable {
		// 检查 auto_refresh_begin_at 是否为空，如果为空则设置为当前时间
		var mp models.MountPoint

		err := s.getDB(ctx).Where("file_id = ?", fileId).First(&mp).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}

		if err == nil {
			if mp.AutoRefreshBeginAt == nil || mp.AutoRefreshBeginAt.IsZero() {
				updates["auto_refresh_begin_at"] = time.Now()
			}
		}
	}

	result := s.getDB(ctx).Where("file_id = ?", fileId).Updates(updates)
	if result.Error != nil {
		ctx.Error("更新挂载点自动刷新状态失败", zap.Error(result.Error), zap.Int64("fileId", fileId), zap.Bool("enable", enable))

		return result.Error
	}

	if result.RowsAffected == 0 {
		if err := s.ensureMountPointFileExists(ctx, fileId); err != nil {
			ctx.Error("更新挂载点自动刷新状态失败，记录不存在", zap.Int64("fileId", fileId), zap.Bool("enable", enable))

			return err
		}
	}

	return nil
}
