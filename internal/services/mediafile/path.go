package mediafile

import (
	"path"
	"path/filepath"
	"strings"
)

func normalizeMediaFilePath(filePath string) string {
	cleaned := strings.TrimSpace(filepath.ToSlash(filePath))
	if cleaned == "" {
		return ""
	}

	cleaned = strings.TrimPrefix(cleaned, "/")
	cleaned = path.Clean(cleaned)

	if cleaned == "." || cleaned == ".." || strings.HasPrefix(cleaned, "../") {
		return ""
	}

	return "/" + cleaned
}

func mediaFilePathCandidates(filePath string) []string {
	normalized := normalizeMediaFilePath(filePath)
	if normalized == "" {
		return nil
	}

	legacy := strings.TrimPrefix(normalized, "/")
	if legacy == "" || legacy == normalized {
		return []string{normalized}
	}

	return []string{normalized, legacy}
}

func mediaFileDiskPath(rootPath, filePath string) (string, error) {
	normalized := normalizeMediaFilePath(filePath)
	if normalized == "" {
		return "", errInvalidMediaFilePath
	}

	rootPath = strings.TrimSpace(rootPath)
	if rootPath == "" {
		return "", errInvalidMediaFilePath
	}

	rootAbs, err := filepath.Abs(rootPath)
	if err != nil {
		return "", err
	}

	diskPath := filepath.Join(rootAbs, strings.TrimPrefix(normalized, "/"))

	diskAbs, err := filepath.Abs(diskPath)
	if err != nil {
		return "", err
	}

	rel, err := filepath.Rel(rootAbs, diskAbs)
	if err != nil {
		return "", err
	}

	rel = filepath.ToSlash(rel)
	if rel == "." || rel == ".." || strings.HasPrefix(rel, "../") {
		return "", errInvalidMediaFilePath
	}

	return diskAbs, nil
}
