package media

import (
	"encoding/json"

	"github.com/xxcheng123/cloudpan189-share/internal/consts"
	"github.com/xxcheng123/cloudpan189-share/internal/framework/httpcontext"
	"github.com/xxcheng123/cloudpan189-share/internal/types/topic"
	"go.uber.org/zap"
)

// Clear 清理媒体文件
// @Summary 清理媒体文件
// @Description 清理媒体存储路径下的所有媒体文件
// @Tags 媒体操作
// @Accept json
// @Produce json
// @Param Authorization header string true "Bearer token"
// @Success 200 {object} httpcontext.Response "清理任务已提交"
// @Failure 400 {object} httpcontext.Response "媒体功能未启用，code=xxxx"
// @Failure 400 {object} httpcontext.Response "提交清理任务失败，code=xxxx"
// @Failure 401 {object} httpcontext.Response "未授权访问"
// @Failure 403 {object} httpcontext.Response "权限不足"
// @Router /api/media/clear [post]
func (h *handler) Clear() httpcontext.HandlerFunc {
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

		// 推送清理任务到消息队列
		taskReq := &topic.MediaClearRequest{}

		body, err := json.Marshal(taskReq)
		if err != nil {
			ctx.GetContext().Error("序列化媒体清理任务失败", zap.Error(err))
			ctx.Fail(codeClearFailed.WithError(err))

			return
		}

		if err = h.taskEngine.PushMessage(
			ctx.GetContext().
				WithValue(consts.CtxKeyInvokeHandlerName, "媒体清理执行器"),
			taskReq.Topic(), body); err != nil {
			ctx.Fail(codeClearFailed.WithError(err))

			return
		}

		ctx.Success()
	}
}
