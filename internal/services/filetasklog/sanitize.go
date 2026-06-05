package filetasklog

import (
	"regexp"
	"strings"

	"github.com/xxcheng123/cloudpan189-share/internal/pkgs/utils"
)

var taskLogURLPattern = regexp.MustCompile(`(?i)(?:[a-z][a-z0-9+.-]*://|/)[^\s"'<>]+`)

func sanitizeTaskLogText(text string) string {
	text = taskLogURLPattern.ReplaceAllStringFunc(text, utils.RedactURLForLog)

	return utils.RedactSensitiveText(text)
}

func sanitizeTaskLogField(key string, value interface{}) interface{} {
	if !isSensitiveTaskLogField(key) {
		return value
	}

	text, ok := value.(string)
	if !ok {
		return value
	}

	return sanitizeTaskLogText(text)
}

func isSensitiveTaskLogField(key string) bool {
	switch strings.ToLower(strings.TrimSpace(key)) {
	case "desc", "error_msg", "result":
		return true
	default:
		return false
	}
}
