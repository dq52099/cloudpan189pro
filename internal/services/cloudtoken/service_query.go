package cloudtoken

import (
	"github.com/pkg/errors"
	"github.com/xxcheng123/cloudpan189-share/internal/framework/context"
	"github.com/xxcheng123/cloudpan189-share/internal/repository/models"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

func (s *service) Query(ctx context.Context, id int64) (*models.CloudToken, error) {
	if id <= 0 {
		return nil, errInvalidCloudTokenID
	}

	return s.query(ctx, s.getDB(ctx).Where("id = ?", id), id)
}

func (s *service) QueryAccessible(ctx context.Context, id, userID int64, isAdmin bool) (*models.CloudToken, error) {
	if id <= 0 {
		return nil, errInvalidCloudTokenID
	}

	query := s.getDB(ctx).Where("id = ?", id)

	if !isAdmin {
		if userID <= 0 {
			return nil, errInvalidCloudTokenUserID
		}

		query = query.Where("user_id = ?", userID)
	}

	return s.query(ctx, query, id)
}

func (s *service) query(ctx context.Context, query *gorm.DB, id int64) (*models.CloudToken, error) {
	var cloudToken models.CloudToken
	if err := query.First(&cloudToken).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.Wrap(gorm.ErrRecordNotFound, "云盘令牌不存在")
		}

		ctx.Error("查询云盘令牌失败", zap.Error(err), zap.Int64("id", id))

		return nil, errors.Wrap(err, "查询云盘令牌失败")
	}

	return &cloudToken, nil
}
