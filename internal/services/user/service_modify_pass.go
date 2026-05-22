package user

import (
	"strings"

	"github.com/xxcheng123/cloudpan189-share/internal/framework/context"
	"github.com/xxcheng123/cloudpan189-share/internal/pkgs/utils"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// ModifyPass 修改用户密码
func (s *service) ModifyPass(ctx context.Context, uid int64, password string) error {
	if uid <= 0 {
		return errInvalidUserID
	}

	if strings.TrimSpace(password) == "" {
		return errInvalidUserPassword
	}

	// 构建更新数据，包含加密后的密码和版本号+1
	updateData := map[string]any{
		"password": utils.MD5(password),
		"version":  gorm.Expr("version + 1"), // 版本号自增1
	}

	// 执行更新操作
	result := s.getDB(ctx).
		Where("id = ?", uid).
		Updates(updateData)

	if result.Error != nil {
		ctx.Error("修改用户密码失败", zap.Error(result.Error), zap.Int64("uid", uid))

		return result.Error
	}

	if result.RowsAffected == 0 {
		ctx.Error("修改用户密码失败，记录不存在", zap.Int64("uid", uid))

		return gorm.ErrRecordNotFound
	}

	return nil
}
