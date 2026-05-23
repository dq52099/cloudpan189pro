package media

import (
	"encoding/json"
	"errors"
	"io"

	"github.com/xxcheng123/cloudpan189-share/internal/consts"
	"github.com/xxcheng123/cloudpan189-share/internal/framework/httpcontext"
	mountpointSvi "github.com/xxcheng123/cloudpan189-share/internal/services/mountpoint"
	"github.com/xxcheng123/cloudpan189-share/internal/types/topic"
	"go.uber.org/zap"
)

type rebuildStrmRequest struct {
	// MountPointIDs 可选；未传时重建全部，显式空数组不派发任务。
	MountPointIDs *[]int64 `json:"mountPointIds,omitempty" example:"[1001,1002]"`
}

type rebuildStrmResponse struct {
	Total   int `json:"total"`
	Success int `json:"success"`
	Failed  int `json:"failed"`
}

// RebuildStrmFile 重建strm文件
// @Summary 重建strm文件
// @Description 重建 STRM 文件。不传 mountPointIds 时走全量重建（消费侧内部并发）；
// @Description 传入 mountPointIds 时按单挂载点派发多条任务，便于针对性重试。
// @Tags 媒体操作
// @Accept json
// @Produce json
// @Param Authorization header string true "Bearer token"
// @Param request body rebuildStrmRequest false "重建参数"
// @Success 200 {object} httpcontext.Response{data=rebuildStrmResponse} "重建任务已提交"
// @Failure 400 {object} httpcontext.Response "媒体功能未启用"
// @Failure 400 {object} httpcontext.Response "提交重建任务失败"
// @Failure 401 {object} httpcontext.Response "未授权访问"
// @Failure 403 {object} httpcontext.Response "权限不足"
// @Router /api/media/rebuild_strm_file [post]
func (h *handler) RebuildStrmFile() httpcontext.HandlerFunc {
	return func(ctx *httpcontext.Context) {
		req := new(rebuildStrmRequest)
		if err := ctx.ShouldBindJSON(req); err != nil && !errors.Is(err, io.EOF) {
			ctx.AbortWithInvalidParams(err)

			return
		}

		// 检查媒体功能是否启用
		cfg, err := h.mediaConfigService.Query(ctx.GetContext())
		if err != nil {
			ctx.Fail(codeMediaNotEnabled.WithError(err))

			return
		}

		if !cfg.Enable {
			ctx.Fail(codeMediaNotEnabled)

			return
		}

		// 未指定挂载点：下发全量重建任务（消费侧自带挂载点级并发）
		if req.MountPointIDs == nil {
			fullReq := &topic.MediaRebuildStrmFileRequest{}

			body, err := json.Marshal(fullReq)
			if err != nil {
				ctx.GetContext().Error("序列化 STRM 全量重建任务失败", zap.Error(err))
				ctx.Fail(codeRebuildFailed.WithError(err))

				return
			}

			if err := h.taskEngine.PushMessage(
				ctx.GetContext().
					WithValue(consts.CtxKeyInvokeHandlerName, "STRM 重建"),
				fullReq.Topic(), body,
			); err != nil {
				ctx.GetContext().Error("推送 STRM 全量重建任务失败", zap.Error(err))
				ctx.Fail(codeRebuildFailed.WithError(err))

				return
			}

			ctx.Success(rebuildStrmResponse{Total: 1, Success: 1, Failed: 0})

			return
		}

		requestedIDs, err := normalizeRebuildMountPointIDs(*req.MountPointIDs)
		if err != nil {
			ctx.AbortWithInvalidParams(err)

			return
		}

		// 指定了挂载点：逐个派发单挂载点重建任务
		mountpoints, err := h.mountpointService.List(ctx.GetContext(), &mountpointSvi.ListRequest{
			NoPaginate: true,
			IsAdmin:    true,
		})
		if err != nil {
			ctx.Fail(codeRebuildFailed.WithError(err))

			return
		}

		idSet := make(map[int64]struct{}, len(requestedIDs))
		for _, id := range requestedIDs {
			idSet[id] = struct{}{}
		}

		successCount := 0
		matchedCount := 0
		totalRequested := len(idSet)

		for _, mp := range mountpoints {
			if _, ok := idSet[mp.ID]; !ok {
				continue
			}

			matchedCount++

			taskReq := &topic.MediaRebuildStrmFileByMountPointRequest{
				MountPointFileId: mp.FileId,
				MountPointPath:   mp.FullPath,
			}

			body, err := json.Marshal(taskReq)
			if err != nil {
				ctx.GetContext().Warn("序列化 STRM 单点重建任务失败",
					zap.Int64("mount_point_id", mp.ID),
					zap.Int64("file_id", mp.FileId),
					zap.Error(err))

				continue
			}

			if err := h.taskEngine.PushMessage(
				ctx.GetContext().
					WithValue(consts.CtxKeyFullPath, mp.FullPath).
					WithValue(consts.CtxKeyInvokeHandlerName, "STRM重建"),
				taskReq.Topic(), body,
			); err != nil {
				ctx.GetContext().Warn("推送 STRM 单点重建任务失败",
					zap.Int64("mount_point_id", mp.ID),
					zap.Int64("file_id", mp.FileId),
					zap.Error(err))

				continue
			}

			successCount++
		}

		if totalRequested > 0 && matchedCount == 0 {
			ctx.Fail(codeRebuildMountPointNotFound)

			return
		}

		ctx.Success(rebuildStrmResponse{
			Total:   totalRequested,
			Success: successCount,
			Failed:  totalRequested - successCount,
		})
	}
}

func normalizeRebuildMountPointIDs(ids []int64) ([]int64, error) {
	seen := make(map[int64]struct{}, len(ids))
	normalized := make([]int64, 0, len(ids))

	for _, id := range ids {
		if id <= 0 {
			return nil, errors.New("mountPointIds 必须全部大于 0")
		}

		if _, ok := seen[id]; ok {
			continue
		}

		seen[id] = struct{}{}
		normalized = append(normalized, id)
	}

	return normalized, nil
}
