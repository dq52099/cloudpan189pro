package user

import (
	"errors"
	"fmt"

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

		tokenIDs = uniquePositiveIDs(tokenIDs)

		mountPointIDs, err := pluckIDs(tx.Model(new(models.MountPoint)).Where("creator_user_id = ?", req.ID))
		if err != nil {
			return err
		}

		mountPointIDs = uniquePositiveIDs(mountPointIDs)

		mountPointFileIDs, err := pluckInt64Column(tx.Model(new(models.MountPoint)).Where("creator_user_id = ?", req.ID), "file_id")
		if err != nil {
			return err
		}

		mountPointFileIDs = uniquePositiveIDs(mountPointFileIDs)

		planIDs, err := pluckIDs(tx.Model(new(models.AutoIngestPlan)).Where("user_id = ?", req.ID))
		if err != nil {
			return err
		}

		planIDs = uniquePositiveIDs(planIDs)

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

func uniquePositiveIDs(ids []int64) []int64 {
	if len(ids) == 0 {
		return nil
	}

	uniqueIDs := make([]int64, 0, len(ids))
	seen := make(map[int64]struct{}, len(ids))

	for _, id := range ids {
		if id <= 0 {
			continue
		}

		if _, ok := seen[id]; ok {
			continue
		}

		seen[id] = struct{}{}
		uniqueIDs = append(uniqueIDs, id)
	}

	return uniqueIDs
}

func clearDeletedUserRelations(tx *gorm.DB, userID int64, tokenIDs, mountPointIDs, mountPointFileIDs, planIDs []int64) error {
	if err := deleteUserMountPointTokenRelations(tx, userID, tokenIDs, mountPointIDs); err != nil {
		return err
	}

	if err := resetDeletedTokenMountPointReferences(tx, tokenIDs); err != nil {
		return err
	}

	if len(planIDs) > 0 {
		if err := deleteMatchedRows(tx, new(models.AutoIngestLog), "delete auto ingest logs", "plan_id IN ?", planIDs); err != nil {
			return err
		}
	}

	if err := deleteExpectedIDs(tx, new(models.AutoIngestPlan), planIDs, "delete auto ingest plans"); err != nil {
		return err
	}

	if err := deleteExpectedIDs(tx, new(models.MountPoint), mountPointIDs, "delete mount points"); err != nil {
		return err
	}

	if err := deleteOwnedMountPointVirtualFiles(tx, mountPointFileIDs); err != nil {
		return err
	}

	if err := deleteExpectedIDs(tx, new(models.CloudToken), tokenIDs, "delete cloud tokens"); err != nil {
		return err
	}

	return nil
}

func ensureRowsAffected(result *gorm.DB, expected int64, operation string) error {
	if result.Error != nil {
		return result.Error
	}

	if result.RowsAffected != expected {
		return fmt.Errorf("%s affected %d rows, expected %d: %w", operation, result.RowsAffected, expected, gorm.ErrRecordNotFound)
	}

	return nil
}

func deleteExpectedIDs(tx *gorm.DB, model any, ids []int64, operation string) error {
	if len(ids) == 0 {
		return nil
	}

	result := tx.Where("id IN ?", ids).Delete(model)

	return ensureRowsAffected(result, int64(len(ids)), operation)
}

func deleteMatchedRows(tx *gorm.DB, model any, operation string, query string, args ...any) error {
	var count int64
	if err := tx.Model(model).Where(query, args...).Count(&count).Error; err != nil {
		return err
	}

	if count == 0 {
		return nil
	}

	result := tx.Where(query, args...).Delete(model)

	return ensureRowsAffected(result, count, operation)
}

func resetDeletedTokenMountPointReferences(tx *gorm.DB, tokenIDs []int64) error {
	if len(tokenIDs) == 0 {
		return nil
	}

	var count int64
	if err := tx.Model(new(models.MountPoint)).Where("token_id IN ?", tokenIDs).Count(&count).Error; err != nil {
		return err
	}

	if count == 0 {
		return nil
	}

	result := tx.Model(new(models.MountPoint)).
		Where("token_id IN ?", tokenIDs).
		Update("token_id", 0)

	return ensureRowsAffected(result, count, "reset deleted token mount point references")
}

func deleteUserMountPointTokenRelations(tx *gorm.DB, userID int64, tokenIDs, mountPointIDs []int64) error {
	query := userMountPointTokenRelationsQuery(tx, userID, tokenIDs, mountPointIDs)

	var count int64
	if err := query.Count(&count).Error; err != nil {
		return err
	}

	if count == 0 {
		return nil
	}

	return ensureRowsAffected(
		userMountPointTokenRelationsQuery(tx, userID, tokenIDs, mountPointIDs).Delete(new(models.UserMountPointToken)),
		count,
		"delete user mount point token relations",
	)
}

func userMountPointTokenRelationsQuery(tx *gorm.DB, userID int64, tokenIDs, mountPointIDs []int64) *gorm.DB {
	query := tx.Model(new(models.UserMountPointToken)).Where("user_id = ?", userID)
	if len(tokenIDs) > 0 {
		query = query.Or("token_id IN ?", tokenIDs)
	}

	if len(mountPointIDs) > 0 {
		query = query.Or("mount_point_id IN ?", mountPointIDs)
	}

	return query
}

func deleteOwnedMountPointVirtualFiles(tx *gorm.DB, fileIDs []int64) error {
	if len(fileIDs) == 0 {
		return nil
	}

	virtualFileIDs, err := pluckIDs(tx.Model(new(models.VirtualFile)).Where("id IN ? OR top_id IN ?", fileIDs, fileIDs))
	if err != nil {
		return err
	}

	virtualFileIDs = uniquePositiveIDs(virtualFileIDs)
	if len(virtualFileIDs) == 0 {
		return nil
	}

	return deleteExpectedIDs(tx, new(models.VirtualFile), virtualFileIDs, "delete mount point virtual files")
}
