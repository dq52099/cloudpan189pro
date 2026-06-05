package autoingestlog

import (
	"regexp"

	"github.com/xxcheng123/cloudpan189-share/internal/pkgs/utils"
)

var autoIngestLogURLPattern = regexp.MustCompile(`(?i)(?:[a-z][a-z0-9+.-]*://|/)[^\s"'<>]+`)

func sanitizeAutoIngestLogContent(content string) string {
	content = autoIngestLogURLPattern.ReplaceAllStringFunc(content, utils.RedactURLForLog)

	return utils.RedactSensitiveText(content)
}
