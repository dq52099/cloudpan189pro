package autoingestplan

import (
	"github.com/xxcheng123/cloudpan189-share/internal/framework/context"
	"github.com/xxcheng123/cloudpan189-share/internal/repository/models"
)

func (s *service) ListByIDs(ctx context.Context, ids []int64) ([]*models.AutoIngestPlan, error) {
	var plans []*models.AutoIngestPlan
	err := s.getDB(ctx).Where("id IN ?", ids).Find(&plans).Error
	return plans, err
}
