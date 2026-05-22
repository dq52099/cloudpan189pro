package user

import (
	stdctx "context"
	"errors"
	"fmt"
	"testing"

	"github.com/xxcheng123/cloudpan189-share/internal/framework/context"
)

func TestListDefaultsPaginationWhenRequestNil(t *testing.T) {
	tDB := setupUserTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())

	for i := 0; i < 12; i++ {
		createUser(t, tDB.db, fmt.Sprintf("user-%03d", i), 1)
	}

	list, err := svc.List(ctx, nil)
	if err != nil {
		t.Fatalf("list users: %v", err)
	}

	if len(list) != defaultUserPageSize {
		t.Fatalf("expected default page size %d, got %d", defaultUserPageSize, len(list))
	}
}

func TestListDefaultsPaginationWhenPageValuesMissing(t *testing.T) {
	tDB := setupUserTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())

	for i := 0; i < 12; i++ {
		createUser(t, tDB.db, fmt.Sprintf("user-%03d", i), 1)
	}

	req := &ListRequest{}

	list, err := svc.List(ctx, req)
	if err != nil {
		t.Fatalf("list users: %v", err)
	}

	if len(list) != defaultUserPageSize {
		t.Fatalf("expected default page size %d, got %d", defaultUserPageSize, len(list))
	}

	if req.CurrentPage != defaultUserCurrentPage {
		t.Fatalf("expected current page normalized to %d, got %d", defaultUserCurrentPage, req.CurrentPage)
	}

	if req.PageSize != defaultUserPageSize {
		t.Fatalf("expected page size normalized to %d, got %d", defaultUserPageSize, req.PageSize)
	}
}

func TestListCapsPageSize(t *testing.T) {
	tDB := setupUserTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())

	for i := 0; i < 3; i++ {
		createUser(t, tDB.db, fmt.Sprintf("user-%03d", i), 1)
	}

	req := &ListRequest{
		CurrentPage: 1,
		PageSize:    maxUserPageSize + 100,
	}

	if _, err := svc.List(ctx, req); err != nil {
		t.Fatalf("list users: %v", err)
	}

	if req.PageSize != maxUserPageSize {
		t.Fatalf("expected page size capped to %d, got %d", maxUserPageSize, req.PageSize)
	}
}

func TestListRejectsInvalidGroupID(t *testing.T) {
	tDB := setupUserTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())

	groupID := int64(-1)

	_, err := svc.List(ctx, &ListRequest{GroupId: &groupID})
	if !errors.Is(err, errInvalidUserGroupID) {
		t.Fatalf("expected invalid user group id, got %v", err)
	}
}

func TestCountRejectsInvalidGroupID(t *testing.T) {
	tDB := setupUserTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())

	groupID := int64(-1)

	_, err := svc.Count(ctx, &ListRequest{GroupId: &groupID})
	if !errors.Is(err, errInvalidUserGroupID) {
		t.Fatalf("expected invalid user group id, got %v", err)
	}
}

func TestListNoPaginateKeepsAllRows(t *testing.T) {
	tDB := setupUserTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())

	for i := 0; i < 12; i++ {
		createUser(t, tDB.db, fmt.Sprintf("user-%03d", i), 1)
	}

	list, err := svc.List(ctx, &ListRequest{NoPaginate: true})
	if err != nil {
		t.Fatalf("list users: %v", err)
	}

	if len(list) != 12 {
		t.Fatalf("expected all rows with no paginate, got %d", len(list))
	}
}
