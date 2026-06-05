package scheduler

import (
	"fmt"
	"regexp"

	"github.com/xxcheng123/cloudpan189-share/internal/pkgs/utils"
)

var schedulerPanicURLPattern = regexp.MustCompile(`(?i)(?:[a-z][a-z0-9+.-]*://|/)[^\s"'<>]+`)

const (
	maxSchedulerPanicTextSize         = 4 * 1024
	truncatedSchedulerPanicTextSuffix = "...[truncated]"
	oversizeSchedulerPanicText        = "[scheduler panic omitted: exceeds log limit]"
)

func sanitizeSchedulerPanicValue(value interface{}) string {
	message := fmt.Sprint(value)
	if len(message) > maxSchedulerPanicTextSize {
		return oversizeSchedulerPanicText + truncatedSchedulerPanicTextSuffix
	}

	message = schedulerPanicURLPattern.ReplaceAllStringFunc(message, utils.RedactURLForLog)

	sanitized := utils.RedactSensitiveText(message)
	if len(sanitized) > maxSchedulerPanicTextSize {
		return sanitized[:maxSchedulerPanicTextSize] + truncatedSchedulerPanicTextSuffix
	}

	return sanitized
}
