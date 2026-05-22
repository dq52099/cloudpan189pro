package autoingestlog

import (
	"fmt"
	"time"

	appContext "github.com/xxcheng123/cloudpan189-share/internal/framework/context"
	"github.com/xxcheng123/cloudpan189-share/internal/repository/models"
	"github.com/xxcheng123/cloudpan189-share/internal/types/autoingest"
)

// DeleteByPlanIds 根据计划 ID 批量删除日志。
func (s *service) DeleteByPlanIds(ctx appContext.Context, planIds []int64) (int64, error) {
	if len(planIds) == 0 {
		return 0, nil
	}

	normalizedPlanIds, err := normalizeAutoIngestLogIDs(planIds, errInvalidAutoIngestLogPlanID)
	if err != nil {
		return 0, err
	}

	result := s.getDB(ctx).Where("plan_id IN ?", normalizedPlanIds).Delete(&models.AutoIngestLog{})

	return result.RowsAffected, result.Error
}

// DeleteByIds 根据日志 ID 批量删除。
func (s *service) DeleteByIds(ctx appContext.Context, ids []int64) (int64, error) {
	if len(ids) == 0 {
		return 0, nil
	}

	normalizedIds, err := normalizeAutoIngestLogIDs(ids, errInvalidAutoIngestLogID)
	if err != nil {
		return 0, err
	}

	result := s.getDB(ctx).Where("id IN ?", normalizedIds).Delete(&models.AutoIngestLog{})

	return result.RowsAffected, result.Error
}

// DeleteErrorLogsByPlanId 删除指定计划的所有错误日志。
// level 使用 autoingest.LogLevelError 常量，避免硬编码字符串。
func (s *service) DeleteErrorLogsByPlanId(ctx appContext.Context, planId int64) (int64, error) {
	if planId <= 0 {
		return 0, errInvalidAutoIngestLogPlanID
	}

	result := s.getDB(ctx).
		Where("plan_id = ? AND level = ?", planId, autoingest.LogLevelError).
		Delete(&models.AutoIngestLog{})

	return result.RowsAffected, result.Error
}

func normalizeAutoIngestLogIDs(ids []int64, invalidErr error) ([]int64, error) {
	seen := make(map[int64]struct{}, len(ids))

	normalized := make([]int64, 0, len(ids))
	for _, id := range ids {
		if id <= 0 {
			return nil, invalidErr
		}

		if _, ok := seen[id]; ok {
			continue
		}

		seen[id] = struct{}{}
		normalized = append(normalized, id)
	}

	return normalized, nil
}

// DeleteAllErrorLogs 删除所有计划的错误日志。
func (s *service) DeleteAllErrorLogs(ctx appContext.Context) (int64, error) {
	result := s.getDB(ctx).
		Where("level = ?", autoingest.LogLevelError).
		Delete(&models.AutoIngestLog{})

	return result.RowsAffected, result.Error
}

// Clear 清空所有自动入库日志。
func (s *service) Clear(ctx appContext.Context) (int64, error) {
	result := s.getDB(ctx).Unscoped().Where("1 = 1").Delete(&models.AutoIngestLog{})

	return result.RowsAffected, result.Error
}

// ClearByDuration 按保留时长清理自动入库日志。
// 支持常见简写（1h/1d/7d/30d/90d），也支持 time.ParseDuration 能解析的格式。
func (s *service) ClearByDuration(ctx appContext.Context, duration string) (int64, error) {
	cutoff, err := resolveAutoIngestLogCutoff(duration)
	if err != nil {
		return 0, err
	}

	result := s.getDB(ctx).Unscoped().
		Where("created_at < ?", cutoff).
		Delete(&models.AutoIngestLog{})

	return result.RowsAffected, result.Error
}

func resolveAutoIngestLogCutoff(duration string) (time.Time, error) {
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

	d, err := time.ParseDuration(duration)
	if err != nil {
		return time.Time{}, fmt.Errorf("不支持的时长格式: %s", duration)
	}

	if d <= 0 {
		return time.Time{}, fmt.Errorf("duration 必须大于 0: %s", duration)
	}

	return now.Add(-d), nil
}
