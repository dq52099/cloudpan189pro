package virtualfile

import (
	"fmt"
	"path"

	"github.com/pkg/errors"
	"github.com/xxcheng123/cloudpan189-share/internal/framework/context"
	"github.com/xxcheng123/cloudpan189-share/internal/pkgs/utils"
	"github.com/xxcheng123/cloudpan189-share/internal/repository/models"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

const maxPathDepth = 100

func (s *service) GetMaxId(ctx context.Context) (maxId int64, err error) {
	if err = s.getDB(ctx).Model(new(models.VirtualFile)).
		Select("COALESCE(MAX(id), 0)").
		Scan(&maxId).Error; err != nil {
		return 0, err
	}

	return maxId, nil
}

// CalFullPath 获取完整路径
func (s *service) CalFullPath(ctx context.Context, id int64) (string, error) {
	return s.calFullPathWithDepth(ctx, id, 0)
}

func (s *service) calFullPathWithDepth(ctx context.Context, id int64, depth int) (string, error) {
	if depth > maxPathDepth {
		return "", fmt.Errorf("路径深度超过限制 %d，可能存在循环引用", maxPathDepth)
	}

	if id == 0 {
		return "/", nil
	}

	if m, err := s.Query(ctx, id); err != nil {
		return "", err
	} else {
		parent, err := s.calFullPathWithDepth(ctx, m.ParentId, depth+1)
		if err != nil {
			return "", err
		}

		return path.Join(parent, utils.SanitizeFileName(m.Name)), nil
	}
}

func (s *service) CalFilePath(ctx context.Context, id int64) (string, error) {
	return s.calFilePath(ctx, id)
}

// calFilePath 计算文件的路径
func (s *service) calFilePath(ctx context.Context, id int64) (string, error) {
	return s.calFilePathWithCache(ctx, id, make(map[int64]*models.VirtualFile))
}

// calFilePathWithCache 使用缓存优化的路径计算方法
func (s *service) calFilePathWithCache(ctx context.Context, id int64, cache map[int64]*models.VirtualFile) (string, error) {
	if id == 0 {
		return "/", nil
	}

	// 检查缓存
	file, exists := cache[id]
	if !exists {
		// 批量查询当前文件及其所有父级文件
		files, err := s.BatchQueryParentFiles(ctx, id)
		if err != nil {
			return "", err
		}

		// 将查询结果加入缓存
		for _, f := range files {
			cache[f.ID] = f
		}

		file, exists = cache[id]
		if !exists {
			return "", gorm.ErrRecordNotFound
		}
	}

	parentPath, err := s.calFilePathWithCache(ctx, file.ParentId, cache)
	if err != nil {
		return "", err
	}

	return path.Join(parentPath, utils.SanitizeFileName(file.Name)), nil
}

func (s *service) BatchQueryParentFiles(ctx context.Context, id int64) ([]*models.VirtualFile, error) {
	var files = make([]*models.VirtualFile, 0)

	var currentId = id

	var ids []int64

	// 收集所有需要查询的ID
	for currentId != 0 {
		ids = append(ids, currentId)

		// 查询当前文件的父ID
		var parentId int64
		if err := s.getDB(ctx).Select("parent_id").Where("id = ?", currentId).Scan(&parentId).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				break
			}

			ctx.Error("查询父ID失败", zap.Int64("parent_id", currentId), zap.Error(err))

			return nil, errors.Wrapf(err, "查询父ID失败 id=%d", currentId)
		}

		currentId = parentId
	}

	// 批量查询所有文件信息
	if err := s.getDB(ctx).Where("id IN ?", ids).Find(&files).Error; err != nil {
		ctx.Error("批量查询文件信息失败", zap.Int64s("id_list", ids), zap.Error(err))

		return nil, errors.Wrap(err, "批量查询文件信息失败")
	}

	return files, nil
}
