package filetasklog

import (
	"errors"
	"time"

	"github.com/xxcheng123/cloudpan189-share/internal/framework/context"
	"github.com/xxcheng123/cloudpan189-share/internal/pkgs/utils"
	"github.com/xxcheng123/cloudpan189-share/internal/repository/models"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

func (s *service) ToggleStatus(ctx context.Context, key LogKey, status string, opts ...utils.Field) (err error) {
	id, err := validateLogKey(key)
	if err != nil {
		return err
	}

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

	result := s.getDB(ctx).
		Where("id = ?", id).
		Select("status", "end_at", "duration", "result", "error_msg", "desc", "completed", "total").
		Updates(mp)
	if err = checkTaskLogUpdateResult(ctx, result, id, "文件任务日志不存在"); err != nil {
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

// CompleteIfProgressDone 在计数已满时终结运行中的任务。
func (s *service) CompleteIfProgressDone(ctx context.Context, key LogKey, opts ...utils.Field) error {
	id, err := validateLogKey(key)
	if err != nil {
		return err
	}

	mp := map[string]interface{}{
		"status": models.StatusCompleted,
		"end_at": time.Now(),
	}
	for _, opt := range opts {
		mp[opt.Key] = opt.Value
	}

	completedResult := s.getDB(ctx).
		Where("id = ?", id).
		Where("status = ?", models.StatusRunning).
		Where("total > 0").
		Where("failed = 0").
		Where("completed >= total").
		Select("status", "end_at", "duration", "result", "error_msg", "desc", "completed", "total").
		Updates(mp)
	if completedResult.Error != nil {
		ctx.Error("自动完成文件任务日志失败", zap.Error(completedResult.Error), zap.Int64("task_id", id))

		return completedResult.Error
	}

	if completedResult.RowsAffected > 0 {
		return nil
	}

	if err := s.ensureTaskLogExists(ctx, id); err != nil {
		ctx.Error("自动完成文件任务日志失败", zap.Error(err), zap.Int64("task_id", id))

		return err
	}

	mp["status"] = models.StatusFailed
	mp["result"] = "批量任务部分失败"

	failedResult := s.getDB(ctx).
		Where("id = ?", id).
		Where("status = ?", models.StatusRunning).
		Where("total > 0").
		Where("failed > 0").
		Where("completed + failed >= total").
		Select("status", "end_at", "duration", "result", "error_msg", "desc", "completed", "total").
		Updates(mp)
	if failedResult.Error != nil {
		ctx.Error("自动失败文件任务日志失败", zap.Error(failedResult.Error), zap.Int64("task_id", id))

		return failedResult.Error
	}

	if err := s.ensureTaskLogExists(ctx, id); err != nil {
		ctx.Error("自动失败文件任务日志失败", zap.Error(err), zap.Int64("task_id", id))

		return err
	}

	return nil
}

func (s *service) ensureTaskLogExists(ctx context.Context, id int64) error {
	var log models.FileTaskLog

	err := s.getDB(ctx).
		Session(&gorm.Session{NewDB: true}).
		Select("id").
		Where("id = ?", id).
		Take(&log).
		Error
	if err == nil {
		return nil
	}

	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}

	return errors.Join(gorm.ErrRecordNotFound, errors.New("文件任务日志不存在"))
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
