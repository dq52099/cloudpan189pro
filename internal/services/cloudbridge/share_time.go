package cloudbridge

import "time"

var shareTimeLayouts = []string{
	time.RFC3339,
	"2006-01-02T15:04:05Z",
	time.DateTime,
	"2006-01-02 15:04:05",
}

func parseShareTime(value string) time.Time {
	for _, layout := range shareTimeLayouts {
		parsed, err := time.Parse(layout, value)
		if err == nil {
			return parsed
		}
	}

	return time.Time{}
}
