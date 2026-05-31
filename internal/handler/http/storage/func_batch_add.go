package storage

import (
	stdContext "context"
	"encoding/json"
	"fmt"

	"github.com/xxcheng123/cloudpan189-share/internal/consts"
	"github.com/xxcheng123/cloudpan189-share/internal/framework/context"
	"github.com/xxcheng123/cloudpan189-share/internal/framework/httpcontext"
	"github.com/xxcheng123/cloudpan189-share/internal/pkgs/datatypes"
	storagefacadeSvi "github.com/xxcheng123/cloudpan189-share/internal/services/storagefacade"
	"github.com/xxcheng123/cloudpan189-share/internal/types/topic"
	"go.uber.org/zap"
)

type batchAddRequest struct {
	Items []addRequest `json:"items" binding:"required,min=1,max=500,dive"`
}

type batchAddResponse struct {
	SuccessCount    int               `json:"successCount"`
	FailCount       int               `json:"failCount"`
	ScanQueuedCount int               `json:"scanQueuedCount"`
	ScanFailedCount int               `json:"scanFailedCount"`
	Results         []addResponseItem `json:"results"`
}

type addResponseItem struct {
	LocalPath  string `json:"localPath"`
	ID         int64  `json:"id,omitempty"`
	Success    bool   `json:"success"`
	Error      string `json:"error,omitempty"`
	ScanQueued bool   `json:"scanQueued"`
	ScanError  string `json:"scanError,omitempty"`
}

type batchAddScanRef struct {
	fileID        int64
	resultIndexes []int
}

// BatchAdd 批量添加存储挂载
// @Summary 批量添加存储挂载
// @Description 批量添加存储挂载点（上限 500 个），每个条目都会走和单条 /add 相同的校验
// @Tags 存储管理
// @Accept json
// @Produce json
// @Param Authorization header string true "Bearer token"
// @Param request body batchAddRequest true "批量存储挂载信息"
// @Success 200 {object} httpcontext.Response{data=batchAddResponse} "批量添加结果"
// @Failure 400 {object} httpcontext.Response "参数验证失败"
// @Failure 401 {object} httpcontext.Response "未授权访问"
// @Failure 403 {object} httpcontext.Response "权限不足"
// @Router /api/storage/batch_add [post]
func (h *handler) BatchAdd() httpcontext.HandlerFunc {
	return func(ctx *httpcontext.Context) {
		req := new(batchAddRequest)
		if err := ctx.ShouldBindJSON(req); err != nil {
			ctx.GetContext().Error("batch_add JSON解析失败", zap.Error(err))
			ctx.AbortWithInvalidParams(err)

			return
		}

		if len(req.Items) == 0 {
			ctx.Fail(busCodeStorageQueryPathFailed.WithError(fmt.Errorf("items不能为空")))

			return
		}

		ctx.GetContext().Info("batch_add请求收到", zap.Int("items", len(req.Items)))

		// 获取当前用户ID
		userID := ctx.GetInt64(consts.CtxKeyUserId)
		isAdmin := ctx.GetBool(consts.CtxKeyIsAdmin)

		results := make([]addResponseItem, 0, len(req.Items))
		scanRefs := make([]batchAddScanRef, 0, len(req.Items))
		scanRefIndexes := make(map[int64]int)

		for i := range req.Items {
			item := req.Items[i]
			result := addResponseItem{LocalPath: item.LocalPath}

			existingMountPoint, err := h.mountPointService.QueryByPath(ctx.GetContext(), item.LocalPath)
			if err != nil {
				result.Error = err.Error()
				results = append(results, result)

				continue
			}

			if existingMountPoint != nil && !isAdmin && (userID <= 0 || existingMountPoint.CreatorUserID != userID) {
				result.Error = "路径已被其他用户挂载"
				results = append(results, result)

				continue
			}

			// 走和 Add 相同的校验链，而非简单拼 addition
			addition, fileId, err := h.buildBatchAddAddition(ctx.GetContext(), &item, userID, isAdmin)
			if err != nil {
				result.Error = err.Error()
				results = append(results, result)

				continue
			}

			id, createErr := h.storageFacadeService.CreateStorage(ctx.GetContext(), &storagefacadeSvi.CreateStorageRequest{
				LocalPath:         item.LocalPath,
				OsType:            item.OsType,
				CloudToken:        item.CloudToken,
				FileId:            fileId,
				Addition:          addition,
				EnableAutoRefresh: item.EnableAutoRefresh,
				AutoRefreshDays:   item.AutoRefreshDays,
				RefreshInterval:   item.RefreshInterval,
				EnableDeepRefresh: item.EnableDeepRefresh,
				CreatorUserID:     userID,
				IsAdmin:           isAdmin,
				AllowExisting:     true, // 批量场景下允许幂等
			})
			if createErr != nil {
				result.Error = createErr.Error()
				results = append(results, result)

				continue
			}

			result.ID = id
			result.Success = true

			results = append(results, result)
			resultIndex := len(results) - 1

			if scanRefIndex, ok := scanRefIndexes[id]; ok {
				scanRefs[scanRefIndex].resultIndexes = append(scanRefs[scanRefIndex].resultIndexes, resultIndex)

				continue
			}

			scanRefIndexes[id] = len(scanRefs)
			scanRefs = append(scanRefs, batchAddScanRef{
				fileID:        id,
				resultIndexes: []int{resultIndex},
			})
		}

		successCount := 0
		failCount := 0

		for _, r := range results {
			if r.Success {
				successCount++
			} else {
				failCount++
			}
		}

		scanQueuedCount := 0
		scanFailedCount := 0

		// 后台推送扫描任务，挂载创建结果和扫描派发结果分别统计。
		for _, ref := range scanRefs {
			mountPoint, err := h.mountPointService.Query(ctx.GetContext(), ref.fileID)
			if err != nil {
				scanFailedCount += len(ref.resultIndexes)
				h.markInitialScanFailed(ctx.GetContext(), ref.fileID, "准备失败", err)

				for _, resultIndex := range ref.resultIndexes {
					results[resultIndex].ScanError = err.Error()
				}

				continue
			}

			taskReq := &topic.FileScanFileRequest{
				FileId:           ref.fileID,
				Deep:             true,
				ExpectedUserID:   userID,
				TriggeredByAdmin: isAdmin,
			}

			body, err := json.Marshal(taskReq)
			if err != nil {
				ctx.GetContext().Warn("序列化扫描任务失败", zap.Int64("id", ref.fileID), zap.Error(err))

				scanFailedCount += len(ref.resultIndexes)
				h.markInitialScanFailed(ctx.GetContext(), ref.fileID, "序列化失败", err)

				for _, resultIndex := range ref.resultIndexes {
					results[resultIndex].ScanError = err.Error()
				}

				continue
			}

			bgCtx := context.NewContext(stdContext.Background())

			fullPath := mountPoint.FullPath
			if fullPath == "" {
				fullPath = mountPoint.Name
			}

			if err := h.taskEngine.PushMessage(
				bgCtx.WithValue(consts.CtxKeyFullPath, fullPath).
					WithValue(consts.CtxKeyInvokeHandlerName, "批量挂载扫描"),
				taskReq.Topic(), body,
			); err != nil {
				ctx.GetContext().Warn("推送批量挂载扫描任务失败", zap.Int64("id", ref.fileID), zap.Error(err))

				scanFailedCount += len(ref.resultIndexes)
				h.markInitialScanFailed(ctx.GetContext(), ref.fileID, "入队失败", err)

				for _, resultIndex := range ref.resultIndexes {
					results[resultIndex].ScanError = err.Error()
				}

				continue
			}

			scanQueuedCount += len(ref.resultIndexes)
			for _, resultIndex := range ref.resultIndexes {
				results[resultIndex].ScanQueued = true
			}
		}

		ctx.GetContext().Info("批量挂载已推送扫描任务",
			zap.Int("success", scanQueuedCount),
			zap.Int("failed", scanFailedCount))

		ctx.Success(&batchAddResponse{
			SuccessCount:    successCount,
			FailCount:       failCount,
			ScanQueuedCount: scanQueuedCount,
			ScanFailedCount: scanFailedCount,
			Results:         results,
		})
	}
}

// buildBatchAddAddition 按 osType 走与 /add 接口相同的校验链。
func (h *handler) buildBatchAddAddition(ctx context.Context, item *addRequest, userID int64, isAdmin bool) (datatypes.JSONMap, string, error) {
	fileId := item.FileId

	switch item.OsType {
	case protocolSubscribe:
		addition, busErr := h.executeOsTypeSubscribe(ctx, item)
		if busErr != nil {
			return nil, "", fmt.Errorf("%s", busErr.GetMessage())
		}

		return addition, fileId, nil
	case protocolSubscribeShare:
		addition, resolvedFileId, busErr := h.executeOsTypeSubscribeShare(ctx, item)
		if busErr != nil {
			return nil, "", fmt.Errorf("%s", busErr.GetMessage())
		}

		return addition, resolvedFileId, nil
	case protocolShare:
		addition, resolvedFileId, busErr := h.executeOsTypeShare(ctx, item)
		if busErr != nil {
			return nil, "", fmt.Errorf("%s", busErr.GetMessage())
		}

		return addition, resolvedFileId, nil
	case protocolPerson:
		if busErr := h.executeOsTypePersonal(ctx, item, userID, isAdmin); busErr != nil {
			return nil, "", fmt.Errorf("%s", busErr.GetMessage())
		}

		return datatypes.JSONMap{}, fileId, nil
	case protocolFamily:
		addition, busErr := h.executeOsTypeFamily(ctx, item, userID, isAdmin)
		if busErr != nil {
			return nil, "", fmt.Errorf("%s", busErr.GetMessage())
		}

		return addition, fileId, nil
	default:
		return nil, "", fmt.Errorf("不支持的挂载类型: %s", item.OsType)
	}
}
