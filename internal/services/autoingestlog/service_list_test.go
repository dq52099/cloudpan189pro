package autoingestlog

import (
	stdctx "context"
	"errors"
	"testing"

	"github.com/xxcheng123/cloudpan189-share/internal/framework/context"
	"github.com/xxcheng123/cloudpan189-share/internal/types/autoingest"
)

func TestListDefaultsPaginationWhenRequestNil(t *testing.T) {
	tDB := setupAutoIngestLogTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())

	for i := 0; i < 12; i++ {
		createAutoIngestLog(t, tDB.db, int64(i+1), autoingest.LogLevelInfo)
	}

	list, err := svc.List(ctx, nil)
	if err != nil {
		t.Fatalf("list auto ingest logs: %v", err)
	}

	if len(list) != defaultAutoIngestLogPageSize {
		t.Fatalf("expected default page size %d, got %d", defaultAutoIngestLogPageSize, len(list))
	}
}

func TestListDefaultsPaginationWhenPageValuesMissing(t *testing.T) {
	tDB := setupAutoIngestLogTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())

	for i := 0; i < 12; i++ {
		createAutoIngestLog(t, tDB.db, int64(i+1), autoingest.LogLevelInfo)
	}

	req := &ListRequest{}

	list, err := svc.List(ctx, req)
	if err != nil {
		t.Fatalf("list auto ingest logs: %v", err)
	}

	if len(list) != defaultAutoIngestLogPageSize {
		t.Fatalf("expected default page size %d, got %d", defaultAutoIngestLogPageSize, len(list))
	}

	if req.CurrentPage != defaultAutoIngestLogCurrentPage {
		t.Fatalf("expected current page normalized to %d, got %d", defaultAutoIngestLogCurrentPage, req.CurrentPage)
	}

	if req.PageSize != defaultAutoIngestLogPageSize {
		t.Fatalf("expected page size normalized to %d, got %d", defaultAutoIngestLogPageSize, req.PageSize)
	}
}

func TestListCapsPageSize(t *testing.T) {
	tDB := setupAutoIngestLogTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())

	for i := 0; i < 3; i++ {
		createAutoIngestLog(t, tDB.db, int64(i+1), autoingest.LogLevelInfo)
	}

	req := &ListRequest{
		CurrentPage: 1,
		PageSize:    maxAutoIngestLogPageSize + 100,
	}

	if _, err := svc.List(ctx, req); err != nil {
		t.Fatalf("list auto ingest logs: %v", err)
	}

	if req.PageSize != maxAutoIngestLogPageSize {
		t.Fatalf("expected page size capped to %d, got %d", maxAutoIngestLogPageSize, req.PageSize)
	}
}

func TestListEmptyPlanIdListReturnsEmpty(t *testing.T) {
	tDB := setupAutoIngestLogTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())

	createAutoIngestLog(t, tDB.db, 1, autoingest.LogLevelInfo)

	list, err := svc.List(ctx, &ListRequest{PlanIdList: []int64{}})
	if err != nil {
		t.Fatalf("list auto ingest logs by empty plan ids: %v", err)
	}

	if len(list) != 0 {
		t.Fatalf("expected empty list for empty plan id filter, got %+v", list)
	}

	count, err := svc.Count(ctx, &ListRequest{PlanIdList: []int64{}})
	if err != nil {
		t.Fatalf("count auto ingest logs by empty plan ids: %v", err)
	}

	if count != 0 {
		t.Fatalf("expected count 0 for empty plan id filter, got %d", count)
	}
}

func TestListRejectsInvalidPlanFilters(t *testing.T) {
	tDB := setupAutoIngestLogTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())

	tests := []struct {
		name string
		req  *ListRequest
	}{
		{name: "negative plan id", req: &ListRequest{PlanId: -1}},
		{name: "invalid plan id list", req: &ListRequest{PlanIdList: []int64{1, 0}}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := svc.List(ctx, tt.req)
			if !errors.Is(err, errInvalidAutoIngestLogPlanID) {
				t.Fatalf("expected invalid auto ingest log plan id, got %v", err)
			}
		})
	}
}

func TestCountRejectsInvalidPlanFilters(t *testing.T) {
	tDB := setupAutoIngestLogTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())

	_, err := svc.Count(ctx, &ListRequest{PlanIdList: []int64{1, -1}})
	if !errors.Is(err, errInvalidAutoIngestLogPlanID) {
		t.Fatalf("expected invalid auto ingest log plan id, got %v", err)
	}
}
