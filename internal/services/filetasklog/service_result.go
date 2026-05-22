package filetasklog

import (
	"github.com/pkg/errors"
	"github.com/xxcheng123/cloudpan189-share/internal/framework/context"
	"github.com/xxcheng123/cloudpan189-share/internal/repository/models"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

func checkTaskLogUpdateResult(ctx context.Context, result *gorm.DB, id int64, notFoundMsg string) error {
	if result.Error != nil {
		return result.Error
	}

	if result.RowsAffected != 0 {
		return nil
	}

	var log models.FileTaskLog

	err := result.Session(&gorm.Session{NewDB: true}).
		WithContext(ctx).
		Model(new(models.FileTaskLog)).
		Select("id").
		Where("id = ?", id).
		Take(&log).
		Error
	if err == nil {
		return nil
	}

	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}

	err = errors.Wrap(gorm.ErrRecordNotFound, notFoundMsg)
	ctx.Error(notFoundMsg, zap.Error(err), zap.Int64("task_id", id))

	return err
}
