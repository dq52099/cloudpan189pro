package filetasklog

import (
	"github.com/xxcheng123/cloudpan189-share/internal/framework/context"
	"github.com/xxcheng123/cloudpan189-share/internal/repository/models"
	"go.uber.org/zap"
)

func (s *service) Clear(ctx context.Context) error {
	if err := s.getDB(ctx).Exec("DELETE FROM file_task_logs").Error; err != nil {
		ctx.Error("清空任务日志失败", zap.Error(err))
		return err
	}
	ctx.Info("清空任务日志成功")
	return nil
}

func (s *service) ClearByDuration(ctx context.Context, duration string) error {
	db := s.getDB(ctx)
	switch duration {
	case "1h":
		db = db.Where("created_at < datetime('now', '-1 hour')")
	case "1d":
		db = db.Where("created_at < datetime('now', '-1 day')")
	case "7d":
		db = db.Where("created_at < datetime('now', '-7 days')")
	case "30d":
		db = db.Where("created_at < datetime('now', '-30 days')")
	}
	if err := db.Unscoped().Delete(&models.FileTaskLog{}).Error; err != nil {
		ctx.Error("清空任务日志失败", zap.Error(err))
		return err
	}
	ctx.Info("清空任务日志成功", zap.String("duration", duration))
	return nil
}
