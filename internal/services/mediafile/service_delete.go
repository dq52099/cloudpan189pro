package mediafile

import (
	"os"
	"path/filepath"

	"github.com/pkg/errors"
	"github.com/xxcheng123/cloudpan189-share/internal/framework/context"
	"github.com/xxcheng123/cloudpan189-share/internal/repository/models"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// DeleteStrm 删除单个 STRM 文件及其数据库记录。
// 磁盘删除失败会记录 warning 并继续删除 DB 记录，避免孤儿 DB 记录阻塞后续流程。
func (s *service) DeleteStrm(ctx context.Context, fid int64, rootPath string) error {
	file, err := s.QueryStrm(ctx, fid)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}

		return err
	}

	// 删除磁盘文件
	if err := os.Remove(filepath.Join(rootPath, file.Path)); err != nil && !os.IsNotExist(err) {
		ctx.Warn("删除 STRM 文件失败（将继续清理 DB 记录）",
			zap.String("path", filepath.Join(rootPath, file.Path)),
			zap.Error(err),
		)
	}

	// 删除记录
	return s.getDB(ctx).Where("id = ?", file.ID).Delete(new(models.MediaFile)).Error
}

// Clear 清除根路径下所有文件夹并清空对应 DB 记录。
// 单个文件删除失败时记录 warning 并继续；所有文件处理完后统一清理 DB。
func (s *service) Clear(ctx context.Context, rootPath string) error {
	entries, err := os.ReadDir(rootPath)
	if err != nil {
		ctx.Error("读取目录失败", zap.String("path", rootPath), zap.Error(err))

		return err
	}

	var failedCount int

	for _, entry := range entries {
		target := filepath.Join(rootPath, entry.Name())
		if err := os.RemoveAll(target); err != nil {
			ctx.Warn("删除文件失败，已跳过", zap.Error(err), zap.String("path", target))
			failedCount++
		}
	}

	if failedCount > 0 {
		ctx.Warn("部分磁盘文件清理失败，将继续清理 DB 记录", zap.Int("failed_count", failedCount))
	}

	// 再删除数据库中的所有记录（即便部分磁盘清理失败也要保持 DB 一致）
	if err := s.getDB(ctx).Where("1 = 1").Delete(new(models.MediaFile)).Error; err != nil {
		ctx.Error("清空数据库失败", zap.Error(err))

		return err
	}

	ctx.Info("清空媒体文件数据成功", zap.String("rootPath", rootPath), zap.Int("disk_failed", failedCount))

	return nil
}

// ClearAll 清空所有媒体文件的 DB 记录（不清除实际文件）。
// 适用于场景：外部手动维护磁盘文件，只需重置 DB 登记。
func (s *service) ClearAll(ctx context.Context) error {
	result := s.getDB(ctx).Unscoped().Where("1 = 1").Delete(new(models.MediaFile))
	if result.Error != nil {
		ctx.Error("清空 media_files 失败", zap.Error(result.Error))

		return result.Error
	}

	ctx.Info("清空 media_files 成功", zap.Int64("deleted", result.RowsAffected))

	return nil
}
