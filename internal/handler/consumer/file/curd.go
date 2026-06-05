package file

import (
	"path"
	"slices"
	"strings"
	"time"

	"github.com/pkg/errors"
	"gorm.io/gorm"

	"github.com/xxcheng123/cloudpan189-share/internal/consts"
	"github.com/xxcheng123/cloudpan189-share/internal/framework/context"
	"github.com/xxcheng123/cloudpan189-share/internal/pkgs/utils"
	"github.com/xxcheng123/cloudpan189-share/internal/repository/models"
	verifySvi "github.com/xxcheng123/cloudpan189-share/internal/services/verify"
	"github.com/xxcheng123/cloudpan189-share/internal/services/virtualfile"
	"github.com/xxcheng123/cloudpan189-share/internal/shared"
	"go.uber.org/zap"
)

// batchDeleteFiles 递归删除文件
func (h *handler) batchDeleteFiles(ctx context.Context, filesToDelete []*models.VirtualFile) (err error) {
	if len(filesToDelete) == 0 {
		return nil
	}

	childFilesByParent := make(map[int64][]*models.VirtualFile, len(filesToDelete))

	for _, file := range filesToDelete {
		if file.IsDir {
			currentPath, _ := ctx.GetString(consts.CtxKeyFileFullPath)
			subCtx := ctx.WithValue(consts.CtxKeyFileFullPath, path.Join(currentPath, file.Name))

			child, childErr := h.virtualFileService.List(subCtx, &virtualfile.ListRequest{
				ParentId: &file.ID,
			})
			if childErr != nil {
				if !errors.Is(childErr, gorm.ErrRecordNotFound) {
					ctx.Error("批量删除文件 - 服务层查询子节点失败", zap.Int64("file_id", file.ID), zap.Error(childErr))

					return childErr
				}

				continue
			}

			if len(child) > 0 {
				childFilesByParent[file.ID] = child
			}
		}
	}

	for _, file := range filesToDelete {
		child := childFilesByParent[file.ID]
		if len(child) == 0 {
			continue
		}

		currentPath, _ := ctx.GetString(consts.CtxKeyFileFullPath)
		subCtx := ctx.WithValue(consts.CtxKeyFileFullPath, path.Join(currentPath, file.Name))

		if err = h.batchDeleteFiles(subCtx, child); err != nil {
			ctx.Error("批量删除文件 - 子节点删除失败", zap.Int64("file_id", file.ID), zap.Error(err))

			return err
		}
	}

	// 分批删除逻辑：避免一次性删除过多数据导致事务过大
	const batchSize = 100

	total := len(filesToDelete)

	for i := 0; i < total; i += batchSize {
		end := i + batchSize
		if end > total {
			end = total
		}

		batch := filesToDelete[i:end]

		ids := make([]int64, 0, len(batch))
		for _, file := range batch {
			ids = append(ids, file.ID)
		}

		// 执行当前批次的删除
		if _, err = h.virtualFileService.BatchDelete(ctx, ids, h.deleteStrmIterator); err != nil {
			ctx.Error("批量删除文件 - 服务层删除失败", zap.Int64s("file_ids", ids), zap.Error(err))

			return err
		}

		// 性能优化：每批次删除后短暂休眠，释放 DB 锁
		time.Sleep(20 * time.Millisecond)
	}

	return nil
}

// clearMountFiles 清理挂载点下的所有文件
func (h *handler) clearMountFiles(ctx context.Context, topId int64) error {
	ctx.Debug("清理挂载文件 - 开始清理", zap.Int64("top_id", topId))

	for {
		files, err := h.virtualFileService.List(ctx, &virtualfile.ListRequest{
			TopId:       &topId,
			CurrentPage: 1,
			PageSize:    200, // 缩小单次查询量
		})
		if err != nil {
			ctx.Error("清理挂载文件 - 服务层查询失败", zap.Int64("top_id", topId), zap.Error(err))

			return err
		}

		if len(files) == 0 {
			break
		}

		filesToDelete := make([]int64, 0, len(files))
		for _, file := range files {
			if file.ID != topId {
				filesToDelete = append(filesToDelete, file.ID)
			}
		}

		if len(filesToDelete) == 0 {
			break
		}

		if _, err = h.virtualFileService.BatchDelete(ctx, filesToDelete, h.deleteStrmIterator); err != nil {
			ctx.Error("批量删除文件 - 服务层删除失败", zap.Int64s("file_ids", filesToDelete), zap.Error(err))

			return err
		}

		time.Sleep(50 * time.Millisecond)
	}

	return nil
}

func (h *handler) cleanupEmptyAncestorFolders(ctx context.Context, parentID int64) {
	scanID := parentID

	for scanID > 0 {
		parent, err := h.virtualFileService.Query(ctx, scanID)
		if err != nil || parent == nil {
			if err != nil {
				h.logger.Debug("查询数据库祖先目录失败，停止清理", zap.Int64("pid", scanID), zap.Error(err))
			}

			break
		}

		nextID := parent.ParentId

		children, err := h.virtualFileService.List(ctx, &virtualfile.ListRequest{
			ParentId:    &scanID,
			CurrentPage: 1,
			PageSize:    1,
		})
		if err != nil {
			h.logger.Warn("查询数据库祖先目录子节点失败，停止清理", zap.Int64("pid", scanID), zap.Error(err))

			break
		}

		if len(children) > 0 {
			h.logger.Debug("数据库目录不为空，停止向上清理", zap.Int64("pid", scanID))

			break
		}

		if err := h.virtualFileService.Delete(ctx, scanID); err != nil {
			h.logger.Warn("删除数据库空目录失败", zap.Int64("pid", scanID), zap.Error(err))

			break
		}

		h.logger.Info("成功清理数据库空目录", zap.Int64("pid", scanID), zap.String("name", parent.Name))

		scanID = nextID
	}
}

func (h *handler) batchCreateFiles(ctx context.Context, pid int64, filesToCreate []*models.VirtualFile) (err error) {
	const batchSize = 50 // 每次事务插入 50 条

	total := len(filesToCreate)

	if total == 0 {
		return nil
	}

	ctx.Debug("批量创建文件 - 开始", zap.Int("total_count", total), zap.Int64("pid", pid))

	for i := 0; i < total; i += batchSize {
		end := i + batchSize
		if end > total {
			end = total
		}

		batch := filesToCreate[i:end]

		// 执行小批量创建
		// 注意：BatchCreate 内部的事务范围是这一小批，而不是整个 filesToCreate
		_, err = h.virtualFileService.BatchCreate(ctx, pid, batch, h.createStrmIteratorfunc)
		if err != nil {
			ctx.Error("批量创建文件 - 分片创建失败", zap.Int64("pid", pid), zap.Int("start_index", i), zap.Error(err))

			return err
		}

		// 关键优化：每批次休眠 20-50ms，彻底解决 SQLite 锁表导致的 Web 端超时
		time.Sleep(20 * time.Millisecond)
	}

	return nil
}

// updateStrmContext 封装一次扫描中需要同步的 STRM 更新信息。
type updateStrmContext struct {
	// oldFile 数据库里当前的状态（更新前）。
	oldFile *models.VirtualFile
	// newFile 云端拉到的最新状态（已拥有 newName/newRev 等字段，但还未写回 DB）。
	newFile *models.VirtualFile
}

func (h *handler) batchUpdateFiles(ctx context.Context, filesToUpdate map[int64][]utils.Field) (err error) {
	if err = h.virtualFileService.BatchUpdate(ctx, filesToUpdate); err != nil {
		ctx.Error("批量更新文件 - 服务层更新失败", zap.Error(err))

		return err
	}

	return nil
}

// syncStrmAfterUpdate 针对扫描时检测到变更的文件同步更新 STRM。
// 逻辑：
//   - 如果新旧文件名不同，先删除旧 STRM 再按新名创建；
//   - 如果仅 Rev 变化（URL 签名需要刷新），按新名重写覆盖；
//   - 仅处理非目录且命中 includedSuffixes 的文件。
//
// 该函数在 batchUpdateFiles 成功后调用，不会破坏事务一致性；
// 任何单文件失败只打日志不阻塞整个扫描。
func (h *handler) syncStrmAfterUpdate(ctx context.Context, updates []*updateStrmContext) {
	mediaConfig := shared.GetMediaConfig()
	if mediaConfig == nil || !mediaConfig.Enable {
		return
	}

	if len(updates) == 0 {
		return
	}

	dirPath, ok := ctx.GetString(consts.CtxKeyFileFullPath)
	if !ok {
		ctx.Warn("同步 STRM - 缺少 ctx 路径信息，跳过")

		return
	}

	for _, u := range updates {
		if u == nil || u.oldFile == nil || u.newFile == nil {
			continue
		}

		if u.oldFile.IsDir || u.newFile.IsDir {
			continue
		}

		extOld := path.Ext(u.oldFile.Name)
		extNew := path.Ext(u.newFile.Name)

		includedOld := len(mediaConfig.IncludedSuffixes) == 0 || slices.Contains(mediaConfig.IncludedSuffixes, extOld)
		includedNew := len(mediaConfig.IncludedSuffixes) == 0 || slices.Contains(mediaConfig.IncludedSuffixes, extNew)

		// 旧文件是媒体、新文件不是（极少见：后缀变化），先清理旧 STRM 即可
		if includedOld && !includedNew {
			oldStrm := strings.TrimSuffix(u.oldFile.Name, extOld) + ".strm"

			oldFull := path.Join(mediaConfig.StoragePath, dirPath, oldStrm)
			if err := h.mediaFileService.DeleteStrmByFullPath(ctx, oldFull); err != nil {
				ctx.Warn("同步 STRM - 删除旧 STRM 失败", zap.String("path", oldFull), zap.Error(err))
			}

			_ = h.mediaFileService.DeleteStrm(ctx, u.oldFile.ID, mediaConfig.StoragePath)

			continue
		}

		// 新文件不是媒体格式，不管旧的情况都跳过创建
		if !includedNew {
			continue
		}

		// 如果名称变了，先删除旧 STRM
		if u.oldFile.Name != u.newFile.Name {
			oldStrm := strings.TrimSuffix(u.oldFile.Name, extOld) + ".strm"

			oldFull := path.Join(mediaConfig.StoragePath, dirPath, oldStrm)
			if err := h.mediaFileService.DeleteStrmByFullPath(ctx, oldFull); err != nil {
				ctx.Warn("同步 STRM - 重命名清理旧 STRM 失败", zap.String("path", oldFull), zap.Error(err))
			}

			if err := h.mediaFileService.DeleteStrm(ctx, u.oldFile.ID, mediaConfig.StoragePath); err != nil {
				ctx.Warn("同步 STRM - 清理旧 STRM DB 记录失败", zap.Error(err))
			}
		}

		// 以新名为文件名重建/更新 STRM
		filename := strings.TrimSuffix(u.newFile.Name, extNew) + ".strm"

		values, err := h.verifyService.SignV1(ctx, u.oldFile.ID, verifySvi.WithV1NoExpire())
		if err != nil {
			ctx.Warn("同步 STRM - 获取签名失败", zap.Int64("file_id", u.oldFile.ID), zap.Error(err))

			continue
		}

		// 如果 conflict policy 是 skip 且 DB 已有记录，WriteStrm 会直接返回 (0,nil) 不覆盖。
		// 这里强制走 replace 语义：先清掉 DB 记录让 WriteStrm 重新创建。
		_ = h.mediaFileService.DeleteStrm(ctx, u.oldFile.ID, mediaConfig.StoragePath)

		if _, err := h.mediaFileService.WriteStrm(
			ctx,
			mediaConfig.GetCar(dirPath, filename),
			u.oldFile.ID,
			shared.JoinDownloadURLWithBase(mediaConfig.BaseURL, u.oldFile.ID, values),
		); err != nil {
			ctx.Warn("同步 STRM - 重写 STRM 失败",
				zap.Int64("file_id", u.oldFile.ID),
				zap.String("name", u.newFile.Name),
				zap.Error(err),
			)
		}
	}
}

func (h *handler) createStrmIteratorfunc(ctx context.Context, result *gorm.DB, files []*models.VirtualFile) {
	mediaConfig := shared.GetMediaConfig()
	if result.Error == nil && mediaConfig != nil && mediaConfig.Enable {
		dirPath, ok := ctx.GetString(consts.CtxKeyFileFullPath)
		if !ok {
			ctx.Error("批量创建文件 - 获取文件路径失败")

			return
		}

		ctx.Debug("批量创建文件 - 创建 strm 文件", zap.Int("file_count", len(files)), zap.String("full_path", dirPath))

		for _, file := range files {
			if file.IsDir {
				continue
			}

			// 获取文件后缀
			extName := path.Ext(file.Name)
			if len(mediaConfig.IncludedSuffixes) > 0 && !slices.Contains(mediaConfig.IncludedSuffixes, extName) {
				continue
			}

			// 重新生成文件名: vidoe.mp4 -> video.strm
			filename := strings.TrimSuffix(file.Name, extName) + ".strm"

			// 生成URL
			values, err := h.verifyService.SignV1(ctx, file.ID, verifySvi.WithV1NoExpire())
			if err != nil {
				ctx.Error("批量创建文件 - 遍历 - 获取文件签名失败", zap.Int64("file_id", file.ID), zap.Error(err))

				continue
			}

			if id, err := h.mediaFileService.WriteStrm(ctx, mediaConfig.GetCar(dirPath, filename), file.ID, shared.JoinDownloadURLWithBase(mediaConfig.BaseURL, file.ID, values)); err != nil {
				ctx.Error("批量创建文件 - 遍历 - 创建 strm 文件失败", zap.Int64("file_id", file.ID), zap.Error(err))

				continue
			} else {
				ctx.Debug("批量创建文件 - 遍历 - 创建 strm 文件成功", zap.Int64("file_id", file.ID), zap.Int64("media_file_id", id))
			}
		}
	}
}

func (h *handler) deleteStrmIterator(ctx context.Context, result *gorm.DB, files []*models.VirtualFile) {
	mediaConfig := shared.GetMediaConfig()
	if result.Error == nil && mediaConfig != nil && mediaConfig.Enable {
		ctx.Debug("批量删除文件 - 删除 strm 文件", zap.Int("file_count", len(files)))

		dirPath, hasPath := ctx.GetString(consts.CtxKeyFileFullPath)

		for _, file := range files {
			if file.IsDir {
				continue
			}

			_ = h.mediaFileService.DeleteStrm(ctx, file.ID, mediaConfig.StoragePath)

			if hasPath {
				extName := path.Ext(file.Name)
				if len(mediaConfig.IncludedSuffixes) > 0 && !slices.Contains(mediaConfig.IncludedSuffixes, extName) {
					continue
				}

				strmName := strings.TrimSuffix(file.Name, extName) + ".strm"
				fullPhysicalPath := path.Join(mediaConfig.StoragePath, dirPath, strmName)

				if err := h.mediaFileService.DeleteStrmByFullPath(ctx, fullPhysicalPath); err != nil {
					ctx.Warn("删除 strm 物理文件失败", zap.String("path", fullPhysicalPath), zap.Error(err))
				}
			}
		}
	}
}
