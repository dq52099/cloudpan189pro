package mediafile

import (
	"github.com/xxcheng123/cloudpan189-share/internal/bootstrap"
	"github.com/xxcheng123/cloudpan189-share/internal/framework/context"
	"github.com/xxcheng123/cloudpan189-share/internal/repository/models"
	"github.com/xxcheng123/cloudpan189-share/internal/types/media"
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
	if err := os.Remove(fullPath); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}
