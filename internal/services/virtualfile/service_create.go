package virtualfile

import (
	"fmt"

	"github.com/pkg/errors"
	"github.com/samber/lo"
	"github.com/xxcheng123/cloudpan189-share/internal/bootstrap"
	"github.com/xxcheng123/cloudpan189-share/internal/framework/context"
	"github.com/xxcheng123/cloudpan189-share/internal/pkgs/utils"
	"github.com/xxcheng123/cloudpan189-share/internal/repository/models"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type BatchCreateHook func(ctx context.Context, result *gorm.DB, files []*models.VirtualFile)

func (s *service) BatchCreate(ctx context.Context, parentId int64, files []*models.VirtualFile, hooks ...BatchCreateHook) (int64, error) {
	ctx.Debug("批量创建文件", zap.Int64("parent_id", parentId), zap.Int("file_count", len(files)))

	// 检查 pid
	if parentId < 0 {
		return 0, errors.New("parent_id is invalid")
	}

	if len(files) == 0 {
		return 0, nil
	}

	names := make([]string, 0, len(files)*2)
	for _, file := range files {
		file.ParentId = parentId
		file.Name = utils.SanitizeFileName(file.Name)
		names = append(names, file.Name)

		if renamed := buildVirtualFileDuplicateName(file.Name, file.Rev); renamed != file.Name {
			names = append(names, renamed)
		}
	}

	existNames, err := s.queryHasExist(ctx, parentId, lo.Uniq(names))
	if err != nil {
		return 0, err
	}

	existNameSet := lo.SliceToMap(existNames, func(item string) (string, bool) {
		return item, true
	})

	for _, file := range files {
		if exist, ok := existNameSet[file.Name]; ok && exist {
			originalName := file.Name

			reservedName, reserveErr := s.reserveVirtualFileName(ctx, parentId, file.Name, file.Rev, existNameSet)
			if reserveErr != nil {
				return 0, reserveErr
			}

			file.Name = reservedName

			ctx.Debug("文件已存在 - 重命名", zap.String("file_name", originalName), zap.String("new_name", file.Name), zap.String("cloud_file_id", file.CloudId))
		} else {
			existNameSet[file.Name] = true
		}
	}

	result := s.withLock(ctx, func(db *gorm.DB) *gorm.DB {
		return db.CreateInBatches(files, 1000)
	})

	for _, hook := range hooks {
		hook(ctx, result, files)
	}

	return result.RowsAffected, result.Error
}

func (s *service) Create(ctx context.Context, parentId int64, file *models.VirtualFile) (int64, error) {
	ctx.Debug("创建文件", zap.Int64("parent_id", parentId), zap.String("file_name", file.Name))

	// 检查 pid
	if parentId < 0 {
		return 0, errors.New("parent_id is invalid")
	}

	file.ParentId = parentId
	file.Name = utils.SanitizeFileName(file.Name)

	names := []string{file.Name}
	if renamed := buildVirtualFileDuplicateName(file.Name, file.Rev); renamed != file.Name {
		names = append(names, renamed)
	}

	existNames, err := s.queryHasExist(ctx, parentId, lo.Uniq(names))
	if err != nil {
		return 0, err
	}

	existNameSet := lo.SliceToMap(existNames, func(item string) (string, bool) {
		return item, true
	})

	if existNameSet[file.Name] {
		file.Name, err = s.reserveVirtualFileName(ctx, parentId, file.Name, file.Rev, existNameSet)
		if err != nil {
			return 0, err
		}
	}

	result := s.withLock(ctx, func(db *gorm.DB) *gorm.DB {
		return db.Create(file)
	})

	return file.ID, result.Error
}

func (s *service) CreateTop(ctx context.Context, parentId int64, file *models.VirtualFile) (int64, error) {
	ctx.Debug("创建挂载点", zap.Int64("parent_id", parentId), zap.String("file_name", file.Name))

	if bootstrap.HasTransactionDB(ctx) {
		return s.createTopInTransaction(ctx, parentId, file)
	}

	var (
		id       int64
		txCtx    context.Context
		hasTxCtx bool
	)

	err := s.svc.GetDB(ctx).Transaction(func(tx *gorm.DB) error {
		txCtx = bootstrap.WithTransactionDB(ctx, tx)
		hasTxCtx = true

		createdID, txErr := s.createTopInTransaction(txCtx, parentId, file)
		if txErr != nil {
			return txErr
		}

		id = createdID

		return nil
	})
	if err != nil {
		return 0, err
	}

	if hasTxCtx {
		bootstrap.RunAfterCommitHooks(txCtx)
	}

	return id, nil
}

func (s *service) createTopInTransaction(ctx context.Context, parentId int64, file *models.VirtualFile) (int64, error) {
	id, err := s.createWithoutRename(ctx, parentId, file)
	if err != nil {
		return 0, err
	}

	if err = s.Update(ctx, id, []utils.Field{utils.WithField("top_id", file.ID)}); err != nil {
		ctx.Error("回写 top_id 失败", zap.Int64("file_id", id), zap.Error(err))

		return 0, err
	}

	return id, nil
}

func (s *service) createWithoutRename(ctx context.Context, parentId int64, file *models.VirtualFile) (int64, error) {
	ctx.Debug("创建文件", zap.Int64("parent_id", parentId), zap.String("file_name", file.Name))

	// 检查 pid
	if parentId < 0 {
		return 0, errors.New("parent_id is invalid")
	}

	file.ParentId = parentId
	file.Name = utils.SanitizeFileName(file.Name)

	result := s.withLock(ctx, func(db *gorm.DB) *gorm.DB {
		return db.Create(file)
	})

	return file.ID, result.Error
}

func buildVirtualFileDuplicateName(name string, rev string) string {
	return utils.SanitizeFileName(fmt.Sprintf("%s(%s)", name, rev))
}

func (s *service) reserveVirtualFileName(ctx context.Context, parentId int64, name string, rev string, existNameSet map[string]bool) (string, error) {
	candidate := buildVirtualFileDuplicateName(name, rev)

	for index := 2; ; index++ {
		if !existNameSet[candidate] {
			existNames, err := s.queryHasExist(ctx, parentId, []string{candidate})
			if err != nil {
				return "", err
			}

			if len(existNames) == 0 {
				existNameSet[candidate] = true

				return candidate, nil
			}

			existNameSet[candidate] = true
		}

		candidate = utils.SanitizeFileName(fmt.Sprintf("%s(%s-%d)", name, rev, index))
	}
}

func (s *service) queryHasExist(ctx context.Context, pid int64, names []string) ([]string, error) {
	if len(names) <= 0 {
		return nil, nil
	}

	var existNames = make([]string, 0)

	if err := s.getDB(ctx).Where("parent_id = ? AND name IN (?)", pid, names).Select("name").Find(&existNames).Error; err != nil {
		return nil, err
	}

	return existNames, nil
}
