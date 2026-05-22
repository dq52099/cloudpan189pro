package file

import "errors"

var (
	errEmptyFileTaskIDs  = errors.New("文件任务 ID 列表不能为空")
	errInvalidFileTaskID = errors.New("文件任务 ID 必须大于 0")
)

func normalizeFileTaskIDs(ids []int64) ([]int64, error) {
	if len(ids) == 0 {
		return nil, errEmptyFileTaskIDs
	}

	seen := make(map[int64]struct{}, len(ids))
	normalized := make([]int64, 0, len(ids))

	for _, id := range ids {
		if id <= 0 {
			return nil, errInvalidFileTaskID
		}

		if _, ok := seen[id]; ok {
			continue
		}

		seen[id] = struct{}{}
		normalized = append(normalized, id)
	}

	return normalized, nil
}
