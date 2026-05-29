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

	tmpFile, err := os.CreateTemp(dir, "."+filepath.Base(fullPath)+".*.tmp")
	if err != nil {
		ctx.Error("创建临时 STRM 文件失败", zap.Error(err), zap.String("dir", dir))

		return 0, err
	}

	tmpPath := tmpFile.Name()
	cleanupTmpFile := func() {
		if removeErr := os.Remove(tmpPath); removeErr != nil && !os.IsNotExist(removeErr) {
			ctx.Warn("清理临时 STRM 文件失败", zap.String("path", tmpPath), zap.Error(removeErr))
		}
	}

	if _, err = tmpFile.Write([]byte(url)); err != nil {
		ctx.Error("写入临时 STRM 文件失败", zap.Error(err), zap.String("path", tmpPath))

		if closeErr := tmpFile.Close(); closeErr != nil {
			ctx.Warn("关闭临时 STRM 文件失败", zap.String("path", tmpPath), zap.Error(closeErr))
		}

		cleanupTmpFile()

		return 0, err
	}

	if err = tmpFile.Chmod(0o644); err != nil {
		ctx.Error("设置临时 STRM 文件权限失败", zap.Error(err), zap.String("path", tmpPath))

		if closeErr := tmpFile.Close(); closeErr != nil {
			ctx.Warn("关闭临时 STRM 文件失败", zap.String("path", tmpPath), zap.Error(closeErr))
		}

		cleanupTmpFile()

		return 0, err
	}

	if err = tmpFile.Close(); err != nil {
		ctx.Error("关闭临时 STRM 文件失败", zap.Error(err), zap.String("path", tmpPath))

		cleanupTmpFile()

		return 0, err
	}

	// 先写入同目录临时文件；DB 成功后再替换目标，避免 DB 唯一冲突时破坏既有 STRM。
	ctx.Debug("写入临时文件成功", zap.String("path", tmpPath))

	file := &models.MediaFile{
		FID:       fid,
		Name:      car.GetName(),
		Path:      normalizeMediaFilePath(car.GetPath()),
		Size:      size,
		MediaType: media.TypeStrm,
		// 保留 hash 字段以备后续校验用途
		Hash: "-",
	}

	// 保存文件元数据；失败时只清理本次创建的临时文件，避免删除既有目标文件。
	if err := s.getDB(ctx).Create(file).Error; err != nil {
		ctx.Error("保存文件元数据失败，清理临时 STRM 文件", zap.Error(err), zap.String("path", tmpPath))

		cleanupTmpFile()

		return 0, err
	}

	if err := replaceStrmFile(ctx, tmpPath, fullPath); err != nil {
		ctx.Error("替换 STRM 文件失败，回滚文件元数据", zap.Error(err), zap.String("path", fullPath), zap.String("tmp_path", tmpPath))

		cleanupTmpFile()

		if deleteErr := s.getDB(ctx).Where("id = ?", file.ID).Delete(new(models.MediaFile)).Error; deleteErr != nil {
			ctx.Warn("回滚 STRM 元数据失败", zap.Int64("id", file.ID), zap.Error(deleteErr))
		}

		return 0, err
	}

	ctx.Debug("写入文件成功", zap.String("path", fullPath))

	return file.ID, nil
}

func replaceStrmFile(ctx context.Context, tmpPath string, fullPath string) error {
	if err := os.Rename(tmpPath, fullPath); err == nil {
		return nil
	} else {
		directRenameErr := err

		if _, statErr := os.Stat(fullPath); statErr != nil {
			if os.IsNotExist(statErr) {
				return directRenameErr
			}

			return errors.Wrap(statErr, "检查目标 STRM 文件状态失败")
		}

		backupPath, err := createStrmBackupPath(filepath.Dir(fullPath), filepath.Base(fullPath))
		if err != nil {
			return errors.Wrap(err, "创建 STRM 备份路径失败")
		}

		if err = os.Rename(fullPath, backupPath); err != nil {
			return errors.Wrapf(err, "备份目标 STRM 文件失败: %v", directRenameErr)
		}

		if err = os.Rename(tmpPath, fullPath); err != nil {
			if restoreErr := os.Rename(backupPath, fullPath); restoreErr != nil {
				return errors.Wrapf(err, "替换 STRM 文件失败，恢复旧文件也失败: %v", restoreErr)
			}

			return err
		}

		if err = os.Remove(backupPath); err != nil && !os.IsNotExist(err) {
			ctx.Warn("清理 STRM 备份文件失败", zap.String("path", backupPath), zap.Error(err))
		}

		return nil
	}
}

func createStrmBackupPath(dir string, base string) (string, error) {
	backupFile, err := os.CreateTemp(dir, "."+base+".*.bak")
	if err != nil {
		return "", err
	}

	backupPath := backupFile.Name()
	if err = backupFile.Close(); err != nil {
		if removeErr := os.Remove(backupPath); removeErr != nil && !os.IsNotExist(removeErr) {
			return "", errors.Wrapf(err, "清理未关闭的 STRM 备份占位文件失败: %v", removeErr)
		}

		return "", err
	}

	if err = os.Remove(backupPath); err != nil {
		return "", err
	}

	return backupPath, nil
}
