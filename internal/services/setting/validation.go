package setting

import (
	"net/url"
	"strings"

	"github.com/xxcheng123/cloudpan189-share/internal/pkgs/utils"
	"github.com/xxcheng123/cloudpan189-share/internal/repository/models"
)

const (
	maxMultipleStreamThreadCount = 64
	maxMultipleStreamChunkSize   = 64 * 1024 * 1024
	maxTaskThreadCount           = 32
	maxWorkerCount               = 32
)

func normalizeSettingBaseURL(value string) (string, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "", errInvalidSettingBaseURL
	}

	parsedURL, err := url.Parse(trimmed)
	if err != nil || parsedURL.Host == "" {
		return "", errInvalidSettingBaseURL
	}

	switch parsedURL.Scheme {
	case "http", "https":
		return trimmed, nil
	default:
		return "", errInvalidSettingBaseURL
	}
}

func normalizeSettingAddition(value interface{}) (models.SettingAddition, error) {
	addition, ok := value.(models.SettingAddition)
	if !ok {
		return models.SettingAddition{}, errInvalidSettingAddition
	}

	localProxyURL, err := utils.NormalizeHTTPProxyURL(addition.LocalProxyURL)
	if err != nil {
		return models.SettingAddition{}, errInvalidSettingLocalProxyURL
	}

	addition.LocalProxyURL = localProxyURL
	addition.ApplyDefaultsForWrite()

	if addition.MultipleStreamThreadCount > maxMultipleStreamThreadCount ||
		addition.MultipleStreamChunkSize > maxMultipleStreamChunkSize ||
		addition.TaskThreadCount > maxTaskThreadCount ||
		addition.WorkerCount > maxWorkerCount {
		return models.SettingAddition{}, errInvalidSettingAddition
	}

	return addition, nil
}
