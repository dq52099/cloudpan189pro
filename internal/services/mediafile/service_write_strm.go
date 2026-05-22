package mediafile

import (
	"os"
	"path/filepath"

	"github.com/pkg/errors"
	"gorm.io/gorm"

	"github.com/xxcheng123/cloudpan189-share/internal/framework/context"
	"github.com/xxcheng123/cloudpan189-share/internal/repository/models"
	"github.com/xxcheng123/cloudpan189-share/internal/types/media"
	"go.uber.org/zap"
)

func (s *service) WriteStrm(ctx context.Context, car media.WriterCar, fid int64, url string) (int64, error) {
	if fid <= 0 {
		return 0, errInvalidMediaFileFID
	}

	fullPath, err := mediaFileDiskPath(car.RootPath(), car.GetPath())
	if err != nil {
		return 0, err
	}

	fileExists := false

	if _, err = os.Stat(fullPath); err == nil {
		fileExists = true
	} else if !os.IsNotExist(err) {
		ctx.Error("检查 STRM 文件状态失败", zap.Error(err), zap.String("path", fullPath))

		return 0, err
	}

	// 先检查记录是否存在
	if _, err := s.QueryStrm(ctx, fid); err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		ctx.Error("查询文件元数据失败", zap.Error(err))

		return 0, err
	} else if err == nil {
		if !fileExists {
			ctx.Warn("检测到 STRM 记录存在但磁盘文件缺失，将重新创建", zap.Int64("fid", fid), zap.String("path", fullPath))

			if err = s.DeleteStrm(ctx, fid, car.RootPath()); err != nil {
				ctx.Error("清理过期 STRM 记录失败", zap.Error(err), zap.Int64("fid", fid))

				return 0, err
			}
		} else if car.GetFileConflictPolicy() == media.FileConflictPolicyReplace {
			if err = s.DeleteStrm(ctx, fid, car.RootPath()); err != nil {
				ctx.Error("删除文件失败", zap.Error(err))

				return 0, err
			}
		} else {
			return 0, nil
		}
	}

	size := int64(len(url))
	dir := filepath.Dir(fullPath)

	// 确保目录存在
	if err := os.MkdirAll(dir, 0o755); err != nil {
		ctx.Error("创建目录失败", zap.Error(err), zap.String("dir", dir))

		return 0, err
	}

	// 写入文件（replace 策略或不存在文件时覆盖写入）
	if err := os.WriteFile(fullPath, []byte(url), 0o644); err != nil {
		ctx.Error("写入文件失败", zap.Error(err), zap.String("path", fullPath))

		return 0, err
	}

	ctx.Debug("写入文件成功", zap.String("path", fullPath))

	file := &models.MediaFile{
		FID:       fid,
		Name:      car.GetName(),
		Path:      normalizeMediaFilePath(car.GetPath()),
		Size:      size,
		MediaType: media.TypeStrm,
		// 保留 hash 字段以备后续校验用途
		Hash: "-",
	}

	// 保存文件元数据；失败时回滚磁盘文件，避免孤儿
	if err := s.getDB(ctx).Create(file).Error; err != nil {
		ctx.Error("保存文件元数据失败，回滚磁盘文件", zap.Error(err), zap.String("path", fullPath))

		if removeErr := os.Remove(fullPath); removeErr != nil && !os.IsNotExist(removeErr) {
			ctx.Warn("回滚 STRM 文件失败", zap.String("path", fullPath), zap.Error(removeErr))
		}

		return 0, err
	}

	return file.ID, nil
}
