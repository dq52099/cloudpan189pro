package group2file

import (
	stdctx "context"
	"errors"
	"reflect"
	"sort"
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

type group2FileTestDB struct {
	db *gorm.DB
}

func (t *group2FileTestDB) GetDB(ctx context.Context) *gorm.DB {
	return t.db.WithContext(ctx)
}

func (t *group2FileTestDB) GetDBWithoutContext() *gorm.DB {
	return t.db
}

func (t *group2FileTestDB) GetLogger(name string, fields ...zap.Field) *zap.Logger {
	return zap.NewNop()
}

func (t *group2FileTestDB) Close() {}

func (t *group2FileTestDB) GetPort() int {
	return 9999
}

func (t *group2FileTestDB) GetTaskEngine() taskengine.TaskEngine {
	return nil
}

func (t *group2FileTestDB) GetHTTPEngine() *gin.Engine {
	return nil
}

var _ bootstrap.ServiceContext = (*group2FileTestDB)(nil)

func setupGroup2FileTestDB(t *testing.T) *group2FileTestDB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}

	if err := db.AutoMigrate(&models.Group2File{}); err != nil {
		t.Fatalf("migrate test db: %v", err)
	}

	return &group2FileTestDB{db: db}
}

func createGroupFileBinding(t *testing.T, db *gorm.DB, groupID, fileID int64) {
	t.Helper()

	if err := db.Create(&models.Group2File{GroupId: groupID, FileId: fileID}).Error; err != nil {
		t.Fatalf("create group file binding: %v", err)
	}
}

func TestBatchBindFilesReplacesExistingBindingsAndDeduplicates(t *testing.T) {
	tDB := setupGroup2FileTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())

	createGroupFileBinding(t, tDB.db, 10, 1001)
	createGroupFileBinding(t, tDB.db, 10, 1002)
	createGroupFileBinding(t, tDB.db, 20, 2001)

	if err := svc.BatchBindFiles(ctx, 10, []int64{3001, 3001, 3002}); err != nil {
		t.Fatalf("batch bind files: %v", err)
	}

	fileIDs, err := svc.GetBindFiles(ctx, 10)
	if err != nil {
		t.Fatalf("get bind files: %v", err)
	}

	sort.Slice(fileIDs, func(i, j int) bool {
		return fileIDs[i] < fileIDs[j]
	})

	if !reflect.DeepEqual(fileIDs, []int64{3001, 3002}) {
		t.Fatalf("expected deduped file ids [3001 3002], got %v", fileIDs)
	}

	otherFileIDs, err := svc.GetBindFiles(ctx, 20)
	if err != nil {
		t.Fatalf("get other group bind files: %v", err)
	}

	if !reflect.DeepEqual(otherFileIDs, []int64{2001}) {
		t.Fatalf("expected other group unchanged, got %v", otherFileIDs)
	}
}

func TestBatchBindFilesAllowsClearingAllBindings(t *testing.T) {
	tDB := setupGroup2FileTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())

	createGroupFileBinding(t, tDB.db, 10, 1001)
	createGroupFileBinding(t, tDB.db, 10, 1002)
	createGroupFileBinding(t, tDB.db, 20, 2001)

	if err := svc.BatchBindFiles(ctx, 10, nil); err != nil {
		t.Fatalf("clear bind files: %v", err)
	}

	fileIDs, err := svc.GetBindFiles(ctx, 10)
	if err != nil {
		t.Fatalf("get bind files: %v", err)
	}

	if len(fileIDs) != 0 {
		t.Fatalf("expected group bindings cleared, got %v", fileIDs)
	}

	otherFileIDs, err := svc.GetBindFiles(ctx, 20)
	if err != nil {
		t.Fatalf("get other group bind files: %v", err)
	}

	if !reflect.DeepEqual(otherFileIDs, []int64{2001}) {
		t.Fatalf("expected other group unchanged, got %v", otherFileIDs)
	}
}

func TestBatchBindFilesRejectsInvalidFileIDWithoutChangingBindings(t *testing.T) {
	tDB := setupGroup2FileTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())

	createGroupFileBinding(t, tDB.db, 10, 1001)
	createGroupFileBinding(t, tDB.db, 10, 1002)

	err := svc.BatchBindFiles(ctx, 10, []int64{3001, 0})
	if err == nil {
		t.Fatal("expected invalid file id to fail")
	}

	if !errors.Is(err, errInvalidFileID) {
		t.Fatalf("expected invalid file id error, got %v", err)
	}

	fileIDs, err := svc.GetBindFiles(ctx, 10)
	if err != nil {
		t.Fatalf("get bind files: %v", err)
	}

	sort.Slice(fileIDs, func(i, j int) bool {
		return fileIDs[i] < fileIDs[j]
	})

	if !reflect.DeepEqual(fileIDs, []int64{1001, 1002}) {
		t.Fatalf("expected existing bindings unchanged, got %v", fileIDs)
	}
}

func TestBatchBindFilesRejectsInvalidGroupIDWithoutChangingBindings(t *testing.T) {
	tDB := setupGroup2FileTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())

	createGroupFileBinding(t, tDB.db, 10, 1001)

	err := svc.BatchBindFiles(ctx, 0, []int64{3001})
	if !errors.Is(err, errInvalidGroupID) {
		t.Fatalf("expected invalid group id, got %v", err)
	}

	fileIDs, err := svc.GetBindFiles(ctx, 10)
	if err != nil {
		t.Fatalf("get bind files: %v", err)
	}

	if !reflect.DeepEqual(fileIDs, []int64{1001}) {
		t.Fatalf("expected existing bindings unchanged, got %v", fileIDs)
	}
}

func TestGetBindFilesRejectsInvalidGroupID(t *testing.T) {
	tDB := setupGroup2FileTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())

	_, err := svc.GetBindFiles(ctx, 0)
	if !errors.Is(err, errInvalidGroupID) {
		t.Fatalf("expected invalid group id, got %v", err)
	}
}

func TestCheckPermissionRejectsInvalidIDs(t *testing.T) {
	tDB := setupGroup2FileTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())

	tests := []struct {
		name    string
		groupID int64
		fileID  int64
		wantErr error
	}{
		{name: "invalid group", groupID: 0, fileID: 1001, wantErr: errInvalidGroupID},
		{name: "invalid file", groupID: 10, fileID: 0, wantErr: errInvalidFileID},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ok, err := svc.CheckPermission(ctx, tt.groupID, tt.fileID)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("expected %v, got %v", tt.wantErr, err)
			}

			if ok {
				t.Fatal("expected permission check to be false")
			}
		})
	}
}

func TestCheckPermissionReturnsBindingState(t *testing.T) {
	tDB := setupGroup2FileTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())

	createGroupFileBinding(t, tDB.db, 10, 1001)

	ok, err := svc.CheckPermission(ctx, 10, 1001)
	if err != nil {
		t.Fatalf("check existing permission: %v", err)
	}

	if !ok {
		t.Fatal("expected permission to exist")
	}

	ok, err = svc.CheckPermission(ctx, 10, 1002)
	if err != nil {
		t.Fatalf("check missing permission: %v", err)
	}

	if ok {
		t.Fatal("expected missing permission to be false")
	}
}
