package usergroup

import (
	stdctx "context"
	"fmt"
	"testing"

	"github.com/xxcheng123/cloudpan189-share/internal/framework/context"
)

func TestListDefaultsPaginationWhenRequestNil(t *testing.T) {
	tDB := setupUserGroupTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())

	for i := 0; i < 12; i++ {
		createUserGroup(t, tDB.db, fmt.Sprintf("group-%03d", i))
	}

	list, err := svc.List(ctx, nil)
	if err != nil {
		t.Fatalf("list user groups: %v", err)
	}

	if len(list) != defaultUserGroupPageSize {
		t.Fatalf("expected default page size %d, got %d", defaultUserGroupPageSize, len(list))
	}
}

func TestListDefaultsPaginationWhenPageValuesMissing(t *testing.T) {
	tDB := setupUserGroupTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())

	for i := 0; i < 12; i++ {
		createUserGroup(t, tDB.db, fmt.Sprintf("group-%03d", i))
	}

	req := &ListRequest{}

	list, err := svc.List(ctx, req)
	if err != nil {
		t.Fatalf("list user groups: %v", err)
	}

	if len(list) != defaultUserGroupPageSize {
		t.Fatalf("expected default page size %d, got %d", defaultUserGroupPageSize, len(list))
	}

	if req.CurrentPage != defaultUserGroupCurrentPage {
		t.Fatalf("expected current page normalized to %d, got %d", defaultUserGroupCurrentPage, req.CurrentPage)
	}

	if req.PageSize != defaultUserGroupPageSize {
		t.Fatalf("expected page size normalized to %d, got %d", defaultUserGroupPageSize, req.PageSize)
	}
}

func TestListCapsPageSize(t *testing.T) {
	tDB := setupUserGroupTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())

	for i := 0; i < 3; i++ {
		createUserGroup(t, tDB.db, fmt.Sprintf("group-%03d", i))
	}

	req := &ListRequest{
		CurrentPage: 1,
		PageSize:    maxUserGroupPageSize + 100,
	}

	if _, err := svc.List(ctx, req); err != nil {
		t.Fatalf("list user groups: %v", err)
	}

	if req.PageSize != maxUserGroupPageSize {
		t.Fatalf("expected page size capped to %d, got %d", maxUserGroupPageSize, req.PageSize)
	}
}

func TestListNoPaginateKeepsAllRows(t *testing.T) {
	tDB := setupUserGroupTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())

	for i := 0; i < 12; i++ {
		createUserGroup(t, tDB.db, fmt.Sprintf("group-%03d", i))
	}

	list, err := svc.List(ctx, &ListRequest{NoPaginate: true})
	if err != nil {
		t.Fatalf("list user groups: %v", err)
	}

	if len(list) != 12 {
		t.Fatalf("expected all rows with no paginate, got %d", len(list))
	}
}
