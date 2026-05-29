package autoingestplan

import (
	"github.com/xxcheng123/cloudpan189-share/internal/framework/context"
	"github.com/xxcheng123/cloudpan189-share/internal/pkgs/utils"
	"github.com/xxcheng123/cloudpan189-share/internal/repository/models"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type UpdateRequest struct {
	ID      int64 // 自动挂载计划ID
	UserID  int64 // 当前用户ID
	IsAdmin bool  // 是否管理员
}

// Update 更新自动挂载计划字段
func (s *service) Update(ctx context.Context, id int64, fields ...utils.Field) error {
	if id <= 0 {
		return errInvalidAutoIngestPlanID
	}

	updateQuery := s.getDB(ctx).Where("id = ?", id)
	existsQuery := s.svc.GetDB(ctx).Model(&models.AutoIngestPlan{}).Where("id = ?", id)

	return s.updateWithQuery(ctx, updateQuery, existsQuery, id, fields...)
}

// UpdateByOwner 按当前用户权限更新自动挂载计划字段。
func (s *service) UpdateByOwner(ctx context.Context, req *UpdateRequest, fields ...utils.Field) error {
	if req == nil || req.ID <= 0 {
		return errInvalidAutoIngestPlanID
	}

	updateQuery := s.getDB(ctx).Where("id = ?", req.ID)

	existsQuery := s.svc.GetDB(ctx).Model(&models.AutoIngestPlan{}).Where("id = ?", req.ID)
	if !req.IsAdmin {
		if req.UserID <= 0 {
			return errInvalidAutoIngestPlanUserID
		}

		updateQuery = updateQuery.Where("user_id = ?", req.UserID)
		existsQuery = existsQuery.Where("user_id = ?", req.UserID)
	}

	return s.updateWithQuery(ctx, updateQuery, existsQuery, req.ID, fields...)
}

func (s *service) updateWithQuery(ctx context.Context, updateQuery *gorm.DB, existsQuery *gorm.DB, id int64, fields ...utils.Field) error {
	mp := make(map[string]interface{})
	for _, field := range fields {
		mp[field.Key] = field.Value
	}

	if len(mp) == 0 {
		return errEmptyAutoIngestPlanUpdateFields
	}

	result := updateQuery.Updates(mp)
	if result.Error != nil {
		ctx.Error("更新自动挂载计划失败", zap.Error(result.Error), zap.Int64("id", id))

		return result.Error
	}

	if result.RowsAffected == 0 {
		if err := s.ensureAutoIngestPlanUpdateTargetExists(existsQuery); err != nil {
			ctx.Error("更新自动挂载计划失败，记录不存在", zap.Int64("id", id))

			return err
		}
	}

	return nil
}

func (s *service) ensureAutoIngestPlanUpdateTargetExists(query *gorm.DB) error {
	var count int64
	if err := query.Count(&count).Error; err != nil {
		return err
	}

	if count == 0 {
		return gorm.ErrRecordNotFound
	}

	return nil
}
