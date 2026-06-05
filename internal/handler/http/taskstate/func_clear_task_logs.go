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

// ClearTaskLogs 清空文件任务日志
// @Summary 清空文件任务日志
// @Description 清空全部文件任务日志；传入 duration 时仅清理指定时长之前的日志
// @Tags 任务状态管理
// @Accept json
// @Produce json
// @Param Authorization header string true "Bearer token"
// @Param request body ClearTaskLogsRequest false "清理参数"
// @Success 200 {object} httpcontext.Response{data=int64} "清理成功，data 为删除数量"
// @Failure 400 {object} httpcontext.Response "参数验证失败，code=99998"
// @Failure 400 {object} httpcontext.Response "清空任务日志失败，code=xxxx"
// @Failure 401 {object} httpcontext.Response "未授权访问"
// @Failure 403 {object} httpcontext.Response "权限不足"
// @Router /api/task_state/file_log/clear [post]
func (h *handler) ClearTaskLogs() httpcontext.HandlerFunc {
	return func(ctx *httpcontext.Context) {
		req := new(ClearTaskLogsRequest)
		if err := ctx.ShouldBindJSON(req); err != nil && !errors.Is(err, io.EOF) {
			ctx.AbortWithInvalidParams(err)

			return
		}

		if !h.ensureFileTaskLogService(ctx, codeClearTaskLogsFailed) {
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
