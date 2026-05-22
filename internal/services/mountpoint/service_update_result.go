package mountpoint

import (
	pkgErrors "github.com/pkg/errors"
	"github.com/xxcheng123/cloudpan189-share/internal/framework/context"
	"gorm.io/gorm"
)

func (s *service) ensureMountPointFileExists(ctx context.Context, fileId int64) error {
	return s.ensureMountPointExists(
		ctx,
		s.getDB(ctx).Where("file_id = ?", fileId),
	)
}

func (s *service) ensureMountPointUpdateTargetExists(ctx context.Context, id, creatorUserID int64, isAdmin bool) error {
	query := s.getDB(ctx).Where("id = ?", id)
	if !isAdmin {
		query = query.Where("creator_user_id = ?", creatorUserID)
	}

	return s.ensureMountPointExists(ctx, query)
}

func (s *service) ensureMountPointExists(ctx context.Context, query *gorm.DB) error {
	var count int64

	if err := query.Limit(1).Count(&count).Error; err != nil {
		return err
	}

	if count == 0 {
		return pkgErrors.Wrap(gorm.ErrRecordNotFound, "挂载点不存在")
	}

	return nil
}
