package mediaconfig

import (
	"fmt"
	"strings"

	"github.com/robfig/cron/v3"
)

const defaultAutoRebuildCron = "0 2 * * *"

func normalizeAutoRebuildCron(expr string) (string, error) {
	trimmed := strings.TrimSpace(expr)
	if trimmed == "" {
		return defaultAutoRebuildCron, nil
	}

	if _, err := cron.ParseStandard(trimmed); err != nil {
		return "", fmt.Errorf("%w: %v", errInvalidAutoRebuildCron, err)
	}

	return trimmed, nil
}
