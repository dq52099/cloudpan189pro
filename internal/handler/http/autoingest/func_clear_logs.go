package autoingest

import (
	"errors"
	"io"

	"github.com/xxcheng123/cloudpan189-share/internal/framework/httpcontext"
)

type ClearLogsRequest struct {
	Duration string `json:"duration" form:"duration"`
}

// ClearLogs 清理日志
// @Summary 清理日志
// @Description 清理自动入库运行日志；duration 为空时清空全部，否则按保留时长清理
// @Tags 自动挂载管理
// @Accept json
// @Produce json
// @Param Authorization header string true "Bearer token"
// @Param body body ClearLogsRequest false "清理参数"
// @Success 200 {object} httpcontext.Response "清空成功"
// @Failure 401 {object} httpcontext.Response "未授权访问"
// @Router /api/auto_ingest/log/clear [post]
func (h *handler) ClearLogs() httpcontext.HandlerFunc {
	return func(ctx *httpcontext.Context) {
		req := new(ClearLogsRequest)
		if err := ctx.ShouldBindJSON(req); err != nil && !errors.Is(err, io.EOF) {
			ctx.AbortWithInvalidParams(err)

			return
		}

		if !h.ensureLogService(ctx, codeLogDeleteFailed) {
			return
		}

		var (
			count int64
			err   error
		)
		if req.Duration == "" {
			count, err = h.logService.Clear(ctx.GetContext())
		} else {
			count, err = h.logService.ClearByDuration(ctx.GetContext(), req.Duration)
		}

		if err != nil {
			ctx.Fail(codeLogDeleteFailed.WithError(err))

			return
		}

		ctx.Success(count)
	}
}
