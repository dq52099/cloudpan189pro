package filetasklog

import (
	"github.com/xxcheng123/cloudpan189-share/internal/framework/context"
	"github.com/xxcheng123/cloudpan189-share/internal/repository/models"
	"go.uber.org/zap"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func (s *service) WithError(ctx context.Context, key LogKey, err error) (retErr error) {
	if err == nil {
		return nil
	}

	id, keyErr := validateLogKey(key)
	if keyErr != nil {
		return keyErr
	}

	db := s.getDB(ctx)
	errorMsg := err.Error()

	// 根据数据库类型选择拼接方式
	var concatExpr clause.Expr

	dbType := db.Name()

	switch dbType {
	case "mysql":
		concatExpr = gorm.Expr("CONCAT(COALESCE(error_msg, ''), ?)", "\n"+errorMsg)
	case "postgres", "postgresql", "sqlite", "sqlite3":
		concatExpr = gorm.Expr("COALESCE(error_msg, '') || ?", "\n"+errorMsg)
	case "sqlserver":
		concatExpr = gorm.Expr("CONCAT(ISNULL(error_msg, ''), ?)", "\n"+errorMsg)
	default:
		concatExpr = gorm.Expr("COALESCE(error_msg, '') || ?", "\n"+errorMsg)
	}

	result := db.Model(new(models.FileTaskLog)).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"error_msg": concatExpr,
		})
	if retErr = checkTaskLogUpdateResult(ctx, result, id, "文件任务日志不存在"); retErr != nil {
		ctx.Error("添加文件任务错误信息失败",
			zap.Error(retErr),
			zap.String("db_type", db.Name()),
		)

		return
	}

	return
}

// WithErrorAndFail 添加错误信息并标记任务失败
func (s *service) WithErrorAndFail(ctx context.Context, key LogKey, err error) error {
	if err == nil {
		return nil
	}

	// 先添加错误信息
	if retErr := s.WithError(ctx, key, err); retErr != nil {
		return retErr
	}

	// 然后标记为失败
	return s.Failed(ctx, key)
}

// ClearError 清空错误信息
func (s *service) ClearError(ctx context.Context, key LogKey) error {
	id, err := validateLogKey(key)
	if err != nil {
		return err
	}

	result := s.getDB(ctx).
		Where("id = ?", id).
		Update("error_msg", "")

	return checkTaskLogUpdateResult(ctx, result, id, "文件任务日志不存在")
}
