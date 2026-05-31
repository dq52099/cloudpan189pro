package mountpoint

import (
	stdctx "context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/xxcheng123/cloudpan189-share/internal/bootstrap"
	"github.com/xxcheng123/cloudpan189-share/internal/framework/context"
	"github.com/xxcheng123/cloudpan189-share/internal/pkgs/taskengine"
	"github.com/xxcheng123/cloudpan189-share/internal/repository/models"
	userMountPointTokenSvi "github.com/xxcheng123/cloudpan189-share/internal/services/userMountPointToken"
	"github.com/xxcheng123/cloudpan189-share/internal/types/topic"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type mountPointTestDB struct {
	db *gorm.DB
}

func (t *mountPointTestDB) GetDB(ctx context.Context) *gorm.DB {
	return t.db.WithContext(ctx)
}

func (t *mountPointTestDB) GetDBWithoutContext() *gorm.DB {
	return t.db
}

func (t *mountPointTestDB) GetLogger(name string, fields ...zap.Field) *zap.Logger {
	return zap.NewNop()
}

func (t *mountPointTestDB) Close() {}

func (t *mountPointTestDB) GetPort() int {
	return 9999
}

func (t *mountPointTestDB) GetTaskEngine() taskengine.TaskEngine {
	return nil
}

func (t *mountPointTestDB) GetHTTPEngine() *gin.Engine {
	return nil
}

var _ bootstrap.ServiceContext = (*mountPointTestDB)(nil)

func setupMountPointTestDB(t *testing.T) *mountPointTestDB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}

	if err := db.AutoMigrate(&models.MountPoint{}, &models.CloudToken{}, &models.UserMountPointToken{}); err != nil {
		t.Fatalf("migrate test db: %v", err)
	}

	return &mountPointTestDB{db: db}
}

func createMountPoint(t *testing.T, db *gorm.DB, fileID, creatorUserID int64, name string) *models.MountPoint {
	t.Helper()

	mountPoint := &models.MountPoint{
		FileId:        fileID,
		OsType:        models.OsTypeFolder,
		TokenId:       1,
		CreatorUserID: creatorUserID,
		Name:          name,
		FullPath:      "/" + name,
	}
	if err := db.Create(mountPoint).Error; err != nil {
		t.Fatalf("create mount point: %v", err)
	}

	return mountPoint
}

func createCloudToken(t *testing.T, db *gorm.DB, id int64, userIDs ...int64) *models.CloudToken {
	t.Helper()

	userID := int64(1)
	if len(userIDs) > 0 {
		userID = userIDs[0]
	}

	token := &models.CloudToken{
		ID:          id,
		Name:        fmt.Sprintf("token-%d", id),
		AccessToken: fmt.Sprintf("access-token-%d", id),
		ExpiresIn:   7200,
		UserID:      userID,
	}
	if err := db.Create(token).Error; err != nil {
		t.Fatalf("create cloud token: %v", err)
	}

	return token
}

func TestQueryByIDUsesMountPointPrimaryKey(t *testing.T) {
	tDB := setupMountPointTestDB(t)
	svc := NewService(tDB, nil, nil, nil)
	ctx := context.NewContext(stdctx.Background())

	mountPoint := createMountPoint(t, tDB.db, 9901, 10, "query-by-id")

	got, err := svc.QueryByID(ctx, mountPoint.ID)
	if err != nil {
		t.Fatalf("query by id: %v", err)
	}

	if got.ID != mountPoint.ID || got.FileId != 9901 {
		t.Fatalf("expected mount point id=%d file_id=9901, got id=%d file_id=%d", mountPoint.ID, got.ID, got.FileId)
	}

	if _, err := svc.QueryByID(ctx, 9901); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("expected file id lookup through QueryByID to miss, got %v", err)
	}
}

func createUserMountPointTokenBinding(t *testing.T, db *gorm.DB, userID, mountPointID, tokenID int64) *models.UserMountPointToken {
	t.Helper()

	binding := &models.UserMountPointToken{
		UserID:       userID,
		MountPointID: mountPointID,
		TokenID:      tokenID,
	}
	if err := db.Create(binding).Error; err != nil {
		t.Fatalf("create user mount point token binding: %v", err)
	}

	return binding
}

func countUserMountPointTokenBindings(t *testing.T, db *gorm.DB, query string, args ...any) int64 {
	t.Helper()

	var count int64
	if err := db.Model(&models.UserMountPointToken{}).Where(query, args...).Count(&count).Error; err != nil {
		t.Fatalf("count user mount point token bindings: %v", err)
	}

	return count
}

func TestListIncludesMountPointsBoundToUserToken(t *testing.T) {
	tDB := setupMountPointTestDB(t)
	userTokenSvc := userMountPointTokenSvi.NewService(tDB)
	svc := NewService(tDB, nil, nil, userTokenSvc)
	ctx := context.NewContext(stdctx.Background())

	owned := createMountPoint(t, tDB.db, 3001, 10, "owned")
	groupShared := createMountPoint(t, tDB.db, 3002, 20, "group-shared")
	bound := createMountPoint(t, tDB.db, 3003, 30, "bound")
	hidden := createMountPoint(t, tDB.db, 3004, 40, "hidden")
	createCloudToken(t, tDB.db, 77)

	if err := userTokenSvc.BindToken(ctx, 10, bound.ID, 77); err != nil {
		t.Fatalf("bind token: %v", err)
	}

	list, err := svc.List(ctx, &ListRequest{
		UserID:       10,
		GroupFileIds: []int64{groupShared.FileId},
		NoPaginate:   true,
	})
	if err != nil {
		t.Fatalf("list mount points: %v", err)
	}

	got := map[int64]bool{}
	for _, item := range list {
		got[item.FileId] = true
	}

	for _, expected := range []int64{owned.FileId, groupShared.FileId, bound.FileId} {
		if !got[expected] {
			t.Fatalf("expected file %d in list, got %+v", expected, got)
		}
	}

	if got[hidden.FileId] {
		t.Fatalf("expected hidden file %d to be excluded, got %+v", hidden.FileId, got)
	}

	count, err := svc.Count(ctx, &ListRequest{
		UserID:       10,
		GroupFileIds: []int64{groupShared.FileId},
	})
	if err != nil {
		t.Fatalf("count mount points: %v", err)
	}

	if count != 3 {
		t.Fatalf("expected count 3, got %d", count)
	}
}

func TestListNonAdminAppliesFiltersToAllAccessibleSources(t *testing.T) {
	tDB := setupMountPointTestDB(t)
	userTokenSvc := userMountPointTokenSvi.NewService(tDB)
	svc := NewService(tDB, nil, nil, userTokenSvc)
	ctx := context.NewContext(stdctx.Background())

	ownedMatch := createMountPoint(t, tDB.db, 3011, 10, "movie-owned")
	groupMatch := createMountPoint(t, tDB.db, 3012, 20, "movie-group")
	boundMatch := createMountPoint(t, tDB.db, 3013, 30, "movie-bound")
	groupNoMatch := createMountPoint(t, tDB.db, 3014, 20, "music-group")
	boundNoMatch := createMountPoint(t, tDB.db, 3015, 30, "music-bound")
	hiddenMatch := createMountPoint(t, tDB.db, 3016, 40, "movie-hidden")
	createCloudToken(t, tDB.db, 77)

	if err := userTokenSvc.BindToken(ctx, 10, boundMatch.ID, 77); err != nil {
		t.Fatalf("bind token for matching mount point: %v", err)
	}

	if err := userTokenSvc.BindToken(ctx, 10, boundNoMatch.ID, 77); err != nil {
		t.Fatalf("bind token for non-matching mount point: %v", err)
	}

	req := &ListRequest{
		UserID:       10,
		GroupFileIds: []int64{groupMatch.FileId, groupNoMatch.FileId},
		Name:         "movie",
		NoPaginate:   true,
	}

	list, err := svc.List(ctx, req)
	if err != nil {
		t.Fatalf("list mount points: %v", err)
	}

	got := map[int64]bool{}
	for _, item := range list {
		got[item.FileId] = true
	}

	for _, expected := range []int64{ownedMatch.FileId, groupMatch.FileId, boundMatch.FileId} {
		if !got[expected] {
			t.Fatalf("expected file %d in list, got %+v", expected, got)
		}
	}

	for _, unexpected := range []int64{groupNoMatch.FileId, boundNoMatch.FileId, hiddenMatch.FileId} {
		if got[unexpected] {
			t.Fatalf("expected file %d to be excluded by filters, got %+v", unexpected, got)
		}
	}

	count, err := svc.Count(ctx, &ListRequest{
		UserID:       10,
		GroupFileIds: []int64{groupMatch.FileId, groupNoMatch.FileId},
		Name:         "movie",
	})
	if err != nil {
		t.Fatalf("count mount points: %v", err)
	}

	if count != 3 {
		t.Fatalf("expected count 3, got %d", count)
	}
}

func TestGetAccessibleMountPointIDsRejectsInvalidNonAdminUserID(t *testing.T) {
	tDB := setupMountPointTestDB(t)
	svc := NewService(tDB, nil, nil, nil)
	ctx := context.NewContext(stdctx.Background())

	createMountPoint(t, tDB.db, 3101, 10, "private")

	ids, err := svc.GetAccessibleMountPointIDs(ctx, 0, false, nil)
	if !errors.Is(err, errInvalidMountPointUserID) {
		t.Fatalf("expected invalid mount point user id, got %v", err)
	}

	if len(ids) != 0 {
		t.Fatalf("expected no accessible ids, got %v", ids)
	}
}

func TestGetAccessibleMountPointIDsRejectsInvalidGroupFileID(t *testing.T) {
	tDB := setupMountPointTestDB(t)
	svc := NewService(tDB, nil, nil, nil)
	ctx := context.NewContext(stdctx.Background())

	_, err := svc.GetAccessibleMountPointIDs(ctx, 10, false, []int64{3101, 0})
	if !errors.Is(err, errInvalidMountPointFileID) {
		t.Fatalf("expected invalid mount point file id, got %v", err)
	}
}

func TestGetAccessibleMountPointIDsIncludesOwnedGroupAndBoundMountPoints(t *testing.T) {
	tDB := setupMountPointTestDB(t)
	userTokenSvc := userMountPointTokenSvi.NewService(tDB)
	svc := NewService(tDB, nil, nil, userTokenSvc)
	ctx := context.NewContext(stdctx.Background())

	owned := createMountPoint(t, tDB.db, 3201, 10, "accessible-owned")
	groupShared := createMountPoint(t, tDB.db, 3202, 20, "accessible-group")
	bound := createMountPoint(t, tDB.db, 3203, 30, "accessible-bound")
	hidden := createMountPoint(t, tDB.db, 3204, 40, "accessible-hidden")
	createCloudToken(t, tDB.db, 88)

	if err := userTokenSvc.BindToken(ctx, 10, bound.ID, 88); err != nil {
		t.Fatalf("bind token: %v", err)
	}

	ids, err := svc.GetAccessibleMountPointIDs(ctx, 10, false, []int64{groupShared.FileId, groupShared.FileId})
	if err != nil {
		t.Fatalf("get accessible mount point ids: %v", err)
	}

	got := map[int64]bool{}
	for _, id := range ids {
		got[id] = true
	}

	for _, expected := range []int64{owned.FileId, groupShared.FileId, bound.FileId} {
		if !got[expected] {
			t.Fatalf("expected file %d in accessible ids, got %+v", expected, got)
		}
	}

	if got[hidden.FileId] {
		t.Fatalf("expected hidden file %d to be excluded, got %+v", hidden.FileId, got)
	}

	if len(ids) != 3 {
		t.Fatalf("expected 3 unique accessible ids, got %v", ids)
	}
}

func TestGetAccessibleMountPointIDsAdminCanQueryWithoutUserID(t *testing.T) {
	tDB := setupMountPointTestDB(t)
	svc := NewService(tDB, nil, nil, nil)
	ctx := context.NewContext(stdctx.Background())

	first := createMountPoint(t, tDB.db, 3301, 10, "admin-accessible-first")
	second := createMountPoint(t, tDB.db, 3302, 20, "admin-accessible-second")

	ids, err := svc.GetAccessibleMountPointIDs(ctx, 0, true, nil)
	if err != nil {
		t.Fatalf("admin get accessible mount point ids: %v", err)
	}

	got := map[int64]bool{}
	for _, id := range ids {
		got[id] = true
	}

	for _, expected := range []int64{first.FileId, second.FileId} {
		if !got[expected] {
			t.Fatalf("expected file %d in admin accessible ids, got %+v", expected, got)
		}
	}
}

func TestListRejectsMissingUserIDWhenRequestNil(t *testing.T) {
	tDB := setupMountPointTestDB(t)
	svc := NewService(tDB, nil, nil, nil)
	ctx := context.NewContext(stdctx.Background())

	createMountPoint(t, tDB.db, 3999, 10, "missing-user-nil")

	list, err := svc.List(ctx, nil)
	if !errors.Is(err, errInvalidMountPointUserID) {
		t.Fatalf("expected invalid mount point user id, got %v", err)
	}

	if len(list) != 0 {
		t.Fatalf("expected no list, got %v", list)
	}
}

func TestCountRejectsMissingUserIDWhenRequestEmpty(t *testing.T) {
	tDB := setupMountPointTestDB(t)
	svc := NewService(tDB, nil, nil, nil)
	ctx := context.NewContext(stdctx.Background())

	createMountPoint(t, tDB.db, 3998, 10, "missing-user-empty")

	count, err := svc.Count(ctx, &ListRequest{})
	if !errors.Is(err, errInvalidMountPointUserID) {
		t.Fatalf("expected invalid mount point user id, got %v", err)
	}

	if count != 0 {
		t.Fatalf("expected zero count, got %d", count)
	}
}

func TestListDefaultsPaginationForAdmin(t *testing.T) {
	tDB := setupMountPointTestDB(t)
	svc := NewService(tDB, nil, nil, nil)
	ctx := context.NewContext(stdctx.Background())

	for i := 0; i < 12; i++ {
		createMountPoint(t, tDB.db, int64(4000+i), 10, fmt.Sprintf("default-%03d", i))
	}

	list, err := svc.List(ctx, &ListRequest{IsAdmin: true})
	if err != nil {
		t.Fatalf("list mount points: %v", err)
	}

	if len(list) != defaultMountPointPageSize {
		t.Fatalf("expected default page size %d, got %d", defaultMountPointPageSize, len(list))
	}
}

func TestListDefaultsPaginationWhenPageValuesMissing(t *testing.T) {
	tDB := setupMountPointTestDB(t)
	svc := NewService(tDB, nil, nil, nil)
	ctx := context.NewContext(stdctx.Background())

	for i := 0; i < 12; i++ {
		createMountPoint(t, tDB.db, int64(4100+i), 10, fmt.Sprintf("missing-%03d", i))
	}

	req := &ListRequest{IsAdmin: true}

	list, err := svc.List(ctx, req)
	if err != nil {
		t.Fatalf("list mount points: %v", err)
	}

	if len(list) != defaultMountPointPageSize {
		t.Fatalf("expected default page size %d, got %d", defaultMountPointPageSize, len(list))
	}

	if req.CurrentPage != defaultMountPointCurrentPage {
		t.Fatalf("expected current page normalized to %d, got %d", defaultMountPointCurrentPage, req.CurrentPage)
	}

	if req.PageSize != defaultMountPointPageSize {
		t.Fatalf("expected page size normalized to %d, got %d", defaultMountPointPageSize, req.PageSize)
	}
}

func TestListCapsPageSize(t *testing.T) {
	tDB := setupMountPointTestDB(t)
	svc := NewService(tDB, nil, nil, nil)
	ctx := context.NewContext(stdctx.Background())

	for i := 0; i < 3; i++ {
		createMountPoint(t, tDB.db, int64(4200+i), 10, fmt.Sprintf("cap-%03d", i))
	}

	req := &ListRequest{
		CurrentPage: 1,
		PageSize:    maxMountPointPageSize + 100,
		IsAdmin:     true,
	}

	if _, err := svc.List(ctx, req); err != nil {
		t.Fatalf("list mount points: %v", err)
	}

	if req.PageSize != maxMountPointPageSize {
		t.Fatalf("expected page size capped to %d, got %d", maxMountPointPageSize, req.PageSize)
	}
}

func TestListFiltersByFileIdList(t *testing.T) {
	tDB := setupMountPointTestDB(t)
	svc := NewService(tDB, nil, nil, nil)
	ctx := context.NewContext(stdctx.Background())

	first := createMountPoint(t, tDB.db, 4301, 10, "file-list-first")
	second := createMountPoint(t, tDB.db, 4302, 10, "file-list-second")
	hidden := createMountPoint(t, tDB.db, 4303, 10, "file-list-hidden")

	list, err := svc.List(ctx, &ListRequest{
		FileIdList: []int64{first.FileId, second.FileId, first.FileId},
		NoPaginate: true,
		IsAdmin:    true,
	})
	if err != nil {
		t.Fatalf("list mount points by file ids: %v", err)
	}

	got := map[int64]bool{}
	for _, item := range list {
		got[item.FileId] = true
	}

	for _, expected := range []int64{first.FileId, second.FileId} {
		if !got[expected] {
			t.Fatalf("expected file %d in list, got %+v", expected, got)
		}
	}

	if got[hidden.FileId] {
		t.Fatalf("expected hidden file %d to be excluded, got %+v", hidden.FileId, got)
	}

	count, err := svc.Count(ctx, &ListRequest{FileIdList: []int64{first.FileId, second.FileId}, IsAdmin: true})
	if err != nil {
		t.Fatalf("count mount points by file ids: %v", err)
	}

	if count != 2 {
		t.Fatalf("expected count 2, got %d", count)
	}
}

func TestListEmptyFileIdListReturnsEmpty(t *testing.T) {
	tDB := setupMountPointTestDB(t)
	svc := NewService(tDB, nil, nil, nil)
	ctx := context.NewContext(stdctx.Background())

	createMountPoint(t, tDB.db, 4311, 10, "empty-file-list-visible")

	list, err := svc.List(ctx, &ListRequest{
		FileIdList: []int64{},
		NoPaginate: true,
		IsAdmin:    true,
	})
	if err != nil {
		t.Fatalf("list mount points by empty file ids: %v", err)
	}

	if len(list) != 0 {
		t.Fatalf("expected empty list for empty file id filter, got %+v", list)
	}

	count, err := svc.Count(ctx, &ListRequest{FileIdList: []int64{}, IsAdmin: true})
	if err != nil {
		t.Fatalf("count mount points by empty file ids: %v", err)
	}

	if count != 0 {
		t.Fatalf("expected count 0 for empty file id filter, got %d", count)
	}
}

func TestListRejectsInvalidIDFilters(t *testing.T) {
	tDB := setupMountPointTestDB(t)
	svc := NewService(tDB, nil, nil, nil)
	ctx := context.NewContext(stdctx.Background())

	invalidFileID := int64(0)
	invalidTokenID := int64(-1)

	tests := []struct {
		name    string
		req     *ListRequest
		wantErr error
	}{
		{name: "invalid file id", req: &ListRequest{FileId: &invalidFileID, IsAdmin: true}, wantErr: errInvalidMountPointFileID},
		{name: "invalid file id list", req: &ListRequest{FileIdList: []int64{100, 0}, IsAdmin: true}, wantErr: errInvalidMountPointFileID},
		{name: "invalid token id", req: &ListRequest{TokenId: &invalidTokenID, IsAdmin: true}, wantErr: errInvalidMountPointTokenID},
		{name: "invalid group file ids", req: &ListRequest{UserID: 10, GroupFileIds: []int64{100, 0}}, wantErr: errInvalidMountPointFileID},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := svc.List(ctx, tt.req)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("expected %v, got %v", tt.wantErr, err)
			}
		})
	}
}

func TestCountRejectsInvalidIDFilters(t *testing.T) {
	tDB := setupMountPointTestDB(t)
	svc := NewService(tDB, nil, nil, nil)
	ctx := context.NewContext(stdctx.Background())

	invalidFileID := int64(-1)

	_, err := svc.Count(ctx, &ListRequest{FileId: &invalidFileID, IsAdmin: true})
	if !errors.Is(err, errInvalidMountPointFileID) {
		t.Fatalf("expected invalid mount point file id, got %v", err)
	}

	_, err = svc.Count(ctx, &ListRequest{FileIdList: []int64{100, -1}, IsAdmin: true})
	if !errors.Is(err, errInvalidMountPointFileID) {
		t.Fatalf("expected invalid mount point file id, got %v", err)
	}
}

func TestCreateRejectsInvalidRequest(t *testing.T) {
	tDB := setupMountPointTestDB(t)
	svc := NewService(tDB, nil, nil, nil)
	ctx := context.NewContext(stdctx.Background())

	tests := []struct {
		name    string
		req     *CreateRequest
		wantErr error
	}{
		{name: "nil request", req: nil, wantErr: errInvalidMountPointFileID},
		{name: "zero file id", req: &CreateRequest{FileId: 0, FullPath: "/invalid"}, wantErr: errInvalidMountPointFileID},
		{name: "negative file id", req: &CreateRequest{FileId: -1, FullPath: "/invalid"}, wantErr: errInvalidMountPointFileID},
		{name: "negative token id", req: &CreateRequest{FileId: 1001, FullPath: "/invalid", TokenId: -1}, wantErr: errInvalidMountPointTokenID},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id, err := svc.Create(ctx, tt.req)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("expected %v, got %v", tt.wantErr, err)
			}

			if id != 0 {
				t.Fatalf("expected zero id, got %d", id)
			}
		})
	}
}

func TestDeleteAdminMountPointRemovesExistingMountPoint(t *testing.T) {
	tDB := setupMountPointTestDB(t)
	svc := NewService(tDB, nil, nil, nil)
	ctx := context.NewContext(stdctx.Background())

	mountPoint := createMountPoint(t, tDB.db, 1001, 10, "admin-delete")
	createUserMountPointTokenBinding(t, tDB.db, 20, mountPoint.ID, 100)

	if err := svc.Delete(ctx, &DeleteRequest{FileId: mountPoint.FileId, IsAdmin: true}); err != nil {
		t.Fatalf("admin delete mount point: %v", err)
	}

	var count int64
	if err := tDB.db.Model(&models.MountPoint{}).Where("file_id = ?", mountPoint.FileId).Count(&count).Error; err != nil {
		t.Fatalf("count mount point: %v", err)
	}

	if count != 0 {
		t.Fatalf("expected mount point deleted, got count %d", count)
	}

	if count := countUserMountPointTokenBindings(t, tDB.db, "mount_point_id = ?", mountPoint.ID); count != 0 {
		t.Fatalf("expected user token bindings deleted, got count %d", count)
	}
}

func TestDeleteAdminMountPointReturnsNotFoundWhenMissing(t *testing.T) {
	tDB := setupMountPointTestDB(t)
	svc := NewService(tDB, nil, nil, nil)
	ctx := context.NewContext(stdctx.Background())

	err := svc.Delete(ctx, &DeleteRequest{FileId: 99999, IsAdmin: true})
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("expected record not found, got %v", err)
	}
}

func TestDeleteRejectsInvalidFileID(t *testing.T) {
	tDB := setupMountPointTestDB(t)
	svc := NewService(tDB, nil, nil, nil)
	ctx := context.NewContext(stdctx.Background())

	tests := []struct {
		name string
		req  *DeleteRequest
	}{
		{name: "nil request", req: nil},
		{name: "zero file id", req: &DeleteRequest{FileId: 0, IsAdmin: true}},
		{name: "negative file id", req: &DeleteRequest{FileId: -1, CreatorUserID: 10}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := svc.Delete(ctx, tt.req)
			if !errors.Is(err, errInvalidMountPointFileID) {
				t.Fatalf("expected invalid mount point file id, got %v", err)
			}
		})
	}
}

func TestDeleteOwnerMountPointRejectsOtherCreator(t *testing.T) {
	tDB := setupMountPointTestDB(t)
	svc := NewService(tDB, nil, nil, nil)
	ctx := context.NewContext(stdctx.Background())

	mountPoint := createMountPoint(t, tDB.db, 1002, 10, "other-owner")
	createUserMountPointTokenBinding(t, tDB.db, 20, mountPoint.ID, 100)

	err := svc.Delete(ctx, &DeleteRequest{FileId: mountPoint.FileId, CreatorUserID: 20})
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("expected record not found for unauthorized delete, got %v", err)
	}

	var count int64
	if err := tDB.db.Model(&models.MountPoint{}).Where("file_id = ?", mountPoint.FileId).Count(&count).Error; err != nil {
		t.Fatalf("count mount point: %v", err)
	}

	if count != 1 {
		t.Fatalf("expected mount point to remain, got count %d", count)
	}

	if count := countUserMountPointTokenBindings(t, tDB.db, "mount_point_id = ?", mountPoint.ID); count != 1 {
		t.Fatalf("expected user token binding to remain, got count %d", count)
	}
}

func TestDeleteRejectsMissingCreatorUserIDForNonAdmin(t *testing.T) {
	tDB := setupMountPointTestDB(t)
	svc := NewService(tDB, nil, nil, nil)
	ctx := context.NewContext(stdctx.Background())

	mountPoint := createMountPoint(t, tDB.db, 1004, 10, "delete-missing-user")

	err := svc.Delete(ctx, &DeleteRequest{FileId: mountPoint.FileId})
	if !errors.Is(err, errInvalidMountPointUserID) {
		t.Fatalf("expected invalid mount point user id, got %v", err)
	}

	var count int64
	if err := tDB.db.Model(&models.MountPoint{}).Where("file_id = ?", mountPoint.FileId).Count(&count).Error; err != nil {
		t.Fatalf("count mount point: %v", err)
	}

	if count != 1 {
		t.Fatalf("expected mount point to remain, got count %d", count)
	}
}

func TestBatchDeleteAdminRemovesExistingMountPoints(t *testing.T) {
	tDB := setupMountPointTestDB(t)
	svc := NewService(tDB, nil, nil, nil)
	ctx := context.NewContext(stdctx.Background())

	first := createMountPoint(t, tDB.db, 2001, 10, "batch-admin-first")
	second := createMountPoint(t, tDB.db, 2002, 20, "batch-admin-second")
	createUserMountPointTokenBinding(t, tDB.db, 20, first.ID, 100)
	createUserMountPointTokenBinding(t, tDB.db, 30, second.ID, 200)

	err := svc.BatchDelete(ctx, &BatchDeleteRequest{
		FileIds: []int64{first.FileId, second.FileId, first.FileId},
		IsAdmin: true,
	})
	if err != nil {
		t.Fatalf("admin batch delete mount points: %v", err)
	}

	var count int64
	if err := tDB.db.Model(&models.MountPoint{}).Where("file_id IN ?", []int64{first.FileId, second.FileId}).Count(&count).Error; err != nil {
		t.Fatalf("count mount points: %v", err)
	}

	if count != 0 {
		t.Fatalf("expected mount points deleted, got count %d", count)
	}

	if count := countUserMountPointTokenBindings(t, tDB.db, "mount_point_id IN ?", []int64{first.ID, second.ID}); count != 0 {
		t.Fatalf("expected user token bindings deleted, got count %d", count)
	}
}

func TestBatchDeleteRemovesAllMountPointsForMatchedFileID(t *testing.T) {
	tDB := setupMountPointTestDB(t)
	svc := NewService(tDB, nil, nil, nil)
	ctx := context.NewContext(stdctx.Background())

	first := createMountPoint(t, tDB.db, 2012, 10, "batch-same-file-first")
	second := createMountPoint(t, tDB.db, 2012, 10, "batch-same-file-second")
	createUserMountPointTokenBinding(t, tDB.db, 20, first.ID, 100)
	createUserMountPointTokenBinding(t, tDB.db, 30, second.ID, 200)

	err := svc.BatchDelete(ctx, &BatchDeleteRequest{
		FileIds:       []int64{first.FileId},
		CreatorUserID: 10,
	})
	if err != nil {
		t.Fatalf("batch delete mount points: %v", err)
	}

	var count int64
	if err := tDB.db.Model(&models.MountPoint{}).Where("file_id = ?", first.FileId).Count(&count).Error; err != nil {
		t.Fatalf("count mount points: %v", err)
	}

	if count != 0 {
		t.Fatalf("expected all mount points for file id deleted, got count %d", count)
	}

	if count := countUserMountPointTokenBindings(t, tDB.db, "mount_point_id IN ?", []int64{first.ID, second.ID}); count != 0 {
		t.Fatalf("expected user token bindings deleted, got count %d", count)
	}
}

func TestBatchDeleteAdminReturnsNotFoundAndKeepsRowsWhenAnyIDMissing(t *testing.T) {
	tDB := setupMountPointTestDB(t)
	svc := NewService(tDB, nil, nil, nil)
	ctx := context.NewContext(stdctx.Background())

	mountPoint := createMountPoint(t, tDB.db, 2003, 10, "batch-admin-missing")
	createUserMountPointTokenBinding(t, tDB.db, 20, mountPoint.ID, 100)

	err := svc.BatchDelete(ctx, &BatchDeleteRequest{
		FileIds: []int64{mountPoint.FileId, 99999},
		IsAdmin: true,
	})
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("expected record not found, got %v", err)
	}

	var count int64
	if err := tDB.db.Model(&models.MountPoint{}).Where("file_id = ?", mountPoint.FileId).Count(&count).Error; err != nil {
		t.Fatalf("count mount point: %v", err)
	}

	if count != 1 {
		t.Fatalf("expected existing mount point to remain, got count %d", count)
	}

	if count := countUserMountPointTokenBindings(t, tDB.db, "mount_point_id = ?", mountPoint.ID); count != 1 {
		t.Fatalf("expected user token binding to remain, got count %d", count)
	}
}

func TestBatchDeleteRejectsInvalidFileIDAndKeepsRows(t *testing.T) {
	tDB := setupMountPointTestDB(t)
	svc := NewService(tDB, nil, nil, nil)
	ctx := context.NewContext(stdctx.Background())

	mountPoint := createMountPoint(t, tDB.db, 2006, 10, "batch-invalid")

	err := svc.BatchDelete(ctx, &BatchDeleteRequest{
		FileIds: []int64{mountPoint.FileId, 0},
		IsAdmin: true,
	})
	if !errors.Is(err, errInvalidMountPointFileID) {
		t.Fatalf("expected invalid mount point file id, got %v", err)
	}

	var count int64
	if err := tDB.db.Model(&models.MountPoint{}).Where("file_id = ?", mountPoint.FileId).Count(&count).Error; err != nil {
		t.Fatalf("count mount point: %v", err)
	}

	if count != 1 {
		t.Fatalf("expected mount point to remain, got count %d", count)
	}
}

func TestBatchDeleteOwnerRejectsOtherCreatorAndKeepsRows(t *testing.T) {
	tDB := setupMountPointTestDB(t)
	svc := NewService(tDB, nil, nil, nil)
	ctx := context.NewContext(stdctx.Background())

	owned := createMountPoint(t, tDB.db, 2004, 10, "batch-owned")
	other := createMountPoint(t, tDB.db, 2005, 20, "batch-other")

	err := svc.BatchDelete(ctx, &BatchDeleteRequest{
		FileIds:       []int64{owned.FileId, other.FileId},
		CreatorUserID: 10,
	})
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("expected record not found for unauthorized batch delete, got %v", err)
	}

	var count int64
	if err := tDB.db.Model(&models.MountPoint{}).Where("file_id IN ?", []int64{owned.FileId, other.FileId}).Count(&count).Error; err != nil {
		t.Fatalf("count mount points: %v", err)
	}

	if count != 2 {
		t.Fatalf("expected mount points to remain, got count %d", count)
	}
}

func TestBatchDeleteRejectsMissingCreatorUserIDForNonAdmin(t *testing.T) {
	tDB := setupMountPointTestDB(t)
	svc := NewService(tDB, nil, nil, nil)
	ctx := context.NewContext(stdctx.Background())

	mountPoint := createMountPoint(t, tDB.db, 2007, 10, "batch-missing-user")

	err := svc.BatchDelete(ctx, &BatchDeleteRequest{
		FileIds: []int64{mountPoint.FileId},
	})
	if !errors.Is(err, errInvalidMountPointUserID) {
		t.Fatalf("expected invalid mount point user id, got %v", err)
	}

	var count int64
	if err := tDB.db.Model(&models.MountPoint{}).Where("file_id = ?", mountPoint.FileId).Count(&count).Error; err != nil {
		t.Fatalf("count mount point: %v", err)
	}

	if count != 1 {
		t.Fatalf("expected mount point to remain, got count %d", count)
	}
}

func TestClearAllRemovesMountPointsAndUserTokenBindings(t *testing.T) {
	tDB := setupMountPointTestDB(t)
	svc := NewService(tDB, nil, nil, nil)
	ctx := context.NewContext(stdctx.Background())

	first := createMountPoint(t, tDB.db, 2010, 10, "clear-all-first")
	second := createMountPoint(t, tDB.db, 2011, 20, "clear-all-second")
	createUserMountPointTokenBinding(t, tDB.db, 20, first.ID, 100)
	createUserMountPointTokenBinding(t, tDB.db, 30, second.ID, 200)

	deleted, err := svc.ClearAll(ctx)
	if err != nil {
		t.Fatalf("clear all mount points: %v", err)
	}

	if deleted != 2 {
		t.Fatalf("expected two deleted mount points, got %d", deleted)
	}

	var mountPointCount int64
	if err := tDB.db.Model(&models.MountPoint{}).Count(&mountPointCount).Error; err != nil {
		t.Fatalf("count mount points: %v", err)
	}

	if mountPointCount != 0 {
		t.Fatalf("expected mount points cleared, got count %d", mountPointCount)
	}

	if count := countUserMountPointTokenBindings(t, tDB.db, "1 = 1"); count != 0 {
		t.Fatalf("expected user token bindings cleared, got count %d", count)
	}
}

func TestClearAllAllowsNoMountPoints(t *testing.T) {
	tDB := setupMountPointTestDB(t)
	svc := NewService(tDB, nil, nil, nil)
	ctx := context.NewContext(stdctx.Background())

	deleted, err := svc.ClearAll(ctx)
	if err != nil {
		t.Fatalf("clear all empty mount points: %v", err)
	}

	if deleted != 0 {
		t.Fatalf("expected no deleted mount points, got %d", deleted)
	}
}

func TestClearAllRemovesOrphanUserTokenBindings(t *testing.T) {
	tDB := setupMountPointTestDB(t)
	svc := NewService(tDB, nil, nil, nil)
	ctx := context.NewContext(stdctx.Background())

	createUserMountPointTokenBinding(t, tDB.db, 20, 99999, 100)

	deleted, err := svc.ClearAll(ctx)
	if err != nil {
		t.Fatalf("clear all orphan token bindings: %v", err)
	}

	if deleted != 0 {
		t.Fatalf("expected no deleted mount points, got %d", deleted)
	}

	if count := countUserMountPointTokenBindings(t, tDB.db, "1 = 1"); count != 0 {
		t.Fatalf("expected orphan token bindings cleared, got count %d", count)
	}
}

func TestQueryRejectsInvalidFileID(t *testing.T) {
	tDB := setupMountPointTestDB(t)
	svc := NewService(tDB, nil, nil, nil)
	ctx := context.NewContext(stdctx.Background())

	_, err := svc.Query(ctx, 0)
	if !errors.Is(err, errInvalidMountPointFileID) {
		t.Fatalf("expected invalid mount point file id, got %v", err)
	}
}

func TestGetAutoRefreshListAllowsNilRequest(t *testing.T) {
	tDB := setupMountPointTestDB(t)
	svc := NewService(tDB, nil, nil, nil)
	ctx := context.NewContext(stdctx.Background())

	beginAt := time.Now().Add(-time.Hour)
	mountPoint := createMountPoint(t, tDB.db, 5001, 10, "auto-refresh-nil")

	if err := tDB.db.Model(&models.MountPoint{}).Where("id = ?", mountPoint.ID).Updates(map[string]any{
		"enable_auto_refresh":   true,
		"auto_refresh_begin_at": beginAt,
		"auto_refresh_days":     1,
	}).Error; err != nil {
		t.Fatalf("enable auto refresh: %v", err)
	}

	list, err := svc.GetAutoRefreshList(ctx, nil)
	if err != nil {
		t.Fatalf("get auto refresh list: %v", err)
	}

	if len(list) != 1 {
		t.Fatalf("expected one auto refresh mount point, got %d", len(list))
	}

	if list[0].FileId != mountPoint.FileId {
		t.Fatalf("expected file id %d, got %d", mountPoint.FileId, list[0].FileId)
	}
}

func TestIsAutoRefreshActiveAtUsesHalfOpenExpiry(t *testing.T) {
	beginAt := time.Date(2026, 5, 22, 12, 0, 0, 0, time.UTC)
	expireAt := beginAt.AddDate(0, 0, 1)
	mp := &models.MountPoint{
		EnableAutoRefresh:  true,
		AutoRefreshBeginAt: &beginAt,
		AutoRefreshDays:    1,
	}

	if !isAutoRefreshActiveAt(mp, beginAt) {
		t.Fatal("expected auto refresh active at begin time")
	}

	if isAutoRefreshActiveAt(mp, expireAt) {
		t.Fatal("expected auto refresh inactive at exact expiry time")
	}
}

func TestIsAutoRefreshActiveAtAllowsNonPositiveDays(t *testing.T) {
	beginAt := time.Date(2026, 5, 22, 12, 0, 0, 0, time.UTC)
	mp := &models.MountPoint{
		EnableAutoRefresh:  true,
		AutoRefreshBeginAt: &beginAt,
		AutoRefreshDays:    0,
	}

	if !isAutoRefreshActiveAt(mp, beginAt.AddDate(0, 0, 365)) {
		t.Fatal("expected non-positive auto refresh days to stay active for compatibility")
	}
}

func TestEnableAutoRefreshReturnsNotFoundWhenMountPointMissing(t *testing.T) {
	tDB := setupMountPointTestDB(t)
	svc := NewService(tDB, nil, nil, nil)
	ctx := context.NewContext(stdctx.Background())

	err := svc.EnableAutoRefresh(ctx, 99999, true)
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("expected record not found, got %v", err)
	}
}

func TestEnableAutoRefreshSetsBeginAtWhenMissing(t *testing.T) {
	tDB := setupMountPointTestDB(t)
	svc := NewService(tDB, nil, nil, nil)
	ctx := context.NewContext(stdctx.Background())

	mountPoint := createMountPoint(t, tDB.db, 6101, 10, "enable-auto-refresh")

	if mountPoint.AutoRefreshBeginAt != nil {
		t.Fatalf("expected initial auto refresh begin time to be nil, got %v", mountPoint.AutoRefreshBeginAt)
	}

	if err := svc.EnableAutoRefresh(ctx, mountPoint.FileId, true); err != nil {
		t.Fatalf("enable auto refresh: %v", err)
	}

	var updated models.MountPoint
	if err := tDB.db.First(&updated, mountPoint.ID).Error; err != nil {
		t.Fatalf("query updated mount point: %v", err)
	}

	if !updated.EnableAutoRefresh {
		t.Fatal("expected auto refresh to be enabled")
	}

	if updated.AutoRefreshBeginAt == nil {
		t.Fatal("expected auto refresh begin time to be set")
	}
}

func TestEnableAutoRefreshReturnsQueryError(t *testing.T) {
	tDB := setupMountPointTestDB(t)
	svc := NewService(tDB, nil, nil, nil)
	ctx := context.NewContext(stdctx.Background())

	mountPoint := createMountPoint(t, tDB.db, 6103, 10, "enable-auto-refresh-query-error")
	queryErr := errors.New("query unavailable")

	callbackName := "mountpoint:test_enable_auto_refresh_query_error"
	if err := tDB.db.Callback().Query().Before("gorm:query").Register(callbackName, func(db *gorm.DB) {
		_ = db.AddError(queryErr)
	}); err != nil {
		t.Fatalf("register query callback: %v", err)
	}

	err := svc.EnableAutoRefresh(ctx, mountPoint.FileId, true)

	if removeErr := tDB.db.Callback().Query().Remove(callbackName); removeErr != nil {
		t.Fatalf("remove query callback: %v", removeErr)
	}

	if !errors.Is(err, queryErr) {
		t.Fatalf("expected query error, got %v", err)
	}

	var updated models.MountPoint
	if err := tDB.db.First(&updated, mountPoint.ID).Error; err != nil {
		t.Fatalf("query updated mount point: %v", err)
	}

	if updated.EnableAutoRefresh {
		t.Fatal("expected auto refresh not to be updated after query error")
	}
}

func TestEnableAutoRefreshAllowsRepeatedSameState(t *testing.T) {
	tDB := setupMountPointTestDB(t)
	svc := NewService(tDB, nil, nil, nil)
	ctx := context.NewContext(stdctx.Background())

	beginAt := time.Now().Add(-time.Hour)
	mountPoint := createMountPoint(t, tDB.db, 6102, 10, "enable-auto-refresh-repeat")

	if err := tDB.db.Model(mountPoint).Updates(map[string]any{
		"enable_auto_refresh":   true,
		"auto_refresh_begin_at": beginAt,
	}).Error; err != nil {
		t.Fatalf("seed auto refresh state: %v", err)
	}

	if err := svc.EnableAutoRefresh(ctx, mountPoint.FileId, true); err != nil {
		t.Fatalf("enable auto refresh with same state: %v", err)
	}
}

func TestEnableAutoRefreshRejectsInvalidFileID(t *testing.T) {
	tDB := setupMountPointTestDB(t)
	svc := NewService(tDB, nil, nil, nil)
	ctx := context.NewContext(stdctx.Background())

	err := svc.EnableAutoRefresh(ctx, 0, true)
	if !errors.Is(err, errInvalidMountPointFileID) {
		t.Fatalf("expected invalid mount point file id, got %v", err)
	}
}

func TestUpdateRefreshConfigReturnsNotFoundWhenMountPointMissing(t *testing.T) {
	tDB := setupMountPointTestDB(t)
	svc := NewService(tDB, nil, nil, nil)
	ctx := context.NewContext(stdctx.Background())

	interval := 30

	err := svc.UpdateRefreshConfig(ctx, 99999, RefreshConfig{RefreshInterval: &interval})
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("expected record not found, got %v", err)
	}
}

func TestUpdateRefreshConfigAllowsNoopUpdate(t *testing.T) {
	tDB := setupMountPointTestDB(t)
	svc := NewService(tDB, nil, nil, nil)
	ctx := context.NewContext(stdctx.Background())

	mountPoint := createMountPoint(t, tDB.db, 6103, 10, "refresh-config-repeat")
	interval := mountPoint.RefreshInterval
	days := mountPoint.AutoRefreshDays
	deepRefresh := mountPoint.EnableDeepRefresh

	if err := svc.UpdateRefreshConfig(ctx, mountPoint.FileId, RefreshConfig{
		RefreshInterval:   &interval,
		AutoRefreshDays:   &days,
		EnableDeepRefresh: &deepRefresh,
	}); err != nil {
		t.Fatalf("update refresh config with same values: %v", err)
	}
}

func TestUpdateRefreshConfigRejectsInvalidFileID(t *testing.T) {
	tDB := setupMountPointTestDB(t)
	svc := NewService(tDB, nil, nil, nil)
	ctx := context.NewContext(stdctx.Background())

	interval := 30

	err := svc.UpdateRefreshConfig(ctx, 0, RefreshConfig{RefreshInterval: &interval})
	if !errors.Is(err, errInvalidMountPointFileID) {
		t.Fatalf("expected invalid mount point file id, got %v", err)
	}
}

func TestUpdateRefreshTimeReturnsNotFoundWhenMountPointMissing(t *testing.T) {
	tDB := setupMountPointTestDB(t)
	svc := NewService(tDB, nil, nil, nil)
	ctx := context.NewContext(stdctx.Background())

	err := svc.UpdateRefreshTime(ctx, 99999)
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("expected record not found, got %v", err)
	}
}

func TestUpdateRefreshTimeRejectsInvalidFileID(t *testing.T) {
	tDB := setupMountPointTestDB(t)
	svc := NewService(tDB, nil, nil, nil)
	ctx := context.NewContext(stdctx.Background())

	err := svc.UpdateRefreshTime(ctx, 0)
	if !errors.Is(err, errInvalidMountPointFileID) {
		t.Fatalf("expected invalid mount point file id, got %v", err)
	}
}

func TestUpdateLastStateReturnsNotFoundWhenMountPointMissing(t *testing.T) {
	tDB := setupMountPointTestDB(t)
	svc := NewService(tDB, nil, nil, nil)
	ctx := context.NewContext(stdctx.Background())

	err := svc.UpdateLastState(ctx, 99999, "失败")
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("expected record not found, got %v", err)
	}
}

func TestUpdateLastStateAllowsRepeatedSameState(t *testing.T) {
	tDB := setupMountPointTestDB(t)
	svc := NewService(tDB, nil, nil, nil)
	ctx := context.NewContext(stdctx.Background())

	mountPoint := createMountPoint(t, tDB.db, 6104, 10, "last-state-repeat")
	if err := svc.UpdateLastState(ctx, mountPoint.FileId, "成功"); err != nil {
		t.Fatalf("update last state first time: %v", err)
	}

	if err := svc.UpdateLastState(ctx, mountPoint.FileId, "成功"); err != nil {
		t.Fatalf("update last state with same value: %v", err)
	}
}

func TestUpdateLastStateRejectsInvalidFileID(t *testing.T) {
	tDB := setupMountPointTestDB(t)
	svc := NewService(tDB, nil, nil, nil)
	ctx := context.NewContext(stdctx.Background())

	err := svc.UpdateLastState(ctx, 0, "失败")
	if !errors.Is(err, errInvalidMountPointFileID) {
		t.Fatalf("expected invalid mount point file id, got %v", err)
	}
}

func TestBatchParseTextRejectsInvalidRequest(t *testing.T) {
	tDB := setupMountPointTestDB(t)
	svc := NewService(tDB, nil, nil, nil)
	ctx := context.NewContext(stdctx.Background())

	tests := []struct {
		name    string
		req     *topic.BatchParseTextRequest
		wantErr error
	}{
		{name: "nil request", req: nil, wantErr: errInvalidMountPointParseRequest},
		{name: "zero cloud token", req: &topic.BatchParseTextRequest{Content: "123", CloudToken: 0}, wantErr: errInvalidMountPointCloudTokenID},
		{name: "negative cloud token", req: &topic.BatchParseTextRequest{Content: "123", CloudToken: -1}, wantErr: errInvalidMountPointCloudTokenID},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			items, err := svc.BatchParseText(ctx, tt.req)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("expected %v, got %v", tt.wantErr, err)
			}

			if len(items) != 0 {
				t.Fatalf("expected no parsed items, got %v", items)
			}
		})
	}
}

func TestModifyTokenUpdatesExistingMountPoint(t *testing.T) {
	tDB := setupMountPointTestDB(t)
	svc := NewService(tDB, nil, nil, nil)
	ctx := context.NewContext(stdctx.Background())

	mountPoint := createMountPoint(t, tDB.db, 1003, 10, "modify-token")
	createCloudToken(t, tDB.db, 99)

	if err := svc.ModifyToken(ctx, &ModifyTokenRequest{ID: mountPoint.ID, TokenId: 99, IsAdmin: true}); err != nil {
		t.Fatalf("modify token: %v", err)
	}

	var updated models.MountPoint
	if err := tDB.db.First(&updated, mountPoint.ID).Error; err != nil {
		t.Fatalf("query updated mount point: %v", err)
	}

	if updated.TokenId != 99 {
		t.Fatalf("expected token id 99, got %d", updated.TokenId)
	}
}

func TestModifyTokenAllowsNoopUpdate(t *testing.T) {
	tDB := setupMountPointTestDB(t)
	svc := NewService(tDB, nil, nil, nil)
	ctx := context.NewContext(stdctx.Background())

	mountPoint := createMountPoint(t, tDB.db, 1004, 10, "modify-token-repeat")
	createCloudToken(t, tDB.db, mountPoint.TokenId)

	if err := svc.ModifyToken(ctx, &ModifyTokenRequest{ID: mountPoint.ID, TokenId: mountPoint.TokenId, IsAdmin: true}); err != nil {
		t.Fatalf("modify token with same value: %v", err)
	}
}

func TestModifyTokenUpdatesOwnedTokenForNonAdmin(t *testing.T) {
	tDB := setupMountPointTestDB(t)
	svc := NewService(tDB, nil, nil, nil)
	ctx := context.NewContext(stdctx.Background())

	mountPoint := createMountPoint(t, tDB.db, 1006, 10, "modify-owned-token")
	createCloudToken(t, tDB.db, 199, 10)

	if err := svc.ModifyToken(ctx, &ModifyTokenRequest{ID: mountPoint.ID, TokenId: 199, CreatorUserID: 10}); err != nil {
		t.Fatalf("modify token with owned token: %v", err)
	}

	var updated models.MountPoint
	if err := tDB.db.First(&updated, mountPoint.ID).Error; err != nil {
		t.Fatalf("query updated mount point: %v", err)
	}

	if updated.TokenId != 199 {
		t.Fatalf("expected token id 199, got %d", updated.TokenId)
	}
}

func TestModifyTokenRejectsTokenOwnedByAnotherUserForNonAdmin(t *testing.T) {
	tDB := setupMountPointTestDB(t)
	svc := NewService(tDB, nil, nil, nil)
	ctx := context.NewContext(stdctx.Background())

	mountPoint := createMountPoint(t, tDB.db, 1007, 10, "modify-other-token")
	createCloudToken(t, tDB.db, 299, 20)

	err := svc.ModifyToken(ctx, &ModifyTokenRequest{ID: mountPoint.ID, TokenId: 299, CreatorUserID: 10})
	if !errors.Is(err, errPermissionDenied) {
		t.Fatalf("expected permission denied, got %v", err)
	}

	var updated models.MountPoint
	if err := tDB.db.First(&updated, mountPoint.ID).Error; err != nil {
		t.Fatalf("query mount point: %v", err)
	}

	if updated.TokenId != mountPoint.TokenId {
		t.Fatalf("expected token id unchanged at %d, got %d", mountPoint.TokenId, updated.TokenId)
	}
}

func TestModifyTokenReturnsNotFoundWhenTokenMissing(t *testing.T) {
	tDB := setupMountPointTestDB(t)
	svc := NewService(tDB, nil, nil, nil)
	ctx := context.NewContext(stdctx.Background())

	mountPoint := createMountPoint(t, tDB.db, 1008, 10, "modify-missing-token")

	err := svc.ModifyToken(ctx, &ModifyTokenRequest{ID: mountPoint.ID, TokenId: 399, IsAdmin: true})
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("expected record not found, got %v", err)
	}

	var updated models.MountPoint
	if err := tDB.db.First(&updated, mountPoint.ID).Error; err != nil {
		t.Fatalf("query mount point: %v", err)
	}

	if updated.TokenId != mountPoint.TokenId {
		t.Fatalf("expected token id unchanged at %d, got %d", mountPoint.TokenId, updated.TokenId)
	}
}

func TestModifyTokenReturnsNotFoundWhenMountPointMissing(t *testing.T) {
	tDB := setupMountPointTestDB(t)
	svc := NewService(tDB, nil, nil, nil)
	ctx := context.NewContext(stdctx.Background())

	err := svc.ModifyToken(ctx, &ModifyTokenRequest{ID: 99999, TokenId: 99, IsAdmin: true})
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("expected record not found, got %v", err)
	}
}

func TestModifyTokenRejectsMissingCreatorUserIDForNonAdmin(t *testing.T) {
	tDB := setupMountPointTestDB(t)
	svc := NewService(tDB, nil, nil, nil)
	ctx := context.NewContext(stdctx.Background())

	mountPoint := createMountPoint(t, tDB.db, 1005, 10, "modify-missing-user")

	err := svc.ModifyToken(ctx, &ModifyTokenRequest{ID: mountPoint.ID, TokenId: 0})
	if !errors.Is(err, errInvalidMountPointUserID) {
		t.Fatalf("expected invalid mount point user id, got %v", err)
	}

	var updated models.MountPoint
	if err := tDB.db.First(&updated, mountPoint.ID).Error; err != nil {
		t.Fatalf("query mount point: %v", err)
	}

	if updated.TokenId != mountPoint.TokenId {
		t.Fatalf("expected token id unchanged at %d, got %d", mountPoint.TokenId, updated.TokenId)
	}
}

func TestModifyTokenRejectsInvalidRequest(t *testing.T) {
	tDB := setupMountPointTestDB(t)
	svc := NewService(tDB, nil, nil, nil)
	ctx := context.NewContext(stdctx.Background())

	tests := []struct {
		name    string
		req     *ModifyTokenRequest
		wantErr error
	}{
		{name: "nil request", req: nil, wantErr: errInvalidMountPointFileID},
		{name: "zero id", req: &ModifyTokenRequest{ID: 0, TokenId: 99, IsAdmin: true}, wantErr: errInvalidMountPointFileID},
		{name: "negative id", req: &ModifyTokenRequest{ID: -1, TokenId: 99, IsAdmin: true}, wantErr: errInvalidMountPointFileID},
		{name: "negative token id", req: &ModifyTokenRequest{ID: 1, TokenId: -1, IsAdmin: true}, wantErr: errInvalidMountPointTokenID},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := svc.ModifyToken(ctx, tt.req)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("expected %v, got %v", tt.wantErr, err)
			}
		})
	}
}
