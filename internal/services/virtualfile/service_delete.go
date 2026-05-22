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

	result := s.withLock(ctx, func(db *gorm.DB) *gorm.DB {
		return db.Where("id = ?", id).Delete(new(models.VirtualFile))
	})

	for _, hook := range hooks {
		hook(ctx, result, id)
	}

	if result.Error != nil {
		return result.Error
	}

	if result.RowsAffected == 0 {
		return errors.Wrap(gorm.ErrRecordNotFound, "文件不存在")
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

	if err := validateVirtualFileIDs(ids); err != nil {
		return nil, err
	}

	// 1. 先查询出完整的文件信息（修复：删除后无法查询文件信息导致无法删除strm的问题）
	var filesToDelete []*models.VirtualFile
	if err := s.getDB(ctx).Where("id IN ?", ids).Find(&filesToDelete).Error; err != nil {
		ctx.Error("批量删除文件 - 数据库查询失败", zap.Int64s("file_ids", ids), zap.Error(err))

		return nil, err
	}

	// 如果没有匹配的记录，直接返回
	if len(filesToDelete) == 0 {
		ctx.Debug("批量删除文件 - 没有找到匹配的记录", zap.Int64s("file_ids", ids))

		return []int64{}, nil
	}

	matchIdList := make([]int64, 0, len(filesToDelete))
	for _, f := range filesToDelete {
		matchIdList = append(matchIdList, f.ID)
	}

	// 2. 执行删除操作
	result := s.withLock(ctx, func(db *gorm.DB) *gorm.DB {
		return db.Where("id IN ?", matchIdList).Delete(new(models.VirtualFile))
	})

	if result.Error != nil {
		ctx.Error("批量删除文件 - 数据库删除失败", zap.Int64s("file_ids", matchIdList), zap.Error(result.Error))

		return nil, result.Error
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

func validateVirtualFileIDs(ids []int64) error {
	for _, id := range ids {
		if id <= 0 {
			return errInvalidVirtualFileID
		}
	}

	return nil
}
