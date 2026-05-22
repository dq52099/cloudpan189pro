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
	"github.com/xxcheng123/cloudpan189-share/internal/repository/models"
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

		nextOffset := plan.Offset

		if plan.SourceType != autoingest.SourceTypeSubscribe {
			logger.Error("计划类型错误", zap.String("source_type", plan.SourceType.String()))

			return errors.New("计划类型错误")
		}

		addition := new(models.AutoIngestPlanSubscribeAddition)
		if err := plan.Addition.Unmarshal(addition); err != nil {
			logger.Error("解析订阅附加信息失败", zap.Error(err), zap.Int64("plan_id", plan.ID))

			return err
		}

		if addition.UpUserId == "" {
			logger.Error("订阅附加信息缺少 UpUserId", zap.Int64("plan_id", plan.ID))

			return errors.New("订阅计划缺少 UpUserId")
		}

		defer func() {
			if err := h.autoIngestPlanService.UpdateOffset(ctx.GetContext(), req.PlanId, nextOffset); err != nil {
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
			item       *cloudbridgeSvi.ShareResourceInfo
			itemOffset int64
		}

		var pendingItems []pendingItem

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

				itemOffset := item.ShareTime.Unix()
				if itemOffset <= plan.Offset {
					if !hasTop {
						shouldNext = false
					}

					// 重试时扫描已存在目录
					if req.IsRetry {
						fullPath := path.Join(plan.ParentPath, item.Name)

						existingFile, err := h.virtualFileService.QueryByPath(ctx.GetContext(), fullPath)
						if err == nil && existingFile != nil {
							scanReq := &topic.FileScanFileRequest{
								FileId: existingFile.ID,
								Deep:   true,
							}

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

				pendingItems = append(pendingItems, pendingItem{
					item:       item,
					itemOffset: itemOffset,
				})

				// 防止单次任务堆积过多对象占用内存
				if len(pendingItems) >= maxPendingItemsPerRefresh {
					logger.Warn("待处理项达到上限，本次先处理一部分",
						zap.Int("max", maxPendingItemsPerRefresh),
					)

					shouldNext = false

					break
				}
			}
		}

		if len(pendingItems) == 0 {
			return nil
		}

		logger.Info("开始并发入库", zap.Int("total", len(pendingItems)), zap.Int("concurrent", concurrentCount))

		enqueueScanTask := func(fullPath string, fileID int64) error {
			taskReq := &topic.FileScanFileRequest{
				FileId: fileID,
				Deep:   true,
			}

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

		var (
			wg               sync.WaitGroup
			itemChan         = make(chan pendingItem, len(pendingItems))
			mu               sync.Mutex
			localAddCount    int64
			localFailedCount int64
			localMaxOffset   = plan.Offset
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

							break
						}

						if exists != nil {
							if exists.CloudId == pItem.item.ID {
								logger.Debug("文件已入库，跳过重复资源", zap.String("path", fullPath), zap.String("cloud_id", pItem.item.ID))

								if err = enqueueScanTask(fullPath, exists.ID); err != nil {
									logger.Error("下发已存在文件扫描任务失败", zap.Error(err))
									mu.Lock()
									localFailedCount++
									mu.Unlock()

									failedRecorded = true

									break
								}

								handled = true

								break
							}

							if plan.OnConflict == autoingest.OnConflictRename {
								fullPath = path.Join(plan.ParentPath, fmt.Sprintf("%s_%d", pItem.item.Name, time.Now().Unix()))

								continue
							}
							// abandon: 跳过，继续处理下一个文件
							logger.Debug("文件已存在，跳过入库", zap.String("path", fullPath))

							handled = true

							break
						}

						id, err := h.storageFacadeService.CreateStorage(ctx.GetContext(),
							&storagefacadeSvi.CreateStorageRequest{
								LocalPath:  fullPath,
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
							},
						)
						if err != nil {
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

										break
									}

									handled = true

									break
								}

								if exists != nil && plan.OnConflict == autoingest.OnConflictRename {
									fullPath = path.Join(plan.ParentPath, fmt.Sprintf("%s_%d", pItem.item.Name, time.Now().Unix()))

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

							if _, logErr := h.autoIngestLogService.Create(ctx.GetContext(),
								req.PlanId, autoingest.LogLevelError,
								fmt.Sprintf("新增入库失败：%s, 错误信息：%s", fullPath, err.Error()),
							); logErr != nil {
								logger.Error("创建入库日志失败", zap.Error(logErr))
							}

							break
						}

						taskReq := &topic.FileScanFileRequest{
							FileId: id,
							Deep:   true,
						}

						body, err := json.Marshal(taskReq)
						if err != nil {
							logger.Error("序列化文件扫描任务失败", zap.Error(err))

							mu.Lock()
							localFailedCount++
							mu.Unlock()

							failedRecorded = true

							if _, logErr := h.autoIngestLogService.Create(ctx.GetContext(),
								req.PlanId, autoingest.LogLevelError,
								fmt.Sprintf("新增入库失败：%s, 错误信息：序列化文件扫描任务失败: %s", fullPath, err.Error()),
							); logErr != nil {
								logger.Error("创建入库日志失败", zap.Error(logErr))
							}

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

							if _, logErr := h.autoIngestLogService.Create(ctx.GetContext(),
								req.PlanId, autoingest.LogLevelError,
								fmt.Sprintf("新增入库失败：%s, 错误信息：下发文件扫描任务失败: %s", fullPath, err.Error()),
							); logErr != nil {
								logger.Error("创建入库日志失败", zap.Error(logErr))
							}

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
						if pItem.itemOffset > localMaxOffset {
							localMaxOffset = pItem.itemOffset
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
