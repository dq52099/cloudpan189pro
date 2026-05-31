package file

import (
	"errors"
	"fmt"
	"strings"

	"github.com/xxcheng123/cloudpan189-share/internal/services/filetasklog"
	"github.com/xxcheng123/cloudpan189-share/internal/shared"

	cloudbridgeSvi "github.com/xxcheng123/cloudpan189-share/internal/services/cloudbridge"

	pkgErrors "github.com/pkg/errors"
	"github.com/samber/lo"
	"github.com/xxcheng123/cloudpan189-share/internal/consts"
	"github.com/xxcheng123/cloudpan189-share/internal/framework/context"
	"github.com/xxcheng123/cloudpan189-share/internal/framework/taskcontext"
	"github.com/xxcheng123/cloudpan189-share/internal/pkgs/utils"
	"github.com/xxcheng123/cloudpan189-share/internal/repository/models"
	"github.com/xxcheng123/cloudpan189-share/internal/types/apierrcode"
	"github.com/xxcheng123/cloudpan189-share/internal/types/converter"
	"github.com/xxcheng123/cloudpan189-share/internal/types/topic"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

var errScanTaskOwnerChanged = pkgErrors.New("刷新任务归属已变化")

func (h *handler) ScanFile() taskcontext.HandlerFunc {
	return func(ctx *taskcontext.Context) (scanErr error) {
		req := new(topic.FileScanFileRequest)

		if err := ctx.Unmarshal(req); err != nil {
			return err
		}

		var (
			logger = ctx.GetContext().Logger
		)

		topFile := models.RootFile()

		if req.FileId != 0 {
			if file, err := h.virtualFileService.Query(ctx.GetContext(), req.FileId); err != nil {
				logger.Error("查询文件失败", zap.Int64("file_id", req.FileId), zap.Error(err))

				return err
			} else {
				topFile = file
			}
		}

		if !topFile.IsDir {
			ctx.GetContext().Error("文件不是文件夹", zap.Int64("file_id", req.FileId))

			return pkgErrors.New("文件不是文件夹，不支持扫描")
		}

		if err := h.ensureScanTaskMountPointOwner(ctx.GetContext(), req, topFile); err != nil {
			if errors.Is(err, errScanTaskOwnerChanged) || errors.Is(err, gorm.ErrRecordNotFound) {
				logger.Warn("刷新任务归属校验失败，跳过扫描", zap.Int64("file_id", req.FileId), zap.Error(err))

				return nil
			}

			logger.Error("刷新任务归属校验失败", zap.Int64("file_id", req.FileId), zap.Error(err))

			return err
		}

		// 对同一挂载点（顶层文件）加内存锁，避免手动刷新 + 定时刷新并发导致
		// 重复 diff 同一棵树，触发 UNIQUE 冲突被重命名成 Name(rev)。
		// 非顶层文件不锁，保持 Deep 模式下的子目录并发。
		if topFile.IsTop && topFile.ID > 0 {
			release, acquired := acquireScanLock(topFile.ID)
			if !acquired {
				logger.Warn("挂载点正在被扫描，跳过本次扫描", zap.Int64("file_id", topFile.ID), zap.String("path", topFile.Name))

				return nil
			}
			defer release()
		}

		tracker, logErr := h.fileTaskLogService.Create(
			ctx.GetContext(),
			req.Topic().String(),
			fmt.Sprintf("扫描目录: %s", ctx.GetContext().String(consts.CtxKeyFullPath, topFile.Name)),
			filetasklog.WithFile(topFile.ID),
			filetasklog.WithDesc(fmt.Sprintf(
				"调用者: %s, 深度扫描: %t, 文件ID: %d, 目录名: %s, 上级ID: %d, 挂载点ID: %d",
				ctx.GetContext().String(consts.CtxKeyInvokeHandlerName, "unknown"),
				req.Deep,
				req.FileId,
				topFile.Name,
				topFile.ParentId,
				topFile.TopId,
			)),
		)
		if logErr != nil {
			logger.Error("创建文件任务日志失败", zap.Int64("file_id", req.FileId), zap.Error(logErr))

			return logErr
		}

		_ = h.fileTaskLogService.Running(ctx.GetContext(), tracker)

		defer func() {
			if scanErr != nil {
				if err := h.fileTaskLogService.Failed(ctx.GetContext(), tracker, tracker.WithCost(), utils.WithField("result", scanErr.Error())); err != nil {
					if errors.Is(err, filetasklog.ErrFileTaskLogTerminalState) {
						logger.Warn("文件任务日志已处于终态，跳过失败状态回写", zap.Int64("file_id", req.FileId), zap.Error(err))
					} else {
						logger.Error("更新文件任务日志失败", zap.Int64("file_id", req.FileId), zap.Error(err))
						scanErr = fmt.Errorf("%w; 更新文件任务日志失败: %w", scanErr, err)
					}
				}
				// 写入挂载点失败状态（顶层文件才写入）
				if req.FileId != 0 && topFile.IsTop {
					if err := h.mountPointService.UpdateLastState(ctx.GetContext(), topFile.ID, "失败: "+scanErr.Error()); err != nil {
						logger.Warn("写入挂载点失败状态失败", zap.Error(err))
					}
				}
			} else {
				if err := h.fileTaskLogService.Completed(
					ctx.GetContext(),
					tracker,
					tracker.WithCost(),
					utils.WithField("completed", gorm.Expr("total")),
				); err != nil {
					if errors.Is(err, filetasklog.ErrFileTaskLogTerminalState) {
						logger.Warn("文件任务日志已处于终态，跳过成功状态回写", zap.Int64("file_id", req.FileId), zap.Error(err))

						return
					}

					logger.Error("更新文件任务日志失败", zap.Int64("file_id", req.FileId), zap.Error(err))

					scanErr = err

					return
				}
				// 写入挂载点成功状态
				if req.FileId != 0 && topFile.IsTop {
					if err := h.mountPointService.UpdateLastState(ctx.GetContext(), topFile.ID, "成功"); err != nil {
						logger.Warn("写入挂载点成功状态失败", zap.Error(err))
					}
				}
			}
		}()

		if shared.MediaConfig != nil && shared.MediaConfig.Enable && shared.MediaConfig.AutoClean {
			defer func() {
				_ = h.mediaFileService.ClearEmptyDir(ctx.GetContext(), shared.MediaConfig.StoragePath)
			}()
		}

		logger.Debug("开始扫描文件", zap.Int64("file_id", req.FileId), zap.Bool("deep", req.Deep))

		if err := h.walkFile(ctx.GetContext(), req.FileId, func(ctx context.Context, inputFile *models.VirtualFile, childrenFiles []*models.VirtualFile) (nextWalkFiles []*models.VirtualFile, err error) {
			if inputFile == nil {
				return make([]*models.VirtualFile, 0), nil
			}

			defer func() {
				counter := filetasklog.WithCompletedOneCounter()
				if err != nil {
					counter = filetasklog.WithFailedCounter(1)
				}

				_ = h.fileTaskLogService.FlushCount(ctx, tracker, counter)
			}()

			_ = h.fileTaskLogService.FlushCount(ctx, tracker, filetasklog.WithTotalCounter(1))

			// 如果是根目录，直接返回子目录
			if inputFile.ID == 0 {
				ctx.Info("根目录直接返回子目录查询")

				return lo.Filter(childrenFiles, func(item *models.VirtualFile, index int) bool {
					return item.IsDir
				}), nil
			}

			// 扫描完成后更新挂载点的更新时间
			defer func() {
				if inputFile.TopId > 0 {
					if err := h.mountPointService.UpdateRefreshTime(ctx, inputFile.TopId); err != nil {
						ctx.Error("更新挂载点刷新时间失败", zap.Int64("mount_point_id", inputFile.TopId), zap.Error(err))
					}
				}
			}()

			var fileConverters []converter.VirtualFileConverter

			switch inputFile.OsType {
			case models.OsTypeSubscribe:
				var (
					upUserId string
					ok       bool
				)

				if upUserId, ok = inputFile.Addition.String(consts.FileAdditionKeyUpUserId); !ok {
					ctx.Error("获取订阅用户失败", zap.Int64("file_id", inputFile.ID))

					return nil, pkgErrors.New("获取订阅用户失败")
				}

				fileConverters, err = h.cloudBridgeService.GetSubscribeUserFiles(ctx, upUserId)
			case models.OsTypeSubscribeShareFolder:
				var (
					upUserId string
					shareId  int64
					isFolder bool
					ok       bool
				)

				if upUserId, ok = inputFile.Addition.String(consts.FileAdditionKeyUpUserId); !ok {
					ctx.Error("获取订阅用户失败", zap.Int64("file_id", inputFile.ID))

					return nil, pkgErrors.New("获取订阅用户失败")
				}

				if shareId, ok = inputFile.Addition.Int64(consts.FileAdditionKeyShareId); !ok {
					ctx.Error("获取分享ID失败", zap.Int64("file_id", inputFile.ID))

					return nil, pkgErrors.New("获取分享ID失败")
				}

				if isFolder, ok = inputFile.Addition.Bool(consts.FileAdditionKeyIsFolder); !ok {
					ctx.Error("获取分享类型失败", zap.Int64("file_id", inputFile.ID))

					return nil, pkgErrors.New("获取分享类型失败")
				}

				fileConverters, err = h.cloudBridgeService.GetSubscribeShareFiles(ctx, upUserId, shareId, inputFile.CloudId, isFolder)
			case models.OsTypeShareFolder:
				var (
					shareId    int64
					shareMode  int
					accessCode string
					isFolder   bool
					ok         bool
				)

				if shareId, ok = inputFile.Addition.Int64(consts.FileAdditionKeyShareId); !ok {
					ctx.Error("获取分享ID失败", zap.Int64("file_id", inputFile.ID))

					return nil, pkgErrors.New("获取分享ID失败")
				}

				if shareMode, ok = inputFile.Addition.Int(consts.FileAdditionKeyShareMode); !ok {
					if v, fOk := inputFile.Addition[consts.FileAdditionKeyShareMode]; fOk {
						if fMode, ok := v.(float64); ok {
							shareMode = int(fMode)
						} else {
							shareMode = 1
						}
					} else {
						shareMode = 1
					}
				}

				accessCode, _ = inputFile.Addition.String(consts.FileAdditionKeyAccessCode)
				// 调试
				logger.Info("准备扫描分享文件",
					zap.Int64("shareId", shareId),
					zap.String("accessCode", accessCode),
					zap.Int("shareMode", shareMode))

				if isFolder, ok = inputFile.Addition.Bool(consts.FileAdditionKeyIsFolder); !ok {
					ctx.Error("获取分享类型失败", zap.Int64("file_id", inputFile.ID))

					return nil, pkgErrors.New("获取分享类型失败")
				}

				fileConverters, err = h.cloudBridgeService.GetShareFiles(ctx, shareId, inputFile.CloudId, shareMode, accessCode, isFolder)
			case models.OsTypePersonFolder:
				mountInfo, mountErr := h.mountPointService.Query(ctx, inputFile.TopId)
				if mountErr != nil {
					ctx.Error("查询挂载点失败", zap.Int64("file_id", inputFile.ID), zap.Error(mountErr))

					return nil, mountErr
				}

				token, queryErr := h.cloudTokenService.Query(ctx, mountInfo.TokenId)
				if queryErr != nil {
					ctx.Error("获取云盘令牌失败", zap.Int64("file_id", inputFile.ID), zap.Error(err))

					return nil, pkgErrors.New("获取云盘令牌失败")
				}

				fileConverters, err = h.cloudBridgeService.GetCloudFiles(ctx, cloudbridgeSvi.NewAuthToken(token.AccessToken, token.ExpiresIn), inputFile.CloudId)
			case models.OsTypeFamilyFolder:
				mountInfo, mountErr := h.mountPointService.Query(ctx, inputFile.TopId)
				if mountErr != nil {
					ctx.Error("查询挂载点失败", zap.Int64("file_id", inputFile.ID), zap.Error(mountErr))

					return nil, mountErr
				}

				token, queryErr := h.cloudTokenService.Query(ctx, mountInfo.TokenId)
				if queryErr != nil {
					ctx.Error("获取云盘令牌失败", zap.Int64("file_id", inputFile.ID), zap.Error(queryErr))

					return nil, pkgErrors.New("获取云盘令牌失败")
				}

				var (
					familyId string
					ok       bool
				)

				if familyId, ok = inputFile.Addition.String(consts.FileAdditionKeyFamilyId); !ok {
					ctx.Error("获取家庭文件夹ID失败", zap.Int64("file_id", inputFile.ID))

					return nil, pkgErrors.New("获取家庭文件夹ID失败")
				}

				fileConverters, err = h.cloudBridgeService.GetCloudFamilyFiles(ctx, cloudbridgeSvi.NewAuthToken(token.AccessToken, token.ExpiresIn), familyId, inputFile.CloudId)
			default:
				return nil, pkgErrors.Errorf("不支持的文件类型 %s", inputFile.OsType)
			}

			if err != nil {
				ctx.Error("获取新文件失败", zap.Int64("file_id", inputFile.ID), zap.String("os_type", inputFile.OsType), zap.Error(err))

				return nil, pkgErrors.Wrap(err, "获取新文件失败")
			}

			var newFiles = make([]*models.VirtualFile, 0, len(fileConverters))
			for _, c := range fileConverters {
				newFiles = append(newFiles, c.TransformVirtualFile(inputFile.TopId, inputFile.ID))
			}

			// 创建映射表，用于快速查找
			var (
				newFileMap = make(map[string]*models.VirtualFile)
				oldFileMap = make(map[string]*models.VirtualFile)
			)

			for _, item := range newFiles {
				key := item.CloudId
				newFileMap[key] = item
			}

			for _, item := range childrenFiles {
				key := item.CloudId
				oldFileMap[key] = item
			}

			var (
				filesToUpdateMap = map[int64][]utils.Field{}
				// strmUpdates 记录所有需要同步 STRM 的文件（仅非目录）
				strmUpdates   = make([]*updateStrmContext, 0)
				pid           = inputFile.ID
				filesToCreate = make([]*models.VirtualFile, 0)
				filesToDelete = make([]*models.VirtualFile, 0)
				// filesToRecurse 为需要继续向下遍历的"已存在"目录集合（去重）。
				// 普通刷新：仅当 Rev 变化时需要递归扫描该目录；
				// 深度刷新：无论 Rev 是否变化，都要递归扫描。
				filesToRecurse = make([]*models.VirtualFile, 0)
			)

			// 遍历扫描到的文件，找出新增和更新的文件
			for cloudId, newFile := range newFileMap {
				if oldFile, exists := oldFileMap[cloudId]; exists {
					// 文件存在，检查是否需要更新（通过Rev比较）
					if oldFile.Rev != newFile.Rev {
						ctx.Debug("文件存在差异 - rev changed",
							zap.Int64("parent_id", pid),
							zap.String("cloud_id", cloudId),
							zap.String("file_name", newFile.Name),
							zap.String("old_rev", oldFile.Rev),
							zap.String("new_rev", newFile.Rev))

						filesToUpdateMap[oldFile.ID] = append(make([]utils.Field, 0, 5),
							utils.WithField("name", utils.SanitizeFileName(newFile.Name)),
							utils.WithField("rev", newFile.Rev),
							utils.WithField("size", newFile.Size),
							utils.WithField("modify_date", newFile.ModifyDate),
							utils.WithField("hash", strings.ToLower(newFile.Hash)),
						)

						// 非目录文件的变更需要同步 STRM
						if !oldFile.IsDir {
							strmUpdates = append(strmUpdates, &updateStrmContext{
								oldFile: oldFile,
								newFile: newFile,
							})
						}

						// 发生变更的目录无论是否深度扫描都应继续向下递归
						if oldFile.IsDir {
							filesToRecurse = append(filesToRecurse, oldFile)
						}
					} else if oldFile.IsDir && req.Deep {
						// 深度扫描：Rev 未变的目录也要继续遍历
						filesToRecurse = append(filesToRecurse, oldFile)
					}
				} else {
					ctx.Debug("发现新文件",
						zap.Int64("parent_id", pid),
						zap.String("cloud_id", cloudId),
						zap.String("file_name", newFile.Name),
						zap.String("rev", newFile.Rev))
					// 文件不存在，需要新增
					newFile.ParentId = pid
					filesToCreate = append(filesToCreate, newFile)
				}
			}

			// 遍历数据库中的文件，找出需要删除的文件
			for cloudId, dbFile := range oldFileMap {
				if _, exists := newFileMap[cloudId]; !exists &&
					!dbFile.IsTop {
					ctx.Debug("文件不存在 - 删除",
						zap.Int64("parent_id", pid),
						zap.String("cloud_id", cloudId),
						zap.String("file_name", dbFile.Name),
						zap.Int64("file_id", dbFile.ID),
						zap.String("rev", dbFile.Rev))
					// 扫描结果中不存在该文件，需要删除

					filesToDelete = append(filesToDelete, dbFile)
				}
			}

			var (
				createCount = len(filesToCreate)
				deleteCount = len(filesToDelete)
				updateCount = len(filesToUpdateMap)
			)

			_ = h.fileTaskLogService.FlushCount(ctx, tracker,
				filetasklog.WithTotalCounter(createCount),
				filetasklog.WithTotalCounter(deleteCount),
				filetasklog.WithTotalCounter(updateCount),
			)

			var (
				errs []error
			)

			// 文件执行顺序 先删除后新增
			// 特殊情况：
			// 相同目录，example.mkv 待删除，example.mp4 待新增，此时已开启 strm 自动创建
			// 如果先增后删 会导致 example.mp4 对应的 strm 文件无法创建成功

			// 先删除文件
			if len(filesToDelete) > 0 {
				if err = h.batchDeleteFiles(ctx, filesToDelete); err != nil {
					ctx.Error("批量删除文件失败", zap.Error(err))

					errs = append(errs, err)
					_ = h.fileTaskLogService.FlushCount(ctx, tracker, filetasklog.WithFailedCounter(deleteCount))
				} else {
					_ = h.fileTaskLogService.FlushCount(ctx, tracker, filetasklog.WithCompletedCounter(deleteCount))
				}
			}

			// 新增文件时会有一个问题 一个目录底下有相同文件名的文件 会导致新增失败
			// 原来的解决办法：在新增文件时，直接忽略。但是会有一个问题，后面的扫描会一直显示这个文件不存在，会尝试创建，然后因为开启了 pid 和 name 的唯一索引，会创建失败
			// 现在的解决办法：如果重复就添加一个随机后缀
			// filesToCreate = lo.UniqBy(filesToCreate, func(item *models.VirtualFile) string {
			// 	return item.Name
			// })
			// 但是还有一个特殊情况，原本没有相同文件，后面同级目录添加一个重复的数据进来了，这个时候也会无限重复添加失败。 解决办法：在新增文件前查询
			// for _, file := range filesToCreate {
			// 	if _, exists := uniqFilesToCreateMap[file.Name]; exists {
			// 		uniqFilesToCreateMap[file.Name]++
			// 		file.Name = fmt.Sprintf("%s(%d)", file.Name, uniqFilesToCreateMap[file.Name])
			// 	} else {
			// 		uniqFilesToCreateMap[file.Name] = 1
			// 	}
			// }

			// 新增文件
			if len(filesToCreate) > 0 {
				if err = h.batchCreateFiles(ctx, pid, filesToCreate); err != nil {
					ctx.Error("批量创建文件失败", zap.Error(err))

					errs = append(errs, err)
					_ = h.fileTaskLogService.FlushCount(ctx, tracker, filetasklog.WithFailedCounter(createCount))
				} else {
					_ = h.fileTaskLogService.FlushCount(ctx, tracker, filetasklog.WithCompletedCounter(createCount))
				}
			}

			// 更新文件
			if len(filesToUpdateMap) > 0 {
				if err = h.batchUpdateFiles(ctx, filesToUpdateMap); err != nil {
					ctx.Error("批量更新文件失败", zap.Error(err))

					errs = append(errs, err)
					_ = h.fileTaskLogService.FlushCount(ctx, tracker, filetasklog.WithFailedCounter(updateCount))
				} else {
					_ = h.fileTaskLogService.FlushCount(ctx, tracker, filetasklog.WithCompletedCounter(updateCount))
					// 更新成功后同步 STRM，失败只打 warning 不影响扫描主流程
					h.syncStrmAfterUpdate(ctx, strmUpdates)
				}
			}

			if len(errs) > 0 {
				return nil, pkgErrors.New("文件处理失败")
			}

			// 组装下一轮遍历：
			// - 新创建的目录（Rev 初始不同，需要拉子级初始化）
			// - Rev 变化或深度模式下 Rev 未变的已存在目录（见上面 filesToRecurse 收集逻辑）
			createdDirFiles := lo.Filter(filesToCreate, func(item *models.VirtualFile, _ int) bool {
				return item.IsDir && item.ID > 0
			})

			nextWalkFiles = append(nextWalkFiles, createdDirFiles...)
			nextWalkFiles = append(nextWalkFiles, filesToRecurse...)

			// 按 ID 去重，避免同一目录被加入两次造成双遍历
			nextWalkFiles = lo.UniqBy(nextWalkFiles, func(item *models.VirtualFile) int64 {
				return item.ID
			})

			if len(nextWalkFiles) > 0 {
				ctx.Debug("继续执行下次遍历",
					zap.Bool("deep", req.Deep),
					zap.Int("next_walk_files_len", len(nextWalkFiles)),
				)
			}

			return nextWalkFiles, nil
		}); err != nil {
			ctx.GetContext().Error("执行时有错误", zap.Error(err))

			if apiErr, ok := apierrcode.As(err); ok {
				return apiErr
			}

			return err
		}

		return nil
	}
}

func (h *handler) ensureScanTaskMountPointOwner(ctx context.Context, req *topic.FileScanFileRequest, topFile *models.VirtualFile) error {
	if req == nil || topFile == nil || req.TriggeredByAdmin || req.ExpectedUserID <= 0 || req.FileId == 0 {
		return nil
	}

	topID := topFile.TopId
	if topID <= 0 {
		topID = topFile.ID
	}

	mountPoint, err := h.mountPointService.Query(ctx, topID)
	if err != nil {
		return err
	}

	if mountPoint.CreatorUserID != req.ExpectedUserID {
		return errScanTaskOwnerChanged
	}

	return nil
}
