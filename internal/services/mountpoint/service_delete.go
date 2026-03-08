package mountpoint

import (
	"github.com/xxcheng123/cloudpan189-share/internal/framework/context"
	"github.com/xxcheng123/cloudpan189-share/internal/repository/models"
	"go.uber.org/zap"
)

func (s *service) Delete(ctx context.Context, fileId int64) error {
	if err := s.getDB(ctx).Debug().Unscoped().Where("file_id = ?", fileId).Delete(&models.MountPoint{}).Error; err != nil {
		ctx.Error("删除挂载点失败", zap.Error(err), zap.Int64("fileId", fileId))
		return err
	}
	return nil
}

func (s *service) BatchDelete(ctx context.Context, ids []int64) error {
	if len(ids) == 0 {
		return nil
	}

	db := s.getDB(ctx).Debug().Unscoped().Where("id IN ?", ids).Delete(&models.MountPoint{})

	if db.Error != nil {
		ctx.Error("批量删除挂载点失败", zap.Error(db.Error), zap.Int64s("ids", ids))
		return db.Error
	}

	ctx.Info("批量删除挂载点执行完成",
		zap.Int64s("req_ids", ids),
		zap.Int64("deleted_rows", db.RowsAffected))

	return nil
}
