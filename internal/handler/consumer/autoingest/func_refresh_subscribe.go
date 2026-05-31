package autoingest

import (
	"encoding/json"
	"errors"
	"fmt"
	"path"
	"strings"
	"sync"
	"time"

	mysqlDriver "github.com/go-sql-driver/mysql"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/xxcheng123/cloudpan189-share/internal/consts"
	"github.com/xxcheng123/cloudpan189-share/internal/framework/taskcontext"
	"github.com/xxcheng123/cloudpan189-share/internal/pkgs/datatypes"
	"github.com/xxcheng123/cloudpan189-share/internal/pkgs/utils"
	"github.com/xxcheng123/cloudpan189-share/internal/repository/models"
	autoingestplanSvi "github.com/xxcheng123/cloudpan189-share/internal/services/autoingestplan"
	"github.com/xxcheng123/cloudpan189-share/internal/types/autoingest"
	"github.com/xxcheng123/cloudpan189-share/internal/types/topic"
	"go.uber.org/zap"
	"gorm.io/gorm"

	cloudbridgeSvi "github.com/xxcheng123/cloudpan189-share/internal/services/cloudbridge"
	storagefacadeSvi "github.com/xxcheng123/cloudpan189-share/internal/services/storagefacade"
)

const (
	retryDelayBase            = 100 * time.Millisecond
	maxConcurrentItems        = 32
	maxPendingItemsPerRefresh = 1000
)

// isDuplicateEntryError 判断错误是否为"唯一约束冲突"。
// 兼容 SQLite / MySQL / PostgreSQL 的不同错误文本。
func isDuplicateEntryError(err error) bool {
	if err == nil {
		return false
	}

	var mysqlErr *mysqlDriver.MySQLError
	if errors.As(err, &mysqlErr) && mysqlErr.Number == 1062 {
		return true
	}

	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return true
	}

	type sqliteCodeError interface {
		Code() int
	}

	var sqliteErr sqliteCodeError
	if errors.As(err, &sqliteErr) {
		switch sqliteErr.Code() {
		case 1555, 2067: // SQLITE_CONSTRAINT_PRIMARYKEY / SQLITE_CONSTRAINT_UNIQUE
			return true
		}
	}

	msg := err.Error()

	return strings.Contains(msg, "UNIQUE constraint failed") ||
		strings.Contains(msg, "Duplicate entry") ||
		strings.Contains(msg, "duplicate key value violates unique constraint")
}

func buildConflictRenamePath(parentPath, name string, retry int) string {
	return path.Join(parentPath, fmt.Sprintf("%s_%d_%d", name, time.Now().UnixNano(), retry+1))
}

type subscribeOffsetCursor struct {
	Second     int64
	ResourceID string
}

func newSubscribeOffsetCursor(item *cloudbridgeSvi.ShareResourceInfo) subscribeOffsetCursor {
	if item == nil {
		return subscribeOffsetCursor{}
	}

	resourceID := item.ID
	if resourceID == "" {
		resourceID = fmt.Sprintf("%d:%s", item.ShareId, item.Name)
	}

	return subscribeOffsetCursor{
		Second:     item.ShareTime.Unix(),
		ResourceID: resourceID,
	}
}

func (c subscribeOffsetCursor) After(other subscribeOffsetCursor) bool {
	if c.Second != other.Second {
		return c.Second > other.Second
	}

	return c.ResourceID > other.ResourceID
}

func (h *handler) RefreshSubscribe() taskcontext.HandlerFunc {
	return func(ctx *taskcontext.Context) error {
		req := new(topic.AutoIngestRefreshSubscribeRequest)

		if err := ctx.Unmarshal(req); err != nil {
			return err
		}

		var (
			pageSize   = 100
			pageNum    = 1
			hasTop     = true
			shouldNext = true

			addCount    int64 = 0
			failedCount int64 = 0
		)

		logger := ctx.GetContext().Logger

		plan, err := h.autoIngestPlanService.Query(ctx.GetContext(), req.PlanId)
		if err != nil {
			logger.Error("查询入库计划信息失败", zap.Error(err), zap.Int64("plan_id", req.PlanId))

			return err
		}

		if !validateRefreshSubscribeOwnerSnapshot(logger, req, plan) {
			return nil
		}

		if plan.SourceType != autoingest.SourceTypeSubscribe {
			logger.Error("计划类型错误", zap.String("source_type", plan.SourceType.String()))

			return errors.New("计划类型错误")
		}

		addition := new(models.AutoIngestPlanSubscribeAddition)
		if err := plan.Addition.Unmarshal(addition); err != nil {
			logger.Error("解析订阅附加信息失败", zap.Error(err), zap.Int64("plan_id", plan.ID))

			return err
		}

		if resetOffset, resetApplied, err := h.resetRefreshSubscribeRetryState(ctx, req, addition); err != nil {
			logger.Error("重置自动入库重试状态失败", zap.Error(err), zap.Int64("plan_id", req.PlanId))

			return err
		} else if resetApplied {
			plan.Offset = resetOffset
			addition.OffsetResourceID = ""
		}

		if addition.UpUserId == "" {
			logger.Error("订阅附加信息缺少 UpUserId", zap.Int64("plan_id", plan.ID))

			return errors.New("订阅计划缺少 UpUserId")
		}

		concurrentCount := plan.ConcurrentCount
		if concurrentCount <= 0 {
			concurrentCount = 4
		}

		if concurrentCount > maxConcurrentItems {
			logger.Warn("订阅并发数超过上限，已截断",
				zap.Int("requested", concurrentCount),
				zap.Int("max", maxConcurrentItems),
			)
			concurrentCount = maxConcurrentItems
		}

		maxRetryCount := plan.MaxRetryCount
		if maxRetryCount <= 0 {
			maxRetryCount = 3
		}

		nextOffset := subscribeOffsetCursor{
			Second:     plan.Offset,
			ResourceID: addition.OffsetResourceID,
		}

		defer func() {
			if err := h.updateRefreshSubscribeOffset(ctx, req.PlanId, addition, nextOffset); err != nil {
				logger.Warn("更新入库计划 offset 失败", zap.Error(err), zap.Int64("plan_id", req.PlanId))
			}

			if err := h.autoIngestPlanService.IncrAddCount(ctx.GetContext(), req.PlanId, addCount); err != nil {
				logger.Warn("更新入库成功计数失败", zap.Error(err), zap.Int64("plan_id", req.PlanId))
			}

			if err := h.autoIngestPlanService.IncrFailedCount(ctx.GetContext(), req.PlanId, failedCount); err != nil {
				logger.Warn("更新入库失败计数失败", zap.Error(err), zap.Int64("plan_id", req.PlanId))
			}
		}()

		type pendingItem struct {
			item   *cloudbridgeSvi.ShareResourceInfo
			cursor subscribeOffsetCursor
		}

		var pendingItems []pendingItem

		appendPendingItem := func(item pendingItem) {
			pendingItems = append(pendingItems, item)
			if len(pendingItems) <= maxPendingItemsPerRefresh {
				return
			}

			dropIndex := 0
			for i := 1; i < len(pendingItems); i++ {
				if pendingItems[i].cursor.After(pendingItems[dropIndex].cursor) {
					dropIndex = i
				}
			}

			pendingItems = append(pendingItems[:dropIndex], pendingItems[dropIndex+1:]...)
		}

		for shouldNext {
			list, _, err := h.cloudbridgeService.GetSubscribeUserShareResource(ctx.GetContext(), addition.UpUserId, func(opt *cloudbridgeSvi.SubscribeUserShareResourceOption) {
				opt.PageNum = pageNum
				opt.PageSize = pageSize
			})
			if err != nil {
				logger.Error("获取订阅号内容时失败了~", zap.String("up_user_id", addition.UpUserId), zap.Int("page_num", pageNum), zap.Int("page_size", pageSize))

				return err
			}

			if len(list) == 0 {
				break
			}

			pageNum++

			for _, item := range list {
				if item.IsTop != 1 {
					hasTop = false
				}

				itemCursor := newSubscribeOffsetCursor(item)
				if !itemCursor.After(nextOffset) {
					if !hasTop && itemCursor.Second < nextOffset.Second {
						shouldNext = false
					}

					// 重试时扫描已存在目录
					if req.IsRetry {
						fullPath := path.Join(plan.ParentPath, item.Name)

						existingFile, err := h.virtualFileService.QueryByPath(ctx.GetContext(), fullPath)
						if err == nil && existingFile != nil {
							scanReq := newAutoIngestFileScanRequest(plan, existingFile.ID)

							scanBody, marshalErr := json.Marshal(scanReq)
							if marshalErr != nil {
								logger.Error("序列化已存在文件扫描任务失败", zap.Error(marshalErr))

								continue
							}

							if err = h.taskEngine.PushMessage(
								ctx.GetContext().
									WithValue(consts.CtxKeyFullPath, fullPath).
									WithValue(consts.CtxKeyInvokeHandlerName, "入库执行器"),
								scanReq.Topic(), scanBody); err != nil {
								logger.Error("下发已存在文件扫描任务失败", zap.Error(err))
							}
						}
					}

					continue
				}

				logger.Debug("发现新的待入库文件", zap.String("name", item.Name))

				appendPendingItem(pendingItem{
					item:   item,
					cursor: itemCursor,
				})
			}
		}

		if len(pendingItems) >= maxPendingItemsPerRefresh {
			logger.Warn("待处理项达到上限，本次先处理最靠近 offset 的一部分",
				zap.Int("max", maxPendingItemsPerRefresh),
			)
		}

		if len(pendingItems) == 0 {
			return nil
		}

		logger.Info("开始并发入库", zap.Int("total", len(pendingItems)), zap.Int("concurrent", concurrentCount))

		enqueueScanTask := func(fullPath string, fileID int64) error {
			taskReq := newAutoIngestFileScanRequest(plan, fileID)

			body, err := json.Marshal(taskReq)
			if err != nil {
				return err
			}

			return h.taskEngine.PushMessage(
				ctx.GetContext().
					WithValue(consts.CtxKeyFullPath, fullPath).
					WithValue(consts.CtxKeyInvokeHandlerName, "入库执行器"),
				taskReq.Topic(), body,
			)
		}

		recordFailureLog := func(fullPath string, message string, err error) {
			if h.autoIngestLogService == nil {
				return
			}

			logMessage := message
			if err != nil {
				logMessage = fmt.Sprintf("%s: %s", message, err.Error())
			}

			if _, logErr := h.autoIngestLogService.Create(ctx.GetContext(),
				req.PlanId, autoingest.LogLevelError,
				fmt.Sprintf("新增入库失败：%s, 错误信息：%s", fullPath, logMessage),
			); logErr != nil {
				logger.Error("创建入库日志失败", zap.Error(logErr))
			}
		}

		var (
			wg               sync.WaitGroup
			itemChan         = make(chan pendingItem, len(pendingItems))
			mu               sync.Mutex
			localAddCount    int64
			localFailedCount int64
			localMaxOffset   = nextOffset
		)

		for _, item := range pendingItems {
			itemChan <- item
		}

		close(itemChan)

		for i := 0; i < concurrentCount; i++ {
			wg.Add(1)
			go func(workerId int) {
				defer wg.Done()

				for pItem := range itemChan {
					fullPath := path.Join(plan.ParentPath, pItem.item.Name)
					handled := false
					failedRecorded := false
					buildStorageRequest := func(targetPath string) *storagefacadeSvi.CreateStorageRequest {
						return &storagefacadeSvi.CreateStorageRequest{
							LocalPath:  targetPath,
							OsType:     models.OsTypeSubscribeShareFolder,
							CloudToken: plan.TokenId,
							FileId:     pItem.item.ID,
							Addition: datatypes.JSONMap{
								consts.FileAdditionKeyUpUserId: addition.UpUserId,
								consts.FileAdditionKeyShareId:  pItem.item.ShareId,
								consts.FileAdditionKeyIsFolder: pItem.item.IsFolder,
							},
							EnableAutoRefresh: plan.RefreshStrategy.EnableAutoRefresh,
							EnableDeepRefresh: plan.RefreshStrategy.EnableDeepRefresh,
							AutoRefreshDays:   plan.RefreshStrategy.AutoRefreshDays,
							RefreshInterval:   plan.RefreshStrategy.RefreshInterval,
							CreatorUserID:     plan.UserID,
							// 自动入库允许路径已存在（上面已经检查过 QueryByPath，这里容忍竞态）
							AllowExisting: true,
						}
					}

					for retry := 0; retry <= maxRetryCount; retry++ {
						if retry > 0 {
							time.Sleep(retryDelayBase * time.Duration(retry))
						}

						exists, err := h.virtualFileService.QueryByPath(ctx.GetContext(), fullPath)
						if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
							logger.Error("查询虚拟文件路径失败 跳过本次自动入库", zap.String("path", fullPath), zap.Error(err))
							mu.Lock()
							localFailedCount++
							mu.Unlock()

							failedRecorded = true

							recordFailureLog(fullPath, "查询虚拟文件路径失败", err)

							break
						}

						if exists != nil {
							if exists.CloudId == pItem.item.ID {
								id, err := h.storageFacadeService.CreateStorage(ctx.GetContext(), buildStorageRequest(fullPath))
								if err != nil {
									if errors.Is(err, storagefacadeSvi.ErrPathAlreadyExists) || errors.Is(err, storagefacadeSvi.ErrExistingPathForbidden) {
										if plan.OnConflict == autoingest.OnConflictRename {
											fullPath = buildConflictRenamePath(plan.ParentPath, pItem.item.Name, retry)

											continue
										}

										logger.Debug("已存在同云端资源但挂载不可复用，按放弃策略跳过", zap.String("path", fullPath), zap.Error(err))

										handled = true

										break
									}

									if retry < maxRetryCount {
										logger.Warn("校验已存在文件挂载失败，正在重试", zap.String("path", fullPath), zap.Error(err), zap.Int("retry", retry+1))

										continue
									}

									logger.Error("校验已存在文件挂载失败", zap.Error(err), zap.String("path", fullPath))
									mu.Lock()
									localFailedCount++
									mu.Unlock()

									failedRecorded = true

									recordFailureLog(fullPath, "校验已存在文件挂载失败", err)

									break
								}

								logger.Debug("文件已入库，跳过重复资源", zap.String("path", fullPath), zap.String("cloud_id", pItem.item.ID))

								if req.IsRetry {
									if err = enqueueScanTask(fullPath, id); err != nil {
										logger.Error("下发已存在文件扫描任务失败", zap.Error(err))
										mu.Lock()
										localFailedCount++
										mu.Unlock()

										failedRecorded = true

										recordFailureLog(fullPath, "下发已存在文件扫描任务失败", err)

										break
									}
								}

								handled = true

								break
							}

							if plan.OnConflict == autoingest.OnConflictRename {
								fullPath = buildConflictRenamePath(plan.ParentPath, pItem.item.Name, retry)

								continue
							}
							// abandon: 跳过，继续处理下一个文件
							logger.Debug("文件已存在，跳过入库", zap.String("path", fullPath))

							handled = true

							break
						}

						id, err := h.storageFacadeService.CreateStorage(ctx.GetContext(),
							buildStorageRequest(fullPath),
						)
						if err != nil {
							if errors.Is(err, storagefacadeSvi.ErrPathAlreadyExists) || errors.Is(err, storagefacadeSvi.ErrExistingPathForbidden) {
								if plan.OnConflict == autoingest.OnConflictRename {
									fullPath = buildConflictRenamePath(plan.ParentPath, pItem.item.Name, retry)

									continue
								}

								logger.Debug("入库时目标路径已存在，按放弃策略跳过", zap.String("path", fullPath), zap.Error(err))

								handled = true

								break
							}

							if isDuplicateEntryError(err) {
								exists, queryErr := h.virtualFileService.QueryByPath(ctx.GetContext(), fullPath)
								if queryErr != nil {
									if errors.Is(queryErr, gorm.ErrRecordNotFound) && retry < maxRetryCount {
										logger.Warn("入库遇到唯一约束冲突但未查询到目标路径，正在重试",
											zap.String("path", fullPath),
											zap.Error(err),
											zap.Int("retry", retry+1),
										)

										continue
									}

									logger.Error("入库遇到唯一约束冲突后查询目标路径失败", zap.String("path", fullPath), zap.Error(queryErr))
									mu.Lock()
									localFailedCount++
									mu.Unlock()

									failedRecorded = true

									recordFailureLog(fullPath, "唯一约束冲突后查询目标路径失败", queryErr)

									break
								}

								if exists != nil && exists.CloudId == pItem.item.ID {
									logger.Debug("入库竞态后发现资源已存在，补发扫描任务", zap.String("path", fullPath), zap.String("cloud_id", pItem.item.ID))

									if err = enqueueScanTask(fullPath, exists.ID); err != nil {
										logger.Error("下发已存在文件扫描任务失败", zap.Error(err))
										mu.Lock()
										localFailedCount++
										mu.Unlock()

										failedRecorded = true

										recordFailureLog(fullPath, "下发已存在文件扫描任务失败", err)

										break
									}

									handled = true

									break
								}

								if exists != nil && plan.OnConflict == autoingest.OnConflictRename {
									fullPath = buildConflictRenamePath(plan.ParentPath, pItem.item.Name, retry)

									continue
								}

								if exists != nil {
									logger.Debug("入库竞态后目标路径已存在，按放弃策略跳过", zap.String("path", fullPath))

									handled = true

									break
								}

								logger.Error("入库遇到唯一约束冲突但目标路径不存在", zap.String("path", fullPath), zap.Error(err))
								mu.Lock()
								localFailedCount++
								mu.Unlock()

								failedRecorded = true

								recordFailureLog(fullPath, "唯一约束冲突但目标路径不存在", err)

								break
							}

							if retry < maxRetryCount {
								logger.Warn("入库失败，正在重试", zap.String("path", fullPath), zap.Error(err), zap.Int("retry", retry+1))

								continue
							}

							logger.Error("入库失败", zap.Error(err), zap.String("path", fullPath))
							mu.Lock()
							localFailedCount++
							mu.Unlock()

							failedRecorded = true

							recordFailureLog(fullPath, "入库失败", err)

							break
						}

						taskReq := newAutoIngestFileScanRequest(plan, id)

						body, err := json.Marshal(taskReq)
						if err != nil {
							logger.Error("序列化文件扫描任务失败", zap.Error(err))

							mu.Lock()
							localFailedCount++
							mu.Unlock()

							failedRecorded = true

							recordFailureLog(fullPath, "序列化文件扫描任务失败", err)

							break
						}

						if err = h.taskEngine.PushMessage(
							ctx.GetContext().
								WithValue(consts.CtxKeyFullPath, fullPath).
								WithValue(consts.CtxKeyInvokeHandlerName, "入库执行器"),
							taskReq.Topic(), body); err != nil {
							logger.Error("下发文件扫描任务失败", zap.Error(err))

							mu.Lock()
							localFailedCount++
							mu.Unlock()

							failedRecorded = true

							recordFailureLog(fullPath, "下发文件扫描任务失败", err)

							break
						}

						if _, logErr := h.autoIngestLogService.Create(ctx.GetContext(),
							req.PlanId, autoingest.LogLevelInfo,
							fmt.Sprintf("新增入库：%s", fullPath),
						); logErr != nil {
							logger.Error("创建入库日志失败", zap.Error(logErr))
						}

						mu.Lock()
						localAddCount++
						mu.Unlock()

						handled = true

						break
					}

					if !handled && !failedRecorded {
						logger.Error("入库失败，无法生成不冲突的目标路径", zap.String("path", fullPath))
						mu.Lock()
						localFailedCount++
						mu.Unlock()

						if _, logErr := h.autoIngestLogService.Create(ctx.GetContext(),
							req.PlanId, autoingest.LogLevelError,
							fmt.Sprintf("新增入库失败：%s, 错误信息：无法生成不冲突的目标路径", fullPath),
						); logErr != nil {
							logger.Error("创建入库日志失败", zap.Error(logErr))
						}
					}

					if handled {
						mu.Lock()
						if pItem.cursor.After(localMaxOffset) {
							localMaxOffset = pItem.cursor
						}
						mu.Unlock()
					}
				}
			}(i)
		}

		wg.Wait()

		addCount = localAddCount

		failedCount = localFailedCount
		if failedCount == 0 {
			nextOffset = localMaxOffset
		}

		logger.Info("并发入库完成", zap.Int64("add_count", addCount), zap.Int64("failed_count", failedCount))

		return nil
	}
}

func newAutoIngestFileScanRequest(plan *models.AutoIngestPlan, fileID int64) *topic.FileScanFileRequest {
	req := &topic.FileScanFileRequest{
		FileId: fileID,
		Deep:   true,
	}

	if plan != nil && plan.UserID > 0 {
		req.ExpectedUserID = plan.UserID
	}

	return req
}

func validateRefreshSubscribeOwnerSnapshot(logger *zap.Logger, req *topic.AutoIngestRefreshSubscribeRequest, plan *models.AutoIngestPlan) bool {
	if req.ExpectedUserID <= 0 || req.TriggeredByAdmin {
		return true
	}

	if plan.UserID == req.ExpectedUserID {
		return true
	}

	logger.Warn("自动入库任务归属已变化，跳过手动触发任务",
		zap.Int64("plan_id", req.PlanId),
		zap.Int64("expected_user_id", req.ExpectedUserID),
		zap.Int64("current_user_id", plan.UserID),
	)

	return false
}

func (h *handler) updateRefreshSubscribeOffset(
	ctx *taskcontext.Context,
	planID int64,
	addition *models.AutoIngestPlanSubscribeAddition,
	cursor subscribeOffsetCursor,
) error {
	if addition != nil {
		addition.OffsetResourceID = cursor.ResourceID
	}

	fields := []utils.Field{utils.WithField("offset", cursor.Second)}
	if addition != nil {
		fields = append(fields, utils.WithField("addition", addition.JSONMap()))
	}

	return h.autoIngestPlanService.Update(ctx.GetContext(), planID, fields...)
}

func (h *handler) resetRefreshSubscribeRetryState(
	ctx *taskcontext.Context,
	req *topic.AutoIngestRefreshSubscribeRequest,
	addition *models.AutoIngestPlanSubscribeAddition,
) (int64, bool, error) {
	if req.RetryReset == nil {
		return 0, false, nil
	}

	fields := []utils.Field{utils.WithField("offset", req.RetryReset.Offset)}

	if addition != nil {
		addition.OffsetResourceID = ""
		fields = append(fields, utils.WithField("addition", addition.JSONMap()))
	}

	if req.RetryReset.ResetCounters {
		fields = append(fields,
			utils.WithField("add_count", int64(0)),
			utils.WithField("failed_count", int64(0)),
		)
	}

	if err := h.autoIngestPlanService.UpdateByOwner(ctx.GetContext(), &autoingestplanSvi.UpdateRequest{
		ID:      req.PlanId,
		UserID:  req.ExpectedUserID,
		IsAdmin: req.TriggeredByAdmin,
	}, fields...); err != nil {
		return 0, false, err
	}

	return req.RetryReset.Offset, true, nil
}
