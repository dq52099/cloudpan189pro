package userMountPointToken

import (
	stdctx "context"
	"errors"
	"fmt"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	mysqlDriver "github.com/go-sql-driver/mysql"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/xxcheng123/cloudpan189-share/internal/bootstrap"
	"github.com/xxcheng123/cloudpan189-share/internal/framework/context"
	"github.com/xxcheng123/cloudpan189-share/internal/pkgs/taskengine"
	"github.com/xxcheng123/cloudpan189-share/internal/repository/models"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type userMountPointTokenTestDB struct {
	db *gorm.DB
}

func (t *userMountPointTokenTestDB) GetDB(ctx context.Context) *gorm.DB {
	return t.db.WithContext(ctx)
}

func (t *userMountPointTokenTestDB) GetDBWithoutContext() *gorm.DB {
	return t.db
}

func (t *userMountPointTokenTestDB) GetLogger(name string, fields ...zap.Field) *zap.Logger {
	return zap.NewNop()
}

func (t *userMountPointTokenTestDB) Close() {}

func (t *userMountPointTokenTestDB) GetPort() int {
	return 9999
}

func (t *userMountPointTokenTestDB) GetTaskEngine() taskengine.TaskEngine {
	return nil
}

func (t *userMountPointTokenTestDB) GetHTTPEngine() *gin.Engine {
	return nil
}

var _ bootstrap.ServiceContext = (*userMountPointTokenTestDB)(nil)

func setupUserMountPointTokenTestDB(t *testing.T) *userMountPointTokenTestDB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}

	if err := db.AutoMigrate(&models.UserMountPointToken{}); err != nil {
		t.Fatalf("migrate test db: %v", err)
	}

	return &userMountPointTokenTestDB{db: db}
}

func TestIsMissingTableErrorRecognizesSupportedDatabases(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "mysql missing table",
			err:  &mysqlDriver.MySQLError{Number: 1146, Message: "Table 'share.user_mount_point_tokens' doesn't exist"},
			want: true,
		},
		{
			name: "wrapped postgres undefined table",
			err:  fmt.Errorf("query bindings: %w", &pgconn.PgError{Code: "42P01", Message: `relation "user_mount_point_tokens" does not exist`}),
			want: true,
		},
		{
			name: "sqlite missing table text",
			err:  errors.New("SQL logic error: no such table: user_mount_point_tokens"),
			want: true,
		},
		{
			name: "mysql text fallback",
			err:  errors.New("Error 1146: Table 'share.user_mount_point_tokens' doesn't exist"),
			want: true,
		},
		{
			name: "unrelated missing table",
			err:  errors.New("no such table: other_table"),
			want: false,
		},
		{
			name: "nil",
			err:  nil,
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isMissingTableError(tt.err); got != tt.want {
				t.Fatalf("expected %v, got %v", tt.want, got)
			}
		})
	}
}

func createBinding(t *testing.T, db *gorm.DB, userID, mountPointID, tokenID int64) {
	t.Helper()

	binding := &models.UserMountPointToken{
		UserID:       userID,
		MountPointID: mountPointID,
		TokenID:      tokenID,
	}
	if err := db.Create(binding).Error; err != nil {
		t.Fatalf("create binding: %v", err)
	}
}

func countBindings(t *testing.T, db *gorm.DB, query string, args ...any) int64 {
	t.Helper()

	var count int64
	if err := db.Model(&models.UserMountPointToken{}).Where(query, args...).Count(&count).Error; err != nil {
		t.Fatalf("count bindings: %v", err)
	}

	return count
}

func TestUnbindTokenDeletesOnlyRequestedUserMountPoint(t *testing.T) {
	tDB := setupUserMountPointTokenTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())

	createBinding(t, tDB.db, 1, 10, 100)
	createBinding(t, tDB.db, 2, 10, 200)
	createBinding(t, tDB.db, 1, 11, 300)

	if err := svc.UnbindToken(ctx, 1, 10); err != nil {
		t.Fatalf("unbind token: %v", err)
	}

	if count := countBindings(t, tDB.db, "user_id = ? AND mount_point_id = ?", 1, 10); count != 0 {
		t.Fatalf("expected requested binding deleted, got count %d", count)
	}

	if count := countBindings(t, tDB.db, "1 = 1"); count != 2 {
		t.Fatalf("expected unrelated bindings to remain, got count %d", count)
	}
}

func TestUnbindTokenWithResultReportsWhetherBindingWasDeleted(t *testing.T) {
	tDB := setupUserMountPointTokenTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())

	createBinding(t, tDB.db, 1, 10, 100)

	deleted, err := svc.UnbindTokenWithResult(ctx, 1, 10)
	if err != nil {
		t.Fatalf("unbind existing token: %v", err)
	}

	if !deleted {
		t.Fatal("expected existing binding to report deleted=true")
	}

	deleted, err = svc.UnbindTokenWithResult(ctx, 1, 10)
	if err != nil {
		t.Fatalf("unbind missing token: %v", err)
	}

	if deleted {
		t.Fatal("expected missing binding to report deleted=false")
	}
}

func TestUnbindTokenRemainsIdempotentWhenBindingMissing(t *testing.T) {
	tDB := setupUserMountPointTokenTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())

	if err := svc.UnbindToken(ctx, 1, 10); err != nil {
		t.Fatalf("expected missing unbind to stay idempotent, got %v", err)
	}
}

func TestUnbindTokenRejectsInvalidIDWithoutDeletingExistingBinding(t *testing.T) {
	tDB := setupUserMountPointTokenTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())

	createBinding(t, tDB.db, 1, 10, 100)

	err := svc.UnbindToken(ctx, 1, 0)
	if err == nil {
		t.Fatal("expected invalid mount point id to fail")
	}

	if !errors.Is(err, errInvalidUnbindTokenID) {
		t.Fatalf("expected invalid unbind error, got %v", err)
	}

	if count := countBindings(t, tDB.db, "user_id = ? AND mount_point_id = ?", 1, 10); count != 1 {
		t.Fatalf("expected existing binding to remain, got count %d", count)
	}
}

func TestBindTokenUpsertsExistingBinding(t *testing.T) {
	tDB := setupUserMountPointTokenTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())

	createBinding(t, tDB.db, 1, 10, 100)
	createBinding(t, tDB.db, 2, 10, 300)

	if err := svc.BindToken(ctx, 1, 10, 999); err != nil {
		t.Fatalf("bind token: %v", err)
	}

	if count := countBindings(t, tDB.db, "user_id = ? AND mount_point_id = ?", 1, 10); count != 1 {
		t.Fatalf("expected one binding after replacement, got count %d", count)
	}

	tokenID, err := svc.GetTokenID(ctx, 1, 10)
	if err != nil {
		t.Fatalf("get token id: %v", err)
	}

	if tokenID != 999 {
		t.Fatalf("expected token id 999, got %d", tokenID)
	}

	if count := countBindings(t, tDB.db, "user_id = ? AND mount_point_id = ?", 2, 10); count != 1 {
		t.Fatalf("expected other user's binding to remain, got count %d", count)
	}
}

func TestBindTokenRejectsDuplicateUserMountPointRows(t *testing.T) {
	tDB := setupUserMountPointTokenTestDB(t)

	createBinding(t, tDB.db, 1, 10, 100)

	err := tDB.db.Create(&models.UserMountPointToken{
		UserID:       1,
		MountPointID: 10,
		TokenID:      200,
	}).Error
	if err == nil {
		t.Fatal("expected unique user mount point binding to reject duplicates")
	}
}

func TestBindTokenRejectsInvalidIDWithoutDeletingExistingBinding(t *testing.T) {
	tDB := setupUserMountPointTokenTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())

	createBinding(t, tDB.db, 1, 10, 100)

	err := svc.BindToken(ctx, 1, 10, 0)
	if err == nil {
		t.Fatal("expected invalid token id to fail")
	}

	if !errors.Is(err, errInvalidBindTokenID) {
		t.Fatalf("expected invalid bind token error, got %v", err)
	}

	if count := countBindings(t, tDB.db, "user_id = ? AND mount_point_id = ? AND token_id = ?", 1, 10, 100); count != 1 {
		t.Fatalf("expected existing binding to remain, got count %d", count)
	}
}

func TestGetTokenIDReturnsZeroWhenBindingMissing(t *testing.T) {
	tDB := setupUserMountPointTokenTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())

	tokenID, err := svc.GetTokenID(ctx, 1, 10)
	if err != nil {
		t.Fatalf("get missing token id: %v", err)
	}

	if tokenID != 0 {
		t.Fatalf("expected missing token id to be 0, got %d", tokenID)
	}
}

func TestGetTokenIDReturnsZeroWhenBindingTableMissing(t *testing.T) {
	tDB := setupUserMountPointTokenTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())

	if err := tDB.db.Migrator().DropTable(&models.UserMountPointToken{}); err != nil {
		t.Fatalf("drop binding table: %v", err)
	}

	tokenID, err := svc.GetTokenID(ctx, 1, 10)
	if err != nil {
		t.Fatalf("expected missing binding table to be treated as no binding, got %v", err)
	}

	if tokenID != 0 {
		t.Fatalf("expected missing binding table token id to be 0, got %d", tokenID)
	}
}

func TestGetTokenIDRejectsInvalidID(t *testing.T) {
	tDB := setupUserMountPointTokenTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())

	tests := []struct {
		name         string
		userID       int64
		mountPointID int64
	}{
		{name: "invalid user", userID: 0, mountPointID: 10},
		{name: "invalid mount point", userID: 1, mountPointID: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := svc.GetTokenID(ctx, tt.userID, tt.mountPointID)
			if !errors.Is(err, errInvalidUnbindTokenID) {
				t.Fatalf("expected invalid token query id, got %v", err)
			}
		})
	}
}

func TestGetUserTokensReturnsOwnBindingsAndDeduplicatesMountPointIDs(t *testing.T) {
	tDB := setupUserMountPointTokenTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())

	createBinding(t, tDB.db, 1, 10, 100)
	createBinding(t, tDB.db, 2, 10, 200)
	createBinding(t, tDB.db, 1, 11, 300)

	tokenMap, err := svc.GetUserTokens(ctx, 1, []int64{10, 10, 11, 999})
	if err != nil {
		t.Fatalf("get user tokens: %v", err)
	}

	if len(tokenMap) != 2 {
		t.Fatalf("expected 2 token bindings, got %d", len(tokenMap))
	}

	if tokenMap[10] != 100 {
		t.Fatalf("expected mount point 10 token 100, got %d", tokenMap[10])
	}

	if tokenMap[11] != 300 {
		t.Fatalf("expected mount point 11 token 300, got %d", tokenMap[11])
	}
}

func TestGetUserTokensRejectsInvalidIDs(t *testing.T) {
	tDB := setupUserMountPointTokenTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())

	tests := []struct {
		name    string
		userID  int64
		ids     []int64
		wantErr error
	}{
		{name: "invalid user", userID: 0, ids: []int64{10}, wantErr: errInvalidUserID},
		{name: "invalid mount point", userID: 1, ids: []int64{10, 0}, wantErr: errInvalidMountPointID},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := svc.GetUserTokens(ctx, tt.userID, tt.ids)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("expected %v, got %v", tt.wantErr, err)
			}
		})
	}
}

func TestGetUserTokensReturnsEmptyForEmptyMountPointIDs(t *testing.T) {
	tDB := setupUserMountPointTokenTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())

	tokenMap, err := svc.GetUserTokens(ctx, 1, nil)
	if err != nil {
		t.Fatalf("get user tokens with empty ids: %v", err)
	}

	if len(tokenMap) != 0 {
		t.Fatalf("expected empty token map, got %d", len(tokenMap))
	}
}

func TestGetUserMountPointIDsRejectsInvalidUserID(t *testing.T) {
	tDB := setupUserMountPointTokenTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())

	_, err := svc.GetUserMountPointIDs(ctx, 0)
	if !errors.Is(err, errInvalidUserID) {
		t.Fatalf("expected invalid user id, got %v", err)
	}
}

func TestGetAnyUserTokensReturnsLatestBindingAndDeduplicatesMountPointIDs(t *testing.T) {
	tDB := setupUserMountPointTokenTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())

	createBinding(t, tDB.db, 1, 10, 100)
	createBinding(t, tDB.db, 2, 10, 200)
	createBinding(t, tDB.db, 1, 11, 300)

	tokenMap, err := svc.GetAnyUserTokens(ctx, []int64{10, 10, 11, 999})
	if err != nil {
		t.Fatalf("get any user tokens: %v", err)
	}

	if len(tokenMap) != 2 {
		t.Fatalf("expected 2 token bindings, got %d", len(tokenMap))
	}

	if tokenMap[10] != 200 {
		t.Fatalf("expected latest mount point 10 token 200, got %d", tokenMap[10])
	}

	if tokenMap[11] != 300 {
		t.Fatalf("expected mount point 11 token 300, got %d", tokenMap[11])
	}
}

func TestGetAnyUserTokensRejectsInvalidMountPointID(t *testing.T) {
	tDB := setupUserMountPointTokenTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())

	_, err := svc.GetAnyUserTokens(ctx, []int64{10, -1})
	if !errors.Is(err, errInvalidMountPointID) {
		t.Fatalf("expected invalid mount point id, got %v", err)
	}
}

func TestNormalizeMountPointIDsDeduplicatesInOrder(t *testing.T) {
	ids, err := normalizeMountPointIDs([]int64{3, 1, 3, 2, 1})
	if err != nil {
		t.Fatalf("normalize mount point ids: %v", err)
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

func TestDeleteByMountPointDeletesAllBindingsForMountPoint(t *testing.T) {
	tDB := setupUserMountPointTokenTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())

	createBinding(t, tDB.db, 1, 10, 100)
	createBinding(t, tDB.db, 2, 10, 200)
	createBinding(t, tDB.db, 1, 11, 100)

	if err := svc.DeleteByMountPoint(ctx, 10); err != nil {
		t.Fatalf("delete by mount point: %v", err)
	}

	if count := countBindings(t, tDB.db, "mount_point_id = ?", 10); count != 0 {
		t.Fatalf("expected mount point bindings deleted, got count %d", count)
	}

	if count := countBindings(t, tDB.db, "mount_point_id = ?", 11); count != 1 {
		t.Fatalf("expected other mount point binding to remain, got count %d", count)
	}
}

func TestDeleteByMountPointRejectsInvalidIDWithoutDeletingBindings(t *testing.T) {
	tDB := setupUserMountPointTokenTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())

	createBinding(t, tDB.db, 1, 10, 100)

	err := svc.DeleteByMountPoint(ctx, 0)
	if err == nil {
		t.Fatal("expected invalid mount point id to fail")
	}

	if !errors.Is(err, errInvalidMountPointID) {
		t.Fatalf("expected invalid mount point error, got %v", err)
	}

	if count := countBindings(t, tDB.db, "1 = 1"); count != 1 {
		t.Fatalf("expected existing binding to remain, got count %d", count)
	}
}

func TestDeleteByTokenDeletesAllBindingsForToken(t *testing.T) {
	tDB := setupUserMountPointTokenTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())

	createBinding(t, tDB.db, 1, 10, 100)
	createBinding(t, tDB.db, 2, 11, 100)
	createBinding(t, tDB.db, 1, 12, 200)

	if err := svc.DeleteByToken(ctx, 100); err != nil {
		t.Fatalf("delete by token: %v", err)
	}

	if count := countBindings(t, tDB.db, "token_id = ?", 100); count != 0 {
		t.Fatalf("expected token bindings deleted, got count %d", count)
	}

	if count := countBindings(t, tDB.db, "token_id = ?", 200); count != 1 {
		t.Fatalf("expected other token binding to remain, got count %d", count)
	}
}

func TestDeleteByTokenRejectsInvalidIDWithoutDeletingBindings(t *testing.T) {
	tDB := setupUserMountPointTokenTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())

	createBinding(t, tDB.db, 1, 10, 100)

	err := svc.DeleteByToken(ctx, 0)
	if err == nil {
		t.Fatal("expected invalid token id to fail")
	}

	if !errors.Is(err, errInvalidTokenID) {
		t.Fatalf("expected invalid token error, got %v", err)
	}

	if count := countBindings(t, tDB.db, "1 = 1"); count != 1 {
		t.Fatalf("expected existing binding to remain, got count %d", count)
	}
}
