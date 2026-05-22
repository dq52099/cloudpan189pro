package virtualfile

import (
	stdctx "context"
	"errors"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/xxcheng123/cloudpan189-share/internal/bootstrap"
	"github.com/xxcheng123/cloudpan189-share/internal/framework/context"
	"github.com/xxcheng123/cloudpan189-share/internal/pkgs/taskengine"
	"github.com/xxcheng123/cloudpan189-share/internal/pkgs/utils"
	"github.com/xxcheng123/cloudpan189-share/internal/repository/models"
	"go.uber.org/zap"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type virtualFileTestDB struct {
	db *gorm.DB
}

func (t *virtualFileTestDB) GetDB(ctx context.Context) *gorm.DB {
	return t.db.WithContext(ctx)
}

func (t *virtualFileTestDB) GetDBWithoutContext() *gorm.DB {
	return t.db
}

func (t *virtualFileTestDB) GetLogger(name string, fields ...zap.Field) *zap.Logger {
	return zap.NewNop()
}

func (t *virtualFileTestDB) Close() {}

func (t *virtualFileTestDB) GetPort() int {
	return 9999
}

func (t *virtualFileTestDB) GetTaskEngine() taskengine.TaskEngine {
	return nil
}

func (t *virtualFileTestDB) GetHTTPEngine() *gin.Engine {
	return nil
}

var _ bootstrap.ServiceContext = (*virtualFileTestDB)(nil)

func setupVirtualFileTestDB(t *testing.T) *virtualFileTestDB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}

	if err := db.AutoMigrate(&models.VirtualFile{}); err != nil {
		t.Fatalf("migrate test db: %v", err)
	}

	return &virtualFileTestDB{db: db}
}

func createVirtualFile(t *testing.T, db *gorm.DB, parentID int64, name string) *models.VirtualFile {
	t.Helper()

	now := time.Now()

	file := &models.VirtualFile{
		CloudId:    name,
		ParentId:   parentID,
		TopId:      parentID,
		IsDir:      false,
		Name:       name,
		OsType:     models.OsTypeFile,
		CreateDate: now,
		ModifyDate: now,
	}
	if err := db.Create(file).Error; err != nil {
		t.Fatalf("create virtual file: %v", err)
	}

	return file
}

func createVirtualDir(t *testing.T, db *gorm.DB, parentID int64, name string) *models.VirtualFile {
	t.Helper()

	now := time.Now()

	file := &models.VirtualFile{
		CloudId:    name,
		ParentId:   parentID,
		TopId:      parentID,
		IsDir:      true,
		Name:       name,
		OsType:     models.OsTypeFolder,
		CreateDate: now,
		ModifyDate: now,
	}
	if err := db.Create(file).Error; err != nil {
		t.Fatalf("create virtual dir: %v", err)
	}

	return file
}

func countVirtualFiles(t *testing.T, db *gorm.DB, query string, args ...any) int64 {
	t.Helper()

	var count int64
	if err := db.Model(&models.VirtualFile{}).Where(query, args...).Count(&count).Error; err != nil {
		t.Fatalf("count virtual files: %v", err)
	}

	return count
}

func newVirtualFileForCreate(name string, rev string) *models.VirtualFile {
	now := time.Now()

	return &models.VirtualFile{
		CloudId:    name + ":" + rev,
		TopId:      1,
		IsDir:      false,
		Name:       name,
		OsType:     models.OsTypeFile,
		Rev:        rev,
		CreateDate: now,
		ModifyDate: now,
	}
}

func TestCreateRenamesWhenSanitizedNameAlreadyExists(t *testing.T) {
	tDB := setupVirtualFileTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())

	createVirtualFile(t, tDB.db, 1, "a_b.txt")

	file := newVirtualFileForCreate("a:b.txt", "rev1")
	if _, err := svc.Create(ctx, 1, file); err != nil {
		t.Fatalf("create virtual file: %v", err)
	}

	if file.Name != "a_b.txt(rev1)" {
		t.Fatalf("expected sanitized duplicate to be renamed, got %q", file.Name)
	}

	if count := countVirtualFiles(t, tDB.db, "parent_id = ? AND name = ?", 1, "a_b.txt(rev1)"); count != 1 {
		t.Fatalf("expected renamed file persisted, got count %d", count)
	}
}

func TestBatchCreateRenamesWhenSanitizedNameAlreadyExists(t *testing.T) {
	tDB := setupVirtualFileTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())

	createVirtualFile(t, tDB.db, 1, "a_b.txt")
	createVirtualFile(t, tDB.db, 1, "a_b.txt(rev1)")

	files := []*models.VirtualFile{
		newVirtualFileForCreate("a:b.txt", "rev1"),
	}

	created, err := svc.BatchCreate(ctx, 1, files)
	if err != nil {
		t.Fatalf("batch create virtual files: %v", err)
	}

	if created != 1 {
		t.Fatalf("expected one created file, got %d", created)
	}

	if files[0].Name != "a_b.txt(rev1-2)" {
		t.Fatalf("expected sanitized duplicate to use second fallback, got %q", files[0].Name)
	}

	if count := countVirtualFiles(t, tDB.db, "parent_id = ? AND name = ?", 1, "a_b.txt(rev1-2)"); count != 1 {
		t.Fatalf("expected second fallback file persisted, got count %d", count)
	}
}

func TestBatchCreateEmptyFilesReturnsWithoutHooks(t *testing.T) {
	tDB := setupVirtualFileTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())
	hookCalled := false

	created, err := svc.BatchCreate(ctx, 1, nil, func(ctx context.Context, result *gorm.DB, files []*models.VirtualFile) {
		hookCalled = true
	})
	if err != nil {
		t.Fatalf("batch create empty files: %v", err)
	}

	if created != 0 {
		t.Fatalf("expected zero created files, got %d", created)
	}

	if hookCalled {
		t.Fatal("expected hooks not to run for empty batch")
	}
}

func TestBatchCreateDeduplicatesSanitizedNamesWithinBatch(t *testing.T) {
	tDB := setupVirtualFileTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())

	files := []*models.VirtualFile{
		newVirtualFileForCreate("movie:name.mkv", "r1"),
		newVirtualFileForCreate("movie_name.mkv", "r2"),
	}

	created, err := svc.BatchCreate(ctx, 1, files)
	if err != nil {
		t.Fatalf("batch create virtual files: %v", err)
	}

	if created != 2 {
		t.Fatalf("expected two created files, got %d", created)
	}

	if files[0].Name != "movie_name.mkv" {
		t.Fatalf("expected first sanitized name, got %q", files[0].Name)
	}

	if files[1].Name != "movie_name.mkv(r2)" {
		t.Fatalf("expected second in-batch duplicate renamed, got %q", files[1].Name)
	}

	if count := countVirtualFiles(t, tDB.db, "parent_id = ? AND name IN ?", 1, []string{"movie_name.mkv", "movie_name.mkv(r2)"}); count != 2 {
		t.Fatalf("expected both sanitized batch files persisted, got count %d", count)
	}
}

func TestListVirtualFileDescListSortsDescending(t *testing.T) {
	tDB := setupVirtualFileTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())

	createVirtualFile(t, tDB.db, 1, "a.txt")
	createVirtualFile(t, tDB.db, 1, "c.txt")
	createVirtualFile(t, tDB.db, 1, "b.txt")

	parentID := int64(1)

	list, err := svc.List(ctx, &ListRequest{
		ParentId: &parentID,
		DescList: []string{"name"},
	})
	if err != nil {
		t.Fatalf("list virtual files: %v", err)
	}

	if len(list) != 3 {
		t.Fatalf("expected 3 files, got %d", len(list))
	}

	got := []string{list[0].Name, list[1].Name, list[2].Name}
	want := []string{"c.txt", "b.txt", "a.txt"}

	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("expected order %v, got %v", want, got)
		}
	}
}

func TestListVirtualFileRejectsInvalidSortFields(t *testing.T) {
	tDB := setupVirtualFileTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())

	tests := []struct {
		name string
		req  *ListRequest
	}{
		{name: "invalid asc", req: &ListRequest{AscList: []string{"name; DROP TABLE virtual_files"}}},
		{name: "invalid desc", req: &ListRequest{DescList: []string{"password"}}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := svc.List(ctx, tt.req)
			if !errors.Is(err, errInvalidVirtualFileSortField) {
				t.Fatalf("expected invalid virtual file sort field, got %v", err)
			}
		})
	}
}

func TestListVirtualFileHandlesNilRequest(t *testing.T) {
	tDB := setupVirtualFileTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())

	createVirtualFile(t, tDB.db, 1, "first.txt")
	createVirtualFile(t, tDB.db, 1, "second.txt")

	list, err := svc.List(ctx, nil)
	if err != nil {
		t.Fatalf("list virtual files: %v", err)
	}

	if len(list) != 2 {
		t.Fatalf("expected 2 files, got %d", len(list))
	}
}

func TestGroupCountByTopIdHandlesNilRequest(t *testing.T) {
	tDB := setupVirtualFileTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())

	createVirtualFile(t, tDB.db, 1001, "first.txt")
	createVirtualFile(t, tDB.db, 1001, "second.txt")
	createVirtualFile(t, tDB.db, 2001, "third.txt")

	list, err := svc.GroupCountByTopId(ctx, nil)
	if err != nil {
		t.Fatalf("group count by top id: %v", err)
	}

	got := make(map[int64]int64, len(list))
	for _, item := range list {
		got[item.TopId] = item.Count
	}

	if got[1001] != 2 {
		t.Fatalf("expected top id 1001 count 2, got %d from %#v", got[1001], got)
	}

	if got[2001] != 1 {
		t.Fatalf("expected top id 2001 count 1, got %d from %#v", got[2001], got)
	}
}

func TestGroupCountByTopIdFiltersTopIdList(t *testing.T) {
	tDB := setupVirtualFileTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())

	createVirtualFile(t, tDB.db, 1001, "first.txt")
	createVirtualFile(t, tDB.db, 1001, "second.txt")
	createVirtualFile(t, tDB.db, 2001, "third.txt")
	createVirtualFile(t, tDB.db, 3001, "fourth.txt")

	list, err := svc.GroupCountByTopId(ctx, &GroupCountByTopIdRequest{
		TopIdList: []int64{1001, 1001, 3001},
	})
	if err != nil {
		t.Fatalf("group count by top id: %v", err)
	}

	got := make(map[int64]int64, len(list))
	for _, item := range list {
		got[item.TopId] = item.Count
	}

	if got[1001] != 2 {
		t.Fatalf("expected top id 1001 count 2, got %d from %#v", got[1001], got)
	}

	if got[3001] != 1 {
		t.Fatalf("expected top id 3001 count 1, got %d from %#v", got[3001], got)
	}

	if _, ok := got[2001]; ok {
		t.Fatalf("did not expect unrequested top id 2001 in %#v", got)
	}
}

func TestGroupCountByTopIdHandlesEmptyTopIdList(t *testing.T) {
	tDB := setupVirtualFileTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())

	createVirtualFile(t, tDB.db, 1001, "first.txt")

	list, err := svc.GroupCountByTopId(ctx, &GroupCountByTopIdRequest{
		TopIdList: []int64{},
	})
	if err != nil {
		t.Fatalf("group count by empty top id list: %v", err)
	}

	if len(list) != 0 {
		t.Fatalf("expected empty result, got %#v", list)
	}
}

func TestGroupCountByTopIdRejectsInvalidTopIdList(t *testing.T) {
	tDB := setupVirtualFileTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())

	_, err := svc.GroupCountByTopId(ctx, &GroupCountByTopIdRequest{
		TopIdList: []int64{1001, 0},
	})
	if !errors.Is(err, errInvalidVirtualFileID) {
		t.Fatalf("expected invalid virtual file id, got %v", err)
	}
}

func TestCountVirtualFileHandlesNilRequest(t *testing.T) {
	tDB := setupVirtualFileTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())

	createVirtualFile(t, tDB.db, 1, "first.txt")
	createVirtualFile(t, tDB.db, 1, "second.txt")

	count, err := svc.Count(ctx, nil)
	if err != nil {
		t.Fatalf("count virtual files: %v", err)
	}

	if count != 2 {
		t.Fatalf("expected 2 files, got %d", count)
	}
}

func TestListVirtualFileNoPaginationKeepsAllRows(t *testing.T) {
	tDB := setupVirtualFileTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())

	for i := range 3 {
		createVirtualFile(t, tDB.db, 1, string(rune('a'+i))+".txt")
	}

	list, err := svc.List(ctx, &ListRequest{})
	if err != nil {
		t.Fatalf("list virtual files: %v", err)
	}

	if len(list) != 3 {
		t.Fatalf("expected all 3 files, got %d", len(list))
	}
}

func TestListVirtualFileDefaultsPartialPagination(t *testing.T) {
	tDB := setupVirtualFileTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())

	createVirtualFile(t, tDB.db, 1, "first.txt")
	createVirtualFile(t, tDB.db, 1, "second.txt")

	req := &ListRequest{PageSize: 1}

	list, err := svc.List(ctx, req)
	if err != nil {
		t.Fatalf("list virtual files: %v", err)
	}

	if len(list) != 1 {
		t.Fatalf("expected 1 file, got %d", len(list))
	}

	if req.CurrentPage != 1 {
		t.Fatalf("expected current page defaulted to 1, got %d", req.CurrentPage)
	}
}

func TestListVirtualFileCapsPageSize(t *testing.T) {
	tDB := setupVirtualFileTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())

	createVirtualFile(t, tDB.db, 1, "first.txt")

	req := &ListRequest{
		CurrentPage: 1,
		PageSize:    maxVirtualFilePageSize + 100,
	}
	if _, err := svc.List(ctx, req); err != nil {
		t.Fatalf("list virtual files: %v", err)
	}

	if req.PageSize != maxVirtualFilePageSize {
		t.Fatalf("expected page size capped to %d, got %d", maxVirtualFilePageSize, req.PageSize)
	}
}

func TestListVirtualFileRejectsInvalidIDFilters(t *testing.T) {
	tDB := setupVirtualFileTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())

	tests := []struct {
		name string
		req  *ListRequest
	}{
		{name: "exclude ids", req: &ListRequest{ExcludeIdList: []int64{1, 0}}},
		{name: "top ids", req: &ListRequest{TopIdList: []int64{1, -1}}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := svc.List(ctx, tt.req)
			if !errors.Is(err, errInvalidVirtualFileID) {
				t.Fatalf("expected invalid virtual file id, got %v", err)
			}

			_, err = svc.Count(ctx, tt.req)
			if !errors.Is(err, errInvalidVirtualFileID) {
				t.Fatalf("expected count invalid virtual file id, got %v", err)
			}
		})
	}
}

func TestListVirtualFileDeduplicatesIDFilters(t *testing.T) {
	tDB := setupVirtualFileTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())

	first := createVirtualFile(t, tDB.db, 1, "first.txt")
	second := createVirtualFile(t, tDB.db, 1, "second.txt")
	createVirtualFile(t, tDB.db, 2, "third.txt")

	list, err := svc.List(ctx, &ListRequest{
		TopIdList:     []int64{1, 1},
		ExcludeIdList: []int64{first.ID, first.ID},
	})
	if err != nil {
		t.Fatalf("list virtual files: %v", err)
	}

	if len(list) != 1 {
		t.Fatalf("expected 1 file, got %d", len(list))
	}

	if list[0].ID != second.ID {
		t.Fatalf("expected second file id %d, got %d", second.ID, list[0].ID)
	}
}

func TestListVirtualFileEmptyTopIdListReturnsEmpty(t *testing.T) {
	tDB := setupVirtualFileTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())

	createVirtualFile(t, tDB.db, 1, "first.txt")

	list, err := svc.List(ctx, &ListRequest{
		TopIdList: []int64{},
	})
	if err != nil {
		t.Fatalf("list virtual files: %v", err)
	}

	if len(list) != 0 {
		t.Fatalf("expected empty list, got %#v", list)
	}

	count, err := svc.Count(ctx, &ListRequest{
		TopIdList: []int64{},
	})
	if err != nil {
		t.Fatalf("count virtual files: %v", err)
	}

	if count != 0 {
		t.Fatalf("expected count 0, got %d", count)
	}
}

func TestDeleteVirtualFileDeletesOnlyRequestedID(t *testing.T) {
	tDB := setupVirtualFileTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())

	target := createVirtualFile(t, tDB.db, 1, "target.txt")
	other := createVirtualFile(t, tDB.db, 1, "other.txt")

	if err := svc.Delete(ctx, target.ID); err != nil {
		t.Fatalf("delete virtual file: %v", err)
	}

	if count := countVirtualFiles(t, tDB.db, "id = ?", target.ID); count != 0 {
		t.Fatalf("expected target file deleted, got count %d", count)
	}

	if count := countVirtualFiles(t, tDB.db, "id = ?", other.ID); count != 1 {
		t.Fatalf("expected other file to remain, got count %d", count)
	}
}

func TestDeleteVirtualFileReturnsNotFoundWhenMissing(t *testing.T) {
	tDB := setupVirtualFileTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())

	err := svc.Delete(ctx, 99999)
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("expected record not found, got %v", err)
	}
}

func TestDeleteVirtualFileRejectsInvalidID(t *testing.T) {
	tDB := setupVirtualFileTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())

	err := svc.Delete(ctx, 0)
	if !errors.Is(err, errInvalidVirtualFileID) {
		t.Fatalf("expected invalid virtual file id, got %v", err)
	}
}

func TestBatchDeleteVirtualFileReturnsMatchedIDsAndFiles(t *testing.T) {
	tDB := setupVirtualFileTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())

	first := createVirtualFile(t, tDB.db, 1, "first.txt")
	second := createVirtualFile(t, tDB.db, 1, "second.txt")
	other := createVirtualFile(t, tDB.db, 1, "other.txt")

	var hookFiles []*models.VirtualFile

	deletedIDs, err := svc.BatchDelete(ctx, []int64{first.ID, second.ID, first.ID}, func(_ context.Context, _ *gorm.DB, files []*models.VirtualFile) {
		hookFiles = files
	})
	if err != nil {
		t.Fatalf("batch delete virtual files: %v", err)
	}

	if len(deletedIDs) != 2 {
		t.Fatalf("expected 2 deleted ids, got %d", len(deletedIDs))
	}

	if len(hookFiles) != 2 {
		t.Fatalf("expected hook to receive 2 files, got %d", len(hookFiles))
	}

	if count := countVirtualFiles(t, tDB.db, "id IN ?", []int64{first.ID, second.ID}); count != 0 {
		t.Fatalf("expected batch files deleted, got count %d", count)
	}

	if count := countVirtualFiles(t, tDB.db, "id = ?", other.ID); count != 1 {
		t.Fatalf("expected unrelated file to remain, got count %d", count)
	}
}

func TestBatchDeleteVirtualFileReturnsNotFoundWhenSomeRequestedIDsMissing(t *testing.T) {
	tDB := setupVirtualFileTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())

	file := createVirtualFile(t, tDB.db, 1, "partial-missing.txt")
	hookCalled := false

	deletedIDs, err := svc.BatchDelete(ctx, []int64{file.ID, 99999}, func(_ context.Context, _ *gorm.DB, _ []*models.VirtualFile) {
		hookCalled = true
	})
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("expected record not found, got %v", err)
	}

	if len(deletedIDs) != 0 {
		t.Fatalf("expected no deleted ids returned, got %v", deletedIDs)
	}

	if hookCalled {
		t.Fatal("expected hook not to be called when requested ids are missing")
	}

	if count := countVirtualFiles(t, tDB.db, "id = ?", file.ID); count != 1 {
		t.Fatalf("expected existing file to remain, got count %d", count)
	}
}

func TestBatchDeleteVirtualFileReturnsNotFoundWhenDeleteAffectsFewerRows(t *testing.T) {
	tDB := setupVirtualFileTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())

	first := createVirtualFile(t, tDB.db, 1, "batch-race-first.txt")
	second := createVirtualFile(t, tDB.db, 1, "batch-race-second.txt")

	callbackName := "virtualfile:test_batch_delete_rows_affected_mismatch"
	if err := tDB.db.Callback().Delete().Before("gorm:delete").Register(callbackName, func(tx *gorm.DB) {
		if tx.Statement.Schema != nil && tx.Statement.Schema.Table == (&models.VirtualFile{}).TableName() {
			_ = tx.Exec("DELETE FROM virtual_files WHERE id = ?", second.ID).Error
		}
	}); err != nil {
		t.Fatalf("register delete callback: %v", err)
	}
	defer func() {
		if removeErr := tDB.db.Callback().Delete().Remove(callbackName); removeErr != nil {
			t.Fatalf("remove delete callback: %v", removeErr)
		}
	}()

	hookCalled := false

	deletedIDs, err := svc.BatchDelete(ctx, []int64{first.ID, second.ID}, func(_ context.Context, _ *gorm.DB, _ []*models.VirtualFile) {
		hookCalled = true
	})
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("expected record not found, got %v", err)
	}

	if len(deletedIDs) != 0 {
		t.Fatalf("expected no deleted ids returned, got %v", deletedIDs)
	}

	if hookCalled {
		t.Fatal("expected hook not to be called when delete affects fewer rows")
	}
}

func TestBatchDeleteVirtualFileRejectsInvalidIDAndKeepsRows(t *testing.T) {
	tDB := setupVirtualFileTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())

	file := createVirtualFile(t, tDB.db, 1, "invalid-batch.txt")

	deletedIDs, err := svc.BatchDelete(ctx, []int64{file.ID, 0})
	if !errors.Is(err, errInvalidVirtualFileID) {
		t.Fatalf("expected invalid virtual file id, got %v", err)
	}

	if len(deletedIDs) != 0 {
		t.Fatalf("expected no deleted ids, got %v", deletedIDs)
	}

	if count := countVirtualFiles(t, tDB.db, "id = ?", file.ID); count != 1 {
		t.Fatalf("expected file to remain, got count %d", count)
	}
}

func TestQueryVirtualFileRejectsInvalidID(t *testing.T) {
	tDB := setupVirtualFileTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())

	_, err := svc.Query(ctx, 0)
	if !errors.Is(err, errInvalidVirtualFileID) {
		t.Fatalf("expected invalid virtual file id, got %v", err)
	}
}

func TestQueryByPathUsesParentAndNameSegments(t *testing.T) {
	tDB := setupVirtualFileTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())

	parentA := createVirtualDir(t, tDB.db, 0, "a")
	parentB := createVirtualDir(t, tDB.db, 0, "b")
	fileA := createVirtualFile(t, tDB.db, parentA.ID, "common.txt")
	fileB := createVirtualFile(t, tDB.db, parentB.ID, "common.txt")

	gotA, err := svc.QueryByPath(ctx, "/a/common.txt")
	if err != nil {
		t.Fatalf("query path /a/common.txt: %v", err)
	}

	if gotA.ID != fileA.ID {
		t.Fatalf("expected /a/common.txt id %d, got %d", fileA.ID, gotA.ID)
	}

	gotB, err := svc.QueryByPath(ctx, "/b/common.txt")
	if err != nil {
		t.Fatalf("query path /b/common.txt: %v", err)
	}

	if gotB.ID != fileB.ID {
		t.Fatalf("expected /b/common.txt id %d, got %d", fileB.ID, gotB.ID)
	}
}

func TestQueryByPathReturnsNotFoundForEmptyPath(t *testing.T) {
	tDB := setupVirtualFileTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())

	for _, path := range []string{"", "/"} {
		t.Run(path, func(t *testing.T) {
			file, err := svc.QueryByPath(ctx, path)
			if !errors.Is(err, gorm.ErrRecordNotFound) {
				t.Fatalf("expected record not found, got file=%#v err=%v", file, err)
			}
		})
	}
}

func TestFindOrCreateAncestorsReusesExistingPathSegments(t *testing.T) {
	tDB := setupVirtualFileTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())

	firstParentID, err := svc.FindOrCreateAncestors(ctx, "/movies/2024/file.mkv")
	if err != nil {
		t.Fatalf("find or create ancestors first call: %v", err)
	}

	secondParentID, err := svc.FindOrCreateAncestors(ctx, "/movies/2024/other.mkv")
	if err != nil {
		t.Fatalf("find or create ancestors second call: %v", err)
	}

	if secondParentID != firstParentID {
		t.Fatalf("expected reused parent id %d, got %d", firstParentID, secondParentID)
	}

	if count := countVirtualFiles(t, tDB.db, "parent_id = ? AND name = ?", 0, "movies"); count != 1 {
		t.Fatalf("expected one movies dir, got %d", count)
	}

	var movies models.VirtualFile
	if err = tDB.db.Where("parent_id = ? AND name = ?", 0, "movies").First(&movies).Error; err != nil {
		t.Fatalf("query movies dir: %v", err)
	}

	if count := countVirtualFiles(t, tDB.db, "parent_id = ? AND name = ?", movies.ID, "2024"); count != 1 {
		t.Fatalf("expected one 2024 dir under movies, got %d", count)
	}

	if count := countVirtualFiles(t, tDB.db, "id = ? AND parent_id = ? AND name = ?", firstParentID, movies.ID, "2024"); count != 1 {
		t.Fatalf("expected final parent to be existing 2024 dir, got count %d", count)
	}
}

func TestClearUnusedAncestorFolderDeletesOnlyEmptyNormalDirs(t *testing.T) {
	tDB := setupVirtualFileTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())

	grandParent := createVirtualDir(t, tDB.db, 0, "movies")
	parent := createVirtualDir(t, tDB.db, grandParent.ID, "2026")
	file := createVirtualFile(t, tDB.db, parent.ID, "movie.mkv")

	if err := svc.ClearUnusedAncestorFolder(ctx, file.ID); err != nil {
		t.Fatalf("clear unused ancestor folders: %v", err)
	}

	if count := countVirtualFiles(t, tDB.db, "id IN ?", []int64{parent.ID, grandParent.ID}); count != 0 {
		t.Fatalf("expected empty normal ancestor dirs deleted, got count %d", count)
	}

	if count := countVirtualFiles(t, tDB.db, "id = ?", file.ID); count != 1 {
		t.Fatalf("expected current file to remain untouched, got count %d", count)
	}
}

func TestClearUnusedAncestorFolderStopsAtNonEmptyParent(t *testing.T) {
	tDB := setupVirtualFileTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())

	parent := createVirtualDir(t, tDB.db, 0, "movies")
	file := createVirtualFile(t, tDB.db, parent.ID, "movie.mkv")
	sibling := createVirtualFile(t, tDB.db, parent.ID, "sibling.mkv")

	if err := svc.ClearUnusedAncestorFolder(ctx, file.ID); err != nil {
		t.Fatalf("clear unused ancestor folders: %v", err)
	}

	if count := countVirtualFiles(t, tDB.db, "id = ?", parent.ID); count != 1 {
		t.Fatalf("expected non-empty parent to remain, got count %d", count)
	}

	if count := countVirtualFiles(t, tDB.db, "id = ?", sibling.ID); count != 1 {
		t.Fatalf("expected sibling to remain, got count %d", count)
	}
}

func TestClearUnusedAncestorFolderStopsAtTopMountPoint(t *testing.T) {
	tDB := setupVirtualFileTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())

	top := createVirtualDir(t, tDB.db, 0, "top")
	if err := tDB.db.Model(&models.VirtualFile{}).
		Where("id = ?", top.ID).
		Updates(map[string]any{
			"is_top":  true,
			"top_id":  top.ID,
			"os_type": models.OsTypePersonFolder,
		}).Error; err != nil {
		t.Fatalf("mark top mount point: %v", err)
	}

	file := createVirtualFile(t, tDB.db, top.ID, "movie.mkv")

	if err := svc.ClearUnusedAncestorFolder(ctx, file.ID); err != nil {
		t.Fatalf("clear unused ancestor folders: %v", err)
	}

	if count := countVirtualFiles(t, tDB.db, "id = ?", top.ID); count != 1 {
		t.Fatalf("expected top mount point to remain, got count %d", count)
	}
}

func TestClearUnusedAncestorFolderStopsAtNonDirectoryParent(t *testing.T) {
	tDB := setupVirtualFileTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())

	parent := createVirtualFile(t, tDB.db, 0, "not-a-dir.txt")
	file := createVirtualFile(t, tDB.db, parent.ID, "child.txt")

	if err := svc.ClearUnusedAncestorFolder(ctx, file.ID); err != nil {
		t.Fatalf("clear unused ancestor folders: %v", err)
	}

	if count := countVirtualFiles(t, tDB.db, "id = ?", parent.ID); count != 1 {
		t.Fatalf("expected non-directory parent to remain, got count %d", count)
	}
}

func TestUpdateVirtualFileReturnsNotFoundWhenMissing(t *testing.T) {
	tDB := setupVirtualFileTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())

	err := svc.Update(ctx, 99999, []utils.Field{utils.WithField("name", "missing.txt")})
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("expected record not found, got %v", err)
	}
}

func TestUpdateVirtualFileRejectsInvalidID(t *testing.T) {
	tDB := setupVirtualFileTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())

	err := svc.Update(ctx, 0, []utils.Field{utils.WithField("name", "invalid.txt")})
	if !errors.Is(err, errInvalidVirtualFileID) {
		t.Fatalf("expected invalid virtual file id, got %v", err)
	}
}

func TestUpdateVirtualFileRejectsEmptyFieldsWithoutHook(t *testing.T) {
	tDB := setupVirtualFileTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())

	file := createVirtualFile(t, tDB.db, 1, "empty-update.txt")

	hookCalled := false

	err := svc.Update(ctx, file.ID, nil, func(_ context.Context, _ *gorm.DB, _ int64, _ []utils.Field) {
		hookCalled = true
	})
	if !errors.Is(err, errEmptyVirtualFileUpdateFields) {
		t.Fatalf("expected empty virtual file update fields, got %v", err)
	}

	if hookCalled {
		t.Fatal("expected update hook not to be called")
	}

	var unchanged models.VirtualFile
	if err := tDB.db.First(&unchanged, file.ID).Error; err != nil {
		t.Fatalf("query unchanged file: %v", err)
	}

	if unchanged.Name != "empty-update.txt" {
		t.Fatalf("expected file unchanged, got name %q", unchanged.Name)
	}
}

func TestUpdateVirtualFileChangesExistingFile(t *testing.T) {
	tDB := setupVirtualFileTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())

	file := createVirtualFile(t, tDB.db, 1, "old.txt")

	if err := svc.Update(ctx, file.ID, []utils.Field{utils.WithField("name", "new.txt")}); err != nil {
		t.Fatalf("update virtual file: %v", err)
	}

	var updated models.VirtualFile
	if err := tDB.db.First(&updated, file.ID).Error; err != nil {
		t.Fatalf("query updated file: %v", err)
	}

	if updated.Name != "new.txt" {
		t.Fatalf("expected updated name new.txt, got %q", updated.Name)
	}
}

func TestUpdateVirtualFileAllowsNoopUpdate(t *testing.T) {
	tDB := setupVirtualFileTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())

	file := createVirtualFile(t, tDB.db, 1, "same.txt")

	if err := svc.Update(ctx, file.ID, []utils.Field{utils.WithField("name", file.Name)}); err != nil {
		t.Fatalf("update virtual file with same value: %v", err)
	}
}

func TestModifyAdditionRejectsInvalidID(t *testing.T) {
	tDB := setupVirtualFileTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())

	err := svc.ModifyAddition(ctx, 0, "key", "value")
	if !errors.Is(err, errInvalidVirtualFileID) {
		t.Fatalf("expected invalid virtual file id, got %v", err)
	}
}

func TestModifyAdditionReturnsNotFoundWhenFileMissing(t *testing.T) {
	tDB := setupVirtualFileTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())

	err := svc.ModifyAddition(ctx, 99999, "key", "value")
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("expected record not found, got %v", err)
	}
}

func TestBatchUpdateRejectsInvalidID(t *testing.T) {
	tDB := setupVirtualFileTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())

	err := svc.BatchUpdate(ctx, map[int64][]utils.Field{
		0: {utils.WithField("name", "invalid.txt")},
	})
	if !errors.Is(err, errInvalidVirtualFileID) {
		t.Fatalf("expected invalid virtual file id, got %v", err)
	}
}

func TestBatchUpdateRejectsEmptyFields(t *testing.T) {
	tDB := setupVirtualFileTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())

	file := createVirtualFile(t, tDB.db, 1, "batch-empty.txt")

	err := svc.BatchUpdate(ctx, map[int64][]utils.Field{
		file.ID: nil,
	})
	if !errors.Is(err, errEmptyVirtualFileUpdateFields) {
		t.Fatalf("expected empty virtual file update fields, got %v", err)
	}

	if count := countVirtualFiles(t, tDB.db, "id = ? AND name = ?", file.ID, "batch-empty.txt"); count != 1 {
		t.Fatalf("expected file to remain unchanged, got count %d", count)
	}
}

func TestBatchUpdateAllowsNoopUpdate(t *testing.T) {
	tDB := setupVirtualFileTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())

	first := createVirtualFile(t, tDB.db, 1, "batch-same-first.txt")
	second := createVirtualFile(t, tDB.db, 1, "batch-same-second.txt")

	err := svc.BatchUpdate(ctx, map[int64][]utils.Field{
		first.ID:  {utils.WithField("name", first.Name)},
		second.ID: {utils.WithField("name", "batch-updated-second.txt")},
	})
	if err != nil {
		t.Fatalf("batch update with noop item: %v", err)
	}

	var updatedSecond models.VirtualFile
	if err := tDB.db.First(&updatedSecond, second.ID).Error; err != nil {
		t.Fatalf("query updated file: %v", err)
	}

	if updatedSecond.Name != "batch-updated-second.txt" {
		t.Fatalf("expected second file updated, got %q", updatedSecond.Name)
	}
}

func TestBatchUpdateReturnsNotFoundWhenAnyFileMissing(t *testing.T) {
	tDB := setupVirtualFileTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())

	file := createVirtualFile(t, tDB.db, 1, "batch.txt")

	err := svc.BatchUpdate(ctx, map[int64][]utils.Field{
		file.ID: {utils.WithField("name", "updated.txt")},
		99999:   {utils.WithField("name", "missing.txt")},
	})
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("expected record not found, got %v", err)
	}
}

func TestBatchUpdatePlusRejectsEmptyConditionsWithoutUpdatingFiles(t *testing.T) {
	tDB := setupVirtualFileTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())

	first := createVirtualFile(t, tDB.db, 1, "first.txt")
	second := createVirtualFile(t, tDB.db, 2, "second.txt")

	err := svc.BatchUpdatePlus(ctx, []utils.Field{utils.WithField("name", "updated.txt")}, nil)
	if !errors.Is(err, errEmptyVirtualFileUpdateConditions) {
		t.Fatalf("expected empty virtual file update conditions, got %v", err)
	}

	if count := countVirtualFiles(t, tDB.db, "id IN ? AND name = ?", []int64{first.ID, second.ID}, "updated.txt"); count != 0 {
		t.Fatalf("expected no files to be updated, got count %d", count)
	}
}

func TestBatchUpdatePlusRejectsNilConditionWithoutUpdatingFiles(t *testing.T) {
	tDB := setupVirtualFileTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())

	file := createVirtualFile(t, tDB.db, 1, "nil-condition.txt")

	err := svc.BatchUpdatePlus(ctx, []utils.Field{utils.WithField("name", "updated.txt")}, []clause.Expression{nil})
	if !errors.Is(err, errEmptyVirtualFileUpdateConditions) {
		t.Fatalf("expected empty virtual file update conditions, got %v", err)
	}

	if count := countVirtualFiles(t, tDB.db, "id = ? AND name = ?", file.ID, "nil-condition.txt"); count != 1 {
		t.Fatalf("expected file to remain unchanged, got count %d", count)
	}
}

func TestBatchUpdatePlusUpdatesMatchingFilesOnly(t *testing.T) {
	tDB := setupVirtualFileTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())

	matched := createVirtualFile(t, tDB.db, 1, "matched.txt")
	unchanged := createVirtualFile(t, tDB.db, 2, "unchanged.txt")

	err := svc.BatchUpdatePlus(
		ctx,
		[]utils.Field{utils.WithField("name", "updated.txt")},
		[]clause.Expression{clause.Eq{Column: "parent_id", Value: int64(1)}},
	)
	if err != nil {
		t.Fatalf("batch update plus: %v", err)
	}

	if count := countVirtualFiles(t, tDB.db, "id = ? AND name = ?", matched.ID, "updated.txt"); count != 1 {
		t.Fatalf("expected matched file to be updated, got count %d", count)
	}

	if count := countVirtualFiles(t, tDB.db, "id = ? AND name = ?", unchanged.ID, "unchanged.txt"); count != 1 {
		t.Fatalf("expected unmatched file to remain unchanged, got count %d", count)
	}
}

func TestBatchUpdatePlusReturnsNotFoundWhenNoFilesMatch(t *testing.T) {
	tDB := setupVirtualFileTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())

	file := createVirtualFile(t, tDB.db, 1, "unmatched.txt")

	err := svc.BatchUpdatePlus(
		ctx,
		[]utils.Field{utils.WithField("name", "updated.txt")},
		[]clause.Expression{clause.Eq{Column: "parent_id", Value: int64(99999)}},
	)
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("expected record not found, got %v", err)
	}

	if count := countVirtualFiles(t, tDB.db, "id = ? AND name = ?", file.ID, "unmatched.txt"); count != 1 {
		t.Fatalf("expected file to remain unchanged, got count %d", count)
	}
}

func TestBatchUpdatePlusAllowsNoopUpdateWhenFilesMatch(t *testing.T) {
	tDB := setupVirtualFileTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())

	file := createVirtualFile(t, tDB.db, 1, "same-batch-plus.txt")

	err := svc.BatchUpdatePlus(
		ctx,
		[]utils.Field{utils.WithField("name", file.Name)},
		[]clause.Expression{clause.Eq{Column: "id", Value: file.ID}},
	)
	if err != nil {
		t.Fatalf("expected no-op batch update plus to succeed, got %v", err)
	}
}

func TestVirtualFilePathCalculationRejectsParentCycle(t *testing.T) {
	tDB := setupVirtualFileTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())

	first := createVirtualDir(t, tDB.db, 0, "first")
	second := createVirtualDir(t, tDB.db, first.ID, "second")

	if err := tDB.db.Model(&models.VirtualFile{}).
		Where("id = ?", first.ID).
		Update("parent_id", second.ID).Error; err != nil {
		t.Fatalf("create parent cycle: %v", err)
	}

	if _, err := svc.BatchQueryParentFiles(ctx, first.ID); !errors.Is(err, errVirtualFilePathCycle) {
		t.Fatalf("expected parent query cycle error, got %v", err)
	}

	if _, err := svc.CalFilePath(ctx, first.ID); !errors.Is(err, errVirtualFilePathCycle) {
		t.Fatalf("expected cal file path cycle error, got %v", err)
	}

	if _, err := svc.CalFullPath(ctx, first.ID); !errors.Is(err, errVirtualFilePathCycle) {
		t.Fatalf("expected cal full path cycle error, got %v", err)
	}
}

func TestBatchQueryParentFilesReturnsNotFoundForMissingStart(t *testing.T) {
	tDB := setupVirtualFileTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())

	files, err := svc.BatchQueryParentFiles(ctx, 99999)
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("expected record not found, got files=%#v err=%v", files, err)
	}

	if path, err := svc.CalFilePath(ctx, 99999); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("expected cal file path record not found, got path=%q err=%v", path, err)
	}
}

func TestBatchQueryParentFilesReturnsNotFoundForBrokenParentChain(t *testing.T) {
	tDB := setupVirtualFileTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())

	file := createVirtualFile(t, tDB.db, 99999, "orphan.txt")

	files, err := svc.BatchQueryParentFiles(ctx, file.ID)
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("expected record not found for broken parent chain, got files=%#v err=%v", files, err)
	}

	if path, err := svc.CalFilePath(ctx, file.ID); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("expected cal file path record not found for broken parent chain, got path=%q err=%v", path, err)
	}
}
