package user

import (
	"errors"

	"github.com/xxcheng123/cloudpan189-share/internal/framework/context"
	"github.com/xxcheng123/cloudpan189-share/internal/repository/models"

	"go.uber.org/zap"
	"gorm.io/gorm"
)

type DelRequest struct {
	ID int64 `json:"id" binding:"required,min=1" example:"1001"` // 用户ID，必须大于1
}

func (s *service) Del(ctx context.Context, req *DelRequest) error {
	if req == nil || req.ID <= 0 {
		return errInvalidUserID
	}

	if req.ID == 1 {
		return errors.New("创始人不能删除")
	}

	if err := s.svc.GetDB(ctx).Transaction(func(tx *gorm.DB) error {
		tokenIDs, err := pluckIDs(tx.Model(new(models.CloudToken)).Where("user_id = ?", req.ID))
		if err != nil {
			return err
		}

		mountPointIDs, err := pluckIDs(tx.Model(new(models.MountPoint)).Where("creator_user_id = ?", req.ID))
		if err != nil {
			return err
		}

		mountPointFileIDs, err := pluckInt64Column(tx.Model(new(models.MountPoint)).Where("creator_user_id = ?", req.ID), "file_id")
		if err != nil {
			return err
		}

		planIDs, err := pluckIDs(tx.Model(new(models.AutoIngestPlan)).Where("user_id = ?", req.ID))
		if err != nil {
			return err
		}

		result := tx.Model(new(models.User)).Where("id = ?", req.ID).Delete(&models.User{})
		if result.Error != nil {
			return result.Error
		}

		if result.RowsAffected == 0 {
			return gorm.ErrRecordNotFound
		}

		if err := clearDeletedUserRelations(tx, req.ID, tokenIDs, mountPointIDs, mountPointFileIDs, planIDs); err != nil {
			return err
		}

		return nil
	}); err != nil {
		ctx.Error("用户删除失败", zap.Error(err), zap.Int64("user_id", req.ID))

		return err
	}

	return nil
}

func pluckIDs(query *gorm.DB) ([]int64, error) {
	return pluckInt64Column(query, "id")
}

func pluckInt64Column(query *gorm.DB, column string) ([]int64, error) {
	ids := make([]int64, 0)
	if err := query.Pluck(column, &ids).Error; err != nil {
		return nil, err
	}

	return ids, nil
}

func clearDeletedUserRelations(tx *gorm.DB, userID int64, tokenIDs, mountPointIDs, mountPointFileIDs, planIDs []int64) error {
	if err := deleteUserMountPointTokenRelations(tx, userID, tokenIDs, mountPointIDs); err != nil {
		return err
	}

	if len(tokenIDs) > 0 {
		if err := tx.Model(new(models.MountPoint)).
			Where("token_id IN ?", tokenIDs).
			Update("token_id", 0).Error; err != nil {
			return err
		}
	}

	if len(planIDs) > 0 {
		if err := tx.Model(new(models.AutoIngestLog)).
			Where("plan_id IN ?", planIDs).
			Delete(new(models.AutoIngestLog)).Error; err != nil {
			return err
		}
	}

	if err := tx.Model(new(models.AutoIngestPlan)).
		Where("user_id = ?", userID).
		Delete(new(models.AutoIngestPlan)).Error; err != nil {
		return err
	}

	if err := tx.Model(new(models.MountPoint)).
		Where("creator_user_id = ?", userID).
		Delete(new(models.MountPoint)).Error; err != nil {
		return err
	}

	if err := deleteOwnedMountPointVirtualFiles(tx, mountPointFileIDs); err != nil {
		return err
	}

	if err := tx.Model(new(models.CloudToken)).
		Where("user_id = ?", userID).
		Delete(new(models.CloudToken)).Error; err != nil {
		return err
	}

	return nil
}

func deleteUserMountPointTokenRelations(tx *gorm.DB, userID int64, tokenIDs, mountPointIDs []int64) error {
	query := tx.Model(new(models.UserMountPointToken)).Where("user_id = ?", userID)
	if len(tokenIDs) > 0 {
		query = query.Or("token_id IN ?", tokenIDs)
	}

	if len(mountPointIDs) > 0 {
		query = query.Or("mount_point_id IN ?", mountPointIDs)
	}

	return query.Delete(new(models.UserMountPointToken)).Error
}

func deleteOwnedMountPointVirtualFiles(tx *gorm.DB, fileIDs []int64) error {
	if len(fileIDs) == 0 {
		return nil
	}

	return tx.
		Where("id IN ? OR top_id IN ?", fileIDs, fileIDs).
		Delete(new(models.VirtualFile)).Error
}
