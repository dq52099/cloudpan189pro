package filetasklog

import (
	stdctx "context"
	"errors"
	"fmt"
	"testing"

	"github.com/xxcheng123/cloudpan189-share/internal/framework/context"
	"github.com/xxcheng123/cloudpan189-share/internal/repository/models"
)

func createFileTaskLogForList(t *testing.T, svc Service, ctx context.Context, index int) {
	t.Helper()

	if _, err := svc.Create(ctx, "scan", fmt.Sprintf("scan %03d", index), WithFile(int64(index+1))); err != nil {
		t.Fatalf("create task log: %v", err)
	}
}

func TestListDefaultsPaginationWhenPageValuesMissing(t *testing.T) {
	tDB := setupFileTaskLogTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())

	for i := 0; i < 12; i++ {
		createFileTaskLogForList(t, svc, ctx, i)
	}

	req := &ListRequest{}

	list, err := svc.List(ctx, req)
	if err != nil {
		t.Fatalf("list task logs: %v", err)
	}

	if len(list) != defaultListPageSize {
		t.Fatalf("expected default page size %d, got %d", defaultListPageSize, len(list))
	}

	if req.CurrentPage != defaultListCurrentPage {
		t.Fatalf("expected current page normalized to %d, got %d", defaultListCurrentPage, req.CurrentPage)
	}

	if req.PageSize != defaultListPageSize {
		t.Fatalf("expected page size normalized to %d, got %d", defaultListPageSize, req.PageSize)
	}
}

func TestListCapsPageSize(t *testing.T) {
	tDB := setupFileTaskLogTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())

	for i := 0; i < 3; i++ {
		createFileTaskLogForList(t, svc, ctx, i)
	}

	req := &ListRequest{
		CurrentPage: 1,
		PageSize:    maxListPageSize + 100,
	}

	if _, err := svc.List(ctx, req); err != nil {
		t.Fatalf("list task logs: %v", err)
	}

	if req.PageSize != maxListPageSize {
		t.Fatalf("expected page size capped to %d, got %d", maxListPageSize, req.PageSize)
	}
}

func TestListRejectsInvalidFileIdFilters(t *testing.T) {
	tDB := setupFileTaskLogTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())

	tests := []struct {
		name string
		req  *ListRequest
	}{
		{name: "negative file id", req: &ListRequest{FileId: -1}},
		{name: "invalid file id list", req: &ListRequest{FileIdList: []int64{1, 0}}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := svc.List(ctx, tt.req)
			if !errors.Is(err, errInvalidFileTaskLogFileID) {
				t.Fatalf("expected invalid file task log file id, got %v", err)
			}
		})
	}
}

func TestListRejectsInvalidSortFields(t *testing.T) {
	tDB := setupFileTaskLogTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())

	tests := []struct {
		name string
		req  *ListRequest
	}{
		{name: "invalid asc", req: &ListRequest{AscList: []string{"title; DROP TABLE file_task_logs"}}},
		{name: "invalid desc", req: &ListRequest{DescList: []string{"password"}}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := svc.List(ctx, tt.req)
			if !errors.Is(err, errInvalidFileTaskLogSortField) {
				t.Fatalf("expected invalid file task log sort field, got %v", err)
			}
		})
	}
}

func TestListAllowsWhitelistedSortFields(t *testing.T) {
	tDB := setupFileTaskLogTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())

	createFileTaskLogForList(t, svc, ctx, 2)
	createFileTaskLogForList(t, svc, ctx, 1)

	list, err := svc.List(ctx, &ListRequest{
		NoPaginate: true,
		AscList:    []string{"title"},
	})
	if err != nil {
		t.Fatalf("list task logs: %v", err)
	}

	if len(list) != 2 {
		t.Fatalf("expected 2 logs, got %d", len(list))
	}

	if list[0].Title > list[1].Title {
		t.Fatalf("expected title asc order, got %q then %q", list[0].Title, list[1].Title)
	}
}

func TestListWithEmptyFileIdListReturnsEmpty(t *testing.T) {
	tDB := setupFileTaskLogTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())

	for i := 0; i < 3; i++ {
		createFileTaskLogForList(t, svc, ctx, i)
	}

	list, err := svc.List(ctx, &ListRequest{
		NoPaginate: true,
		FileIdList: []int64{},
	})
	if err != nil {
		t.Fatalf("list task logs: %v", err)
	}

	if len(list) != 0 {
		t.Fatalf("expected empty list for explicit empty file id list, got %d", len(list))
	}

	count, err := svc.Count(ctx, &ListRequest{FileIdList: []int64{}})
	if err != nil {
		t.Fatalf("count task logs: %v", err)
	}

	if count != 0 {
		t.Fatalf("expected count 0 for explicit empty file id list, got %d", count)
	}
}

func TestListWithNilFileIdListDoesNotFilterByFileID(t *testing.T) {
	tDB := setupFileTaskLogTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())

	for i := 0; i < 3; i++ {
		createFileTaskLogForList(t, svc, ctx, i)
	}

	list, err := svc.List(ctx, &ListRequest{
		NoPaginate: true,
		FileIdList: nil,
	})
	if err != nil {
		t.Fatalf("list task logs: %v", err)
	}

	if len(list) != 3 {
		t.Fatalf("expected nil file id list to keep unfiltered behavior, got %d", len(list))
	}
}

func TestCountRejectsInvalidFileIdFilters(t *testing.T) {
	tDB := setupFileTaskLogTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())

	_, err := svc.Count(ctx, &ListRequest{FileIdList: []int64{1, -1}})
	if !errors.Is(err, errInvalidFileTaskLogFileID) {
		t.Fatalf("expected invalid file task log file id, got %v", err)
	}
}

func TestFindByFileIDRejectsInvalidFileID(t *testing.T) {
	tDB := setupFileTaskLogTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())

	_, err := svc.FindByFileID(ctx, 0)
	if !errors.Is(err, errInvalidFileTaskLogFileID) {
		t.Fatalf("expected invalid file task log file id, got %v", err)
	}
}

func TestListLatestFileIDsByStatusUsesOnlyLatestLogPerFile(t *testing.T) {
	tDB := setupFileTaskLogTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())

	fileCompleted := int64(9001)

	oldFailed, err := svc.Create(ctx, "scan", "old failed", WithFile(fileCompleted))
	if err != nil {
		t.Fatalf("create old failed log: %v", err)
	}

	if err := svc.Failed(ctx, oldFailed); err != nil {
		t.Fatalf("mark old failed: %v", err)
	}

	newCompleted, err := svc.Create(ctx, "scan", "new completed", WithFile(fileCompleted))
	if err != nil {
		t.Fatalf("create new completed log: %v", err)
	}

	if err := svc.Completed(ctx, newCompleted); err != nil {
		t.Fatalf("mark new completed: %v", err)
	}

	fileFailed := int64(9002)

	latestFailed, err := svc.Create(ctx, "scan", "latest failed", WithFile(fileFailed))
	if err != nil {
		t.Fatalf("create latest failed log: %v", err)
	}

	if err := svc.Failed(ctx, latestFailed); err != nil {
		t.Fatalf("mark latest failed: %v", err)
	}

	failedIDs, err := svc.ListLatestFileIDsByStatus(ctx, models.StatusFailed)
	if err != nil {
		t.Fatalf("list latest failed file ids: %v", err)
	}

	if !sameInt64Set(failedIDs, []int64{fileFailed}) {
		t.Fatalf("expected only latest failed file %d, got %v", fileFailed, failedIDs)
	}

	completedIDs, err := svc.ListLatestFileIDsByStatus(ctx, models.StatusCompleted)
	if err != nil {
		t.Fatalf("list latest completed file ids: %v", err)
	}

	if !sameInt64Set(completedIDs, []int64{fileCompleted}) {
		t.Fatalf("expected only latest completed file %d, got %v", fileCompleted, completedIDs)
	}

	emptyIDs, err := svc.ListLatestFileIDsByStatus(ctx, "")
	if err != nil {
		t.Fatalf("list empty status file ids: %v", err)
	}

	if len(emptyIDs) != 0 {
		t.Fatalf("expected empty ids for empty status, got %v", emptyIDs)
	}
}

func sameInt64Set(got, want []int64) bool {
	if len(got) != len(want) {
		return false
	}

	counts := make(map[int64]int, len(want))
	for _, item := range want {
		counts[item]++
	}

	for _, item := range got {
		count, ok := counts[item]
		if !ok || count == 0 {
			return false
		}

		counts[item]--
	}

	return true
}
