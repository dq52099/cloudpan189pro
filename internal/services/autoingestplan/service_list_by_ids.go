package autoingestplan

import (
	"github.com/xxcheng123/cloudpan189-share/internal/framework/context"
	"github.com/xxcheng123/cloudpan189-share/internal/repository/models"
)

func (s *service) ListByIDs(ctx context.Context, ids []int64) ([]*models.AutoIngestPlan, error) {
	var plans []*models.AutoIngestPlan
	if len(ids) == 0 {
		return plans, nil
	}

	normalizedIDs, err := normalizeAutoIngestPlanIDs(ids)
	if err != nil {
		return nil, err
	}

	err = s.getDB(ctx).Where("id IN ?", normalizedIDs).Find(&plans).Error

	return plans, err
}

func normalizeAutoIngestPlanIDs(ids []int64) ([]int64, error) {
	seen := make(map[int64]struct{}, len(ids))

	normalized := make([]int64, 0, len(ids))
	for _, id := range ids {
		if id <= 0 {
			return nil, errInvalidAutoIngestPlanID
		}

		if _, ok := seen[id]; ok {
			continue
		}

		seen[id] = struct{}{}
		normalized = append(normalized, id)
	}

	return normalized, nil
}
