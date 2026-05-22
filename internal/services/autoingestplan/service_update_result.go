package autoingestplan

import (
	pkgErrors "github.com/pkg/errors"
	"github.com/xxcheng123/cloudpan189-share/internal/framework/context"
	"github.com/xxcheng123/cloudpan189-share/internal/repository/models"
	"gorm.io/gorm"
)

func (s *service) ensurePlanExists(ctx context.Context, id int64) error {
	var count int64

	if err := s.getDB(ctx).
		Model(new(models.AutoIngestPlan)).
		Where("id = ?", id).
		Limit(1).
		Count(&count).Error; err != nil {
		return err
	}

	if count == 0 {
		return pkgErrors.Wrap(gorm.ErrRecordNotFound, "自动挂载计划不存在")
	}

	return nil
}
