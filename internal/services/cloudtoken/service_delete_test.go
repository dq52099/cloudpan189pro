package cloudtoken

import (
	stdctx "context"
	"errors"
	"fmt"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/xxcheng123/cloudpan189-interface/client"
	"github.com/xxcheng123/cloudpan189-share/internal/bootstrap"
	"github.com/xxcheng123/cloudpan189-share/internal/framework/context"
	"github.com/xxcheng123/cloudpan189-share/internal/pkgs/taskengine"
	"github.com/xxcheng123/cloudpan189-share/internal/repository/models"
	"github.com/xxcheng123/cloudpan189-share/internal/types/autoingest"
	"go.uber.org/zap"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

type cloudTokenTestDB struct {
	db *gorm.DB
}

func (t *cloudTokenTestDB) GetDB(ctx context.Context) *gorm.DB {
	return t.db.WithContext(ctx)
}

func (t *cloudTokenTestDB) GetDBWithoutContext() *gorm.DB {
	return t.db
}

func (t *cloudTokenTestDB) GetLogger(name string, fields ...zap.Field) *zap.Logger {
	return zap.NewNop()
}

func (t *cloudTokenTestDB) Close() {}

func (t *cloudTokenTestDB) GetPort() int {
	return 9999
}

func (t *cloudTokenTestDB) GetTaskEngine() taskengine.TaskEngine {
	return nil
}

func (t *cloudTokenTestDB) GetHTTPEngine() *gin.Engine {
	return nil
}

var _ bootstrap.ServiceContext = (*cloudTokenTestDB)(nil)

func setupCloudTokenTestDB(t *testing.T) *cloudTokenTestDB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}

	if err := db.AutoMigrate(
		&models.CloudToken{},
		&models.MountPoint{},
		&models.UserMountPointToken{},
		&models.AutoIngestPlan{},
	); err != nil {
		t.Fatalf("migrate test db: %v", err)
	}

	return &cloudTokenTestDB{db: db}
}

func createCloudToken(t *testing.T, db *gorm.DB, userID int64, name string) *models.CloudToken {
	t.Helper()

	token := &models.CloudToken{
		Name:        name,
		AccessToken: "access-token",
		ExpiresIn:   3600,
		Status:      1,
		UserID:      userID,
	}
	if err := db.Create(token).Error; err != nil {
		t.Fatalf("create cloud token: %v", err)
	}

	return token
}

func createAutoIngestPlanWithToken(t *testing.T, db *gorm.DB, userID, tokenID int64, name string) *models.AutoIngestPlan {
	t.Helper()

	plan := &models.AutoIngestPlan{
		Name:       name,
		SourceType: autoingest.SourceTypeSubscribe,
		ParentPath: "/" + name,
		TokenId:    tokenID,
		UserID:     userID,
	}
	if err := db.Create(plan).Error; err != nil {
		t.Fatalf("create auto ingest plan: %v", err)
	}

	return plan
}

func TestCheckCloudTokenUpdateResultAllowsNoopWhenTokenExists(t *testing.T) {
	tDB := setupCloudTokenTestDB(t)
	svc := &service{svc: tDB}
	ctx := context.NewContext(stdctx.Background())

	token := createCloudToken(t, tDB.db, 10, "noop-existing")

	if err := svc.checkCloudTokenUpdateResult(ctx, &gorm.DB{RowsAffected: 0}, token.ID, 10, false, "令牌不存在"); err != nil {
		t.Fatalf("expected noop update to pass for existing token, got %v", err)
	}
}

func TestCheckCloudTokenUpdateResultReturnsNotFoundWhenTokenMissing(t *testing.T) {
	tDB := setupCloudTokenTestDB(t)
	svc := &service{svc: tDB}
	ctx := context.NewContext(stdctx.Background())

	err := svc.checkCloudTokenUpdateResult(ctx, &gorm.DB{RowsAffected: 0}, 99999, 10, false, "令牌不存在")
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("expected record not found, got %v", err)
	}
}

func TestCheckCloudTokenUpdateResultRestrictsNoopByUser(t *testing.T) {
	tDB := setupCloudTokenTestDB(t)
	svc := &service{svc: tDB}
	ctx := context.NewContext(stdctx.Background())

	token := createCloudToken(t, tDB.db, 20, "noop-other-user")

	err := svc.checkCloudTokenUpdateResult(ctx, &gorm.DB{RowsAffected: 0}, token.ID, 10, false, "令牌不存在")
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("expected record not found for inaccessible token, got %v", err)
	}
}

func TestCheckCloudTokenUpdateResultAllowsIDOnlyNoopWhenTokenExists(t *testing.T) {
	tDB := setupCloudTokenTestDB(t)
	svc := &service{svc: tDB}
	ctx := context.NewContext(stdctx.Background())

	token := createCloudToken(t, tDB.db, 10, "noop-id-only")

	if err := svc.checkCloudTokenUpdateResult(ctx, &gorm.DB{RowsAffected: 0}, token.ID, 0, true, "令牌不存在"); err != nil {
		t.Fatalf("expected id-only noop update to pass for existing token, got %v", err)
	}
}

func TestQueryReturnsExistingToken(t *testing.T) {
	tDB := setupCloudTokenTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())

	token := createCloudToken(t, tDB.db, 10, "query-target")

	got, err := svc.Query(ctx, token.ID)
	if err != nil {
		t.Fatalf("query cloud token: %v", err)
	}

	if got.ID != token.ID {
		t.Fatalf("expected token id %d, got %d", token.ID, got.ID)
	}
}

func TestQueryAccessibleRestrictsNonAdminTokens(t *testing.T) {
	tDB := setupCloudTokenTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())

	ownToken := createCloudToken(t, tDB.db, 10, "own")
	otherToken := createCloudToken(t, tDB.db, 20, "other")

	got, err := svc.QueryAccessible(ctx, ownToken.ID, 10, false)
	if err != nil {
		t.Fatalf("query own token: %v", err)
	}

	if got.ID != ownToken.ID {
		t.Fatalf("expected own token id %d, got %d", ownToken.ID, got.ID)
	}

	_, err = svc.QueryAccessible(ctx, otherToken.ID, 10, false)
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("expected record not found for other token, got %v", err)
	}

	got, err = svc.QueryAccessible(ctx, otherToken.ID, 0, true)
	if err != nil {
		t.Fatalf("admin query other token: %v", err)
	}

	if got.ID != otherToken.ID {
		t.Fatalf("expected other token id %d, got %d", otherToken.ID, got.ID)
	}
}

func TestQueryAccessibleRejectsMissingUserID(t *testing.T) {
	tDB := setupCloudTokenTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())

	token := createCloudToken(t, tDB.db, 10, "missing-user")

	_, err := svc.QueryAccessible(ctx, token.ID, 0, false)
	if !errors.Is(err, errInvalidCloudTokenUserID) {
		t.Fatalf("expected invalid user id, got %v", err)
	}
}

func TestQueryReturnsRecordNotFoundWhenTokenMissing(t *testing.T) {
	tDB := setupCloudTokenTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())

	_, err := svc.Query(ctx, 99999)
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("expected record not found, got %v", err)
	}
}

func TestQueryRejectsInvalidID(t *testing.T) {
	tDB := setupCloudTokenTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())

	for _, id := range []int64{0, -1} {
		t.Run(fmt.Sprintf("id_%d", id), func(t *testing.T) {
			_, err := svc.Query(ctx, id)
			if !errors.Is(err, errInvalidCloudTokenID) {
				t.Fatalf("expected invalid cloud token id, got %v", err)
			}
		})
	}
}

func stubLoginQuery(t *testing.T, resp *client.AccessTokenResponse, err error) {
	t.Helper()

	original := loginQuery
	loginQuery = func(uuid string) (*client.AccessTokenResponse, error) {
		return resp, err
	}

	t.Cleanup(func() {
		loginQuery = original
	})
}

func stubLoginQueryWithCallCount(t *testing.T) *int {
	t.Helper()

	calls := 0
	original := loginQuery
	loginQuery = func(uuid string) (*client.AccessTokenResponse, error) {
		calls++

		return &client.AccessTokenResponse{AccessToken: "new-access-token", ExpiresIn: 7200}, nil
	}

	t.Cleanup(func() {
		loginQuery = original
	})

	return &calls
}

func TestCheckQrcodeUpdatesExistingToken(t *testing.T) {
	tDB := setupCloudTokenTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())

	token := createCloudToken(t, tDB.db, 10, "scan")
	stubLoginQuery(t, &client.AccessTokenResponse{AccessToken: "new-access-token", ExpiresIn: 7200}, nil)

	if err := svc.CheckQrcode(ctx, &CheckQrcodeRequest{ID: token.ID, UUID: "uuid", UserID: 10}); err != nil {
		t.Fatalf("check qrcode: %v", err)
	}

	var updated models.CloudToken
	if err := tDB.db.First(&updated, token.ID).Error; err != nil {
		t.Fatalf("query updated token: %v", err)
	}

	if updated.AccessToken != "new-access-token" {
		t.Fatalf("expected updated access token, got %q", updated.AccessToken)
	}

	if updated.ExpiresIn != 7200 {
		t.Fatalf("expected expires in 7200, got %d", updated.ExpiresIn)
	}
}

func TestCheckQrcodeAllowsNoopUpdateForExistingToken(t *testing.T) {
	tDB := setupCloudTokenTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())

	token := createCloudToken(t, tDB.db, 10, "scan-noop")
	stubLoginQuery(t, &client.AccessTokenResponse{AccessToken: "access-token", ExpiresIn: 3600}, nil)

	if err := svc.CheckQrcode(ctx, &CheckQrcodeRequest{ID: token.ID, UUID: "uuid", UserID: 10}); err != nil {
		t.Fatalf("check qrcode noop update: %v", err)
	}

	var updated models.CloudToken
	if err := tDB.db.First(&updated, token.ID).Error; err != nil {
		t.Fatalf("query updated token: %v", err)
	}

	if updated.AccessToken != "access-token" {
		t.Fatalf("expected access token unchanged, got %q", updated.AccessToken)
	}

	if updated.ExpiresIn != 3600 {
		t.Fatalf("expected expires in unchanged, got %d", updated.ExpiresIn)
	}
}

func TestCheckQrcodeReturnsNotFoundWhenTokenMissing(t *testing.T) {
	tDB := setupCloudTokenTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())

	stubLoginQuery(t, &client.AccessTokenResponse{AccessToken: "new-access-token", ExpiresIn: 7200}, nil)

	err := svc.CheckQrcode(ctx, &CheckQrcodeRequest{ID: 99999, UUID: "uuid", UserID: 10})
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("expected record not found, got %v", err)
	}
}

func TestCheckQrcodeRestrictsNonAdminTokenBeforeLoginQuery(t *testing.T) {
	tDB := setupCloudTokenTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())
	calls := stubLoginQueryWithCallCount(t)

	token := createCloudToken(t, tDB.db, 20, "other-scan")

	err := svc.CheckQrcode(ctx, &CheckQrcodeRequest{ID: token.ID, UUID: "uuid", UserID: 10})
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("expected record not found, got %v", err)
	}

	if *calls != 0 {
		t.Fatalf("expected login query not to be called, got %d calls", *calls)
	}

	var unchanged models.CloudToken
	if err := tDB.db.First(&unchanged, token.ID).Error; err != nil {
		t.Fatalf("query unchanged token: %v", err)
	}

	if unchanged.AccessToken != "access-token" {
		t.Fatalf("expected token access token unchanged, got %q", unchanged.AccessToken)
	}
}

func TestCheckQrcodeAllowsAdminTokenUpdate(t *testing.T) {
	tDB := setupCloudTokenTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())

	token := createCloudToken(t, tDB.db, 20, "admin-scan")
	stubLoginQuery(t, &client.AccessTokenResponse{AccessToken: "admin-access-token", ExpiresIn: 7200}, nil)

	if err := svc.CheckQrcode(ctx, &CheckQrcodeRequest{ID: token.ID, UUID: "uuid", IsAdmin: true}); err != nil {
		t.Fatalf("admin check qrcode: %v", err)
	}

	var updated models.CloudToken
	if err := tDB.db.First(&updated, token.ID).Error; err != nil {
		t.Fatalf("query updated token: %v", err)
	}

	if updated.AccessToken != "admin-access-token" {
		t.Fatalf("expected admin-updated access token, got %q", updated.AccessToken)
	}
}

func TestCheckQrcodeRejectsInvalidIDBeforeLoginQuery(t *testing.T) {
	tDB := setupCloudTokenTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())
	calls := stubLoginQueryWithCallCount(t)

	err := svc.CheckQrcode(ctx, &CheckQrcodeRequest{ID: -1, UUID: "uuid", UserID: 10})
	if !errors.Is(err, errInvalidCloudTokenID) {
		t.Fatalf("expected invalid token id, got %v", err)
	}

	if *calls != 0 {
		t.Fatalf("expected login query not to be called, got %d calls", *calls)
	}
}

func TestCheckQrcodeRejectsNilRequestBeforeLoginQuery(t *testing.T) {
	tDB := setupCloudTokenTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())
	calls := stubLoginQueryWithCallCount(t)

	err := svc.CheckQrcode(ctx, nil)
	if !errors.Is(err, errInvalidCloudTokenUserID) {
		t.Fatalf("expected invalid user id, got %v", err)
	}

	if *calls != 0 {
		t.Fatalf("expected login query not to be called, got %d calls", *calls)
	}
}

func TestCheckQrcodeRejectsMissingUserIDBeforeLoginQuery(t *testing.T) {
	tDB := setupCloudTokenTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())
	calls := stubLoginQueryWithCallCount(t)

	err := svc.CheckQrcode(ctx, &CheckQrcodeRequest{UUID: "uuid"})
	if !errors.Is(err, errInvalidCloudTokenUserID) {
		t.Fatalf("expected invalid user id, got %v", err)
	}

	if *calls != 0 {
		t.Fatalf("expected login query not to be called, got %d calls", *calls)
	}
}

func TestModifyNameUpdatesExistingToken(t *testing.T) {
	tDB := setupCloudTokenTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())

	token := createCloudToken(t, tDB.db, 10, "old")

	if err := svc.ModifyName(ctx, &ModifyNameRequest{ID: token.ID, Name: "new", UserID: 10}); err != nil {
		t.Fatalf("modify token name: %v", err)
	}

	var updated models.CloudToken
	if err := tDB.db.First(&updated, token.ID).Error; err != nil {
		t.Fatalf("query updated token: %v", err)
	}

	if updated.Name != "new" {
		t.Fatalf("expected token name new, got %q", updated.Name)
	}
}

func TestModifyNameReturnsNotFoundWhenTokenMissing(t *testing.T) {
	tDB := setupCloudTokenTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())

	err := svc.ModifyName(ctx, &ModifyNameRequest{ID: 99999, Name: "missing", UserID: 10})
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("expected record not found, got %v", err)
	}
}

func TestModifyNameRestrictsNonAdminTokens(t *testing.T) {
	tDB := setupCloudTokenTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())

	token := createCloudToken(t, tDB.db, 20, "other")

	err := svc.ModifyName(ctx, &ModifyNameRequest{ID: token.ID, Name: "blocked", UserID: 10})
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("expected record not found, got %v", err)
	}

	var unchanged models.CloudToken
	if err := tDB.db.First(&unchanged, token.ID).Error; err != nil {
		t.Fatalf("query unchanged token: %v", err)
	}

	if unchanged.Name != "other" {
		t.Fatalf("expected token name unchanged, got %q", unchanged.Name)
	}

	if err := svc.ModifyName(ctx, &ModifyNameRequest{ID: token.ID, Name: "admin", IsAdmin: true}); err != nil {
		t.Fatalf("admin modify token name: %v", err)
	}

	var updated models.CloudToken
	if err := tDB.db.First(&updated, token.ID).Error; err != nil {
		t.Fatalf("query updated token: %v", err)
	}

	if updated.Name != "admin" {
		t.Fatalf("expected admin-updated token name, got %q", updated.Name)
	}
}

func TestModifyNameRejectsInvalidID(t *testing.T) {
	tDB := setupCloudTokenTestDB(t)
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
			if !errors.Is(err, errInvalidCloudTokenID) {
				t.Fatalf("expected invalid token id, got %v", err)
			}
		})
	}
}

func TestDeleteRejectsInvalidID(t *testing.T) {
	tDB := setupCloudTokenTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())

	tests := []struct {
		name string
		req  *DeleteRequest
	}{
		{name: "nil request", req: nil},
		{name: "zero id", req: &DeleteRequest{ID: 0, IsAdmin: true}},
		{name: "negative id", req: &DeleteRequest{ID: -1, UserID: 10}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := svc.Delete(ctx, tt.req)
			if !errors.Is(err, errInvalidCloudTokenID) {
				t.Fatalf("expected invalid token id, got %v", err)
			}
		})
	}
}

func TestDeleteOwnerToken(t *testing.T) {
	tDB := setupCloudTokenTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())

	ownToken := createCloudToken(t, tDB.db, 10, "own")
	otherToken := createCloudToken(t, tDB.db, 20, "other")

	if err := svc.Delete(ctx, &DeleteRequest{ID: ownToken.ID, UserID: 10}); err != nil {
		t.Fatalf("delete own token: %v", err)
	}

	var count int64
	if err := tDB.db.Model(&models.CloudToken{}).Where("id = ?", ownToken.ID).Count(&count).Error; err != nil {
		t.Fatalf("count own token: %v", err)
	}

	if count != 0 {
		t.Fatalf("expected own token deleted, got count %d", count)
	}

	err := svc.Delete(ctx, &DeleteRequest{ID: otherToken.ID, UserID: 10})
	if err == nil {
		t.Fatal("expected deleting another user's token to fail")
	}

	if err := tDB.db.Model(&models.CloudToken{}).Where("id = ?", otherToken.ID).Count(&count).Error; err != nil {
		t.Fatalf("count other token: %v", err)
	}

	if count != 1 {
		t.Fatalf("expected other token to remain, got count %d", count)
	}
}

func TestDeleteOwnerTokenClearsReferences(t *testing.T) {
	tDB := setupCloudTokenTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())

	token := createCloudToken(t, tDB.db, 10, "owner-delete")

	mountPoint := &models.MountPoint{
		FileId:        1001,
		OsType:        "folder",
		TokenId:       token.ID,
		CreatorUserID: 10,
		Name:          "mount",
	}
	if err := tDB.db.Create(mountPoint).Error; err != nil {
		t.Fatalf("create mount point: %v", err)
	}

	binding := &models.UserMountPointToken{
		UserID:       10,
		MountPointID: mountPoint.ID,
		TokenID:      token.ID,
	}
	if err := tDB.db.Create(binding).Error; err != nil {
		t.Fatalf("create token binding: %v", err)
	}

	plan := createAutoIngestPlanWithToken(t, tDB.db, 10, token.ID, "owner-plan")

	if err := svc.Delete(ctx, &DeleteRequest{ID: token.ID, UserID: 10}); err != nil {
		t.Fatalf("delete owner token: %v", err)
	}

	var updatedMountPoint models.MountPoint
	if err := tDB.db.First(&updatedMountPoint, mountPoint.ID).Error; err != nil {
		t.Fatalf("query mount point: %v", err)
	}

	if updatedMountPoint.TokenId != 0 {
		t.Fatalf("expected mount point token reset to 0, got %d", updatedMountPoint.TokenId)
	}

	var bindingCount int64
	if err := tDB.db.Model(&models.UserMountPointToken{}).Where("token_id = ?", token.ID).Count(&bindingCount).Error; err != nil {
		t.Fatalf("count token binding: %v", err)
	}

	if bindingCount != 0 {
		t.Fatalf("expected token bindings deleted, got count %d", bindingCount)
	}

	var updatedPlan models.AutoIngestPlan
	if err := tDB.db.First(&updatedPlan, plan.ID).Error; err != nil {
		t.Fatalf("query auto ingest plan: %v", err)
	}

	if updatedPlan.TokenId != 0 {
		t.Fatalf("expected auto ingest plan token reset to 0, got %d", updatedPlan.TokenId)
	}
}

func TestDeleteOtherUserTokenKeepsReferences(t *testing.T) {
	tDB := setupCloudTokenTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())

	token := createCloudToken(t, tDB.db, 20, "other-delete")

	mountPoint := &models.MountPoint{
		FileId:        1001,
		OsType:        "folder",
		TokenId:       token.ID,
		CreatorUserID: 20,
		Name:          "mount",
	}
	if err := tDB.db.Create(mountPoint).Error; err != nil {
		t.Fatalf("create mount point: %v", err)
	}

	binding := &models.UserMountPointToken{
		UserID:       20,
		MountPointID: mountPoint.ID,
		TokenID:      token.ID,
	}
	if err := tDB.db.Create(binding).Error; err != nil {
		t.Fatalf("create token binding: %v", err)
	}

	plan := createAutoIngestPlanWithToken(t, tDB.db, 20, token.ID, "other-plan")

	err := svc.Delete(ctx, &DeleteRequest{ID: token.ID, UserID: 10})
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("expected record not found, got %v", err)
	}

	var updatedMountPoint models.MountPoint
	if err := tDB.db.First(&updatedMountPoint, mountPoint.ID).Error; err != nil {
		t.Fatalf("query mount point: %v", err)
	}

	if updatedMountPoint.TokenId != token.ID {
		t.Fatalf("expected mount point token unchanged, got %d", updatedMountPoint.TokenId)
	}

	var bindingCount int64
	if err := tDB.db.Model(&models.UserMountPointToken{}).Where("token_id = ?", token.ID).Count(&bindingCount).Error; err != nil {
		t.Fatalf("count token binding: %v", err)
	}

	if bindingCount != 1 {
		t.Fatalf("expected token binding to remain, got count %d", bindingCount)
	}

	var updatedPlan models.AutoIngestPlan
	if err := tDB.db.First(&updatedPlan, plan.ID).Error; err != nil {
		t.Fatalf("query auto ingest plan: %v", err)
	}

	if updatedPlan.TokenId != token.ID {
		t.Fatalf("expected auto ingest plan token unchanged, got %d", updatedPlan.TokenId)
	}
}

func TestDeleteRejectsMissingUserIDForNonAdmin(t *testing.T) {
	tDB := setupCloudTokenTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())

	token := createCloudToken(t, tDB.db, 10, "missing-user")

	mountPoint := &models.MountPoint{
		FileId:        1001,
		OsType:        "folder",
		TokenId:       token.ID,
		CreatorUserID: 10,
		Name:          "mount",
	}
	if err := tDB.db.Create(mountPoint).Error; err != nil {
		t.Fatalf("create mount point: %v", err)
	}

	binding := &models.UserMountPointToken{
		UserID:       10,
		MountPointID: mountPoint.ID,
		TokenID:      token.ID,
	}
	if err := tDB.db.Create(binding).Error; err != nil {
		t.Fatalf("create token binding: %v", err)
	}

	err := svc.Delete(ctx, &DeleteRequest{ID: token.ID})
	if !errors.Is(err, errInvalidCloudTokenUserID) {
		t.Fatalf("expected invalid user id, got %v", err)
	}

	var tokenCount int64
	if err := tDB.db.Model(&models.CloudToken{}).Where("id = ?", token.ID).Count(&tokenCount).Error; err != nil {
		t.Fatalf("count token: %v", err)
	}

	if tokenCount != 1 {
		t.Fatalf("expected token to remain, got count %d", tokenCount)
	}

	var updatedMountPoint models.MountPoint
	if err := tDB.db.First(&updatedMountPoint, mountPoint.ID).Error; err != nil {
		t.Fatalf("query mount point: %v", err)
	}

	if updatedMountPoint.TokenId != token.ID {
		t.Fatalf("expected mount point token unchanged, got %d", updatedMountPoint.TokenId)
	}

	var bindingCount int64
	if err := tDB.db.Model(&models.UserMountPointToken{}).Where("token_id = ?", token.ID).Count(&bindingCount).Error; err != nil {
		t.Fatalf("count token binding: %v", err)
	}

	if bindingCount != 1 {
		t.Fatalf("expected token binding to remain, got count %d", bindingCount)
	}
}

func TestDeleteAdminTokenClearsReferences(t *testing.T) {
	tDB := setupCloudTokenTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())

	token := createCloudToken(t, tDB.db, 10, "admin-delete")

	mountPoint := &models.MountPoint{
		FileId:        1001,
		OsType:        "folder",
		TokenId:       token.ID,
		CreatorUserID: 10,
		Name:          "mount",
	}
	if err := tDB.db.Create(mountPoint).Error; err != nil {
		t.Fatalf("create mount point: %v", err)
	}

	binding := &models.UserMountPointToken{
		UserID:       10,
		MountPointID: mountPoint.ID,
		TokenID:      token.ID,
	}
	if err := tDB.db.Create(binding).Error; err != nil {
		t.Fatalf("create token binding: %v", err)
	}

	plan := createAutoIngestPlanWithToken(t, tDB.db, 10, token.ID, "admin-plan")

	if err := svc.Delete(ctx, &DeleteRequest{ID: token.ID, IsAdmin: true}); err != nil {
		t.Fatalf("admin delete token: %v", err)
	}

	var tokenCount int64
	if err := tDB.db.Model(&models.CloudToken{}).Where("id = ?", token.ID).Count(&tokenCount).Error; err != nil {
		t.Fatalf("count token: %v", err)
	}

	if tokenCount != 0 {
		t.Fatalf("expected token deleted, got count %d", tokenCount)
	}

	var updatedMountPoint models.MountPoint
	if err := tDB.db.First(&updatedMountPoint, mountPoint.ID).Error; err != nil {
		t.Fatalf("query mount point: %v", err)
	}

	if updatedMountPoint.TokenId != 0 {
		t.Fatalf("expected mount point token reset to 0, got %d", updatedMountPoint.TokenId)
	}

	var bindingCount int64
	if err := tDB.db.Model(&models.UserMountPointToken{}).Where("token_id = ?", token.ID).Count(&bindingCount).Error; err != nil {
		t.Fatalf("count token binding: %v", err)
	}

	if bindingCount != 0 {
		t.Fatalf("expected token bindings deleted, got count %d", bindingCount)
	}

	var updatedPlan models.AutoIngestPlan
	if err := tDB.db.First(&updatedPlan, plan.ID).Error; err != nil {
		t.Fatalf("query auto ingest plan: %v", err)
	}

	if updatedPlan.TokenId != 0 {
		t.Fatalf("expected auto ingest plan token reset to 0, got %d", updatedPlan.TokenId)
	}
}

func TestUpdateAdditionUpdatesExistingToken(t *testing.T) {
	tDB := setupCloudTokenTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())

	token := createCloudToken(t, tDB.db, 10, "addition")

	addition := map[string]interface{}{
		"auto_login_result": "ok",
		"auto_login_times":  float64(3),
	}
	if err := svc.UpdateAddition(ctx, token.ID, addition); err != nil {
		t.Fatalf("update addition: %v", err)
	}

	var updated models.CloudToken
	if err := tDB.db.First(&updated, token.ID).Error; err != nil {
		t.Fatalf("query updated token: %v", err)
	}

	if got := updated.Addition["auto_login_result"]; got != "ok" {
		t.Fatalf("expected auto_login_result ok, got %v", got)
	}

	if got := updated.Addition["auto_login_times"]; fmt.Sprint(got) != "3" {
		t.Fatalf("expected auto_login_times 3, got %v", got)
	}
}

func TestUpdateAdditionAllowsEmptyMap(t *testing.T) {
	tDB := setupCloudTokenTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())

	token := createCloudToken(t, tDB.db, 10, "empty-addition")
	if err := tDB.db.Model(&models.CloudToken{}).Where("id = ?", token.ID).
		Update("addition", datatypes.JSONMap{"old": "value"}).Error; err != nil {
		t.Fatalf("seed addition: %v", err)
	}

	if err := svc.UpdateAddition(ctx, token.ID, map[string]interface{}{}); err != nil {
		t.Fatalf("update addition: %v", err)
	}

	var updated models.CloudToken
	if err := tDB.db.First(&updated, token.ID).Error; err != nil {
		t.Fatalf("query updated token: %v", err)
	}

	if len(updated.Addition) != 0 {
		t.Fatalf("expected empty addition, got %+v", updated.Addition)
	}
}

func TestUpdateAdditionReturnsNotFoundWhenTokenMissing(t *testing.T) {
	tDB := setupCloudTokenTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())

	err := svc.UpdateAddition(ctx, 99999, map[string]interface{}{"key": "value"})
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("expected record not found, got %v", err)
	}

	var count int64
	if err := tDB.db.Model(&models.CloudToken{}).Where("id = ?", 99999).Count(&count).Error; err != nil {
		t.Fatalf("count missing token: %v", err)
	}

	if count != 0 {
		t.Fatalf("expected no upserted token, got count %d", count)
	}
}

func TestUpdateAdditionRejectsInvalidID(t *testing.T) {
	tDB := setupCloudTokenTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())

	err := svc.UpdateAddition(ctx, 0, map[string]interface{}{"key": "value"})
	if !errors.Is(err, errInvalidCloudTokenID) {
		t.Fatalf("expected invalid token id, got %v", err)
	}
}
