package group2file

import (
	"errors"

	pkgErrors "github.com/pkg/errors"
	"github.com/xxcheng123/cloudpan189-share/internal/framework/context"
	"github.com/xxcheng123/cloudpan189-share/internal/repository/models"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

var errInvalidGroupID = errors.New("groupId 必须大于 0")
var errInvalidFileID = errors.New("fileIds 必须全部大于 0")

// BatchBindFiles 批量绑定文件权限到用户组 先删除 再绑定
func (s *service) BatchBindFiles(ctx context.Context, groupId int64, fileIds []int64) error {
	if groupId <= 0 {
		return errInvalidGroupID
	}

	uniqueFileIDs, err := normalizeFileIDs(fileIds)
	if err != nil {
		return err
	}

	if err := s.getDB(ctx).Transaction(func(tx *gorm.DB) error {
		if err := ensureUserGroupExists(tx, groupId); err != nil {
			return err
		}

		if len(uniqueFileIDs) > 0 {
			if err := ensureVirtualFilesExist(tx, uniqueFileIDs); err != nil {
				return err
			}
		}

		// 删除所有旧绑定
		if err := tx.Where("group_id = ?", groupId).Delete(new(models.Group2File)).Error; err != nil {
			return err
		}

		if len(uniqueFileIDs) == 0 {
			return nil
		}

		// 添加新的分组
		items := make([]models.Group2File, 0, len(uniqueFileIDs))
		for _, fileId := range uniqueFileIDs {
			items = append(items, models.Group2File{
				GroupId: groupId,
				FileId:  fileId,
			})
		}

		return tx.Create(&items).Error
	}); err != nil {
		ctx.Error("批量绑定文件权限失败", zap.Error(err), zap.Int64("groupId", groupId), zap.Int64s("fileIds", fileIds))

		return err
	}

	ctx.Info("批量绑定文件权限成功", zap.Int64("groupId", groupId), zap.Int("fileCount", len(uniqueFileIDs)))

	return nil
}

func ensureUserGroupExists(tx *gorm.DB, groupID int64) error {
	var count int64
	if err := tx.Model(new(models.UserGroup)).Where("id = ?", groupID).Count(&count).Error; err != nil {
		return err
	}

	if count != 1 {
		return pkgErrors.Wrap(gorm.ErrRecordNotFound, "用户组不存在")
	}

	return nil
}

func ensureVirtualFilesExist(tx *gorm.DB, fileIDs []int64) error {
	var count int64
	if err := tx.Model(new(models.VirtualFile)).Where("id IN ?", fileIDs).Count(&count).Error; err != nil {
		return err
	}

	if count != int64(len(fileIDs)) {
		return pkgErrors.Wrap(gorm.ErrRecordNotFound, "部分文件不存在")
	}

	return nil
}

func uniqueFileIDs(fileIDs []int64) []int64 {
	seen := make(map[int64]struct{}, len(fileIDs))

	result := make([]int64, 0, len(fileIDs))
	for _, fileID := range fileIDs {
		if _, ok := seen[fileID]; ok {
			continue
		}

		seen[fileID] = struct{}{}
		result = append(result, fileID)
	}

	return result
}

func normalizeFileIDs(fileIDs []int64) ([]int64, error) {
	result := uniqueFileIDs(fileIDs)

	for _, fileID := range result {
		if fileID <= 0 {
			return nil, errInvalidFileID
		}
	}

	return result, nil
}
