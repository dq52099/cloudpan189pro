package cloudbridge

import (
	"testing"
	"time"

	"github.com/xxcheng123/cloudpan189-interface/client"
)

func TestParseShareTimeSupportsKnownFormats(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  time.Time
	}{
		{
			name:  "rfc3339",
			input: "2026-05-31T08:45:12Z",
			want:  time.Date(2026, 5, 31, 8, 45, 12, 0, time.UTC),
		},
		{
			name:  "date time",
			input: "2026-05-31 08:45:12",
			want:  time.Date(2026, 5, 31, 8, 45, 12, 0, time.UTC),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseShareTime(tt.input)
			if !got.Equal(tt.want) {
				t.Fatalf("expected %s, got %s", tt.want.Format(time.RFC3339), got.Format(time.RFC3339))
			}
		})
	}
}

func TestParseShareTimeReturnsZeroForInvalidValue(t *testing.T) {
	if got := parseShareTime("not-a-time"); !got.IsZero() {
		t.Fatalf("expected zero time for invalid value, got %s", got.Format(time.RFC3339))
	}
}

func TestNewShareResourceInfoExposesShareURL(t *testing.T) {
	shareTime := time.Date(2026, 5, 31, 8, 45, 12, 0, time.UTC)
	info := newShareResourceInfo("up-user", &client.ShareFileInfo{
		Name:      `bad/name`,
		Folder:    1,
		AccessURL: "https://cloud.189.cn/t/abcDEF",
		ShareId:   12345,
		Id:        client.String("cloud-file-id"),
		IsTop:     1,
	}, shareTime)

	if info.ShareURL != "https://cloud.189.cn/t/abcDEF" {
		t.Fatalf("expected share url alias, got %q", info.ShareURL)
	}

	if info.AccessCode != info.ShareURL {
		t.Fatalf("expected backward-compatible accessCode to match shareUrl, got accessCode=%q shareUrl=%q", info.AccessCode, info.ShareURL)
	}

	if info.UserId != "up-user" || info.ShareId != 12345 || info.ID != "cloud-file-id" || !info.IsFolder || info.IsTop != 1 {
		t.Fatalf("unexpected share resource info: %+v", info)
	}

	if info.Name == `bad/name` {
		t.Fatalf("expected file name to be sanitized, got %q", info.Name)
	}

	if !info.ShareTime.Equal(shareTime) {
		t.Fatalf("expected share time %s, got %s", shareTime.Format(time.RFC3339), info.ShareTime.Format(time.RFC3339))
	}
}

func TestShouldFetchNextSubscribeSharePageUsesTotalCount(t *testing.T) {
	tests := []struct {
		name             string
		pageNum          int64
		pageSize         int64
		totalCount       int64
		currentPageCount int
		want             bool
	}{
		{
			name:             "exact full page reaches total",
			pageNum:          1,
			pageSize:         100,
			totalCount:       100,
			currentPageCount: 100,
			want:             false,
		},
		{
			name:             "full page before total",
			pageNum:          2,
			pageSize:         100,
			totalCount:       250,
			currentPageCount: 100,
			want:             true,
		},
		{
			name:             "short page stops",
			pageNum:          3,
			pageSize:         100,
			totalCount:       250,
			currentPageCount: 50,
			want:             false,
		},
		{
			name:             "unknown total keeps fetching full pages",
			pageNum:          1,
			pageSize:         100,
			totalCount:       0,
			currentPageCount: 100,
			want:             true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := shouldFetchNextSubscribeSharePage(tt.pageNum, tt.pageSize, tt.totalCount, tt.currentPageCount)
			if got != tt.want {
				t.Fatalf("expected %v, got %v", tt.want, got)
			}
		})
	}
}
