package virtualfile

import (
	"github.com/xxcheng123/cloudpan189-share/internal/framework/context"
	"go.uber.org/zap"
)

type GroupCountByTopIdRequest struct {
	TopId     int64
	TopIdList []int64
}

type GroupCountByTopId struct {
	TopId int64 `gorm:"column:top_id" json:"topId"`
	Count int64 `gorm:"column:count" json:"count"`
}

func (s *service) GroupCountByTopId(ctx context.Context, req *GroupCountByTopIdRequest) ([]*GroupCountByTopId, error) {
	query := s.getDB(ctx).Select("top_id, COUNT(*) as count").Where("top_id != id").Group("top_id")

	if req != nil && req.TopId != 0 {
		query = query.Where("top_id = ?", req.TopId)
	}

	if req != nil && req.TopIdList != nil {
		topIds, err := normalizeVirtualFileIDs(req.TopIdList)
		if err != nil {
			return nil, err
		}

		if len(topIds) == 0 {
			query = query.Where("1 = 0")
		} else {
			query = query.Where("top_id IN ?", topIds)
		}
	}

	var result []*GroupCountByTopId

	if err := query.Find(&result).Error; err != nil {
		ctx.Error("按TopId分组统计失败", zap.Error(err))

		return nil, err
	}

	return result, nil
}
