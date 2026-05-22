package mediafile

import (
	"github.com/xxcheng123/cloudpan189-share/internal/framework/context"
	"github.com/xxcheng123/cloudpan189-share/internal/repository/models"
	"github.com/xxcheng123/cloudpan189-share/internal/types/media"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

func (s *service) QueryStrm(ctx context.Context, fid int64) (*models.MediaFile, error) {
	if fid <= 0 {
		return nil, errInvalidMediaFileFID
	}

	file := new(models.MediaFile)
	if err := s.getDB(ctx).Where("fid = ?", fid).Where("media_type = ?", media.TypeStrm).First(&file).Error; err != nil {
		ctx.Debug("文件查询信息失败", zap.Int64("fid", fid), zap.Error(err))

		return nil, err
	}

	return file, nil
}

func (s *service) QueryByPath(ctx context.Context, path string) (*models.MediaFile, error) {
	file := new(models.MediaFile)

	candidates := mediaFilePathCandidates(path)
	if len(candidates) == 0 {
		return nil, gorm.ErrRecordNotFound
	}

	if err := s.getDB(ctx).Where("path IN ?", candidates).First(&file).Error; err != nil {
		ctx.Debug("文件查询信息失败", zap.String("path", path), zap.Error(err))

		return nil, err
	}

	return file, nil
}
