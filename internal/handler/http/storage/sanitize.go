package storage

import (
	"regexp"

	"github.com/xxcheng123/cloudpan189-share/internal/pkgs/utils"
)

var storageURLPattern = regexp.MustCompile(`(?i)(?:[a-z][a-z0-9+.-]*://|/)[^\s"'<>]+`)

func sanitizeStorageText(text string) string {
	text = storageURLPattern.ReplaceAllStringFunc(text, utils.RedactURLForLog)

	return utils.RedactSensitiveText(text)
}

func sanitizeStorageError(err error) string {
	if err == nil {
		return ""
	}

	return sanitizeStorageText(err.Error())
}
