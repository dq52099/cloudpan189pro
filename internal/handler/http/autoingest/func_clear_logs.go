package autoingest

import (
	"github.com/xxcheng123/cloudpan189-share/internal/framework/httpcontext"
)

// ClearLogs 清空所有日志
// @Summary 清空所有日志
// @Description 清空自动入库的所有运行日志
// @Tags 自动挂载管理
// @Accept json
// @Produce json
// @Param Authorization header string true "Bearer token"
// @Success 200 {object} httpcontext.Response "清空成功"
// @Failure 401 {object} httpcontext.Response "未授权访问"
// @Router /api/auto_ingest/log/clear [post]
func (h *handler) ClearLogs() httpcontext.HandlerFunc {
	return func(ctx *httpcontext.Context) {
		count, err := h.logService.Clear(ctx.GetContext())
		if err != nil {
			ctx.Fail(codeLogDeleteFailed.WithError(err))
			return
		}

		ctx.Success(count)
	}
}
