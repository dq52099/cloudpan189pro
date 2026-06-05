package loginlog

import (
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/xxcheng123/cloudpan189-share/internal/framework/httpcontext"
)

type clearRequest struct {
	// Duration 保留时长（支持 1h/1d/7d/30d/90d 或 Go time.ParseDuration 格式），为空表示清空全部。
	Duration string `json:"duration" example:"30d"`
}

// Clear 清空或按保留时长裁剪登录日志。
// @Summary 清空登录日志
// @Description 清空登录日志，或按保留时长裁剪（传入 duration）。空字符串表示清空全部。
// @Tags 登录日志
// @Accept json
// @Produce json
// @Param Authorization header string true "Bearer token"
// @Param request body clearRequest false "清理参数"
// @Success 200 {object} httpcontext.Response "清理成功，data=删除条数"
// @Failure 400 {object} httpcontext.Response "参数验证失败"
// @Failure 401 {object} httpcontext.Response "未授权访问"
// @Failure 403 {object} httpcontext.Response "权限不足"
// @Router /api/login_log/clear [post]
func (h *handler) Clear() httpcontext.HandlerFunc {
	return func(ctx *httpcontext.Context) {
		var req clearRequest

		if err := ctx.ShouldBindJSON(&req); err != nil && !errors.Is(err, io.EOF) {
			ctx.AbortWithInvalidParams(err)

			return
		}

		if !h.ensureLoginLogService(ctx, codeClearFailed) {
			return
		}

		if req.Duration == "" {
			count, err := h.loginLogService.ClearAll(ctx.GetContext())
			if err != nil {
				ctx.Fail(codeClearFailed.WithError(err))

				return
			}

			ctx.Success(count)

			return
		}

		before, err := resolveCutoff(req.Duration)
		if err != nil {
			ctx.AbortWithInvalidParams(err)

			return
		}

		count, err := h.loginLogService.ClearBefore(ctx.GetContext(), before)
		if err != nil {
			ctx.Fail(codeClearFailed.WithError(err))

			return
		}

		ctx.Success(count)
	}
}

// resolveCutoff 把保留时长文字转换为截止时间（当前时间 - duration）。
func resolveCutoff(duration string) (time.Time, error) {
	now := time.Now()

	switch duration {
	case "1h":
		return now.Add(-time.Hour), nil
	case "1d":
		return now.AddDate(0, 0, -1), nil
	case "7d":
		return now.AddDate(0, 0, -7), nil
	case "30d":
		return now.AddDate(0, 0, -30), nil
	case "90d":
		return now.AddDate(0, 0, -90), nil
	case "180d":
		return now.AddDate(0, 0, -180), nil
	case "365d":
		return now.AddDate(0, 0, -365), nil
	}

	d, err := time.ParseDuration(duration)
	if err != nil {
		return time.Time{}, err
	}

	if d <= 0 {
		return time.Time{}, fmt.Errorf("duration 必须大于 0: %s", duration)
	}

	return now.Add(-d), nil
}
