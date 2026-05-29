package virtualfile

import (
	"github.com/pkg/errors"
	"github.com/xxcheng123/cloudpan189-share/internal/framework/context"
	"github.com/xxcheng123/cloudpan189-share/internal/repository/models"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type DeleteHook func(ctx context.Context, result *gorm.DB, id int64)

func (s *service) Delete(ctx context.Context, id int64, hooks ...DeleteHook) error {
	ctx.Debug("删除文件", zap.Int64("file_id", id))

	if id <= 0 {
		return errInvalidVirtualFileID
	}

	var result *gorm.DB

	err := s.withWriteLock(ctx, func(db *gorm.DB) error {
		return db.Transaction(func(tx *gorm.DB) error {
			result = tx.Where("id = ?", id).Delete(new(models.VirtualFile))
			if result.Error != nil {
				return result.Error
			}

			if result.RowsAffected == 0 {
				return nil
			}

			if err := deleteGroup2FileBindingsByFileIDs(tx, []int64{id}); err != nil {
				return err
			}

			return nil
		})
	})
	if err != nil {
		return err
	}

	if result == nil {
		return errors.Wrap(gorm.ErrRecordNotFound, "文件不存在")
	}

	if result.RowsAffected == 0 {
		return errors.Wrap(gorm.ErrRecordNotFound, "文件不存在")
	}

	for _, hook := range hooks {
		hook(ctx, result, id)
	}

	return nil
}

// BatchDeleteHook 修改参数，传递文件对象而不是ID，以便后续能获取文件名进行物理删除
type BatchDeleteHook func(ctx context.Context, result *gorm.DB, files []*models.VirtualFile)

func (s *service) BatchDelete(ctx context.Context, ids []int64, hooks ...BatchDeleteHook) (deletedIdList []int64, err error) {
	ctx.Debug("批量删除文件", zap.Int64s("file_ids", ids))

	// 如果输入为空，直接返回
	if len(ids) == 0 {
		return []int64{}, nil
	}

	normalizedIDs, err := normalizeVirtualFileIDs(ids)
	if err != nil {
		return nil, err
	}

	// 1. 先查询出完整的文件信息（修复：删除后无法查询文件信息导致无法删除strm的问题）
	var filesToDelete []*models.VirtualFile
	if err := s.getDB(ctx).Where("id IN ?", normalizedIDs).Find(&filesToDelete).Error; err != nil {
		ctx.Error("批量删除文件 - 数据库查询失败", zap.Int64s("file_ids", normalizedIDs), zap.Error(err))

		return nil, err
	}

	if len(filesToDelete) != len(normalizedIDs) {
		ctx.Debug("批量删除文件 - 部分文件不存在",
			zap.Int64s("file_ids", normalizedIDs),
			zap.Int("matched_rows", len(filesToDelete)),
			zap.Int("requested_rows", len(normalizedIDs)),
		)

		return nil, errors.Wrap(gorm.ErrRecordNotFound, "部分文件不存在")
	}

	matchIdList := make([]int64, 0, len(filesToDelete))
	for _, f := range filesToDelete {
		matchIdList = append(matchIdList, f.ID)
	}

	// 2. 执行删除操作
	var result *gorm.DB

	err = s.withWriteLock(ctx, func(db *gorm.DB) error {
		return db.Transaction(func(tx *gorm.DB) error {
			result = tx.Where("id IN ?", matchIdList).Delete(new(models.VirtualFile))
			if result.Error != nil {
				return result.Error
			}

			if result.RowsAffected == 0 {
				return nil
			}

			if err := deleteGroup2FileBindingsByFileIDs(tx, matchIdList); err != nil {
				return err
			}

			return nil
		})
	})
	if err != nil {
		ctx.Error("批量删除文件 - 数据库删除失败", zap.Int64s("file_ids", matchIdList), zap.Error(err))

		return nil, err
	}

	if result == nil {
		ctx.Error("批量删除文件 - 删除结果为空", zap.Int64s("file_ids", matchIdList))

		return nil, errors.Wrap(gorm.ErrRecordNotFound, "部分文件未删除")
	}

	if result.RowsAffected != int64(len(matchIdList)) {
		ctx.Error("批量删除文件 - 删除行数不一致",
			zap.Int64s("file_ids", matchIdList),
			zap.Int64("affected_rows", result.RowsAffected),
			zap.Int("matched_rows", len(matchIdList)),
		)

		return nil, errors.Wrap(gorm.ErrRecordNotFound, "部分文件未删除")
	}

	// 3. 执行Hook，传递文件对象列表
	for _, hook := range hooks {
		hook(ctx, result, filesToDelete)
	}

	ctx.Debug("批量删除文件完成", zap.Int64s("deleted_ids", matchIdList), zap.Int64("affected_rows", result.RowsAffected))

	return matchIdList, nil
}

func deleteGroup2FileBindingsByFileIDs(db *gorm.DB, fileIDs []int64) error {
	if len(fileIDs) == 0 {
		return nil
	}

	return db.Session(&gorm.Session{NewDB: true}).
		Where("file_id IN ?", fileIDs).
		Delete(new(models.Group2File)).Error
}
