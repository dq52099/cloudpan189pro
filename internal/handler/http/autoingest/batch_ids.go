package autoingest

import (
	"errors"
	"fmt"
)

const maxBatchIDs = 500

func uniqueInt64s(ids []int64) []int64 {
	seen := make(map[int64]struct{}, len(ids))
	result := make([]int64, 0, len(ids))

	for _, id := range ids {
		if _, ok := seen[id]; ok {
			continue
		}

		seen[id] = struct{}{}
		result = append(result, id)
	}

	return result
}

func normalizeBatchIDs(ids []int64) ([]int64, error) {
	result := uniqueInt64s(ids)
	if len(result) == 0 {
		return nil, errors.New("ids 不能为空")
	}

	for _, id := range result {
		if id <= 0 {
			return nil, errors.New("ids 必须全部大于 0")
		}
	}

	if len(result) > maxBatchIDs {
		return nil, fmt.Errorf("ids 去重后不能超过 %d 个", maxBatchIDs)
	}

	return result, nil
}
