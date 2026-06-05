package virtualfile

import (
	"time"

	"github.com/xxcheng123/cloudpan189-share/internal/consts"
	"go.uber.org/zap"

	"github.com/pkg/errors"
	"github.com/xxcheng123/cloudpan189-share/internal/framework/context"
	"github.com/xxcheng123/cloudpan189-share/internal/pkgs/datatypes"
	"github.com/xxcheng123/cloudpan189-share/internal/pkgs/utils"
	"github.com/xxcheng123/cloudpan189-share/internal/repository/models"
	"gorm.io/gorm"
)

func (s *service) Query(ctx context.Context, fid int64) (*models.VirtualFile, error) {
	if fid <= 0 {
		return nil, errInvalidVirtualFileID
	}

	file := new(models.VirtualFile)

	if err := s.getDB(ctx).Where("id = ?", fid).First(file).Error; err != nil {
		ctx.Error("文件查询信息失败", zap.Int64("id", fid), zap.Error(err))

		return nil, err
	}

	return file, nil
}

func (s *service) QueryByPath(ctx context.Context, path string) (*models.VirtualFile, error) {
	var paths []string

	if path != "" {
		var normalizeErr error

		_, paths, normalizeErr = utils.NormalizeStoragePathParts(path)
		if normalizeErr != nil {
			return nil, errInvalidVirtualFilePath
		}
	}

	if len(paths) == 0 {
		return nil, errors.Wrap(gorm.ErrRecordNotFound, "文件不存在")
	}

	var pid int64

	var m *models.VirtualFile

	for _, name := range paths {
		m = new(models.VirtualFile)

		if queryErr := s.getDB(ctx).Model(new(models.VirtualFile)).Where("name = ?", name).Where("parent_id = ?", pid).First(m).Error; queryErr != nil {
			return nil, queryErr
		}

		pid = m.ID
	}

	return m, nil
}

// QueryTop 查询这个文件的挂载点信息
func (s *service) QueryTop(ctx context.Context, fid int64) (*models.VirtualFile, error) {
	if m, err := s.Query(ctx, fid); err != nil {
		return nil, err
	} else if m.TopId == m.ID {
		return m, nil
	} else {
		return s.Query(ctx, m.TopId)
	}
}

// FindOrCreateAncestors 查找或创建所有祖先路径（本级不创建），返回最后一级的父ID
func (s *service) FindOrCreateAncestors(ctx context.Context, path string) (int64, error) {
	_, paths, err := utils.NormalizeStoragePathParts(path)
	if err != nil {
		return 0, errInvalidVirtualFilePath
	}

	var pid int64

	// 去掉最后一级路径
	if len(paths) > 0 {
		paths = paths[:len(paths)-1]
	}

	for _, name := range paths {
		var m = new(models.VirtualFile)

		err = s.getDB(ctx).Model(new(models.VirtualFile)).Where("name = ?", name).Where("parent_id = ?", pid).First(m).Error
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				// 不存在则创建
				now := time.Now()
				m = &models.VirtualFile{
					ParentId:   pid,
					Name:       name,
					IsDir:      true,
					Size:       0,
					OsType:     models.OsTypeFolder,
					CreateDate: now,
					ModifyDate: now,
					Rev:        now.Format(consts.RevFormat),
					Addition:   datatypes.JSONMap{},
				}

				// 这里只创建路径祖先目录，不是挂载点顶层节点。
				if _, err = s.Create(ctx, pid, m); err != nil {
					return 0, err
				}

				pid = m.ID

				continue
			}

			return 0, err
		}

		pid = m.ID
	}

	return pid, nil
}
