package mediafile

import (
	"github.com/xxcheng123/cloudpan189-share/internal/bootstrap"
	"github.com/xxcheng123/cloudpan189-share/internal/framework/context"
	"github.com/xxcheng123/cloudpan189-share/internal/repository/models"
	"github.com/xxcheng123/cloudpan189-share/internal/types/media"
	"go.uber.org/zap"
	"gorm.io/gorm"
	"os"
)

// Service 面向 MediaFile 的服务接口
type Service interface {
	WriteStrm(
		ctx context.Context,
		car media.WriterCar,
		fid int64,
		url string) (int64, error)
	QueryStrm(ctx context.Context, fid int64) (*models.MediaFile, error)
	QueryByPath(ctx context.Context, path string) (*models.MediaFile, error)
	DeleteStrm(ctx context.Context, fid int64, rootPath string) error
	DeleteStrmByFullPath(ctx context.Context, fullPath string) error
	ClearEmptyDir(ctx context.Context, entryPath string) error
	Clear(ctx context.Context, rootPath string) error
	ClearAll(ctx context.Context) error
}

type service struct {
	svc bootstrap.ServiceContext
}

func NewService(svc bootstrap.ServiceContext) Service {
	return &service{
		svc: svc,
	}
}

func (s *service) getDB(ctx context.Context) *gorm.DB {
	return s.svc.GetDB(ctx).Model(new(models.MediaFile))
}

func (s *service) DeleteStrmByFullPath(ctx context.Context, fullPath string) error {
	// 清理磁盘文件
	if err := os.Remove(fullPath); err != nil && !os.IsNotExist(err) {
		return err
	}

	// 同步清理 DB 记录，避免数据-磁盘不一致
	// path 字段存的是相对路径，这里提取 root 之下的相对路径较复杂，
	// 保守按 fullPath 作为 suffix 匹配删除。
	if err := s.getDB(ctx).Where("? LIKE '%' || path", fullPath).Delete(new(models.MediaFile)).Error; err != nil {
		ctx.Warn("清理 STRM DB 记录失败（磁盘已删除）", zap.String("path", fullPath), zap.Error(err))
	}

	return nil
}
