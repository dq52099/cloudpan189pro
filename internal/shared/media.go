package shared

import (
	"time"

	"github.com/xxcheng123/cloudpan189-share/internal/repository/models"
)

var (
	MediaConfig *models.MediaConfig
)

func SetMediaConfig(cfg *models.MediaConfig) {
	configMu.Lock()
	defer configMu.Unlock()

	MediaConfig = cloneMediaConfig(cfg)
}

func GetMediaConfig() *models.MediaConfig {
	configMu.RLock()
	defer configMu.RUnlock()

	return cloneMediaConfig(MediaConfig)
}

func SetMediaConfigLastRebuildTime(lastRebuildTime time.Time) {
	configMu.Lock()
	defer configMu.Unlock()

	if MediaConfig != nil {
		MediaConfig.LastRebuildTime = lastRebuildTime
	}
}

func cloneMediaConfig(cfg *models.MediaConfig) *models.MediaConfig {
	if cfg == nil {
		return nil
	}

	cloned := *cfg
	cloned.IncludedSuffixes = append([]string(nil), cfg.IncludedSuffixes...)

	return &cloned
}
