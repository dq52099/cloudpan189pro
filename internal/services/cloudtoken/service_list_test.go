package cloudtoken

import (
	stdctx "context"
	"errors"
	"fmt"
	"testing"

	"github.com/xxcheng123/cloudpan189-share/internal/framework/context"
)

func TestListRejectsMissingUserID(t *testing.T) {
	tDB := setupCloudTokenTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())

	tests := []struct {
		name string
		req  *ListRequest
	}{
		{name: "nil request", req: nil},
		{name: "empty request", req: &ListRequest{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := svc.List(ctx, tt.req)
			if !errors.Is(err, errInvalidCloudTokenUserID) {
				t.Fatalf("expected invalid user id, got %v", err)
			}
		})
	}

	_, err := svc.Count(ctx, nil)
	if !errors.Is(err, errInvalidCloudTokenUserID) {
		t.Fatalf("expected invalid user id for count, got %v", err)
	}
}

func TestListDefaultsPaginationWhenRequestNilForAdmin(t *testing.T) {
	tDB := setupCloudTokenTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())

	for i := 0; i < 12; i++ {
		createCloudToken(t, tDB.db, 10, fmt.Sprintf("token-%03d", i))
	}

	req := &ListRequest{IsAdmin: true}

	list, err := svc.List(ctx, req)
	if err != nil {
		t.Fatalf("list cloud tokens: %v", err)
	}

	if len(list) != defaultCloudTokenPageSize {
		t.Fatalf("expected default page size %d, got %d", defaultCloudTokenPageSize, len(list))
	}
}

func TestListRestrictsNonAdminTokens(t *testing.T) {
	tDB := setupCloudTokenTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())

	ownToken := createCloudToken(t, tDB.db, 10, "own")
	createCloudToken(t, tDB.db, 20, "other")

	list, err := svc.List(ctx, &ListRequest{UserID: 10, NoPaginate: true})
	if err != nil {
		t.Fatalf("list own tokens: %v", err)
	}

	if len(list) != 1 || list[0].ID != ownToken.ID {
		t.Fatalf("expected only own token %d, got %+v", ownToken.ID, list)
	}

	count, err := svc.Count(ctx, &ListRequest{UserID: 10})
	if err != nil {
		t.Fatalf("count own tokens: %v", err)
	}

	if count != 1 {
		t.Fatalf("expected own token count 1, got %d", count)
	}
}

func TestListDefaultsPaginationWhenPageValuesMissing(t *testing.T) {
	tDB := setupCloudTokenTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())

	for i := 0; i < 12; i++ {
		createCloudToken(t, tDB.db, 10, fmt.Sprintf("token-%03d", i))
	}

	req := &ListRequest{IsAdmin: true}

	list, err := svc.List(ctx, req)
	if err != nil {
		t.Fatalf("list cloud tokens: %v", err)
	}

	if len(list) != defaultCloudTokenPageSize {
		t.Fatalf("expected default page size %d, got %d", defaultCloudTokenPageSize, len(list))
	}

	if req.CurrentPage != defaultCloudTokenCurrentPage {
		t.Fatalf("expected current page normalized to %d, got %d", defaultCloudTokenCurrentPage, req.CurrentPage)
	}

	if req.PageSize != defaultCloudTokenPageSize {
		t.Fatalf("expected page size normalized to %d, got %d", defaultCloudTokenPageSize, req.PageSize)
	}
}

func TestListCapsPageSize(t *testing.T) {
	tDB := setupCloudTokenTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())

	for i := 0; i < 3; i++ {
		createCloudToken(t, tDB.db, 10, fmt.Sprintf("token-%03d", i))
	}

	req := &ListRequest{
		CurrentPage: 1,
		PageSize:    maxCloudTokenPageSize + 100,
		IsAdmin:     true,
	}

	if _, err := svc.List(ctx, req); err != nil {
		t.Fatalf("list cloud tokens: %v", err)
	}

	if req.PageSize != maxCloudTokenPageSize {
		t.Fatalf("expected page size capped to %d, got %d", maxCloudTokenPageSize, req.PageSize)
	}
}

func TestListRejectsInvalidIDList(t *testing.T) {
	tDB := setupCloudTokenTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())

	_, err := svc.List(ctx, &ListRequest{IdList: []int64{1, 0}, IsAdmin: true})
	if !errors.Is(err, errInvalidCloudTokenID) {
		t.Fatalf("expected invalid cloud token id, got %v", err)
	}
}

func TestCountRejectsInvalidIDList(t *testing.T) {
	tDB := setupCloudTokenTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())

	_, err := svc.Count(ctx, &ListRequest{IdList: []int64{1, -1}, IsAdmin: true})
	if !errors.Is(err, errInvalidCloudTokenID) {
		t.Fatalf("expected invalid cloud token id, got %v", err)
	}
}

func TestListWithEmptyIDListReturnsNoTokens(t *testing.T) {
	tDB := setupCloudTokenTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())

	createCloudToken(t, tDB.db, 10, "first")
	createCloudToken(t, tDB.db, 20, "second")

	req := &ListRequest{
		IdList:     []int64{},
		NoPaginate: true,
		IsAdmin:    true,
	}

	list, err := svc.List(ctx, req)
	if err != nil {
		t.Fatalf("list cloud tokens: %v", err)
	}

	if len(list) != 0 {
		t.Fatalf("expected no tokens for empty id list, got %d", len(list))
	}

	count, err := svc.Count(ctx, req)
	if err != nil {
		t.Fatalf("count cloud tokens: %v", err)
	}

	if count != 0 {
		t.Fatalf("expected count 0 for empty id list, got %d", count)
	}
}

func TestListDeduplicatesIDList(t *testing.T) {
	tDB := setupCloudTokenTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())

	first := createCloudToken(t, tDB.db, 10, "first")
	second := createCloudToken(t, tDB.db, 10, "second")
	createCloudToken(t, tDB.db, 10, "other")

	list, err := svc.List(ctx, &ListRequest{
		IdList:     []int64{first.ID, first.ID, second.ID},
		NoPaginate: true,
		IsAdmin:    true,
	})
	if err != nil {
		t.Fatalf("list cloud tokens: %v", err)
	}

	if len(list) != 2 {
		t.Fatalf("expected two tokens, got %d", len(list))
	}
}
