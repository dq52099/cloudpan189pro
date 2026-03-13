package filetasklog

import (
	"time"

	"github.com/xxcheng123/cloudpan189-share/internal/framework/context"
	"github.com/xxcheng123/cloudpan189-share/internal/pkgs/utils"
	"github.com/xxcheng123/cloudpan189-share/internal/repository/models"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

func (s *service) ToggleStatus(ctx context.Context, key LogKey, status string, opts ...utils.Field) (err error) {
	mp := map[string]interface{}{
		"status": status,
	}
	hasCompletedField := false

	for _, opt := range opts {
		mp[opt.Key] = opt.Value
		if opt.Key == "completed" {
			hasCompletedField = true
		}
	}

	if status == models.StatusCompleted && !hasCompletedField {
		mp["completed"] = gorm.Expr("total")
	}

	if err = s.getDB(ctx).
		Where("id = ?", key.GetID()).
		Select("status", "end_at", "duration", "result", "error_msg", "desc", "completed", "total").
		Updates(mp).Error; err != nil {
		ctx.Error("切换文件任务状态失败", zap.Error(err))
	}

	return
}

func (s *service) Pending(ctx context.Context, key LogKey, opts ...utils.Field) error {
	return s.ToggleStatus(ctx, key, models.StatusPending, opts...)
}

func (s *service) Running(ctx context.Context, key LogKey, opts ...utils.Field) error {
	return s.ToggleStatus(ctx, key, models.StatusRunning, opts...)
}

func (s *service) Completed(ctx context.Context, key LogKey, opts ...utils.Field) error {
	opts = append([]utils.Field{{Key: "end_at", Value: time.Now()}}, opts...)

	return s.ToggleStatus(ctx, key, models.StatusCompleted, opts...)
}

func (s *service) Failed(ctx context.Context, key LogKey, opts ...utils.Field) error {
	opts = append([]utils.Field{{Key: "end_at", Value: time.Now()}}, opts...)

	return s.ToggleStatus(ctx, key, models.StatusFailed, opts...)
}

// CompletedWithProgress 完成任务并记录进度信息
func (s *service) CompletedWithProgress(ctx context.Context, key LogKey, processed, total int) error {
	opts := []utils.Field{
		{Key: "end_at", Value: time.Now()},
		{Key: "completed", Value: processed},
		{Key: "total", Value: total},
	}

	return s.ToggleStatus(ctx, key, models.StatusCompleted, opts...)
}

// FailedWithReason 失败任务并记录失败原因
func (s *service) FailedWithReason(ctx context.Context, key LogKey, reason string) error {
	opts := []utils.Field{
		{Key: "end_at", Value: time.Now()},
		{Key: "desc", Value: reason},
	}

	return s.ToggleStatus(ctx, key, models.StatusFailed, opts...)
}
