package storagefacade

import (
	stdctx "context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	mysqlDriver "github.com/go-sql-driver/mysql"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/xxcheng123/cloudpan189-share/internal/bootstrap"
	"github.com/xxcheng123/cloudpan189-share/internal/consts"
	"github.com/xxcheng123/cloudpan189-share/internal/framework/context"
	"github.com/xxcheng123/cloudpan189-share/internal/pkgs/datatypes"
	"github.com/xxcheng123/cloudpan189-share/internal/pkgs/taskengine"
	"github.com/xxcheng123/cloudpan189-share/internal/repository/models"
	cloudtokenSvi "github.com/xxcheng123/cloudpan189-share/internal/services/cloudtoken"
	mountPointSvi "github.com/xxcheng123/cloudpan189-share/internal/services/mountpoint"
	virtualFileSvi "github.com/xxcheng123/cloudpan189-share/internal/services/virtualfile"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type storageFacadeTestDB struct {
	db *gorm.DB
}

func (t *storageFacadeTestDB) GetDB(ctx context.Context) *gorm.DB {
	return bootstrap.DBFromContext(ctx, t.db)
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

type mockCreateStorageMountPointService struct {
	mountPointSvi.Service

	queryByPathResult *models.MountPoint
	queryByPathErr    error
}

func (m *mockCreateStorageMountPointService) QueryByPath(ctx context.Context, filePath string) (*models.MountPoint, error) {
	return m.queryByPathResult, m.queryByPathErr
}

type mockCreateStorageVirtualFileService struct {
	virtualFileSvi.Service

	queryResult *models.VirtualFile
	queryErr    error
	queriedIDs  []int64
}

func (m *mockCreateStorageVirtualFileService) Query(ctx context.Context, fid int64) (*models.VirtualFile, error) {
	m.queriedIDs = append(m.queriedIDs, fid)

	return m.queryResult, m.queryErr
}

type mockCreateStorageCloudTokenService struct {
	cloudtokenSvi.Service

	token   *models.CloudToken
	err     error
	queries []int64
}

func (m *mockCreateStorageCloudTokenService) QueryAccessible(
	ctx context.Context,
	id int64,
	userID int64,
	isAdmin bool,
) (*models.CloudToken, error) {
	m.queries = append(m.queries, id)
	if m.err != nil {
		return nil, m.err
	}

	return m.token, nil
}

type storageFacadeDuplicateSQLiteCodeError struct {
	code int
}

func (e storageFacadeDuplicateSQLiteCodeError) Error() string {
	return "sqlite constraint error"
}

func (e storageFacadeDuplicateSQLiteCodeError) Code() int {
	return e.code
}

func TestIsUniqueConstraintErrorRecognizesDriverErrors(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "mysql duplicate",
			err:  &mysqlDriver.MySQLError{Number: 1062, Message: "Duplicate entry"},
			want: true,
		},
		{
			name: "postgres duplicate",
			err:  fmt.Errorf("create storage: %w", &pgconn.PgError{Code: "23505", Message: "duplicate key value violates unique constraint"}),
			want: true,
		},
		{
			name: "sqlite unique constraint code",
			err:  storageFacadeDuplicateSQLiteCodeError{code: 2067},
			want: true,
		},
		{
			name: "sqlite primary key constraint code",
			err:  fmt.Errorf("wrapped: %w", storageFacadeDuplicateSQLiteCodeError{code: 1555}),
			want: true,
		},
		{
			name: "sqlite text fallback",
			err:  errors.New("UNIQUE constraint failed: virtual_files.parent_id, virtual_files.name"),
			want: true,
		},
		{
			name: "non duplicate",
			err:  errors.New("database is locked"),
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
			if got := isUniqueConstraintError(tt.err); got != tt.want {
				t.Fatalf("expected %v, got %v for %v", tt.want, got, tt.err)
			}
		})
	}
}

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
		CloudId:    "cloud-id",
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

func countStorageFacadeVirtualFilesByNames(t *testing.T, db *gorm.DB, names ...string) int64 {
	t.Helper()

	var count int64
	if err := db.Model(&models.VirtualFile{}).Where("name IN ?", names).Count(&count).Error; err != nil {
		t.Fatalf("count virtual files: %v", err)
	}

	return count
}

func countStorageFacadeMountPointsByPath(t *testing.T, db *gorm.DB, fullPath string) int64 {
	t.Helper()

	var count int64
	if err := db.Model(&models.MountPoint{}).Where("full_path = ?", fullPath).Count(&count).Error; err != nil {
		t.Fatalf("count mount points: %v", err)
	}

	return count
}

func registerStorageFacadeMountPointCreateError(t *testing.T, db *gorm.DB, err error) {
	t.Helper()

	const callbackName = "storage_facade_test_mount_point_create_error"

	if err := db.Callback().Create().Before("gorm:create").Register(callbackName, func(tx *gorm.DB) {
		if tx.Statement.Table == "mount_points" {
			_ = tx.AddError(err)
		}
	}); err != nil {
		t.Fatalf("register create callback: %v", err)
	}

	t.Cleanup(func() {
		_ = db.Callback().Create().Remove(callbackName)
	})
}

func TestCreateStorageRollsBackAncestorsAndTopFileWhenMountPointCreateFails(t *testing.T) {
	tDB := setupStorageFacadeTestDB(t)
	svc := NewService(tDB)
	createErr := errors.New("mount point create failed")
	registerStorageFacadeMountPointCreateError(t, tDB.db, createErr)

	_, err := svc.CreateStorage(context.NewContext(stdctx.Background()), &CreateStorageRequest{
		LocalPath:     "/rollback/a/mount",
		OsType:        models.OsTypeFolder,
		CloudToken:    0,
		FileId:        "cloud-id",
		Addition:      datatypes.JSONMap{},
		CreatorUserID: 100,
	})
	if !errors.Is(err, createErr) {
		t.Fatalf("expected injected mount point error, got %v", err)
	}

	if count := countStorageFacadeVirtualFilesByNames(t, tDB.db, "rollback", "a", "mount"); count != 0 {
		t.Fatalf("expected created ancestors and top file to rollback, got %d virtual files", count)
	}

	if count := countStorageFacadeMountPointsByPath(t, tDB.db, "/rollback/a/mount"); count != 0 {
		t.Fatalf("expected no mount point after rollback, got %d", count)
	}
}

func TestCreateStorageKeepsExistingAncestorWhenMountPointCreateFails(t *testing.T) {
	tDB := setupStorageFacadeTestDB(t)
	existing := createStorageFacadeVirtualDir(t, tDB.db, 0, "existing")
	svc := NewService(tDB)
	createErr := errors.New("mount point create failed")
	registerStorageFacadeMountPointCreateError(t, tDB.db, createErr)

	_, err := svc.CreateStorage(context.NewContext(stdctx.Background()), &CreateStorageRequest{
		LocalPath:     "/existing/mount",
		OsType:        models.OsTypeFolder,
		CloudToken:    0,
		FileId:        "cloud-id",
		Addition:      datatypes.JSONMap{},
		CreatorUserID: 100,
	})
	if !errors.Is(err, createErr) {
		t.Fatalf("expected injected mount point error, got %v", err)
	}

	var persisted models.VirtualFile
	if err := tDB.db.First(&persisted, existing.ID).Error; err != nil {
		t.Fatalf("expected existing ancestor to remain: %v", err)
	}

	if count := countStorageFacadeVirtualFilesByNames(t, tDB.db, "mount"); count != 0 {
		t.Fatalf("expected new top file to rollback, got %d", count)
	}

	if count := countStorageFacadeMountPointsByPath(t, tDB.db, "/existing/mount"); count != 0 {
		t.Fatalf("expected no mount point after rollback, got %d", count)
	}
}

func TestCreateStorageUsesExistingTransactionContext(t *testing.T) {
	tDB := setupStorageFacadeTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())
	rollbackErr := errors.New("force outer rollback")

	err := tDB.db.Transaction(func(tx *gorm.DB) error {
		txCtx := bootstrap.WithTransactionDB(ctx, tx)

		id, err := svc.CreateStorage(txCtx, &CreateStorageRequest{
			LocalPath:     "/outer-tx/mount",
			OsType:        models.OsTypeFolder,
			CloudToken:    0,
			FileId:        "cloud-id",
			Addition:      datatypes.JSONMap{},
			CreatorUserID: 100,
		})
		if err != nil {
			return err
		}

		if id <= 0 {
			t.Fatalf("expected virtual file id, got %d", id)
		}

		return rollbackErr
	})
	if !errors.Is(err, rollbackErr) {
		t.Fatalf("expected outer rollback error, got %v", err)
	}

	if count := countStorageFacadeVirtualFilesByNames(t, tDB.db, "outer-tx", "mount"); count != 0 {
		t.Fatalf("expected outer transaction rollback to remove virtual files, got %d", count)
	}

	if count := countStorageFacadeMountPointsByPath(t, tDB.db, "/outer-tx/mount"); count != 0 {
		t.Fatalf("expected outer transaction rollback to remove mount point, got %d", count)
	}
}

func TestCreateStorageAllowsTokenlessShareAndSubscribeMounts(t *testing.T) {
	tDB := setupStorageFacadeTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())

	tests := []struct {
		name      string
		localPath string
		osType    string
		fileID    string
		addition  datatypes.JSONMap
	}{
		{
			name:      "share",
			localPath: "/tokenless/share",
			osType:    models.OsTypeShareFolder,
			fileID:    "share-file-id",
			addition: datatypes.JSONMap{
				consts.FileAdditionKeyShareId:    int64(123),
				consts.FileAdditionKeyIsFolder:   true,
				consts.FileAdditionKeyShareMode:  1,
				consts.FileAdditionKeyAccessCode: "abcd",
			},
		},
		{
			name:      "subscribe",
			localPath: "/tokenless/subscribe",
			osType:    models.OsTypeSubscribe,
			fileID:    "",
			addition: datatypes.JSONMap{
				consts.FileAdditionKeyUpUserId: "up-user",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id, err := svc.CreateStorage(ctx, &CreateStorageRequest{
				LocalPath:     tt.localPath,
				OsType:        tt.osType,
				CloudToken:    0,
				FileId:        tt.fileID,
				Addition:      tt.addition,
				CreatorUserID: 100,
			})
			if err != nil {
				t.Fatalf("create storage: %v", err)
			}

			if id <= 0 {
				t.Fatalf("expected created virtual file id, got %d", id)
			}

			var mountPoint models.MountPoint
			if err = tDB.db.Where("full_path = ?", tt.localPath).First(&mountPoint).Error; err != nil {
				t.Fatalf("query mount point: %v", err)
			}

			if mountPoint.TokenId != 0 {
				t.Fatalf("expected token id 0, got %d", mountPoint.TokenId)
			}

			if mountPoint.OsType != tt.osType {
				t.Fatalf("expected os type %q, got %q", tt.osType, mountPoint.OsType)
			}
		})
	}
}

func TestCreateStorageRejectsNegativeCloudToken(t *testing.T) {
	tDB := setupStorageFacadeTestDB(t)
	svc := NewService(tDB)

	_, err := svc.CreateStorage(context.NewContext(stdctx.Background()), &CreateStorageRequest{
		LocalPath:     "/invalid-token",
		OsType:        models.OsTypeShareFolder,
		CloudToken:    -1,
		FileId:        "share-file-id",
		Addition:      datatypes.JSONMap{},
		CreatorUserID: 100,
	})
	if !errors.Is(err, errInvalidCloudToken) {
		t.Fatalf("expected invalid cloud token error, got %v", err)
	}

	if count := countStorageFacadeMountPointsByPath(t, tDB.db, "/invalid-token"); count != 0 {
		t.Fatalf("expected no mount point created, got %d", count)
	}
}

func TestCreateStorageNormalizesLocalPathBeforePersisting(t *testing.T) {
	tDB := setupStorageFacadeTestDB(t)
	svc := NewService(tDB)

	id, err := svc.CreateStorage(context.NewContext(stdctx.Background()), &CreateStorageRequest{
		LocalPath:     "/media//%E4%B8%AD%E6%96%87%20/a%3ab/",
		OsType:        models.OsTypeShareFolder,
		CloudToken:    0,
		FileId:        "share-file-id",
		Addition:      datatypes.JSONMap{},
		CreatorUserID: 100,
	})
	if err != nil {
		t.Fatalf("create storage: %v", err)
	}

	if id <= 0 {
		t.Fatalf("expected created virtual file id, got %d", id)
	}

	var mountPoint models.MountPoint
	if err = tDB.db.Where("full_path = ?", "/media/中文/a_b").First(&mountPoint).Error; err != nil {
		t.Fatalf("query normalized mount point: %v", err)
	}

	if mountPoint.Name != "a_b" {
		t.Fatalf("expected mount point name a_b, got %q", mountPoint.Name)
	}

	if count := countStorageFacadeMountPointsByPath(t, tDB.db, "/media//%E4%B8%AD%E6%96%87%20/a%3ab/"); count != 0 {
		t.Fatalf("expected no raw-path mount point, got %d", count)
	}

	if count := countStorageFacadeVirtualFilesByNames(t, tDB.db, "media", "中文", "a_b"); count != 3 {
		t.Fatalf("expected normalized ancestors and top file, got %d virtual files", count)
	}
}

func TestCreateStorageAllowsEscapedPercentInNormalizedPath(t *testing.T) {
	tDB := setupStorageFacadeTestDB(t)
	svc := NewService(tDB)

	id, err := svc.CreateStorage(context.NewContext(stdctx.Background()), &CreateStorageRequest{
		LocalPath:     "/percent/a%25b",
		OsType:        models.OsTypeShareFolder,
		CloudToken:    0,
		FileId:        "share-file-id",
		Addition:      datatypes.JSONMap{},
		CreatorUserID: 100,
	})
	if err != nil {
		t.Fatalf("create storage: %v", err)
	}

	if id <= 0 {
		t.Fatalf("expected created virtual file id, got %d", id)
	}

	var mountPoint models.MountPoint
	if err = tDB.db.Where("full_path = ?", "/percent/a%25b").First(&mountPoint).Error; err != nil {
		t.Fatalf("query percent mount point: %v", err)
	}

	if mountPoint.Name != "a%b" {
		t.Fatalf("expected mount point name a%%b, got %q", mountPoint.Name)
	}

	if count := countStorageFacadeVirtualFilesByNames(t, tDB.db, "percent", "a%b"); count != 2 {
		t.Fatalf("expected percent path ancestor and top file, got %d virtual files", count)
	}
}

func TestCreateStorageAllowsDoubleEscapedSeparatorText(t *testing.T) {
	tDB := setupStorageFacadeTestDB(t)
	svc := NewService(tDB)

	id, err := svc.CreateStorage(context.NewContext(stdctx.Background()), &CreateStorageRequest{
		LocalPath:     "/literal/a%252Fb",
		OsType:        models.OsTypeShareFolder,
		CloudToken:    0,
		FileId:        "share-file-id",
		Addition:      datatypes.JSONMap{},
		CreatorUserID: 100,
	})
	if err != nil {
		t.Fatalf("create storage: %v", err)
	}

	if id <= 0 {
		t.Fatalf("expected created virtual file id, got %d", id)
	}

	var mountPoint models.MountPoint
	if err = tDB.db.Where("full_path = ?", "/literal/a%252Fb").First(&mountPoint).Error; err != nil {
		t.Fatalf("query literal encoded separator mount point: %v", err)
	}

	if mountPoint.Name != "a%2Fb" {
		t.Fatalf("expected mount point name a%%2Fb, got %q", mountPoint.Name)
	}

	if count := countStorageFacadeVirtualFilesByNames(t, tDB.db, "literal", "a%2Fb"); count != 2 {
		t.Fatalf("expected literal encoded separator ancestor and top file, got %d virtual files", count)
	}
}

func TestCreateStorageTrimsCloudResourceID(t *testing.T) {
	tDB := setupStorageFacadeTestDB(t)
	svc := NewService(tDB)

	id, err := svc.CreateStorage(context.NewContext(stdctx.Background()), &CreateStorageRequest{
		LocalPath:     "/trimmed-cloud-id",
		OsType:        models.OsTypeShareFolder,
		CloudToken:    0,
		FileId:        " cloud-id ",
		Addition:      datatypes.JSONMap{},
		CreatorUserID: 100,
	})
	if err != nil {
		t.Fatalf("create storage: %v", err)
	}

	var file models.VirtualFile
	if err = tDB.db.First(&file, id).Error; err != nil {
		t.Fatalf("query created virtual file: %v", err)
	}

	if file.CloudId != "cloud-id" {
		t.Fatalf("expected trimmed cloud id, got %q", file.CloudId)
	}
}

func TestCreateStorageRejectsNormalizedEquivalentExistingPath(t *testing.T) {
	tDB := setupStorageFacadeTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())

	_, err := svc.CreateStorage(ctx, &CreateStorageRequest{
		LocalPath:     "/normalized/a:b",
		OsType:        models.OsTypeShareFolder,
		CloudToken:    0,
		FileId:        "share-file-id",
		Addition:      datatypes.JSONMap{},
		CreatorUserID: 100,
	})
	if err != nil {
		t.Fatalf("create first storage: %v", err)
	}

	_, err = svc.CreateStorage(ctx, &CreateStorageRequest{
		LocalPath:     "/normalized/a_b",
		OsType:        models.OsTypeShareFolder,
		CloudToken:    0,
		FileId:        "share-file-id",
		Addition:      datatypes.JSONMap{},
		CreatorUserID: 100,
	})
	if !errors.Is(err, ErrPathAlreadyExists) {
		t.Fatalf("expected normalized duplicate path error, got %v", err)
	}
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
		CreatorUserID: 100,
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

func TestCreateStorageAllowExistingReturnsNotFoundWhenExistingRootFileIsNil(t *testing.T) {
	virtualFileService := &mockCreateStorageVirtualFileService{}
	svc := &service{
		mountPointService: &mockCreateStorageMountPointService{
			queryByPathResult: &models.MountPoint{
				ID:            200,
				FileId:        100,
				FullPath:      "/exists",
				TokenId:       0,
				CreatorUserID: 7,
			},
		},
		virtualFileService: virtualFileService,
	}

	_, err := svc.createStorageInTransaction(
		context.NewContext(stdctx.Background()),
		&CreateStorageRequest{
			LocalPath:     "/exists",
			OsType:        models.OsTypeFolder,
			CloudToken:    0,
			FileId:        "cloud-id",
			Addition:      datatypes.JSONMap{},
			AllowExisting: true,
			CreatorUserID: 7,
		},
		[]string{"exists"},
	)
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("expected record not found for nil existing root file, got %v", err)
	}

	if len(virtualFileService.queriedIDs) != 1 || virtualFileService.queriedIDs[0] != 100 {
		t.Fatalf("expected existing root file 100 to be queried, got %v", virtualFileService.queriedIDs)
	}
}

func TestCreateStorageAllowExistingReturnsNormalizedEquivalentMountPoint(t *testing.T) {
	tDB := setupStorageFacadeTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())

	firstID, err := svc.CreateStorage(ctx, &CreateStorageRequest{
		LocalPath:     "/normalized-existing/a:b",
		OsType:        models.OsTypeFolder,
		CloudToken:    0,
		FileId:        "cloud-id",
		Addition:      datatypes.JSONMap{},
		CreatorUserID: 100,
	})
	if err != nil {
		t.Fatalf("create first storage: %v", err)
	}

	secondID, err := svc.CreateStorage(ctx, &CreateStorageRequest{
		LocalPath:     "/normalized-existing/a_b",
		OsType:        models.OsTypeFolder,
		CloudToken:    0,
		FileId:        "cloud-id",
		Addition:      datatypes.JSONMap{},
		CreatorUserID: 100,
		AllowExisting: true,
	})
	if err != nil {
		t.Fatalf("reuse normalized-equivalent mount point: %v", err)
	}

	if secondID != firstID {
		t.Fatalf("expected existing file id %d, got %d", firstID, secondID)
	}

	if count := countStorageFacadeMountPointsByPath(t, tDB.db, "/normalized-existing/a_b"); count != 1 {
		t.Fatalf("expected one normalized mount point, got %d", count)
	}

	if count := countStorageFacadeMountPointsByPath(t, tDB.db, "/normalized-existing/a:b"); count != 0 {
		t.Fatalf("expected no raw-equivalent mount point, got %d", count)
	}
}

func TestCreateStorageAllowExistingReturnsExistingSubscribeMountWithSameAddition(t *testing.T) {
	tDB := setupStorageFacadeTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())
	addition := datatypes.JSONMap{
		consts.FileAdditionKeyUpUserId: "up-user",
	}

	firstID, err := svc.CreateStorage(ctx, &CreateStorageRequest{
		LocalPath:     "/subscribe-existing/same",
		OsType:        models.OsTypeSubscribe,
		CloudToken:    0,
		FileId:        "",
		Addition:      addition,
		CreatorUserID: 100,
	})
	if err != nil {
		t.Fatalf("create first subscribe storage: %v", err)
	}

	secondID, err := svc.CreateStorage(ctx, &CreateStorageRequest{
		LocalPath:     "/subscribe-existing/same",
		OsType:        models.OsTypeSubscribe,
		CloudToken:    0,
		FileId:        "",
		Addition:      datatypes.JSONMap{consts.FileAdditionKeyUpUserId: "up-user"},
		CreatorUserID: 100,
		AllowExisting: true,
	})
	if err != nil {
		t.Fatalf("reuse same subscribe mount: %v", err)
	}

	if secondID != firstID {
		t.Fatalf("expected existing file id %d, got %d", firstID, secondID)
	}
}

func TestCreateStorageAllowExistingRejectsDifferentSubscribeAddition(t *testing.T) {
	tDB := setupStorageFacadeTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())

	_, err := svc.CreateStorage(ctx, &CreateStorageRequest{
		LocalPath:     "/subscribe-existing/conflict",
		OsType:        models.OsTypeSubscribe,
		CloudToken:    0,
		FileId:        "",
		Addition:      datatypes.JSONMap{consts.FileAdditionKeyUpUserId: "up-user-a"},
		CreatorUserID: 100,
	})
	if err != nil {
		t.Fatalf("create first subscribe storage: %v", err)
	}

	_, err = svc.CreateStorage(ctx, &CreateStorageRequest{
		LocalPath:     "/subscribe-existing/conflict",
		OsType:        models.OsTypeSubscribe,
		CloudToken:    0,
		FileId:        "",
		Addition:      datatypes.JSONMap{consts.FileAdditionKeyUpUserId: "up-user-b"},
		CreatorUserID: 100,
		AllowExisting: true,
	})
	if !errors.Is(err, ErrPathAlreadyExists) {
		t.Fatalf("expected ErrPathAlreadyExists for different subscribe addition, got %v", err)
	}
}

func TestCreateStorageAllowExistingReturnsExistingShareMountWithSameAddition(t *testing.T) {
	tDB := setupStorageFacadeTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())
	addition := datatypes.JSONMap{
		consts.FileAdditionKeyShareId:    int64(123),
		consts.FileAdditionKeyIsFolder:   true,
		consts.FileAdditionKeyShareMode:  1,
		consts.FileAdditionKeyAccessCode: "abcd",
	}

	firstID, err := svc.CreateStorage(ctx, &CreateStorageRequest{
		LocalPath:     "/share-existing/same",
		OsType:        models.OsTypeShareFolder,
		CloudToken:    0,
		FileId:        "share-file-id",
		Addition:      addition,
		CreatorUserID: 100,
	})
	if err != nil {
		t.Fatalf("create first share storage: %v", err)
	}

	secondID, err := svc.CreateStorage(ctx, &CreateStorageRequest{
		LocalPath:  "/share-existing/same",
		OsType:     models.OsTypeShareFolder,
		CloudToken: 0,
		FileId:     "share-file-id",
		Addition: datatypes.JSONMap{
			consts.FileAdditionKeyShareId:    int64(123),
			consts.FileAdditionKeyIsFolder:   true,
			consts.FileAdditionKeyShareMode:  1,
			consts.FileAdditionKeyAccessCode: "abcd",
		},
		CreatorUserID: 100,
		AllowExisting: true,
	})
	if err != nil {
		t.Fatalf("reuse same share mount: %v", err)
	}

	if secondID != firstID {
		t.Fatalf("expected existing file id %d, got %d", firstID, secondID)
	}
}

func TestCreateStorageAllowExistingRejectsSameCloudIDDifferentAddition(t *testing.T) {
	tDB := setupStorageFacadeTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())

	_, err := svc.CreateStorage(ctx, &CreateStorageRequest{
		LocalPath:  "/share-existing/conflict",
		OsType:     models.OsTypeShareFolder,
		CloudToken: 0,
		FileId:     "share-file-id",
		Addition: datatypes.JSONMap{
			consts.FileAdditionKeyShareId:    int64(123),
			consts.FileAdditionKeyIsFolder:   true,
			consts.FileAdditionKeyShareMode:  1,
			consts.FileAdditionKeyAccessCode: "abcd",
		},
		CreatorUserID: 100,
	})
	if err != nil {
		t.Fatalf("create first share storage: %v", err)
	}

	_, err = svc.CreateStorage(ctx, &CreateStorageRequest{
		LocalPath:  "/share-existing/conflict",
		OsType:     models.OsTypeShareFolder,
		CloudToken: 0,
		FileId:     "share-file-id",
		Addition: datatypes.JSONMap{
			consts.FileAdditionKeyShareId:    int64(456),
			consts.FileAdditionKeyIsFolder:   true,
			consts.FileAdditionKeyShareMode:  1,
			consts.FileAdditionKeyAccessCode: "wxyz",
		},
		CreatorUserID: 100,
		AllowExisting: true,
	})
	if !errors.Is(err, ErrPathAlreadyExists) {
		t.Fatalf("expected ErrPathAlreadyExists for different share addition, got %v", err)
	}
}

func TestCreateStorageAllowExistingRejectsDifferentCloudID(t *testing.T) {
	tDB := setupStorageFacadeTestDB(t)

	existingFile := createStorageFacadeVirtualDir(t, tDB.db, 0, "mounted-different")
	if err := tDB.db.Model(existingFile).Update("cloud_id", "other-cloud-id").Error; err != nil {
		t.Fatalf("update existing file cloud id: %v", err)
	}

	mountPoint := &models.MountPoint{
		FileId:        existingFile.ID,
		Name:          "mounted-different",
		FullPath:      "/mounted-different",
		OsType:        models.OsTypeFolder,
		TokenId:       0,
		CreatorUserID: 100,
	}
	if err := tDB.db.Create(mountPoint).Error; err != nil {
		t.Fatalf("create mount point: %v", err)
	}

	svc := NewService(tDB)

	_, err := svc.CreateStorage(context.NewContext(stdctx.Background()), &CreateStorageRequest{
		LocalPath:     "/mounted-different",
		OsType:        models.OsTypeFolder,
		CloudToken:    0,
		FileId:        "cloud-id",
		Addition:      datatypes.JSONMap{},
		CreatorUserID: 100,
		AllowExisting: true,
	})
	if !errors.Is(err, ErrPathAlreadyExists) {
		t.Fatalf("expected ErrPathAlreadyExists for different cloud id, got %v", err)
	}
}

func TestCreateStorageAllowExistingRejectsDifferentOsType(t *testing.T) {
	tDB := setupStorageFacadeTestDB(t)

	existingFile := createStorageFacadeVirtualDir(t, tDB.db, 0, "mounted-different-type")

	mountPoint := &models.MountPoint{
		FileId:        existingFile.ID,
		Name:          "mounted-different-type",
		FullPath:      "/mounted-different-type",
		OsType:        models.OsTypeFolder,
		TokenId:       0,
		CreatorUserID: 100,
	}
	if err := tDB.db.Create(mountPoint).Error; err != nil {
		t.Fatalf("create mount point: %v", err)
	}

	svc := NewService(tDB)

	_, err := svc.CreateStorage(context.NewContext(stdctx.Background()), &CreateStorageRequest{
		LocalPath:     "/mounted-different-type",
		OsType:        models.OsTypeShareFolder,
		CloudToken:    0,
		FileId:        "cloud-id",
		Addition:      datatypes.JSONMap{},
		CreatorUserID: 100,
		AllowExisting: true,
	})
	if !errors.Is(err, ErrPathAlreadyExists) {
		t.Fatalf("expected ErrPathAlreadyExists for different os type, got %v", err)
	}
}

func TestCreateStorageAllowExistingReturnsExistingTokenMountWithSameToken(t *testing.T) {
	tDB := setupStorageFacadeTestDB(t)
	token := createStorageFacadeCloudToken(t, tDB.db, 100, "same-token")
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())

	firstID, err := svc.CreateStorage(ctx, &CreateStorageRequest{
		LocalPath:     "/token-existing/same",
		OsType:        models.OsTypePersonFolder,
		CloudToken:    token.ID,
		FileId:        "cloud-id",
		Addition:      datatypes.JSONMap{},
		CreatorUserID: 100,
	})
	if err != nil {
		t.Fatalf("create first token storage: %v", err)
	}

	secondID, err := svc.CreateStorage(ctx, &CreateStorageRequest{
		LocalPath:     "/token-existing/same",
		OsType:        models.OsTypePersonFolder,
		CloudToken:    token.ID,
		FileId:        "cloud-id",
		Addition:      datatypes.JSONMap{},
		CreatorUserID: 100,
		AllowExisting: true,
	})
	if err != nil {
		t.Fatalf("reuse same token mount: %v", err)
	}

	if secondID != firstID {
		t.Fatalf("expected existing file id %d, got %d", firstID, secondID)
	}
}

func TestCreateStorageAllowExistingRejectsDifferentCloudToken(t *testing.T) {
	tDB := setupStorageFacadeTestDB(t)
	firstToken := createStorageFacadeCloudToken(t, tDB.db, 100, "first-token")
	secondToken := createStorageFacadeCloudToken(t, tDB.db, 100, "second-token")
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())

	_, err := svc.CreateStorage(ctx, &CreateStorageRequest{
		LocalPath:     "/token-existing/conflict",
		OsType:        models.OsTypePersonFolder,
		CloudToken:    firstToken.ID,
		FileId:        "cloud-id",
		Addition:      datatypes.JSONMap{},
		CreatorUserID: 100,
	})
	if err != nil {
		t.Fatalf("create first token storage: %v", err)
	}

	_, err = svc.CreateStorage(ctx, &CreateStorageRequest{
		LocalPath:     "/token-existing/conflict",
		OsType:        models.OsTypePersonFolder,
		CloudToken:    secondToken.ID,
		FileId:        "cloud-id",
		Addition:      datatypes.JSONMap{},
		CreatorUserID: 100,
		AllowExisting: true,
	})
	if !errors.Is(err, ErrPathAlreadyExists) {
		t.Fatalf("expected ErrPathAlreadyExists for different cloud token, got %v", err)
	}
}

func TestCreateStorageAllowExistingRejectsExistingMountPointOwnedByOtherUser(t *testing.T) {
	tDB := setupStorageFacadeTestDB(t)
	existingFile := createStorageFacadeVirtualDir(t, tDB.db, 0, "mounted-by-other")

	mountPoint := &models.MountPoint{
		FileId:        existingFile.ID,
		Name:          "mounted-by-other",
		FullPath:      "/mounted-by-other",
		OsType:        models.OsTypeFolder,
		TokenId:       0,
		CreatorUserID: 100,
	}
	if err := tDB.db.Create(mountPoint).Error; err != nil {
		t.Fatalf("create mount point: %v", err)
	}

	svc := NewService(tDB)

	_, err := svc.CreateStorage(context.NewContext(stdctx.Background()), &CreateStorageRequest{
		LocalPath:     "/mounted-by-other",
		OsType:        models.OsTypeFolder,
		CloudToken:    0,
		FileId:        "cloud-id",
		Addition:      datatypes.JSONMap{},
		CreatorUserID: 200,
		AllowExisting: true,
	})
	if !errors.Is(err, ErrExistingPathForbidden) {
		t.Fatalf("expected ErrExistingPathForbidden, got %v", err)
	}
}

func TestCreateStorageRejectsOwnerlessNonAdminRequest(t *testing.T) {
	tDB := setupStorageFacadeTestDB(t)
	svc := NewService(tDB)

	_, err := svc.CreateStorage(context.NewContext(stdctx.Background()), &CreateStorageRequest{
		LocalPath:     "/ownerless",
		OsType:        models.OsTypeFolder,
		CloudToken:    0,
		FileId:        "cloud-id",
		Addition:      datatypes.JSONMap{},
		AllowExisting: true,
	})
	if !errors.Is(err, errInvalidCreatorUser) {
		t.Fatalf("expected invalid creator user error, got %v", err)
	}

	var mountPointCount int64
	if err = tDB.db.Model(&models.MountPoint{}).Where("full_path = ?", "/ownerless").Count(&mountPointCount).Error; err != nil {
		t.Fatalf("count mount points: %v", err)
	}

	if mountPointCount != 0 {
		t.Fatalf("expected no mount point created, got %d", mountPointCount)
	}
}

func TestCreateStorageAllowExistingAllowsAdminToReuseExistingMountPoint(t *testing.T) {
	tDB := setupStorageFacadeTestDB(t)
	existingFile := createStorageFacadeVirtualDir(t, tDB.db, 0, "admin-reuse")

	mountPoint := &models.MountPoint{
		FileId:        existingFile.ID,
		Name:          "admin-reuse",
		FullPath:      "/admin-reuse",
		OsType:        models.OsTypeFolder,
		TokenId:       0,
		CreatorUserID: 100,
	}
	if err := tDB.db.Create(mountPoint).Error; err != nil {
		t.Fatalf("create mount point: %v", err)
	}

	svc := NewService(tDB)

	id, err := svc.CreateStorage(context.NewContext(stdctx.Background()), &CreateStorageRequest{
		LocalPath:     "/admin-reuse",
		OsType:        models.OsTypeFolder,
		CloudToken:    0,
		FileId:        "cloud-id",
		Addition:      datatypes.JSONMap{},
		CreatorUserID: 200,
		IsAdmin:       true,
		AllowExisting: true,
	})
	if err != nil {
		t.Fatalf("create storage as admin: %v", err)
	}

	if id != existingFile.ID {
		t.Fatalf("expected existing file id %d, got %d", existingFile.ID, id)
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

func TestCreateStorageRejectsTypedNilCloudTokenService(t *testing.T) {
	tDB := setupStorageFacadeTestDB(t)

	var cloudTokenService *mockCreateStorageCloudTokenService

	svc := &service{
		svc:               tDB,
		cloudTokenService: cloudTokenService,
	}

	_, err := svc.CreateStorage(context.NewContext(stdctx.Background()), &CreateStorageRequest{
		LocalPath:     "/typed-nil-token",
		OsType:        models.OsTypePersonFolder,
		CloudToken:    99,
		FileId:        "cloud-id",
		Addition:      datatypes.JSONMap{},
		CreatorUserID: 100,
	})
	if !errors.Is(err, errInvalidCloudToken) {
		t.Fatalf("expected invalid cloud token error, got %v", err)
	}

	if got := countStorageFacadeVirtualFilesByNames(t, tDB.db, "typed-nil-token"); got != 0 {
		t.Fatalf("expected no virtual files created, got %d", got)
	}
}
