package utils

import (
	"errors"
	"net/url"
	"strings"
)

var ErrInvalidHTTPProxyURL = errors.New("代理地址必须是有效的 http/https 地址")

func NormalizeHTTPProxyURL(value string) (string, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "", nil
	}

	parsedURL, err := url.Parse(trimmed)
	if err != nil {
		return "", ErrInvalidHTTPProxyURL
	}

	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return "", ErrInvalidHTTPProxyURL
	}

	if parsedURL.Host == "" {
		return "", ErrInvalidHTTPProxyURL
	}

	return parsedURL.String(), nil
}
