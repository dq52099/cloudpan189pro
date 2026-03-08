package virtualfile

import (
	"github.com/xxcheng123/cloudpan189-share/internal/framework/context"
	"go.uber.org/zap"
)

func (s *service) ClearAll(ctx context.Context) error {
	db := s.svc.GetDB(ctx).Exec("DELETE FROM virtual_files")
	if db.Error != nil {
		ctx.Error("清空 virtual_files 失败", zap.Error(db.Error))
		return db.Error
	}
	ctx.Info("清空 virtual_files 成功", zap.Int64("deleted", db.RowsAffected))
	return nil
}
