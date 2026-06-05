package autoingest

import "github.com/xxcheng123/cloudpan189-share/internal/pkgs/utils"

func sanitizeAutoIngestHTTPText(text string) string {
	return utils.RedactSensitiveText(utils.RedactURLsInTextForLog(text))
}

func sanitizeAutoIngestHTTPError(err error) string {
	if err == nil {
		return ""
	}

	return sanitizeAutoIngestHTTPText(err.Error())
}

func normalizeAutoIngestParentPath(parentPath string) (string, error) {
	return utils.NormalizeStoragePath(parentPath)
}
