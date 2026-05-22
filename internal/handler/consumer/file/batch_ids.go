package file

import "errors"

var errInvalidFileTaskID = errors.New("文件任务 ID 必须大于 0")

func normalizeFileTaskIDs(ids []int64) ([]int64, error) {
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
