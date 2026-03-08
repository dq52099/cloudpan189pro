package taskstate

import (
	"github.com/xxcheng123/cloudpan189-share/internal/framework/httpcontext"
)

type ClearTaskLogsRequest struct {
	Duration string `json:"duration" form:"duration"`
}

var codeClearTaskLogsFailed = bi.Next("清空任务日志失败")

func (h *handler) ClearTaskLogs() httpcontext.HandlerFunc {
	return func(ctx *httpcontext.Context) {
		req := new(ClearTaskLogsRequest)
		if err := ctx.ShouldBind(req); err != nil {
			req.Duration = ""
		}

		var err error
		if req.Duration == "" {
			err = h.fileTaskLogService.Clear(ctx.GetContext())
		} else {
			err = h.fileTaskLogService.ClearByDuration(ctx.GetContext(), req.Duration)
		}

		if err != nil {
			ctx.Fail(codeClearTaskLogsFailed.WithError(err))
			return
		}

		ctx.Success(nil)
	}
}
