package autoingestlog

import (
	appContext "github.com/xxcheng123/cloudpan189-share/internal/framework/context"
	"github.com/xxcheng123/cloudpan189-share/internal/repository/models"
	"github.com/xxcheng123/cloudpan189-share/internal/types/autoingest"
)

// DeleteByPlanIds 根据计划 ID 批量删除日志。
func (s *service) DeleteByPlanIds(ctx appContext.Context, planIds []int64) (int64, error) {
	if len(planIds) == 0 {
		return 0, nil
	}

	result := s.getDB(ctx).Where("plan_id IN ?", planIds).Delete(&models.AutoIngestLog{})

	return result.RowsAffected, result.Error
}

// DeleteByIds 根据日志 ID 批量删除。
func (s *service) DeleteByIds(ctx appContext.Context, ids []int64) (int64, error) {
	if len(ids) == 0 {
		return 0, nil
	}

	result := s.getDB(ctx).Where("id IN ?", ids).Delete(&models.AutoIngestLog{})

	return result.RowsAffected, result.Error
}

// DeleteErrorLogsByPlanId 删除指定计划的所有错误日志。
// level 使用 autoingest.LogLevelError 常量，避免硬编码字符串。
func (s *service) DeleteErrorLogsByPlanId(ctx appContext.Context, planId int64) (int64, error) {
	result := s.getDB(ctx).
		Where("plan_id = ? AND level = ?", planId, autoingest.LogLevelError).
		Delete(&models.AutoIngestLog{})

	return result.RowsAffected, result.Error
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
