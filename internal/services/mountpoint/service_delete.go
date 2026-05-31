package mountpoint

import (
	"github.com/pkg/errors"
	"github.com/xxcheng123/cloudpan189-share/internal/framework/context"
	"github.com/xxcheng123/cloudpan189-share/internal/repository/models"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type DeleteRequest struct {
	FileId        int64 `json:"fileId"`
	CreatorUserID int64 // 当前用户ID
	IsAdmin       bool  // 是否管理员
}

func (s *service) Delete(ctx context.Context, req *DeleteRequest) error {
	if req == nil || req.FileId <= 0 {
		return errInvalidMountPointFileID
	}

	if !req.IsAdmin && req.CreatorUserID <= 0 {
		return errInvalidMountPointUserID
	}

	if err := s.getDB(ctx).Transaction(func(tx *gorm.DB) error {
		query := tx.Unscoped().Model(&models.MountPoint{}).Where("file_id = ?", req.FileId)
		if !req.IsAdmin {
			// 非管理员只能删除自己创建的挂载点
			query = query.Where("creator_user_id = ?", req.CreatorUserID)
		}

		var mountPoints []*models.MountPoint
		if err := query.Find(&mountPoints).Error; err != nil {
			return err
		}

		if len(mountPoints) == 0 {
			if !req.IsAdmin {
				return errors.Wrap(gorm.ErrRecordNotFound, "挂载点不存在或无权限删除")
			}

			return errors.Wrap(gorm.ErrRecordNotFound, "挂载点不存在")
		}

		mountPointIDs := mountPointIDList(mountPoints)

		result := tx.Unscoped().Where("id IN ?", mountPointIDs).Delete(&models.MountPoint{})
		if result.Error != nil {
			return result.Error
		}

		if result.RowsAffected != int64(len(mountPointIDs)) {
			return errors.Wrap(gorm.ErrRecordNotFound, "挂载点未删除")
		}

		return deleteUserTokenBindingsByMountPointIDs(tx, mountPointIDs)
	}); err != nil {
		if !req.IsAdmin {
			ctx.Error("删除挂载点失败，无权限或不存在", zap.Error(err), zap.Int64("fileId", req.FileId), zap.Int64("creator_user_id", req.CreatorUserID))
		} else {
			ctx.Error("删除挂载点失败", zap.Error(err), zap.Int64("fileId", req.FileId))
		}

		return err
	}

	return nil
}

type BatchDeleteRequest struct {
	FileIds       []int64 `json:"fileIds"`
	CreatorUserID int64   // 当前用户ID
	IsAdmin       bool    // 是否管理员
}

func (s *service) BatchDelete(ctx context.Context, req *BatchDeleteRequest) error {
	if req == nil || len(req.FileIds) == 0 {
		return nil
	}

	fileIDs, err := normalizeFileIDs(req.FileIds)
	if err != nil {
		return err
	}

	if !req.IsAdmin && req.CreatorUserID <= 0 {
		return errInvalidMountPointUserID
	}

	if err := s.getDB(ctx).Transaction(func(tx *gorm.DB) error {
		query := tx.Unscoped().Model(&models.MountPoint{}).Where("file_id IN ?", fileIDs)
		if !req.IsAdmin {
			// 非管理员只能删除自己创建的挂载点
			query = query.Where("creator_user_id = ?", req.CreatorUserID)
		}

		var mountPoints []*models.MountPoint
		if err := query.Find(&mountPoints).Error; err != nil {
			return err
		}

		if !allFileIDsMatched(fileIDs, mountPoints) {
			return errors.Wrap(gorm.ErrRecordNotFound, "部分挂载点不存在或无权限删除")
		}

		mountPointIDs := mountPointIDList(mountPoints)

		result := tx.Unscoped().Where("id IN ?", mountPointIDs).Delete(&models.MountPoint{})
		if result.Error != nil {
			return result.Error
		}

		if result.RowsAffected != int64(len(mountPointIDs)) {
			return errors.Wrap(gorm.ErrRecordNotFound, "部分挂载点未删除")
		}

		return deleteUserTokenBindingsByMountPointIDs(tx, mountPointIDs)
	}); err != nil {
		ctx.Error("批量删除挂载点失败", zap.Error(err), zap.Int64s("fileIds", fileIDs))

		return err
	}

	ctx.Info("批量删除挂载点执行完成",
		zap.Int64s("fileIds", fileIDs),
		zap.Int("deleted_rows", len(fileIDs)))

	return nil
}

func normalizeFileIDs(fileIDs []int64) ([]int64, error) {
	seen := make(map[int64]struct{}, len(fileIDs))

	result := make([]int64, 0, len(fileIDs))
	for _, fileID := range fileIDs {
		if fileID <= 0 {
			return nil, errInvalidMountPointFileID
		}

		if _, ok := seen[fileID]; ok {
			continue
		}

		seen[fileID] = struct{}{}
		result = append(result, fileID)
	}

	return result, nil
}

func allFileIDsMatched(fileIDs []int64, mountPoints []*models.MountPoint) bool {
	matched := make(map[int64]struct{}, len(mountPoints))
	for _, mountPoint := range mountPoints {
		matched[mountPoint.FileId] = struct{}{}
	}

	for _, fileID := range fileIDs {
		if _, ok := matched[fileID]; !ok {
			return false
		}
	}

	return true
}

func mountPointIDList(mountPoints []*models.MountPoint) []int64 {
	ids := make([]int64, 0, len(mountPoints))
	for _, mountPoint := range mountPoints {
		ids = append(ids, mountPoint.ID)
	}

	return ids
}

func deleteUserTokenBindingsByMountPointIDs(tx *gorm.DB, mountPointIDs []int64) error {
	if len(mountPointIDs) == 0 {
		return nil
	}

	return tx.
		Model(new(models.UserMountPointToken)).
		Where("mount_point_id IN ?", mountPointIDs).
		Delete(new(models.UserMountPointToken)).Error
}

func (s *service) ClearAll(ctx context.Context) (int64, error) {
	var (
		deletedMountPoints   int64
		deletedTokenBindings int64
	)

	if err := s.svc.GetDB(ctx).Transaction(func(tx *gorm.DB) error {
		bindingResult := tx.
			Where("1 = 1").
			Delete(new(models.UserMountPointToken))
		if bindingResult.Error != nil {
			return bindingResult.Error
		}

		deletedTokenBindings = bindingResult.RowsAffected

		mountPointResult := tx.
			Unscoped().
			Where("1 = 1").
			Delete(new(models.MountPoint))
		if mountPointResult.Error != nil {
			return mountPointResult.Error
		}

		deletedMountPoints = mountPointResult.RowsAffected

		return nil
	}); err != nil {
		ctx.Error("清空所有挂载点失败", zap.Error(err))

		return 0, err
	}

	ctx.Info("清空所有挂载点完成",
		zap.Int64("deleted_rows", deletedMountPoints),
		zap.Int64("deleted_token_bindings", deletedTokenBindings),
	)

	return deletedMountPoints, nil
}
