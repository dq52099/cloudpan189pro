package storagefacade

import (
	stdctx "context"
	"errors"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/xxcheng123/cloudpan189-share/internal/bootstrap"
	"github.com/xxcheng123/cloudpan189-share/internal/consts"
	"github.com/xxcheng123/cloudpan189-share/internal/framework/context"
	"github.com/xxcheng123/cloudpan189-share/internal/pkgs/datatypes"
	"github.com/xxcheng123/cloudpan189-share/internal/pkgs/taskengine"
	"github.com/xxcheng123/cloudpan189-share/internal/repository/models"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type storageFacadeTestDB struct {
	db *gorm.DB
}

func (t *storageFacadeTestDB) GetDB(ctx context.Context) *gorm.DB {
	return t.db.WithContext(ctx)
}

func (t *storageFacadeTestDB) GetDBWithoutContext() *gorm.DB {
	return t.db
}

func (t *storageFacadeTestDB) GetLogger(name string, fields ...zap.Field) *zap.Logger {
	return zap.NewNop()
}

func (t *storageFacadeTestDB) Close() {}

func (t *storageFacadeTestDB) GetPort() int {
	return 9999
}

func (t *storageFacadeTestDB) GetTaskEngine() taskengine.TaskEngine {
	return nil
}

func (t *storageFacadeTestDB) GetHTTPEngine() *gin.Engine {
	return nil
}

var _ bootstrap.ServiceContext = (*storageFacadeTestDB)(nil)

func setupStorageFacadeTestDB(t *testing.T) *storageFacadeTestDB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}

	if err = db.AutoMigrate(&models.VirtualFile{}, &models.MountPoint{}, &models.UserMountPointToken{}, &models.CloudToken{}); err != nil {
		t.Fatalf("migrate test db: %v", err)
	}

	return &storageFacadeTestDB{db: db}
}

func createStorageFacadeVirtualDir(t *testing.T, db *gorm.DB, parentID int64, name string) *models.VirtualFile {
	t.Helper()

	now := time.Now()

	file := &models.VirtualFile{
		ParentId:   parentID,
		Name:       name,
		IsTop:      true,
		IsDir:      true,
		Size:       0,
		OsType:     models.OsTypeFolder,
		CreateDate: now,
		ModifyDate: now,
		Rev:        now.Format(consts.RevFormat),
		Addition:   datatypes.JSONMap{},
	}
	if err := db.Create(file).Error; err != nil {
		t.Fatalf("create virtual dir: %v", err)
	}

	if err := db.Model(file).Update("top_id", file.ID).Error; err != nil {
		t.Fatalf("update top id: %v", err)
	}

	return file
}

func createStorageFacadeCloudToken(t *testing.T, db *gorm.DB, userID int64, name string) *models.CloudToken {
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

func TestCreateStorageAllowExistingRejectsPlainVirtualFile(t *testing.T) {
	tDB := setupStorageFacadeTestDB(t)
	createStorageFacadeVirtualDir(t, tDB.db, 0, "taken")

	svc := NewService(tDB)

	_, err := svc.CreateStorage(context.NewContext(stdctx.Background()), &CreateStorageRequest{
		LocalPath:     "/taken",
		OsType:        models.OsTypeFolder,
		CloudToken:    0,
		FileId:        "cloud-id",
		Addition:      datatypes.JSONMap{},
		CreatorUserID: 100,
		AllowExisting: true,
	})
	if !errors.Is(err, ErrPathAlreadyExists) {
		t.Fatalf("expected ErrPathAlreadyExists, got %v", err)
	}

	var mountPointCount int64
	if err = tDB.db.Model(&models.MountPoint{}).Where("full_path = ?", "/taken").Count(&mountPointCount).Error; err != nil {
		t.Fatalf("count mount points: %v", err)
	}

	if mountPointCount != 0 {
		t.Fatalf("expected no mount point created, got %d", mountPointCount)
	}
}

func TestCreateStorageAllowExistingReturnsExistingMountPointRoot(t *testing.T) {
	tDB := setupStorageFacadeTestDB(t)
	existingFile := createStorageFacadeVirtualDir(t, tDB.db, 0, "mounted")

	mountPoint := &models.MountPoint{
		FileId:        existingFile.ID,
		Name:          "mounted",
		FullPath:      "/mounted",
		OsType:        models.OsTypeFolder,
		TokenId:       0,
		CreatorUserID: 100,
	}
	if err := tDB.db.Create(mountPoint).Error; err != nil {
		t.Fatalf("create mount point: %v", err)
	}

	svc := NewService(tDB)

	id, err := svc.CreateStorage(context.NewContext(stdctx.Background()), &CreateStorageRequest{
		LocalPath:     "/mounted",
		OsType:        models.OsTypeFolder,
		CloudToken:    0,
		FileId:        "cloud-id",
		Addition:      datatypes.JSONMap{},
		CreatorUserID: 200,
		AllowExisting: true,
	})
	if err != nil {
		t.Fatalf("create storage: %v", err)
	}

	if id != existingFile.ID {
		t.Fatalf("expected existing file id %d, got %d", existingFile.ID, id)
	}

	var mountPointCount int64
	if err = tDB.db.Model(&models.MountPoint{}).Where("full_path = ?", "/mounted").Count(&mountPointCount).Error; err != nil {
		t.Fatalf("count mount points: %v", err)
	}

	if mountPointCount != 1 {
		t.Fatalf("expected existing mount point only, got %d", mountPointCount)
	}
}

func TestCreateStorageRejectsInaccessibleCloudToken(t *testing.T) {
	tDB := setupStorageFacadeTestDB(t)
	token := createStorageFacadeCloudToken(t, tDB.db, 200, "other-user-token")

	svc := NewService(tDB)

	_, err := svc.CreateStorage(context.NewContext(stdctx.Background()), &CreateStorageRequest{
		LocalPath:     "/private-token",
		OsType:        models.OsTypePersonFolder,
		CloudToken:    token.ID,
		FileId:        "cloud-id",
		Addition:      datatypes.JSONMap{},
		CreatorUserID: 100,
	})
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("expected record not found for inaccessible token, got %v", err)
	}

	var mountPointCount int64
	if err = tDB.db.Model(&models.MountPoint{}).Where("full_path = ?", "/private-token").Count(&mountPointCount).Error; err != nil {
		t.Fatalf("count mount points: %v", err)
	}

	if mountPointCount != 0 {
		t.Fatalf("expected no mount point created, got %d", mountPointCount)
	}

	var virtualFileCount int64
	if err = tDB.db.Model(&models.VirtualFile{}).Where("name = ?", "private-token").Count(&virtualFileCount).Error; err != nil {
		t.Fatalf("count virtual files: %v", err)
	}

	if virtualFileCount != 0 {
		t.Fatalf("expected no virtual file created, got %d", virtualFileCount)
	}
}

func TestCreateStorageAllowsAdminCloudTokenAccess(t *testing.T) {
	tDB := setupStorageFacadeTestDB(t)
	token := createStorageFacadeCloudToken(t, tDB.db, 200, "admin-access-token")

	svc := NewService(tDB)

	id, err := svc.CreateStorage(context.NewContext(stdctx.Background()), &CreateStorageRequest{
		LocalPath:     "/admin-token",
		OsType:        models.OsTypePersonFolder,
		CloudToken:    token.ID,
		FileId:        "cloud-id",
		Addition:      datatypes.JSONMap{},
		CreatorUserID: 100,
		IsAdmin:       true,
	})
	if err != nil {
		t.Fatalf("create storage as admin: %v", err)
	}

	if id <= 0 {
		t.Fatalf("expected created virtual file id, got %d", id)
	}

	var mountPoint models.MountPoint
	if err = tDB.db.Where("full_path = ?", "/admin-token").First(&mountPoint).Error; err != nil {
		t.Fatalf("query mount point: %v", err)
	}

	if mountPoint.TokenId != token.ID {
		t.Fatalf("expected token id %d, got %d", token.ID, mountPoint.TokenId)
	}
}

func TestCreateStorageAllowExistingStillValidatesCloudTokenAccess(t *testing.T) {
	tDB := setupStorageFacadeTestDB(t)
	token := createStorageFacadeCloudToken(t, tDB.db, 200, "other-user-token")
	existingFile := createStorageFacadeVirtualDir(t, tDB.db, 0, "mounted-private")

	mountPoint := &models.MountPoint{
		FileId:        existingFile.ID,
		Name:          "mounted-private",
		FullPath:      "/mounted-private",
		OsType:        models.OsTypePersonFolder,
		TokenId:       token.ID,
		CreatorUserID: 200,
	}
	if err := tDB.db.Create(mountPoint).Error; err != nil {
		t.Fatalf("create mount point: %v", err)
	}

	svc := NewService(tDB)

	_, err := svc.CreateStorage(context.NewContext(stdctx.Background()), &CreateStorageRequest{
		LocalPath:     "/mounted-private",
		OsType:        models.OsTypePersonFolder,
		CloudToken:    token.ID,
		FileId:        "cloud-id",
		Addition:      datatypes.JSONMap{},
		CreatorUserID: 100,
		AllowExisting: true,
	})
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("expected record not found before returning existing mount point, got %v", err)
	}
}
