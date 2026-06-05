package group2file

import (
	"github.com/xxcheng123/cloudpan189-share/internal/framework/context"
	"go.uber.org/zap"
)

// GetBindFiles 获取组绑定的存储挂载点文件ID
func (s *service) GetBindFiles(ctx context.Context, groupId int64) ([]int64, error) {
	if groupId <= 0 {
		return nil, errInvalidGroupID
	}

	fileIds := make([]int64, 0)
	if err := s.getDB(ctx).
		Joins("INNER JOIN mount_points ON mount_points.file_id = group2files.file_id").
		Where("group2files.group_id = ?", groupId).
		Distinct("group2files.file_id").
		Order("group2files.file_id ASC").
		Pluck("group2files.file_id", &fileIds).Error; err != nil {
		ctx.Error("查询用户组文件权限失败", zap.Int64("groupId", groupId), zap.Error(err))

		return nil, err
	}

	return fileIds, nil
}
