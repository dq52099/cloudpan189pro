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

func (s *service) DeleteStrm(ctx context.Context, fid int64, rootPath string) error {
	file, err := s.QueryStrm(ctx, fid)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}

		return err
	}

	// 删除文件
	_ = os.Remove(filepath.Join(rootPath, file.Path))
	// 删除记录
	return s.getDB(ctx).Where("id = ?", file.ID).Delete(new(models.MediaFile)).Error
}

// ClearStrm 清除所有文件（夹）
func (s *service) Clear(ctx context.Context, rootPath string) error {
	entries, err := os.ReadDir(rootPath)
	if err != nil {
		ctx.Error("读取目录失败", zap.String("path", rootPath), zap.Error(err))

		return err
	}

	for _, entry := range entries {
		// 删除文件
		if err := os.RemoveAll(filepath.Join(rootPath, entry.Name())); err != nil {
			ctx.Error("删除文件失败", zap.Error(err), zap.String("path", filepath.Join(rootPath, entry.Name())))

			return err
		}
	}

	// 再删除数据库中的所有记录
	if err := s.getDB(ctx).Where("1 = 1").Delete(new(models.MediaFile)).Error; err != nil {
		ctx.Error("清空数据库失败", zap.Error(err))

		return err
	}

	ctx.Info("清空媒体文件数据成功", zap.String("rootPath", rootPath))

	return nil
}

// ClearAll 清空所有媒体文件记录（不清除实际文件）
func (s *service) ClearAll(ctx context.Context) error {
	db := s.svc.GetDB(ctx).Exec("DELETE FROM media_files")
	if db.Error != nil {
		ctx.Error("清空 media_files 失败", zap.Error(db.Error))
		return db.Error
	}
	ctx.Info("清空 media_files 成功", zap.Int64("deleted", db.RowsAffected))
	return nil
}
