package media

import (
	"encoding/json"

	"github.com/xxcheng123/cloudpan189-share/internal/consts"
	"github.com/xxcheng123/cloudpan189-share/internal/framework/httpcontext"
	mountpointSvi "github.com/xxcheng123/cloudpan189-share/internal/services/mountpoint"
	"github.com/xxcheng123/cloudpan189-share/internal/types/topic"
	"go.uber.org/zap"
)

type rebuildStrmResponse struct {
	Total   int `json:"total"`
	Success int `json:"success"`
	Failed  int `json:"failed"`
}

// RebuildStrmFile 重建strm文件
// @Summary 重建strm文件
// @Description 为每个挂载点创建独立的STRM重建任务，由工作流并发处理
// @Tags 媒体操作
// @Accept json
// @Produce json
// @Param Authorization header string true "Bearer token"
// @Success 200 {object} httpcontext.Response "重建任务已提交"
// @Failure 400 {object} httpcontext.Response "媒体功能未启用，code=xxxx"
// @Failure 400 {object} httpcontext.Response "提交重建任务失败，code=xxxx"
// @Failure 401 {object} httpcontext.Response "未授权访问"
// @Failure 403 {object} httpcontext.Response "权限不足"
// @Router /api/media/rebuild_strm_file [post]
func (h *handler) RebuildStrmFile() httpcontext.HandlerFunc {
	return func(ctx *httpcontext.Context) {
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

		// 获取所有挂载点
		mountpoints, err := h.mountpointService.List(ctx.GetContext(), &mountpointSvi.ListRequest{
			NoPaginate: true,
		})
		if err != nil {
			ctx.Fail(codeRebuildFailed.WithError(err))
			return
		}

		if len(mountpoints) == 0 {
			ctx.Success("没有挂载点，无需重建")
			return
		}

		// 为每个挂载点创建独立任务
		successCount := 0
		for _, mp := range mountpoints {
			taskReq := &topic.MediaRebuildStrmFileByMountPointRequest{
				MountPointFileId: mp.FileId,
				MountPointPath:   mp.FullPath,
			}
			body, _ := json.Marshal(taskReq)
			if err = h.taskEngine.PushMessage(
				ctx.GetContext().
					WithValue(consts.CtxKeyFullPath, mp.FullPath).
					WithValue(consts.CtxKeyInvokeHandlerName, "STRM重建"),
				taskReq.Topic(), body); err != nil {
				ctx.GetContext().Error("推送STRM重建任务失败", zap.Int64("file_id", mp.FileId))
				continue
			}
			successCount++
		}

		ctx.Success(rebuildStrmResponse{
			Total:   len(mountpoints),
			Success: successCount,
			Failed:  len(mountpoints) - successCount,
		})
	}
}
