package mountpoint

import (
	"github.com/pkg/errors"
	"github.com/xxcheng123/cloudpan189-share/internal/framework/context"
	"github.com/xxcheng123/cloudpan189-share/internal/repository/models"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type DeleteRequest struct {
	FileId        int64 `json:"fileId"`
	CreatorUserID int64 // 当前用户ID
	IsAdmin       bool  // 是否管理员
}

func (s *service) Delete(ctx context.Context, req *DeleteRequest) error {
	// 非管理员只能删除自己创建的挂载点
	if !req.IsAdmin && req.CreatorUserID > 0 {
		result := s.getDB(ctx).Debug().Unscoped().Where("file_id = ? AND creator_user_id = ?", req.FileId, req.CreatorUserID).Delete(&models.MountPoint{})
		if result.Error != nil {
			ctx.Error("删除挂载点失败", zap.Error(result.Error), zap.Int64("fileId", req.FileId))
			return result.Error
		}
		if result.RowsAffected == 0 {
			ctx.Error("删除挂载点失败，无权限或不存在", zap.Int64("fileId", req.FileId), zap.Int64("creator_user_id", req.CreatorUserID))
			return errors.Wrap(gorm.ErrRecordNotFound, "挂载点不存在或无权限删除")
		}
		return nil
	}

	// 管理员可以删除任何挂载点
	if err := s.getDB(ctx).Debug().Unscoped().Where("file_id = ?", req.FileId).Delete(&models.MountPoint{}).Error; err != nil {
		ctx.Error("删除挂载点失败", zap.Error(err), zap.Int64("fileId", req.FileId))
		return err
	}
	return nil
}

type BatchDeleteRequest struct {
	FileIds       []int64 `json:"fileIds"`
	CreatorUserID int64   // 当前用户ID
	IsAdmin       bool    // 是否管理员
}

func (s *service) BatchDelete(ctx context.Context, req *BatchDeleteRequest) error {
	if len(req.FileIds) == 0 {
		return nil
	}

	var db *gorm.DB
	// 非管理员只能删除自己创建的挂载点
	if !req.IsAdmin && req.CreatorUserID > 0 {
		db = s.getDB(ctx).Debug().Unscoped().Where("file_id IN ? AND creator_user_id = ?", req.FileIds, req.CreatorUserID).Delete(&models.MountPoint{})
	} else {
		// 管理员可以删除任何挂载点
		db = s.getDB(ctx).Debug().Unscoped().Where("file_id IN ?", req.FileIds).Delete(&models.MountPoint{})
	}

	if db.Error != nil {
		ctx.Error("批量删除挂载点失败", zap.Error(db.Error), zap.Int64s("fileIds", req.FileIds))
		return db.Error
	}

	ctx.Info("批量删除挂载点执行完成",
		zap.Int64s("fileIds", req.FileIds),
		zap.Int64("deleted_rows", db.RowsAffected))

	return nil
}

func (s *service) ClearAll(ctx context.Context) (int64, error) {
	db := s.svc.GetDB(ctx).Exec("DELETE FROM mount_points")
	if db.Error != nil {
		ctx.Error("清空所有挂载点失败", zap.Error(db.Error))
		return 0, db.Error
	}

	ctx.Info("清空所有挂载点完成", zap.Int64("deleted_rows", db.RowsAffected))
	return db.RowsAffected, nil
}
