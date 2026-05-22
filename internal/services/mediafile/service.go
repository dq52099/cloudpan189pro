package mediafile

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/xxcheng123/cloudpan189-share/internal/bootstrap"
	"github.com/xxcheng123/cloudpan189-share/internal/framework/context"
	"github.com/xxcheng123/cloudpan189-share/internal/repository/models"
	"github.com/xxcheng123/cloudpan189-share/internal/shared"
	"github.com/xxcheng123/cloudpan189-share/internal/types/media"
	"go.uber.org/zap"
	"gorm.io/gorm"
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

// mediaRoot 读取当前媒体根路径（StoragePath）。shared.MediaConfig 为 nil 时返回空串。
func mediaRoot() string {
	cfg := shared.MediaConfig
	if cfg == nil {
		return ""
	}

	return cfg.StoragePath
}

// DeleteStrmByFullPath 根据磁盘绝对路径删除媒体根目录内的 STRM 文件及其 DB 记录。
func (s *service) DeleteStrmByFullPath(ctx context.Context, fullPath string) error {
	relPath := deriveRelativeMediaPath(fullPath)
	if relPath == "" {
		return nil
	}

	root := mediaRoot()

	diskPath, err := mediaFileDiskPath(root, relPath)
	if err != nil {
		return err
	}

	if err = os.Remove(diskPath); err != nil && !os.IsNotExist(err) {
		return err
	}

	if err := s.getDB(ctx).Where("path IN ?", mediaFilePathCandidates(relPath)).Delete(new(models.MediaFile)).Error; err != nil {
		ctx.Warn("清理 STRM DB 记录失败（磁盘已删除）", zap.String("path", fullPath), zap.Error(err))

		return err
	}

	return nil
}

// deriveRelativeMediaPath 把磁盘绝对路径转换为 DB 中存储的相对 path。
// 依赖 shared.MediaConfig.StoragePath，如果无法推断则返回空串。
func deriveRelativeMediaPath(fullPath string) string {
	root := mediaRoot()
	if root == "" {
		return ""
	}

	// 把 rootPath 和 fullPath 都规范为绝对路径比较
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return ""
	}

	absFull, err := filepath.Abs(fullPath)
	if err != nil {
		return ""
	}

	rel, err := filepath.Rel(absRoot, absFull)
	if err != nil {
		return ""
	}

	// DB 里 path 使用 slash 风格
	rel = filepath.ToSlash(rel)

	// rel 必须是下级路径；只有 `..` 或 `../...` 表示 fullPath 不在 root 之下。
	if rel == ".." || strings.HasPrefix(rel, "../") {
		return ""
	}

	return "/" + strings.TrimPrefix(rel, "/")
}
