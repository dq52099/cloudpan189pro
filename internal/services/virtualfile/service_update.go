package virtualfile

import (
	"time"

	"github.com/xxcheng123/cloudpan189-share/internal/framework/context"
	"github.com/xxcheng123/cloudpan189-share/internal/pkgs/utils"
	"github.com/xxcheng123/cloudpan189-share/internal/repository/models"
	"go.uber.org/zap"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type UpdateHook func(ctx context.Context, result *gorm.DB, id int64, opts []utils.Field)

func (s *service) Update(ctx context.Context, id int64, opts []utils.Field, hooks ...UpdateHook) error {
	ctx.Debug("更新文件", zap.Int64("file_id", id), zap.Int("field_count", len(opts)))

	if id <= 0 {
		return errInvalidVirtualFileID
	}

	updates := make(map[string]interface{})
	for _, opt := range opts {
		updates[opt.Key] = opt.Value
	}

	if len(updates) == 0 {
		return errEmptyVirtualFileUpdateFields
	}

	result := s.withLock(ctx, func(db *gorm.DB) *gorm.DB {
		return db.Where("id = ?", id).Updates(updates)
	})

	for _, hook := range hooks {
		hook(ctx, result, id, opts)
	}

	if result.Error != nil {
		return result.Error
	}

	if result.RowsAffected == 0 {
		return s.ensureVirtualFileExists(ctx, id)
	}

	return nil
}

func (s *service) ModifyAddition(ctx context.Context, id int64, key string, value any) error {
	ctx.Debug("修改文件附加信息", zap.Int64("file_id", id), zap.String("key", key))

	if id <= 0 {
		return errInvalidVirtualFileID
	}

	result := s.withLock(ctx, func(db *gorm.DB) *gorm.DB {
		path := "$." + key

		var expr interface{}

		switch db.Name() {
		case "postgres":
			expr = gorm.Expr("jsonb_set(addition, ?, ?::jsonb)", path, value)
		default:
			expr = gorm.Expr("JSON_SET(addition, ?, ?)", path, value)
		}

		return db.Where("id = ?", id).Update("addition", expr)
	})
	if result.Error != nil {
		return result.Error
	}

	if result.RowsAffected == 0 {
		return s.ensureVirtualFileExists(ctx, id)
	}

	return nil
}

func (s *service) BatchUpdatePlus(ctx context.Context, values []utils.Field, exps []clause.Expression) error {
	ctx.Debug("批量更新文件", zap.Int("field_count", len(values)), zap.Int("condition_count", len(exps)))

	updates := make(map[string]interface{})
	for _, field := range values {
		updates[field.Key] = field.Value
	}

	if len(updates) == 0 {
		return errEmptyVirtualFileUpdateFields
	}

	if len(exps) == 0 {
		return errEmptyVirtualFileUpdateConditions
	}

	for _, exp := range exps {
		if exp == nil {
			return errEmptyVirtualFileUpdateConditions
		}
	}

	result := s.withLock(ctx, func(db *gorm.DB) *gorm.DB {
		query := db
		for _, exp := range exps {
			query = query.Where(exp)
		}

		return query.Updates(updates)
	})

	if result.Error != nil {
		return result.Error
	}

	if result.RowsAffected == 0 {
		return s.ensureBatchUpdatePlusTargetExists(ctx, exps)
	}

	return nil
}

func (s *service) ensureBatchUpdatePlusTargetExists(ctx context.Context, exps []clause.Expression) error {
	query := s.getDB(ctx)
	for _, exp := range exps {
		query = query.Where(exp)
	}

	var count int64
	if err := query.Limit(1).Count(&count).Error; err != nil {
		return err
	}

	if count == 0 {
		return gorm.ErrRecordNotFound
	}

	return nil
}

func (s *service) BatchUpdate(ctx context.Context, filesToUpdate map[int64][]utils.Field) error {
	total := len(filesToUpdate)
	ctx.Debug("批量更新文件(Map模式)", zap.Int("total_count", total))

	if total == 0 {
		return nil
	}

	// 1. 将 map 转换为 slice 以便分批处理
	type updateItem struct {
		ID     int64
		Fields []utils.Field
	}

	items := make([]updateItem, 0, total)

	for id, fields := range filesToUpdate {
		if id <= 0 {
			return errInvalidVirtualFileID
		}

		items = append(items, updateItem{ID: id, Fields: fields})
	}

	// 2. 定义批次大小
	batchSize := 10 // 每次事务处理 10 条

	for i := 0; i < total; i += batchSize {
		end := i + batchSize
		if end > total {
			end = total
		}

		batchItems := items[i:end]

		// 3. 执行小事务
		var err error

		result := s.withLock(ctx, func(db *gorm.DB) *gorm.DB {
			err = db.Transaction(func(tx *gorm.DB) error {
				for _, item := range batchItems {
					updates := make(map[string]interface{})
					for _, opt := range item.Fields {
						updates[opt.Key] = opt.Value
					}

					if len(updates) == 0 {
						return errEmptyVirtualFileUpdateFields
					}

					// 执行单条更新
					result := tx.Model(&models.VirtualFile{}).Where("id = ?", item.ID).Updates(updates)
					if result.Error != nil {
						return result.Error
					}

					if result.RowsAffected == 0 {
						return ensureVirtualFileExistsWithDB(tx, item.ID)
					}
				}

				return nil
			})

			return db
		})

		if result.Error != nil {
			ctx.Error("批量更新分片失败", zap.Int("start_index", i), zap.Error(result.Error))

			return result.Error
		}

		if err != nil {
			ctx.Error("批量更新分片失败", zap.Int("start_index", i), zap.Error(err))

			return err
		}

		// 4. 休眠释放锁
		time.Sleep(20 * time.Millisecond)
	}

	return nil
}
