package usergroup

import (
	"github.com/xxcheng123/cloudpan189-share/internal/framework/context"
	"github.com/xxcheng123/cloudpan189-share/internal/repository/models"
	"go.uber.org/zap"
)

func (s *service) Query(ctx context.Context, gid int64) (*models.UserGroup, error) {
	if gid <= 0 {
		return nil, errInvalidUserGroupID
	}

	var (
		ug = new(models.UserGroup)
	)

	if err := s.getDB(ctx).Where("id = ?", gid).First(ug).Error; err != nil {
		ctx.Error("用户组查询失败", zap.Int64("group_id", gid), zap.Error(err))

		return nil, err
	}

	return ug, nil
}

func (s *service) BatchQuery(ctx context.Context, idList []int64) ([]*models.UserGroup, error) {
	if len(idList) == 0 {
		return []*models.UserGroup{}, nil
	}

	ids, err := normalizeUserGroupIDs(idList)
	if err != nil {
		return nil, err
	}

	var (
		list = make([]*models.UserGroup, 0)
	)

	if err := s.getDB(ctx).Where("id IN ?", ids).Find(&list).Error; err != nil {
		ctx.Error("用户组批量查询失败", zap.Any("group_list", idList), zap.Error(err))

		return nil, err
	}

	return list, nil
}

func normalizeUserGroupIDs(ids []int64) ([]int64, error) {
	seen := make(map[int64]struct{}, len(ids))
	normalized := make([]int64, 0, len(ids))

	for _, id := range ids {
		if id <= 0 {
			return nil, errInvalidUserGroupID
		}

		if _, ok := seen[id]; ok {
			continue
		}

		seen[id] = struct{}{}
		normalized = append(normalized, id)
	}

	return normalized, nil
}
