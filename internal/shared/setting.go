package shared

import (
	"fmt"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/patrickmn/go-cache"
	"github.com/xxcheng123/cloudpan189-share/internal/consts"
	"github.com/xxcheng123/cloudpan189-share/internal/repository/models"
)

var (
	SaltKey    string
	BaseURL    string
	EnableAuth bool

	SettingAddition = models.SettingAddition{}

	configMu sync.RWMutex
)

var (
	ShareCache = cache.New(5*time.Minute, 10*time.Minute)
)

func JoinDownloadURL(fileId int64, values url.Values) string {
	return JoinDownloadURLWithBase(GetBaseURL(), fileId, values)
}

func JoinDownloadURLWithBase(baseURL string, fileId int64, values url.Values) string {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if baseURL == "" {
		// 默认使用 localhost 和常用端口
		baseURL = "http://localhost:12395"
	}

	return fmt.Sprintf("%s%s", baseURL, fmt.Sprintf(consts.DownloadURLFormat, fileId, values.Encode()))
}

func SetSetting(saltKey string, baseURL string, enableAuth bool, addition models.SettingAddition) {
	configMu.Lock()
	defer configMu.Unlock()

	SaltKey = saltKey
	BaseURL = baseURL
	EnableAuth = enableAuth
	SettingAddition = cloneSettingAddition(addition)
}

func GetSaltKey() string {
	configMu.RLock()
	defer configMu.RUnlock()

	return SaltKey
}

func GetBaseURL() string {
	configMu.RLock()
	defer configMu.RUnlock()

	return BaseURL
}

func IsAuthEnabled() bool {
	configMu.RLock()
	defer configMu.RUnlock()

	return EnableAuth
}

func GetSettingAddition() models.SettingAddition {
	configMu.RLock()
	defer configMu.RUnlock()

	return cloneSettingAddition(SettingAddition)
}

func cloneSettingAddition(addition models.SettingAddition) models.SettingAddition {
	addition.WebDAVAllowedSuffixes = append([]string(nil), addition.WebDAVAllowedSuffixes...)

	return addition
}
