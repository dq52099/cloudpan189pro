package cloudtoken

import (
	"github.com/pkg/errors"
	"github.com/xxcheng123/cloudpan189-share/internal/framework/context"
	"github.com/xxcheng123/cloudpan189-share/internal/repository/models"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// DeleteRequest 删除云盘令牌请求
type DeleteRequest struct {
	ID      int64 `json:"id" binding:"required" example:"1"` // 云盘令牌ID
	UserID  int64 `json:"-"`                                 // 当前用户ID
	IsAdmin bool  `json:"-"`                                 // 是否管理员
}

func (s *service) Delete(ctx context.Context, req *DeleteRequest) (err error) {
	if req == nil || req.ID <= 0 {
		return errInvalidCloudTokenID
	}

	if !req.IsAdmin {
		if req.UserID <= 0 {
			return errInvalidCloudTokenUserID
		}
	}

	if err := s.svc.GetDB(ctx).Transaction(func(tx *gorm.DB) error {
		query := tx.Model(new(models.CloudToken)).Where("id = ?", req.ID)
		if !req.IsAdmin {
			query = query.Where("user_id = ?", req.UserID)

			if err := ensureCloudTokenDeleteTargetExists(ctx, query, req.ID, req.UserID); err != nil {
				return err
			}

			if err := ensureNoOtherUserCloudTokenReferences(ctx, tx, req.ID, req.UserID); err != nil {
				return err
			}
		}

		result := query.Delete(new(models.CloudToken))
		if result.Error != nil {
			ctx.Error("删除云盘令牌失败", zap.Error(result.Error), zap.Int64("id", req.ID), zap.Int64("user_id", req.UserID))

			return errors.Wrap(result.Error, "删除云盘令牌失败")
		}

		if result.RowsAffected == 0 {
			ctx.Error("删除云盘令牌失败，无权限或不存在", zap.Int64("id", req.ID), zap.Int64("user_id", req.UserID))

			return errors.Wrap(gorm.ErrRecordNotFound, "令牌不存在或无权限删除")
		}

		if err := clearCloudTokenReferences(ctx, tx, req.ID); err != nil {
			return err
		}

		return nil
	}); err != nil {
		return err
	}

	return nil
}

func ensureCloudTokenDeleteTargetExists(ctx context.Context, query *gorm.DB, tokenID, userID int64) error {
	var count int64
	if err := query.Count(&count).Error; err != nil {
		ctx.Error("检查云盘令牌删除目标失败", zap.Error(err), zap.Int64("id", tokenID), zap.Int64("user_id", userID))

		return errors.Wrap(err, "检查云盘令牌删除目标失败")
	}

	if count == 0 {
		ctx.Error("删除云盘令牌失败，无权限或不存在", zap.Int64("id", tokenID), zap.Int64("user_id", userID))

		return errors.Wrap(gorm.ErrRecordNotFound, "令牌不存在或无权限删除")
	}

	return nil
}

func ensureNoOtherUserCloudTokenReferences(ctx context.Context, tx *gorm.DB, tokenID, userID int64) error {
	checks := []struct {
		name  string
		query *gorm.DB
	}{
		{
			name:  "挂载点",
			query: tx.Model(new(models.MountPoint)).Where("token_id = ? AND creator_user_id <> ?", tokenID, userID),
		},
		{
			name:  "用户挂载点令牌绑定",
			query: tx.Model(new(models.UserMountPointToken)).Where("token_id = ? AND user_id <> ?", tokenID, userID),
		},
		{
			name:  "自动转存计划",
			query: tx.Model(new(models.AutoIngestPlan)).Where("token_id = ? AND user_id <> ?", tokenID, userID),
		},
	}

	for _, check := range checks {
		var count int64
		if err := check.query.Count(&count).Error; err != nil {
			ctx.Error("检查云盘令牌外部引用失败", zap.Error(err), zap.Int64("id", tokenID), zap.String("ref", check.name))

			return errors.Wrap(err, "检查云盘令牌引用失败")
		}

		if count > 0 {
			ctx.Warn("拒绝删除被其他用户资源引用的云盘令牌", zap.Int64("id", tokenID), zap.Int64("user_id", userID), zap.String("ref", check.name), zap.Int64("count", count))

			return ErrTokenReferencedByOtherUser
		}
	}

	return nil
}

func clearCloudTokenReferences(ctx context.Context, tx *gorm.DB, tokenID int64) error {
	if err := tx.
		Model(new(models.MountPoint)).
		Where("token_id = ?", tokenID).
		Update("token_id", 0).Error; err != nil {
		ctx.Error("清理挂载点令牌引用失败", zap.Error(err), zap.Int64("id", tokenID))

		return errors.Wrap(err, "清理挂载点令牌引用失败")
	}

	if err := tx.
		Where("token_id = ?", tokenID).
		Delete(new(models.UserMountPointToken)).Error; err != nil {
		ctx.Error("清理用户挂载点令牌绑定失败", zap.Error(err), zap.Int64("id", tokenID))

		return errors.Wrap(err, "清理用户挂载点令牌绑定失败")
	}

	if err := tx.
		Model(new(models.AutoIngestPlan)).
		Where("token_id = ?", tokenID).
		Update("token_id", 0).Error; err != nil {
		ctx.Error("清理自动转存计划令牌引用失败", zap.Error(err), zap.Int64("id", tokenID))

		return errors.Wrap(err, "清理自动转存计划令牌引用失败")
	}

	return nil
}
