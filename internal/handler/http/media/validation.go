package media

import (
	"fmt"
	"net/url"
	"strings"
)

func normalizeMediaBaseURL(value string) (string, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "", fmt.Errorf("媒体基础 URL 不能为空")
	}

	parsedURL, err := url.Parse(trimmed)
	if err != nil || parsedURL.Host == "" {
		return "", fmt.Errorf("媒体基础 URL 必须是有效的 http/https 地址")
	}

	switch parsedURL.Scheme {
	case "http", "https":
		return trimmed, nil
	default:
		return "", fmt.Errorf("媒体基础 URL 必须是有效的 http/https 地址")
	}
}
