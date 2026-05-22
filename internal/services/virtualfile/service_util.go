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
	return s.calFullPathWithDepth(ctx, id, make(map[int64]struct{}), 0)
}

func (s *service) calFullPathWithDepth(ctx context.Context, id int64, visiting map[int64]struct{}, depth int) (string, error) {
	if depth > maxPathDepth {
		return "", fmt.Errorf("%w: %d", errVirtualFilePathTooDeep, maxPathDepth)
	}

	if id == 0 {
		return "/", nil
	}

	if _, exists := visiting[id]; exists {
		return "", fmt.Errorf("%w: file_id=%d", errVirtualFilePathCycle, id)
	}

	visiting[id] = struct{}{}
	defer delete(visiting, id)

	if m, err := s.Query(ctx, id); err != nil {
		return "", err
	} else {
		parent, err := s.calFullPathWithDepth(ctx, m.ParentId, visiting, depth+1)
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
	return s.calFilePathWithCache(ctx, id, make(map[int64]*models.VirtualFile), make(map[int64]struct{}), 0)
}

// calFilePathWithCache 使用缓存优化的路径计算方法
func (s *service) calFilePathWithCache(ctx context.Context, id int64, cache map[int64]*models.VirtualFile, visiting map[int64]struct{}, depth int) (string, error) {
	if depth > maxPathDepth {
		return "", fmt.Errorf("%w: %d", errVirtualFilePathTooDeep, maxPathDepth)
	}

	if id == 0 {
		return "/", nil
	}

	if _, exists := visiting[id]; exists {
		return "", fmt.Errorf("%w: file_id=%d", errVirtualFilePathCycle, id)
	}

	visiting[id] = struct{}{}
	defer delete(visiting, id)

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

	parentPath, err := s.calFilePathWithCache(ctx, file.ParentId, cache, visiting, depth+1)
	if err != nil {
		return "", err
	}

	return path.Join(parentPath, utils.SanitizeFileName(file.Name)), nil
}

func (s *service) BatchQueryParentFiles(ctx context.Context, id int64) ([]*models.VirtualFile, error) {
	var files = make([]*models.VirtualFile, 0)

	var currentId = id

	var ids []int64

	visited := make(map[int64]struct{})

	// 收集所有需要查询的ID
	for depth := 0; currentId != 0; depth++ {
		if depth > maxPathDepth {
			return nil, fmt.Errorf("%w: %d", errVirtualFilePathTooDeep, maxPathDepth)
		}

		if _, exists := visited[currentId]; exists {
			return nil, fmt.Errorf("%w: file_id=%d", errVirtualFilePathCycle, currentId)
		}

		visited[currentId] = struct{}{}
		ids = append(ids, currentId)

		// 查询当前文件的父ID
		var parent struct {
			ParentId int64
		}
		if err := s.getDB(ctx).
			Model(new(models.VirtualFile)).
			Select("parent_id").
			Where("id = ?", currentId).
			Take(&parent).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, errors.Wrapf(gorm.ErrRecordNotFound, "文件不存在 id=%d", currentId)
			}

			ctx.Error("查询父ID失败", zap.Int64("parent_id", currentId), zap.Error(err))

			return nil, errors.Wrapf(err, "查询父ID失败 id=%d", currentId)
		}

		currentId = parent.ParentId
	}

	if len(ids) == 0 {
		return files, nil
	}

	// 批量查询所有文件信息
	if err := s.getDB(ctx).Where("id IN ?", ids).Find(&files).Error; err != nil {
		ctx.Error("批量查询文件信息失败", zap.Int64s("id_list", ids), zap.Error(err))

		return nil, errors.Wrap(err, "批量查询文件信息失败")
	}

	return files, nil
}
