package loginlog

import (
	"regexp"
	"unicode/utf8"

	"github.com/xxcheng123/cloudpan189-share/internal/pkgs/utils"
	"github.com/xxcheng123/cloudpan189-share/internal/repository/models"
)

const (
	maxLoginLogReasonRunes    = 255
	maxLoginLogUserAgentRunes = 512
)

var loginLogURLPattern = regexp.MustCompile(`(?i)(?:[a-z][a-z0-9+.-]*://|/)[^\s"'<>]+`)

func normalizeLoginLogForCreate(log *models.LoginLog) {
	if log == nil {
		return
	}

	log.Reason = truncateLoginLogText(sanitizeLoginLogText(log.Reason), maxLoginLogReasonRunes)
	log.UserAgent = truncateLoginLogText(sanitizeLoginLogText(log.UserAgent), maxLoginLogUserAgentRunes)
}

func sanitizeLoginLogText(text string) string {
	text = loginLogURLPattern.ReplaceAllStringFunc(text, utils.RedactURLForLog)

	return utils.RedactSensitiveText(text)
}

func truncateLoginLogText(text string, maxRunes int) string {
	if maxRunes <= 0 || utf8.RuneCountInString(text) <= maxRunes {
		return text
	}

	runes := []rune(text)

	return string(runes[:maxRunes])
}
