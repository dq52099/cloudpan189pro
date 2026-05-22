package group2file

import (
	"github.com/xxcheng123/cloudpan189-share/internal/framework/context"
	"go.uber.org/zap"
)

// CheckPermission 检查用户组是否有文件访问权限
func (s *service) CheckPermission(ctx context.Context, groupId int64, fileId int64) (bool, error) {
	if groupId <= 0 {
		return false, errInvalidGroupID
	}

	if fileId <= 0 {
		return false, errInvalidFileID
	}

	var count int64
	if err := s.getDB(ctx).Where("group_id = ? and file_id = ?", groupId, fileId).Count(&count).Error; err != nil {
		ctx.Error("数据查询失败", zap.Int64("groupId", groupId), zap.Int64("fileId", fileId))

		return false, err
	}

	if count > 0 {
		return true, nil
	}

	return false, nil
}
