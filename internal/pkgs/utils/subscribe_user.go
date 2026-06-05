package utils

import (
	"net/url"
	"regexp"
	"strings"
)

var cloud189SubscribeUserLinkPattern = regexp.MustCompile(
	`(?i)(?:^|[^a-z0-9_+./:@-])(?:(?:https?://|//)?)content\.21cn\.com(?::\d+)?(?:/[^\s"'<>]*)?[?&]uuid=([^&#\s"'<>]+)`,
)

var cloud189SubscribeUserLabelPattern = regexp.MustCompile(
	`(?i)(?:订阅号(?:ID)?|订阅用户(?:ID)?|\bsubscribe[\s_-]?user(?:id)?\b|\bup[\s_-]?user(?:id)?\b|\buuid\b)[:：=]\s*([^\s,，;；。()（）]+)`,
)

const cloud189SubscribeUserBoundaryTrimChars = " \t\r\n.,，。;；:：!?！？、()[]【】<>《》「」『』“”‘’\"'"

func ParseCloud189SubscribeUserID(input string) string {
	matches := cloud189SubscribeUserLinkPattern.FindStringSubmatch(input)
	if len(matches) > 1 {
		return normalizeCloud189SubscribeUserCandidate(matches[1])
	}

	if ContainsURLLike(input) {
		return ""
	}

	matches = cloud189SubscribeUserLabelPattern.FindStringSubmatch(input)
	if len(matches) > 1 {
		return normalizeCloud189SubscribeUserCandidate(matches[1])
	}

	return ""
}

func normalizeCloud189SubscribeUserCandidate(input string) string {
	subscribeUserID := strings.Trim(input, cloud189SubscribeUserBoundaryTrimChars)
	if subscribeUserID == "" {
		return ""
	}

	if decoded, err := url.QueryUnescape(subscribeUserID); err == nil {
		subscribeUserID = strings.Trim(decoded, cloud189SubscribeUserBoundaryTrimChars)
	}

	return strings.TrimSpace(subscribeUserID)
}

func NormalizeCloud189SubscribeUserID(input string) string {
	if parsed := ParseCloud189SubscribeUserID(input); parsed != "" {
		return parsed
	}

	input = strings.Trim(input, cloud189SubscribeUserBoundaryTrimChars)
	if ContainsURLLike(input) {
		return ""
	}

	return input
}
