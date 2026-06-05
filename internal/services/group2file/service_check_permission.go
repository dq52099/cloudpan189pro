package group2file

import (
	"github.com/xxcheng123/cloudpan189-share/internal/framework/context"
	"go.uber.org/zap"
)

// CheckPermission 检查用户组是否有存储挂载点访问权限
func (s *service) CheckPermission(ctx context.Context, groupId int64, fileId int64) (bool, error) {
	if groupId <= 0 {
		return false, errInvalidGroupID
	}

	if fileId <= 0 {
		return false, errInvalidFileID
	}

	var count int64
	if err := s.getDB(ctx).
		Joins("INNER JOIN mount_points ON mount_points.file_id = group2files.file_id").
		Where("group2files.group_id = ? and group2files.file_id = ?", groupId, fileId).
		Count(&count).Error; err != nil {
		ctx.Error("数据查询失败", zap.Int64("groupId", groupId), zap.Int64("fileId", fileId))

		return false, err
	}

	if count > 0 {
		return true, nil
	}

	return false, nil
}
