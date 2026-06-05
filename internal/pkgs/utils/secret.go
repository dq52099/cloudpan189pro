package utils

import (
	"encoding/json"
	"net/url"
	"regexp"
	"strings"
)

var shareAccessCodePattern = regexp.MustCompile(`(?i)((?:访问码|提取码|\bshare[ _-]?access[ _-]?code\b|\baccess[ _-]?code\b)\s*[:：]\s*)([a-z0-9]+)`)
var sensitiveKeyValuePattern = regexp.MustCompile(`(?i)((?:[a-z0-9]*[ _-]?token(?:[ _-]?encrypted)?|ssk[ _-]?auth|password|old[ _-]?password|new[ _-]?password|super[ _-]?password|authorization|cookie|set[ _-]?cookie|session[ _-]?id|session[ _-]?key|x[ _-]?api[ _-]?key|api[ _-]?key|client[ _-]?secret|secret|secret[ _-]?key|share[ _-]?access[ _-]?code|access[ _-]?code)\s*[:=]\s*)([^,&;\n]+)`)
var telegramBotTokenURLPattern = regexp.MustCompile(`(?i)(/bot)([^/?#\s]+)(/[^/?#\s]+)`)
var urlUserInfoPattern = regexp.MustCompile(`(?i)(\b[a-z][a-z0-9+.-]*://)([^/?#@\s]+@)`)
var logURLPattern = regexp.MustCompile(`(?i)(?:[a-z][a-z0-9+.-]*://|/)[^\s"'<>]+`)

const RedactedSecret = "[REDACTED]"

var sensitiveLogKeys = map[string]struct{}{
	"accesstoken":        {},
	"apikey":             {},
	"authorization":      {},
	"clientsecret":       {},
	"cookie":             {},
	"newpassword":        {},
	"oldpassword":        {},
	"password":           {},
	"proxyauthorization": {},
	"refreshtoken":       {},
	"secret":             {},
	"session":            {},
	"sessionid":          {},
	"sessionkey":         {},
	"setcookie":          {},
	"shareaccesscode":    {},
	"secretkey":          {},
	"sskauth":            {},
	"sskaccesstoken":     {},
	"sskrefreshtoken":    {},
	"superpassword":      {},
	"token":              {},
	"xapikey":            {},
	"xauthtoken":         {},
	"accesscode":         {},
}

var sensitiveLogKeySuffixes = []string{
	"accesscode",
	"apikey",
	"authorization",
	"cookie",
	"password",
	"secret",
	"secretkey",
	"sessionid",
	"sessionkey",
	"token",
	"tokenencrypted",
}

func MaskSecret(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}

	runes := []rune(trimmed)
	if len(runes) <= 4 {
		return strings.Repeat("*", len(runes))
	}

	return string(runes[:2]) + strings.Repeat("*", len(runes)-4) + string(runes[len(runes)-2:])
}

func MaskShareCodeForLog(value string) string {
	shareCode, _ := ParseCloud189ShareCode(value, "")
	shareCode = strings.TrimSpace(shareCode)

	if shareCode == "" {
		return ""
	}

	runes := []rune(shareCode)
	if len(runes) <= 2 {
		return strings.Repeat("*", len(runes))
	}

	return string(runes[:2]) + strings.Repeat("*", len(runes)-2)
}

func RedactShareAccessCodeText(text string) string {
	if text == "" {
		return ""
	}

	return shareAccessCodePattern.ReplaceAllStringFunc(text, func(match string) string {
		parts := shareAccessCodePattern.FindStringSubmatch(match)
		if len(parts) != 3 {
			return match
		}

		return parts[1] + MaskSecret(parts[2])
	})
}

func IsSensitiveLogKey(key string) bool {
	normalizedKey := normalizeLogKey(key)
	if _, ok := sensitiveLogKeys[normalizedKey]; ok {
		return true
	}

	for _, suffix := range sensitiveLogKeySuffixes {
		if strings.HasSuffix(normalizedKey, suffix) {
			return true
		}
	}

	return false
}

func RedactSensitiveText(text string) string {
	if text == "" {
		return ""
	}

	var payload interface{}
	if err := json.Unmarshal([]byte(text), &payload); err == nil {
		redactedPayload := redactSensitiveLogValue("", payload)

		bytes, marshalErr := json.Marshal(redactedPayload)
		if marshalErr == nil {
			return string(bytes)
		}
	}

	return redactSensitivePlainText(text)
}

func RedactURLForLog(rawURL string) string {
	if rawURL == "" {
		return ""
	}

	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return RedactSensitiveText(redactURLUserInfoText(rawURL))
	}

	hasUserInfo := parsedURL.User != nil
	if hasUserInfo {
		parsedURL.User = url.User(RedactedSecret)
	}

	hasFragment := parsedURL.Fragment != "" || parsedURL.RawFragment != ""
	if parsedURL.RawQuery == "" && !hasFragment && !hasUserInfo {
		return RedactSensitiveText(rawURL)
	}

	if parsedURL.RawQuery != "" {
		parsedURL.RawQuery = RedactedSecret
	}

	if hasFragment {
		parsedURL.Fragment = RedactedSecret
		parsedURL.RawFragment = ""
	}

	return RedactSensitiveText(parsedURL.String())
}

func RedactURLsInTextForLog(text string) string {
	if text == "" {
		return ""
	}

	return logURLPattern.ReplaceAllStringFunc(text, RedactURLForLog)
}

func redactURLUserInfoText(text string) string {
	return urlUserInfoPattern.ReplaceAllString(text, "${1}"+RedactedSecret+"@")
}

func normalizeLogKey(key string) string {
	replacer := strings.NewReplacer("_", "", "-", "", " ", "")

	return strings.ToLower(replacer.Replace(strings.TrimSpace(key)))
}

func redactSensitiveLogValue(key string, value interface{}) interface{} {
	if key != "" && IsSensitiveLogKey(key) {
		return RedactedSecret
	}

	switch typedValue := value.(type) {
	case map[string]interface{}:
		redacted := make(map[string]interface{}, len(typedValue))
		for itemKey, itemValue := range typedValue {
			redacted[itemKey] = redactSensitiveLogValue(itemKey, itemValue)
		}

		return redacted
	case []interface{}:
		redacted := make([]interface{}, 0, len(typedValue))
		for _, itemValue := range typedValue {
			redacted = append(redacted, redactSensitiveLogValue("", itemValue))
		}

		return redacted
	case string:
		return redactSensitiveLogStringValue(typedValue)
	default:
		return typedValue
	}
}

func redactSensitiveLogStringValue(text string) string {
	text = logURLPattern.ReplaceAllStringFunc(text, RedactURLForLog)

	return redactSensitivePlainText(text)
}

func redactSensitivePlainText(text string) string {
	redacted := redactURLUserInfoText(RedactShareAccessCodeText(text))
	redacted = telegramBotTokenURLPattern.ReplaceAllString(redacted, "${1}"+RedactedSecret+"${3}")

	return sensitiveKeyValuePattern.ReplaceAllStringFunc(redacted, func(match string) string {
		parts := sensitiveKeyValuePattern.FindStringSubmatch(match)
		if len(parts) != 3 {
			return match
		}

		return parts[1] + RedactedSecret
	})
}
