package virtualfile

import (
	pkgErrors "github.com/pkg/errors"
	"github.com/xxcheng123/cloudpan189-share/internal/framework/context"
	"github.com/xxcheng123/cloudpan189-share/internal/repository/models"
	"gorm.io/gorm"
)

func (s *service) ensureVirtualFileExists(ctx context.Context, id int64) error {
	return ensureVirtualFileExistsWithDB(s.getDB(ctx), id)
}

func ensureVirtualFileExistsWithDB(db *gorm.DB, id int64) error {
	var count int64

	if err := db.Model(&models.VirtualFile{}).Where("id = ?", id).Limit(1).Count(&count).Error; err != nil {
		return err
	}

	if count == 0 {
		return pkgErrors.Wrap(gorm.ErrRecordNotFound, "文件不存在")
	}

	return nil
}
