package cloudtoken

import (
	"github.com/pkg/errors"
	"github.com/xxcheng123/cloudpan189-share/internal/framework/context"
	"github.com/xxcheng123/cloudpan189-share/internal/repository/models"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

func (s *service) checkCloudTokenUpdateResult(
	ctx context.Context,
	result *gorm.DB,
	id int64,
	userID int64,
	isAdmin bool,
	notFoundMsg string,
) error {
	if result.Error != nil {
		return result.Error
	}

	if result.RowsAffected != 0 {
		return nil
	}

	query := s.getDB(ctx).Where("id = ?", id)

	if !isAdmin {
		if userID <= 0 {
			return errInvalidCloudTokenUserID
		}

		query = query.Where("user_id = ?", userID)
	}

	var count int64
	if err := query.Model(new(models.CloudToken)).Count(&count).Error; err != nil {
		ctx.Error("确认云盘令牌是否存在失败", zap.Error(err), zap.Int64("id", id), zap.Int64("user_id", userID))

		return errors.Wrap(err, "确认云盘令牌是否存在失败")
	}

	if count != 0 {
		return nil
	}

	ctx.Error("云盘令牌更新失败，记录不存在或无权限", zap.Int64("id", id), zap.Int64("user_id", userID), zap.Bool("is_admin", isAdmin))

	return errors.Wrap(gorm.ErrRecordNotFound, notFoundMsg)
}
