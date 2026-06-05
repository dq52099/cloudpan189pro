package utils

import (
	"regexp"
	"strings"
)

var (
	cloud189ShareLinkPattern  = regexp.MustCompile(`(?i)(?:^|[^a-z0-9_+./:@-])(?:(?:https?:\/\/|\/\/)?)(?:www\.)?cloud\.189\.cn(?::\d+)?\/t\/([^\s/?#()（）,，;；。:：]+)`)
	cloud189ShareCodePattern  = regexp.MustCompile(`(?i)(?:分享码|\bshare[\s_-]?code\b|\bshare\b)[:：]\s*([^\s,，;；。()（）]+)`)
	cloud189AccessCodePattern = regexp.MustCompile(`(?i)(访问码|提取码|\baccess[\s_-]?code\b|\baccess\b|\bcode\b)[:：]\s*([^\s,，;；。()（）]+)`)
	cloud189LabelPrefix       = regexp.MustCompile(`(?i)^(?:访问码|提取码|分享码|\baccess[\s_-]?code\b|\baccess\b|\bcode\b|\bshare[\s_-]?code\b|\bshare\b)[:：]?`)
	cloud189ShareCodeValue    = regexp.MustCompile(`^[a-zA-Z0-9]+$`)
	cloud189AccessCodeValue   = regexp.MustCompile(`^[a-zA-Z0-9]+$`)
	cloud189URLLikePattern    = regexp.MustCompile(`(?i)(?:[a-z][a-z0-9+.-]*://|//)[^\s"'<>]+|(?:^|[^a-z0-9_+./:@-])(?:[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?\.)+[a-z]{2,}(?::\d+)?(?:/[^\s"'<>]*|\?[^\s"'<>]*)`)
)

const (
	cloud189CodeBoundaryTrimChars       = " \t\r\n.,，。;；:：!?！？、()[]【】<>《》「」『』“”‘’\"'"
	cloud189EmbeddedLabelSeparatorChars = "、!！?？]】》〉」』”’"
)

func ParseCloud189ShareCode(input string, explicitAccessCode string) (string, string) {
	cleanCode := normalizeCloud189ShareCode(input)
	accessCode := extractCloud189AccessCode(cleanCode)

	explicitAccessCode = normalizeExplicitCloud189AccessCode(explicitAccessCode)
	if explicitAccessCode != "" {
		accessCode = explicitAccessCode
	}

	return extractCloud189ShareCode(cleanCode), accessCode
}

func IsCloud189ShareLink(value string) bool {
	return cloud189ShareLinkPattern.MatchString(normalizeCloud189ShareCode(value))
}

// ContainsURLLike 判断文本中是否包含 URL 形态的片段。
func ContainsURLLike(value string) bool {
	return cloud189URLLikePattern.MatchString(strings.TrimSpace(value))
}

func IsCloud189ShareCode(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" {
		return false
	}

	return cloud189ShareCodeValue.MatchString(value)
}

func IsCloud189AccessCode(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" {
		return false
	}

	return cloud189AccessCodeValue.MatchString(value)
}

func normalizeExplicitCloud189AccessCode(input string) string {
	cleanCode := normalizeCloud189ShareCode(input)
	if cleanCode == "" {
		return ""
	}

	if accessCode := extractCloud189AccessCode(cleanCode); accessCode != "" {
		return accessCode
	}

	return cleanCloud189ExtractedCodeCandidate(cleanCode)
}

func normalizeCloud189ShareCode(input string) string {
	cleanCode := strings.ReplaceAll(input, "（", "(")
	cleanCode = strings.ReplaceAll(cleanCode, "）", ")")
	cleanCode = strings.ReplaceAll(cleanCode, "：", ":")

	return strings.TrimSpace(cleanCode)
}

func extractCloud189AccessCode(cleanCode string) string {
	codeMatches := cloud189AccessCodePattern.FindAllStringSubmatchIndex(cleanCode, -1)
	for _, codeMatch := range codeMatches {
		if len(codeMatch) < 6 {
			continue
		}

		label := strings.ToLower(cleanCode[codeMatch[2]:codeMatch[3]])
		if label == "code" && hasShareLabelBeforeCode(cleanCode[:codeMatch[0]]) {
			continue
		}

		return cleanCloud189ExtractedCodeCandidate(cleanCode[codeMatch[4]:codeMatch[5]])
	}

	parts := strings.Fields(cleanCode)
	if len(parts) <= 1 {
		return ""
	}

	lastPart := cleanCloud189ExtractedCodeCandidate(parts[len(parts)-1])
	if len(lastPart) == 4 {
		return lastPart
	}

	return ""
}

func hasShareLabelBeforeCode(prefix string) bool {
	prefix = strings.ToLower(strings.TrimSpace(prefix))
	prefix = strings.TrimRight(prefix, "_- ")

	return strings.HasSuffix(prefix, "share")
}

func cleanCloud189ExtractedCodeCandidate(value string) string {
	value = strings.TrimSpace(value)
	value = cutCloud189EmbeddedLabel(value)

	return strings.Trim(value, cloud189CodeBoundaryTrimChars)
}

func cutCloud189EmbeddedLabel(value string) string {
	for idx, char := range value {
		if !strings.ContainsRune(cloud189EmbeddedLabelSeparatorChars, char) {
			continue
		}

		prefix := strings.TrimSpace(value[:idx])

		suffix := strings.Trim(value[idx+len(string(char)):], cloud189CodeBoundaryTrimChars)
		if prefix != "" && cloud189LabelPrefix.MatchString(suffix) {
			return prefix
		}
	}

	return value
}

func extractCloud189ShareCode(cleanCode string) string {
	if matches := cloud189ShareLinkPattern.FindStringSubmatch(cleanCode); len(matches) > 1 {
		return cleanCloud189ExtractedCodeCandidate(matches[1])
	}

	if matches := cloud189ShareCodePattern.FindStringSubmatch(cleanCode); len(matches) > 1 {
		return cleanCloud189ExtractedCodeCandidate(matches[1])
	}

	if idx := strings.Index(cleanCode, "("); idx > -1 {
		return strings.TrimSpace(cleanCode[:idx])
	}

	if ContainsURLLike(cleanCode) {
		return ""
	}

	parts := strings.Fields(cleanCode)
	if len(parts) > 0 {
		return cleanCloud189ExtractedCodeCandidate(parts[0])
	}

	return cleanCode
}
