package virtualfile

import (
	"github.com/xxcheng123/cloudpan189-share/internal/framework/context"
	"github.com/xxcheng123/cloudpan189-share/internal/repository/models"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

func (s *service) ClearAll(ctx context.Context) error {
	var deleted int64

	if err := s.withWriteLock(ctx, func(db *gorm.DB) error {
		return db.Transaction(func(tx *gorm.DB) error {
			if err := tx.Session(&gorm.Session{NewDB: true}).Where("1 = 1").Delete(new(models.Group2File)).Error; err != nil {
				return err
			}

			result := tx.Exec("DELETE FROM virtual_files")
			if result.Error != nil {
				return result.Error
			}

			deleted = result.RowsAffected

			return nil
		})
	}); err != nil {
		ctx.Error("清空 virtual_files 失败", zap.Error(err))

		return err
	}

	ctx.Info("清空 virtual_files 成功", zap.Int64("deleted", deleted))

	return nil
}
