package usergroup

import (
	stdctx "context"
	"errors"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/xxcheng123/cloudpan189-share/internal/bootstrap"
	"github.com/xxcheng123/cloudpan189-share/internal/framework/context"
	"github.com/xxcheng123/cloudpan189-share/internal/pkgs/taskengine"
	"github.com/xxcheng123/cloudpan189-share/internal/repository/models"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type userGroupTestDB struct {
	db *gorm.DB
}

func (t *userGroupTestDB) GetDB(ctx context.Context) *gorm.DB {
	return t.db.WithContext(ctx)
}

func (t *userGroupTestDB) GetDBWithoutContext() *gorm.DB {
	return t.db
}

func (t *userGroupTestDB) GetLogger(name string, fields ...zap.Field) *zap.Logger {
	return zap.NewNop()
}

func (t *userGroupTestDB) Close() {}

func (t *userGroupTestDB) GetPort() int {
	return 9999
}

func (t *userGroupTestDB) GetTaskEngine() taskengine.TaskEngine {
	return nil
}

func (t *userGroupTestDB) GetHTTPEngine() *gin.Engine {
	return nil
}

var _ bootstrap.ServiceContext = (*userGroupTestDB)(nil)

func setupUserGroupTestDB(t *testing.T) *userGroupTestDB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}

	if err := db.AutoMigrate(&models.UserGroup{}, &models.User{}, &models.Group2File{}); err != nil {
		t.Fatalf("migrate test db: %v", err)
	}

	return &userGroupTestDB{db: db}
}

func createUserGroup(t *testing.T, db *gorm.DB, name string) *models.UserGroup {
	t.Helper()

	group := &models.UserGroup{Name: name}
	if err := db.Create(group).Error; err != nil {
		t.Fatalf("create user group: %v", err)
	}

	return group
}

func TestAddRejectsInvalidName(t *testing.T) {
	tDB := setupUserGroupTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())

	tests := []struct {
		name string
		req  *AddRequest
	}{
		{name: "nil request", req: nil},
		{name: "empty name", req: &AddRequest{Name: ""}},
		{name: "blank name", req: &AddRequest{Name: "   "}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := svc.Add(ctx, tt.req)
			if !errors.Is(err, errInvalidUserGroupName) {
				t.Fatalf("expected invalid user group name, got %v", err)
			}

			if resp != nil {
				t.Fatalf("expected nil response, got %#v", resp)
			}
		})
	}

	var count int64
	if err := tDB.db.Model(&models.UserGroup{}).Count(&count).Error; err != nil {
		t.Fatalf("count user groups: %v", err)
	}

	if count != 0 {
		t.Fatalf("expected no user groups to be created, got %d", count)
	}
}

func TestAddTrimsNameAndCreatesGroup(t *testing.T) {
	tDB := setupUserGroupTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())

	resp, err := svc.Add(ctx, &AddRequest{Name: "  operators  "})
	if err != nil {
		t.Fatalf("add user group: %v", err)
	}

	if resp == nil || resp.ID <= 0 {
		t.Fatalf("expected created user group id, got %#v", resp)
	}

	var group models.UserGroup
	if err := tDB.db.First(&group, resp.ID).Error; err != nil {
		t.Fatalf("query user group: %v", err)
	}

	if group.Name != "operators" {
		t.Fatalf("expected trimmed group name, got %q", group.Name)
	}
}

func TestQueryRejectsInvalidID(t *testing.T) {
	tDB := setupUserGroupTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())

	_, err := svc.Query(ctx, 0)
	if !errors.Is(err, errInvalidUserGroupID) {
		t.Fatalf("expected invalid user group id, got %v", err)
	}
}

func TestQueryReturnsExistingGroup(t *testing.T) {
	tDB := setupUserGroupTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())

	group := createUserGroup(t, tDB.db, "query-target")

	got, err := svc.Query(ctx, group.ID)
	if err != nil {
		t.Fatalf("query user group: %v", err)
	}

	if got.ID != group.ID {
		t.Fatalf("expected user group id %d, got %d", group.ID, got.ID)
	}
}

func TestQueryReturnsNotFoundWhenGroupMissing(t *testing.T) {
	tDB := setupUserGroupTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())

	_, err := svc.Query(ctx, 99999)
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("expected record not found, got %v", err)
	}
}

func TestModifyNameReturnsNotFoundWhenGroupMissing(t *testing.T) {
	tDB := setupUserGroupTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())

	err := svc.ModifyName(ctx, &ModifyNameRequest{ID: 99999, Name: "missing"})
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("expected record not found, got %v", err)
	}
}

func TestModifyNameRejectsInvalidID(t *testing.T) {
	tDB := setupUserGroupTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())

	tests := []struct {
		name string
		req  *ModifyNameRequest
	}{
		{name: "nil request", req: nil},
		{name: "zero id", req: &ModifyNameRequest{ID: 0, Name: "invalid"}},
		{name: "negative id", req: &ModifyNameRequest{ID: -1, Name: "invalid"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := svc.ModifyName(ctx, tt.req)
			if !errors.Is(err, errInvalidUserGroupID) {
				t.Fatalf("expected invalid user group id, got %v", err)
			}
		})
	}
}

func TestModifyNameRejectsBlankName(t *testing.T) {
	tDB := setupUserGroupTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())

	group := createUserGroup(t, tDB.db, "old")

	err := svc.ModifyName(ctx, &ModifyNameRequest{ID: group.ID, Name: "   "})
	if !errors.Is(err, errInvalidUserGroupName) {
		t.Fatalf("expected invalid user group name, got %v", err)
	}

	var updated models.UserGroup
	if err := tDB.db.First(&updated, group.ID).Error; err != nil {
		t.Fatalf("query group: %v", err)
	}

	if updated.Name != "old" {
		t.Fatalf("expected existing name unchanged, got %q", updated.Name)
	}
}

func TestModifyNameUpdatesExistingGroup(t *testing.T) {
	tDB := setupUserGroupTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())

	group := createUserGroup(t, tDB.db, "old")

	if err := svc.ModifyName(ctx, &ModifyNameRequest{ID: group.ID, Name: "new"}); err != nil {
		t.Fatalf("modify name: %v", err)
	}

	var updated models.UserGroup
	if err := tDB.db.First(&updated, group.ID).Error; err != nil {
		t.Fatalf("query updated group: %v", err)
	}

	if updated.Name != "new" {
		t.Fatalf("expected updated name new, got %q", updated.Name)
	}
}

func TestModifyNameNoOpExistingGroupSucceeds(t *testing.T) {
	tDB := setupUserGroupTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())

	group := createUserGroup(t, tDB.db, "same-name")

	if err := svc.ModifyName(ctx, &ModifyNameRequest{ID: group.ID, Name: group.Name}); err != nil {
		t.Fatalf("expected no-op modify name to succeed, got %v", err)
	}
}

func TestCheckUserGroupUpdateResultAllowsExistingNoop(t *testing.T) {
	tDB := setupUserGroupTestDB(t)
	svc := NewService(tDB).(*service)
	ctx := context.NewContext(stdctx.Background())

	group := createUserGroup(t, tDB.db, "noop-existing")

	if err := svc.checkUserGroupUpdateResult(ctx, &gorm.DB{RowsAffected: 0}, group.ID); err != nil {
		t.Fatalf("expected existing no-op update to succeed, got %v", err)
	}
}

func TestCheckUserGroupUpdateResultReturnsNotFoundWhenMissing(t *testing.T) {
	tDB := setupUserGroupTestDB(t)
	svc := NewService(tDB).(*service)
	ctx := context.NewContext(stdctx.Background())

	err := svc.checkUserGroupUpdateResult(ctx, &gorm.DB{RowsAffected: 0}, 99999)
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("expected record not found, got %v", err)
	}
}

func TestDeleteReturnsNotFoundWhenGroupMissing(t *testing.T) {
	tDB := setupUserGroupTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())

	err := svc.Delete(ctx, &DeleteRequest{ID: 99999})
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("expected record not found, got %v", err)
	}
}

func TestDeleteRejectsInvalidID(t *testing.T) {
	tDB := setupUserGroupTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())

	tests := []struct {
		name string
		req  *DeleteRequest
	}{
		{name: "nil request", req: nil},
		{name: "zero id", req: &DeleteRequest{ID: 0}},
		{name: "negative id", req: &DeleteRequest{ID: -1}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := svc.Delete(ctx, tt.req)
			if !errors.Is(err, errInvalidUserGroupID) {
				t.Fatalf("expected invalid user group id, got %v", err)
			}
		})
	}
}

func TestDeleteRemovesExistingGroup(t *testing.T) {
	tDB := setupUserGroupTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())

	group := createUserGroup(t, tDB.db, "delete-me")

	if err := svc.Delete(ctx, &DeleteRequest{ID: group.ID}); err != nil {
		t.Fatalf("delete group: %v", err)
	}

	var count int64
	if err := tDB.db.Model(&models.UserGroup{}).Where("id = ?", group.ID).Count(&count).Error; err != nil {
		t.Fatalf("count group: %v", err)
	}

	if count != 0 {
		t.Fatalf("expected group deleted, got count %d", count)
	}
}

func TestDeleteResetsUsersAndClearsFileBindings(t *testing.T) {
	tDB := setupUserGroupTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())

	group := createUserGroup(t, tDB.db, "delete-with-relations")

	user := &models.User{
		Username: "group-user",
		Password: "password",
		Status:   1,
		GroupID:  group.ID,
	}
	if err := tDB.db.Create(user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}

	binding := &models.Group2File{
		GroupId: group.ID,
		FileId:  1001,
	}
	if err := tDB.db.Create(binding).Error; err != nil {
		t.Fatalf("create group file binding: %v", err)
	}

	if err := svc.Delete(ctx, &DeleteRequest{ID: group.ID}); err != nil {
		t.Fatalf("delete user group: %v", err)
	}

	var updatedUser models.User
	if err := tDB.db.First(&updatedUser, user.ID).Error; err != nil {
		t.Fatalf("query user: %v", err)
	}

	if updatedUser.GroupID != 0 {
		t.Fatalf("expected user reset to default group, got %d", updatedUser.GroupID)
	}

	var bindingCount int64
	if err := tDB.db.Model(&models.Group2File{}).Where("group_id = ?", group.ID).Count(&bindingCount).Error; err != nil {
		t.Fatalf("count group file bindings: %v", err)
	}

	if bindingCount != 0 {
		t.Fatalf("expected group file bindings cleared, got %d", bindingCount)
	}
}

func TestBatchQueryReturnsEmptyForEmptyIDList(t *testing.T) {
	tDB := setupUserGroupTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())

	list, err := svc.BatchQuery(ctx, nil)
	if err != nil {
		t.Fatalf("batch query empty user groups: %v", err)
	}

	if len(list) != 0 {
		t.Fatalf("expected empty user groups, got %d", len(list))
	}
}

func TestBatchQueryRejectsInvalidID(t *testing.T) {
	tDB := setupUserGroupTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())

	tests := []struct {
		name string
		ids  []int64
	}{
		{name: "zero", ids: []int64{1, 0}},
		{name: "negative", ids: []int64{-1}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := svc.BatchQuery(ctx, tt.ids)
			if !errors.Is(err, errInvalidUserGroupID) {
				t.Fatalf("expected invalid user group id, got %v", err)
			}
		})
	}
}

func TestBatchQueryDeduplicatesIDsAndIgnoresMissingGroups(t *testing.T) {
	tDB := setupUserGroupTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())

	first := createUserGroup(t, tDB.db, "first")
	second := createUserGroup(t, tDB.db, "second")

	list, err := svc.BatchQuery(ctx, []int64{first.ID, first.ID, 99999, second.ID})
	if err != nil {
		t.Fatalf("batch query user groups: %v", err)
	}

	if len(list) != 2 {
		t.Fatalf("expected 2 user groups, got %d", len(list))
	}

	got := make(map[int64]bool, len(list))
	for _, group := range list {
		got[group.ID] = true
	}

	for _, wantID := range []int64{first.ID, second.ID} {
		if !got[wantID] {
			t.Fatalf("expected user group id %d in result, got %#v", wantID, got)
		}
	}
}

func TestNormalizeUserGroupIDsDeduplicatesInOrder(t *testing.T) {
	ids, err := normalizeUserGroupIDs([]int64{3, 1, 3, 2, 1})
	if err != nil {
		t.Fatalf("normalize user group ids: %v", err)
	}

	want := []int64{3, 1, 2}
	if len(ids) != len(want) {
		t.Fatalf("expected %d ids, got %d", len(want), len(ids))
	}

	for i := range want {
		if ids[i] != want[i] {
			t.Fatalf("expected ids %v, got %v", want, ids)
		}
	}
}
