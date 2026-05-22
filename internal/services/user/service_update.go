package user

import (
	"github.com/xxcheng123/cloudpan189-share/internal/framework/context"
	"github.com/xxcheng123/cloudpan189-share/internal/pkgs/utils"
	"github.com/xxcheng123/cloudpan189-share/internal/repository/models"
	"go.uber.org/zap"
)

func (s *service) Update(ctx context.Context, uid int64, fields ...utils.Field) error {
	if uid <= 0 {
		return errInvalidUserID
	}

	mp := make(map[string]interface{})

	for _, field := range fields {
		mp[field.Key] = field.Value
	}

	if len(mp) == 0 {
		return errEmptyUserUpdateFields
	}

	result := s.getDB(ctx).Model(new(models.User)).Where("id = ?", uid).Updates(mp)
	if err := s.checkUserUpdateResult(ctx, result, uid); err != nil {
		ctx.Error("更新用户信息失败", zap.Error(err), zap.Int64("user_id", uid))

		return err
	}

	return nil
}
