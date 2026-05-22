package mountpoint

import (
	"errors"

	"github.com/samber/lo"
	"github.com/xxcheng123/cloudpan189-share/internal/framework/context"
	"github.com/xxcheng123/cloudpan189-share/internal/repository/models"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

func (s *service) Query(ctx context.Context, fileId int64) (*models.MountPoint, error) {
	if fileId <= 0 {
		return nil, errInvalidMountPointFileID
	}

	var mountPoint models.MountPoint

	if err := s.getDB(ctx).Where("file_id = ?", fileId).First(&mountPoint).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}

		ctx.Error("查询挂载点失败", zap.Error(err), zap.Int64("fileId", fileId))

		return nil, err
	}

	return &mountPoint, nil
}

func (s *service) QueryByPath(ctx context.Context, fullPath string) (*models.MountPoint, error) {
	var mountPoint models.MountPoint

	if err := s.getDB(ctx).Where("full_path = ?", fullPath).First(&mountPoint).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}

		ctx.Error("根据路径查询挂载点失败", zap.Error(err), zap.String("fullPath", fullPath))

		return nil, err
	}

	return &mountPoint, nil
}

// GetAccessibleMountPointIDs 获取用户可访问的挂载点ID列表（用于文件浏览）
// 管理员返回所有挂载点，普通用户可以访问：1.自己创建的 2.用户组分享的 3.自己绑定了令牌的
func (s *service) GetAccessibleMountPointIDs(ctx context.Context, userID int64, isAdmin bool, groupFileIds []int64) ([]int64, error) {
	var ids []int64

	query := s.getDB(ctx).Model(new(models.MountPoint))

	if !isAdmin {
		if userID <= 0 {
			return nil, errInvalidMountPointUserID
		}

		query = query.Where("creator_user_id = ?", userID)

		if len(groupFileIds) > 0 {
			normalizedGroupFileIDs, err := normalizeMountPointFileIDs(groupFileIds)
			if err != nil {
				return nil, err
			}

			query = query.Or("file_id IN ?", normalizedGroupFileIDs)
		}

		if s.userMountPointTokenService != nil {
			boundMountPointIDs, err := s.userMountPointTokenService.GetUserMountPointIDs(ctx, userID)
			if err != nil {
				return nil, err
			}

			if len(boundMountPointIDs) > 0 {
				query = query.Or("id IN ?", boundMountPointIDs)
			}
		}
	}
	// 管理员：可以访问所有挂载点

	if err := query.Pluck("file_id", &ids).Error; err != nil {
		ctx.Error("获取可访问挂载点ID失败", zap.Error(err), zap.Int64("user_id", userID), zap.Bool("is_admin", isAdmin))

		return nil, err
	}

	return lo.Uniq(ids), nil
}
