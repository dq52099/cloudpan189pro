package user

import (
	stdctx "context"
	"errors"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/xxcheng123/cloudpan189-share/internal/bootstrap"
	"github.com/xxcheng123/cloudpan189-share/internal/framework/context"
	"github.com/xxcheng123/cloudpan189-share/internal/pkgs/taskengine"
	"github.com/xxcheng123/cloudpan189-share/internal/pkgs/utils"
	"github.com/xxcheng123/cloudpan189-share/internal/repository/models"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type userTestDB struct {
	db *gorm.DB
}

func (t *userTestDB) GetDB(ctx context.Context) *gorm.DB {
	return t.db.WithContext(ctx)
}

func (t *userTestDB) GetDBWithoutContext() *gorm.DB {
	return t.db
}

func (t *userTestDB) GetLogger(name string, fields ...zap.Field) *zap.Logger {
	return zap.NewNop()
}

func (t *userTestDB) Close() {}

func (t *userTestDB) GetPort() int {
	return 9999
}

func (t *userTestDB) GetTaskEngine() taskengine.TaskEngine {
	return nil
}

func (t *userTestDB) GetHTTPEngine() *gin.Engine {
	return nil
}

var _ bootstrap.ServiceContext = (*userTestDB)(nil)

func setupUserTestDB(t *testing.T) *userTestDB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}

	if err := db.AutoMigrate(
		&models.User{},
		&models.CloudToken{},
		&models.MountPoint{},
		&models.VirtualFile{},
		&models.UserMountPointToken{},
		&models.AutoIngestPlan{},
		&models.AutoIngestLog{},
	); err != nil {
		t.Fatalf("migrate test db: %v", err)
	}

	return &userTestDB{db: db}
}

func createUser(t *testing.T, db *gorm.DB, username string, groupID int64) *models.User {
	t.Helper()

	user := &models.User{
		Username: username,
		Password: "password",
		GroupID:  groupID,
	}
	if err := db.Create(user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}

	return user
}

func TestAddRejectsInvalidRequest(t *testing.T) {
	tDB := setupUserTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())

	tests := []struct {
		name    string
		req     *AddRequest
		wantErr error
	}{
		{name: "nil request", req: nil, wantErr: errInvalidUsername},
		{name: "empty username", req: &AddRequest{Password: "password"}, wantErr: errInvalidUsername},
		{name: "empty password", req: &AddRequest{Username: "new-user"}, wantErr: errInvalidUserPassword},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := svc.Add(ctx, tt.req)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("expected %v, got %v", tt.wantErr, err)
			}
		})
	}
}

func TestQueryRejectsInvalidUserID(t *testing.T) {
	tDB := setupUserTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())

	_, err := svc.Query(ctx, 0)
	if !errors.Is(err, errInvalidUserID) {
		t.Fatalf("expected invalid user id, got %v", err)
	}
}

func TestQueryReturnsExistingUser(t *testing.T) {
	tDB := setupUserTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())

	user := createUser(t, tDB.db, "query-target", 1)

	got, err := svc.Query(ctx, user.ID)
	if err != nil {
		t.Fatalf("query user: %v", err)
	}

	if got.ID != user.ID {
		t.Fatalf("expected user id %d, got %d", user.ID, got.ID)
	}
}

func TestQueryReturnsNotFoundWhenUserMissing(t *testing.T) {
	tDB := setupUserTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())

	_, err := svc.Query(ctx, 99999)
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("expected record not found, got %v", err)
	}
}

func TestQueryByUsernameRejectsBlankUsername(t *testing.T) {
	tDB := setupUserTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())

	tests := []string{"", "   "}
	for _, username := range tests {
		t.Run("blank", func(t *testing.T) {
			_, err := svc.QueryByUsername(ctx, username)
			if !errors.Is(err, errInvalidUsername) {
				t.Fatalf("expected invalid username, got %v", err)
			}
		})
	}
}

func TestQueryByUsernameUsesExactUsername(t *testing.T) {
	tDB := setupUserTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())

	user := createUser(t, tDB.db, "query-name", 1)

	got, err := svc.QueryByUsername(ctx, user.Username)
	if err != nil {
		t.Fatalf("query user by username: %v", err)
	}

	if got.ID != user.ID {
		t.Fatalf("expected user id %d, got %d", user.ID, got.ID)
	}

	_, err = svc.QueryByUsername(ctx, " "+user.Username+" ")
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("expected spaced username to preserve exact matching, got %v", err)
	}
}

func TestBindGroupUpdatesOnlyRequestedUser(t *testing.T) {
	tDB := setupUserTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())

	target := createUser(t, tDB.db, "target", 1)
	other := createUser(t, tDB.db, "other", 2)

	if err := svc.BindGroup(ctx, &BindGroupRequest{UserID: target.ID, GroupID: 3}); err != nil {
		t.Fatalf("bind group: %v", err)
	}

	var updatedTarget models.User
	if err := tDB.db.First(&updatedTarget, target.ID).Error; err != nil {
		t.Fatalf("query target user: %v", err)
	}

	if updatedTarget.GroupID != 3 {
		t.Fatalf("expected target group id 3, got %d", updatedTarget.GroupID)
	}

	var unchangedOther models.User
	if err := tDB.db.First(&unchangedOther, other.ID).Error; err != nil {
		t.Fatalf("query other user: %v", err)
	}

	if unchangedOther.GroupID != 2 {
		t.Fatalf("expected other group id unchanged at 2, got %d", unchangedOther.GroupID)
	}
}

func TestBindGroupReturnsNotFoundWhenUserMissing(t *testing.T) {
	tDB := setupUserTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())

	err := svc.BindGroup(ctx, &BindGroupRequest{UserID: 99999, GroupID: 3})
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("expected record not found, got %v", err)
	}
}

func TestBindGroupNoOpExistingUserSucceeds(t *testing.T) {
	tDB := setupUserTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())

	user := createUser(t, tDB.db, "target", 3)

	if err := svc.BindGroup(ctx, &BindGroupRequest{UserID: user.ID, GroupID: user.GroupID}); err != nil {
		t.Fatalf("expected no-op bind group to succeed, got %v", err)
	}
}

func TestBindGroupRejectsInvalidUserID(t *testing.T) {
	tDB := setupUserTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())

	tests := []struct {
		name string
		req  *BindGroupRequest
	}{
		{name: "nil request", req: nil},
		{name: "zero id", req: &BindGroupRequest{UserID: 0, GroupID: 3}},
		{name: "negative id", req: &BindGroupRequest{UserID: -1, GroupID: 3}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := svc.BindGroup(ctx, tt.req)
			if !errors.Is(err, errInvalidUserID) {
				t.Fatalf("expected invalid user id, got %v", err)
			}
		})
	}
}

func TestBindGroupRejectsInvalidGroupID(t *testing.T) {
	tDB := setupUserTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())

	user := createUser(t, tDB.db, "target", 1)

	err := svc.BindGroup(ctx, &BindGroupRequest{UserID: user.ID, GroupID: -1})
	if !errors.Is(err, errInvalidUserGroupID) {
		t.Fatalf("expected invalid user group id, got %v", err)
	}

	var updated models.User
	if err := tDB.db.First(&updated, user.ID).Error; err != nil {
		t.Fatalf("query user: %v", err)
	}

	if updated.GroupID != 1 {
		t.Fatalf("expected group id unchanged at 1, got %d", updated.GroupID)
	}
}

func TestUpdateReturnsNotFoundWhenUserMissing(t *testing.T) {
	tDB := setupUserTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())

	err := svc.Update(ctx, 99999, utils.WithField("status", int8(2)))
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("expected record not found, got %v", err)
	}
}

func TestUpdateNoOpExistingUserSucceeds(t *testing.T) {
	tDB := setupUserTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())

	user := createUser(t, tDB.db, "target", 1)

	if err := svc.Update(ctx, user.ID, utils.WithField("status", user.Status)); err != nil {
		t.Fatalf("expected no-op update to succeed, got %v", err)
	}
}

func TestCheckUserUpdateResultAllowsExistingNoop(t *testing.T) {
	tDB := setupUserTestDB(t)
	svc := NewService(tDB).(*service)
	ctx := context.NewContext(stdctx.Background())

	user := createUser(t, tDB.db, "target", 1)

	if err := svc.checkUserUpdateResult(ctx, &gorm.DB{RowsAffected: 0}, user.ID); err != nil {
		t.Fatalf("expected existing no-op update to succeed, got %v", err)
	}
}

func TestCheckUserUpdateResultReturnsNotFoundWhenMissing(t *testing.T) {
	tDB := setupUserTestDB(t)
	svc := NewService(tDB).(*service)
	ctx := context.NewContext(stdctx.Background())

	err := svc.checkUserUpdateResult(ctx, &gorm.DB{RowsAffected: 0}, 99999)
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("expected record not found, got %v", err)
	}
}

func TestUpdateRejectsInvalidUserID(t *testing.T) {
	tDB := setupUserTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())

	err := svc.Update(ctx, 0, utils.WithField("status", int8(2)))
	if !errors.Is(err, errInvalidUserID) {
		t.Fatalf("expected invalid user id, got %v", err)
	}
}

func TestUpdateRejectsEmptyFields(t *testing.T) {
	tDB := setupUserTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())

	user := createUser(t, tDB.db, "target", 1)

	err := svc.Update(ctx, user.ID)
	if !errors.Is(err, errEmptyUserUpdateFields) {
		t.Fatalf("expected empty update fields, got %v", err)
	}
}

func TestModifyPassReturnsNotFoundWhenUserMissing(t *testing.T) {
	tDB := setupUserTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())

	err := svc.ModifyPass(ctx, 99999, "new-password")
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("expected record not found, got %v", err)
	}
}

func TestModifyPassRejectsInvalidUserID(t *testing.T) {
	tDB := setupUserTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())

	err := svc.ModifyPass(ctx, 0, "new-password")
	if !errors.Is(err, errInvalidUserID) {
		t.Fatalf("expected invalid user id, got %v", err)
	}
}

func TestModifyPassRejectsBlankPassword(t *testing.T) {
	tDB := setupUserTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())

	user := createUser(t, tDB.db, "target", 1)

	err := svc.ModifyPass(ctx, user.ID, "   ")
	if !errors.Is(err, errInvalidUserPassword) {
		t.Fatalf("expected invalid user password, got %v", err)
	}

	var updated models.User
	if err := tDB.db.First(&updated, user.ID).Error; err != nil {
		t.Fatalf("query user: %v", err)
	}

	if updated.Password != "password" {
		t.Fatalf("expected password unchanged, got %q", updated.Password)
	}
}

func TestDelReturnsNotFoundWhenUserMissing(t *testing.T) {
	tDB := setupUserTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())

	err := svc.Del(ctx, &DelRequest{ID: 99999})
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("expected record not found, got %v", err)
	}
}

func TestDelRejectsInvalidUserID(t *testing.T) {
	tDB := setupUserTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())

	tests := []struct {
		name string
		req  *DelRequest
	}{
		{name: "nil request", req: nil},
		{name: "zero id", req: &DelRequest{ID: 0}},
		{name: "negative id", req: &DelRequest{ID: -1}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := svc.Del(ctx, tt.req)
			if !errors.Is(err, errInvalidUserID) {
				t.Fatalf("expected invalid user id, got %v", err)
			}
		})
	}
}

func TestDelRemovesExistingUser(t *testing.T) {
	tDB := setupUserTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())

	createUser(t, tDB.db, "founder", 0)
	user := createUser(t, tDB.db, "delete-me", 0)

	if err := svc.Del(ctx, &DelRequest{ID: user.ID}); err != nil {
		t.Fatalf("delete user: %v", err)
	}

	var count int64
	if err := tDB.db.Model(&models.User{}).Where("id = ?", user.ID).Count(&count).Error; err != nil {
		t.Fatalf("count user: %v", err)
	}

	if count != 0 {
		t.Fatalf("expected user deleted, got count %d", count)
	}
}

func TestDelCleansOwnedResourcesAndReferences(t *testing.T) {
	tDB := setupUserTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())

	createUser(t, tDB.db, "founder", 0)
	user := createUser(t, tDB.db, "delete-with-relations", 0)
	otherUser := createUser(t, tDB.db, "other-user", 0)

	token := &models.CloudToken{
		Name:        "owned-token",
		AccessToken: "access",
		ExpiresIn:   3600,
		Status:      1,
		UserID:      user.ID,
	}
	if err := tDB.db.Create(token).Error; err != nil {
		t.Fatalf("create cloud token: %v", err)
	}

	ownedMountPoint := &models.MountPoint{
		FileId:        1001,
		OsType:        models.OsTypeFolder,
		TokenId:       token.ID,
		CreatorUserID: user.ID,
		Name:          "owned",
		FullPath:      "/owned",
	}
	if err := tDB.db.Create(ownedMountPoint).Error; err != nil {
		t.Fatalf("create owned mount point: %v", err)
	}

	otherMountPoint := &models.MountPoint{
		FileId:        1002,
		OsType:        models.OsTypeFolder,
		TokenId:       token.ID,
		CreatorUserID: otherUser.ID,
		Name:          "other",
		FullPath:      "/other",
	}
	if err := tDB.db.Create(otherMountPoint).Error; err != nil {
		t.Fatalf("create other mount point: %v", err)
	}

	virtualFiles := []*models.VirtualFile{
		{
			ID:       ownedMountPoint.FileId,
			TopId:    ownedMountPoint.FileId,
			IsTop:    true,
			IsDir:    true,
			Name:     "owned",
			OsType:   models.OsTypeFolder,
			ParentId: 0,
		},
		{
			ID:       1003,
			TopId:    ownedMountPoint.FileId,
			IsDir:    true,
			Name:     "owned-child",
			OsType:   models.OsTypeFolder,
			ParentId: ownedMountPoint.FileId,
		},
		{
			ID:       otherMountPoint.FileId,
			TopId:    otherMountPoint.FileId,
			IsTop:    true,
			IsDir:    true,
			Name:     "other",
			OsType:   models.OsTypeFolder,
			ParentId: 0,
		},
	}
	if err := tDB.db.Create(&virtualFiles).Error; err != nil {
		t.Fatalf("create virtual files: %v", err)
	}

	bindings := []*models.UserMountPointToken{
		{UserID: user.ID, MountPointID: ownedMountPoint.ID, TokenID: token.ID},
		{UserID: otherUser.ID, MountPointID: ownedMountPoint.ID, TokenID: token.ID},
		{UserID: otherUser.ID, MountPointID: otherMountPoint.ID, TokenID: token.ID},
	}
	if err := tDB.db.Create(&bindings).Error; err != nil {
		t.Fatalf("create user mount point token bindings: %v", err)
	}

	plan := &models.AutoIngestPlan{
		Name:       "owned-plan",
		SourceType: "subscribe",
		ParentPath: "/auto",
		UserID:     user.ID,
	}
	if err := tDB.db.Create(plan).Error; err != nil {
		t.Fatalf("create auto ingest plan: %v", err)
	}

	planLog := &models.AutoIngestLog{
		PlanId:  plan.ID,
		Level:   "info",
		Content: "created",
	}
	if err := tDB.db.Create(planLog).Error; err != nil {
		t.Fatalf("create auto ingest log: %v", err)
	}

	if err := svc.Del(ctx, &DelRequest{ID: user.ID}); err != nil {
		t.Fatalf("delete user: %v", err)
	}

	assertCount(t, tDB.db, &models.User{}, "id = ?", 0, user.ID)
	assertCount(t, tDB.db, &models.CloudToken{}, "id = ?", 0, token.ID)
	assertCount(t, tDB.db, &models.MountPoint{}, "id = ?", 0, ownedMountPoint.ID)
	assertCount(t, tDB.db, &models.VirtualFile{}, "id = ?", 0, ownedMountPoint.FileId)
	assertCount(t, tDB.db, &models.VirtualFile{}, "top_id = ?", 0, ownedMountPoint.FileId)
	assertCount(t, tDB.db, &models.AutoIngestPlan{}, "id = ?", 0, plan.ID)
	assertCount(t, tDB.db, &models.AutoIngestLog{}, "plan_id = ?", 0, plan.ID)
	assertCount(t, tDB.db, &models.UserMountPointToken{}, "1 = 1", 0)

	var updatedMountPoint models.MountPoint
	if err := tDB.db.First(&updatedMountPoint, otherMountPoint.ID).Error; err != nil {
		t.Fatalf("query other mount point: %v", err)
	}

	if updatedMountPoint.TokenId != 0 {
		t.Fatalf("expected other mount point token reset, got %d", updatedMountPoint.TokenId)
	}

	assertCount(t, tDB.db, &models.User{}, "id = ?", 1, otherUser.ID)
	assertCount(t, tDB.db, &models.VirtualFile{}, "id = ?", 1, otherMountPoint.FileId)
}

func TestDelMissingUserKeepsRelations(t *testing.T) {
	tDB := setupUserTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())

	token := &models.CloudToken{
		Name:        "orphan-check",
		AccessToken: "access",
		ExpiresIn:   3600,
		Status:      1,
		UserID:      99999,
	}
	if err := tDB.db.Create(token).Error; err != nil {
		t.Fatalf("create cloud token: %v", err)
	}

	err := svc.Del(ctx, &DelRequest{ID: 99999})
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("expected record not found, got %v", err)
	}

	assertCount(t, tDB.db, &models.CloudToken{}, "id = ?", 1, token.ID)
}

func assertCount(t *testing.T, db *gorm.DB, model any, query string, want int64, args ...any) {
	t.Helper()

	var count int64
	if err := db.Model(model).Where(query, args...).Count(&count).Error; err != nil {
		t.Fatalf("count model: %v", err)
	}

	if count != want {
		t.Fatalf("expected count %d, got %d", want, count)
	}
}
