package taskstate

import (
	"errors"
	"io"

	"github.com/xxcheng123/cloudpan189-share/internal/framework/httpcontext"
)

type ClearTaskLogsRequest struct {
	Duration string `json:"duration" form:"duration"`
}

var codeClearTaskLogsFailed = bi.Next("清空任务日志失败")

func (h *handler) ClearTaskLogs() httpcontext.HandlerFunc {
	return func(ctx *httpcontext.Context) {
		req := new(ClearTaskLogsRequest)
		if err := ctx.ShouldBindJSON(req); err != nil && !errors.Is(err, io.EOF) {
			ctx.AbortWithInvalidParams(err)

			return
		}

		var (
			count int64
			err   error
		)
		if req.Duration == "" {
			count, err = h.fileTaskLogService.Clear(ctx.GetContext())
		} else {
			count, err = h.fileTaskLogService.ClearByDuration(ctx.GetContext(), req.Duration)
		}

		if err != nil {
			ctx.Fail(codeClearTaskLogsFailed.WithError(err))

			return
		}

		ctx.Success(count)
	}
}
