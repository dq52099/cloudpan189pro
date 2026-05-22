package user

import (
	"strings"

	"github.com/xxcheng123/cloudpan189-share/internal/framework/context"
	"github.com/xxcheng123/cloudpan189-share/internal/repository/models"
	"go.uber.org/zap"
)

func (s *service) Query(ctx context.Context, uid int64) (*models.User, error) {
	if uid <= 0 {
		return nil, errInvalidUserID
	}

	var (
		user = new(models.User)
	)

	if err := s.getDB(ctx).Where("id = ?", uid).First(user).Error; err != nil {
		ctx.Error("用户查询失败", zap.Int64("user_id", uid), zap.Error(err))

		return nil, err
	}

	return user, nil
}

func (s *service) QueryByUsername(ctx context.Context, username string) (*models.User, error) {
	if strings.TrimSpace(username) == "" {
		return nil, errInvalidUsername
	}

	var (
		user = new(models.User)
	)

	if err := s.getDB(ctx).Where("username = ?", username).First(user).Error; err != nil {
		ctx.Error("用户查询失败", zap.String("username", username), zap.Error(err))

		return nil, err
	}

	return user, nil
}
