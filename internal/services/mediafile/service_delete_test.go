package mediafile

import (
	stdctx "context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/xxcheng123/cloudpan189-share/internal/bootstrap"
	"github.com/xxcheng123/cloudpan189-share/internal/framework/context"
	"github.com/xxcheng123/cloudpan189-share/internal/pkgs/taskengine"
	"github.com/xxcheng123/cloudpan189-share/internal/repository/models"
	"github.com/xxcheng123/cloudpan189-share/internal/shared"
	"github.com/xxcheng123/cloudpan189-share/internal/types/media"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type mediaFileTestDB struct {
	db *gorm.DB
}

func (t *mediaFileTestDB) GetDB(ctx context.Context) *gorm.DB {
	return t.db.WithContext(ctx)
}

func (t *mediaFileTestDB) GetDBWithoutContext() *gorm.DB {
	return t.db
}

func (t *mediaFileTestDB) GetLogger(name string, fields ...zap.Field) *zap.Logger {
	return zap.NewNop()
}

func (t *mediaFileTestDB) Close() {}

func (t *mediaFileTestDB) GetPort() int {
	return 9999
}

func (t *mediaFileTestDB) GetTaskEngine() taskengine.TaskEngine {
	return nil
}

func (t *mediaFileTestDB) GetHTTPEngine() *gin.Engine {
	return nil
}

var _ bootstrap.ServiceContext = (*mediaFileTestDB)(nil)

func setupMediaFileTestDB(t *testing.T) *mediaFileTestDB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}

	if err := db.AutoMigrate(&models.MediaFile{}); err != nil {
		t.Fatalf("migrate test db: %v", err)
	}

	return &mediaFileTestDB{db: db}
}

func withMediaConfig(t *testing.T, cfg *models.MediaConfig) {
	t.Helper()

	oldConfig := shared.MediaConfig
	shared.MediaConfig = cfg

	t.Cleanup(func() {
		shared.MediaConfig = oldConfig
	})
}

func createMediaFile(t *testing.T, db *gorm.DB, fid int64, filePath string) *models.MediaFile {
	t.Helper()

	file := &models.MediaFile{
		FID:       fid,
		Name:      filepath.Base(filePath),
		Path:      filePath,
		Size:      12,
		MediaType: media.TypeStrm,
		Hash:      "-",
	}
	if err := db.Create(file).Error; err != nil {
		t.Fatalf("create media file: %v", err)
	}

	return file
}

type unsafeWriterCar struct {
	rootPath       string
	fullPath       string
	filePath       string
	name           string
	conflictPolicy media.FileConflictPolicy
}

func (c *unsafeWriterCar) RootPath() string {
	return c.rootPath
}

func (c *unsafeWriterCar) NewSubCar(filePath string) media.WriterCar {
	return &unsafeWriterCar{
		rootPath:       c.rootPath,
		fullPath:       c.fullPath,
		filePath:       filePath,
		name:           filepath.Base(filePath),
		conflictPolicy: c.conflictPolicy,
	}
}

func (c *unsafeWriterCar) GetFileConflictPolicy() media.FileConflictPolicy {
	return c.conflictPolicy
}

func (c *unsafeWriterCar) GetBaseURL() string {
	return "http://example.test"
}

func (c *unsafeWriterCar) GetFullPath() string {
	return c.fullPath
}

func (c *unsafeWriterCar) GetPath() string {
	return c.filePath
}

func (c *unsafeWriterCar) GetName() string {
	return c.name
}

func TestDeleteStrmRemovesFileAndDBRecord(t *testing.T) {
	tDB := setupMediaFileTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())
	root := t.TempDir()
	relPath := "/movies/example.strm"
	fullPath := filepath.Join(root, relPath)

	if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
		t.Fatalf("create strm dir: %v", err)
	}

	if err := os.WriteFile(fullPath, []byte("http://example.test"), 0o644); err != nil {
		t.Fatalf("write strm file: %v", err)
	}

	file := createMediaFile(t, tDB.db, 1001, relPath)

	if err := svc.DeleteStrm(ctx, file.FID, root); err != nil {
		t.Fatalf("delete strm: %v", err)
	}

	if _, err := os.Stat(fullPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected strm file removed, got %v", err)
	}

	var count int64
	if err := tDB.db.Model(&models.MediaFile{}).Where("id = ?", file.ID).Count(&count).Error; err != nil {
		t.Fatalf("count media file: %v", err)
	}

	if count != 0 {
		t.Fatalf("expected media file DB record removed, got count %d", count)
	}
}

func TestQueryStrmRejectsInvalidFID(t *testing.T) {
	tDB := setupMediaFileTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())

	_, err := svc.QueryStrm(ctx, 0)
	if !errors.Is(err, errInvalidMediaFileFID) {
		t.Fatalf("expected invalid media file fid, got %v", err)
	}
}

func TestDeleteStrmRejectsInvalidFID(t *testing.T) {
	tDB := setupMediaFileTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())

	err := svc.DeleteStrm(ctx, 0, t.TempDir())
	if !errors.Is(err, errInvalidMediaFileFID) {
		t.Fatalf("expected invalid media file fid, got %v", err)
	}
}

func TestWriteStrmRejectsInvalidFIDWithoutSideEffects(t *testing.T) {
	tDB := setupMediaFileTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())
	root := t.TempDir()
	car := media.NewWriterCar(root, media.FileConflictPolicyReplace, "http://example.test").NewSubCar("invalid.strm")

	id, err := svc.WriteStrm(ctx, car, 0, "http://example.test/invalid")
	if !errors.Is(err, errInvalidMediaFileFID) {
		t.Fatalf("expected invalid media file fid, got %v", err)
	}

	if id != 0 {
		t.Fatalf("expected zero id, got %d", id)
	}

	if _, statErr := os.Stat(car.GetFullPath()); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("expected strm file not created, got %v", statErr)
	}

	var count int64
	if err := tDB.db.Model(&models.MediaFile{}).Count(&count).Error; err != nil {
		t.Fatalf("count media files: %v", err)
	}

	if count != 0 {
		t.Fatalf("expected no DB records, got count %d", count)
	}
}

func TestWriteStrmRecreatesMissingFileWhenRecordExists(t *testing.T) {
	tDB := setupMediaFileTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())
	root := t.TempDir()
	car := media.NewWriterCar(root, media.FileConflictPolicySkip, "http://example.test").NewSubCar("movies/rebuild.strm")

	createMediaFile(t, tDB.db, 2001, "/movies/rebuild.strm")

	id, err := svc.WriteStrm(ctx, car, 2001, "http://example.test/rebuild")
	if err != nil {
		t.Fatalf("write strm: %v", err)
	}

	if id == 0 {
		t.Fatal("expected new media file id")
	}

	if _, statErr := os.Stat(car.GetFullPath()); statErr != nil {
		t.Fatalf("expected strm file to be recreated, got %v", statErr)
	}

	var count int64
	if err := tDB.db.Model(&models.MediaFile{}).Where("fid = ? AND path = ?", 2001, "/movies/rebuild.strm").Count(&count).Error; err != nil {
		t.Fatalf("count media files: %v", err)
	}

	if count != 1 {
		t.Fatalf("expected one recreated media file record, got count %d", count)
	}
}

func TestWriteStrmIgnoresUnsafeFullPathFromWriterCar(t *testing.T) {
	tDB := setupMediaFileTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())
	root := t.TempDir()
	outsidePath := filepath.Join(t.TempDir(), "escape.strm")
	car := &unsafeWriterCar{
		rootPath:       root,
		fullPath:       outsidePath,
		filePath:       "safe/inside.strm",
		name:           "inside.strm",
		conflictPolicy: media.FileConflictPolicyReplace,
	}

	id, err := svc.WriteStrm(ctx, car, 2002, "http://example.test/safe")
	if err != nil {
		t.Fatalf("write strm: %v", err)
	}

	if id == 0 {
		t.Fatal("expected new media file id")
	}

	if _, err := os.Stat(outsidePath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected outside path untouched, got %v", err)
	}

	if _, err := os.Stat(filepath.Join(root, "safe", "inside.strm")); err != nil {
		t.Fatalf("expected safe strm file created, got %v", err)
	}
}

func TestDeleteStrmByFullPathKeepsDBRecordOutsideMediaRoot(t *testing.T) {
	tDB := setupMediaFileTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())

	root := t.TempDir()
	withMediaConfig(t, &models.MediaConfig{StoragePath: root})
	file := createMediaFile(t, tDB.db, 1002, "/outside/example.strm")

	outsidePath := filepath.Join(t.TempDir(), "example.strm")
	if err := os.WriteFile(outsidePath, []byte("http://example.test"), 0o644); err != nil {
		t.Fatalf("write outside strm: %v", err)
	}

	if err := svc.DeleteStrmByFullPath(ctx, outsidePath); err != nil {
		t.Fatalf("delete strm by full path: %v", err)
	}

	if _, err := os.Stat(outsidePath); err != nil {
		t.Fatalf("expected outside strm file to remain, got %v", err)
	}

	var count int64
	if err := tDB.db.Model(&models.MediaFile{}).Where("id = ?", file.ID).Count(&count).Error; err != nil {
		t.Fatalf("count media file: %v", err)
	}

	if count != 1 {
		t.Fatalf("expected outside DB record to remain, got count %d", count)
	}
}

func TestDeleteStrmByFullPathHandlesDotPrefixedDirectoryInsideMediaRoot(t *testing.T) {
	tDB := setupMediaFileTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())

	root := t.TempDir()
	withMediaConfig(t, &models.MediaConfig{StoragePath: root})

	relPath := "/..hidden/example.strm"
	fullPath := filepath.Join(root, relPath)

	if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
		t.Fatalf("create hidden dir: %v", err)
	}

	if err := os.WriteFile(fullPath, []byte("http://example.test"), 0o644); err != nil {
		t.Fatalf("write hidden strm: %v", err)
	}

	file := createMediaFile(t, tDB.db, 1003, relPath)

	if err := svc.DeleteStrmByFullPath(ctx, fullPath); err != nil {
		t.Fatalf("delete strm by hidden full path: %v", err)
	}

	var count int64
	if err := tDB.db.Model(&models.MediaFile{}).Where("id = ?", file.ID).Count(&count).Error; err != nil {
		t.Fatalf("count media file: %v", err)
	}

	if count != 0 {
		t.Fatalf("expected hidden media DB record removed, got count %d", count)
	}
}

func TestDeleteStrmByFullPathDeletesLegacyRelativePathRecord(t *testing.T) {
	tDB := setupMediaFileTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())

	root := t.TempDir()
	withMediaConfig(t, &models.MediaConfig{StoragePath: root})

	legacyPath := "legacy/example.strm"
	fullPath := filepath.Join(root, legacyPath)

	if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
		t.Fatalf("create legacy dir: %v", err)
	}

	if err := os.WriteFile(fullPath, []byte("http://example.test"), 0o644); err != nil {
		t.Fatalf("write legacy strm: %v", err)
	}

	file := createMediaFile(t, tDB.db, 1004, legacyPath)

	if err := svc.DeleteStrmByFullPath(ctx, fullPath); err != nil {
		t.Fatalf("delete strm by full path: %v", err)
	}

	var count int64
	if err := tDB.db.Model(&models.MediaFile{}).Where("id = ?", file.ID).Count(&count).Error; err != nil {
		t.Fatalf("count media file: %v", err)
	}

	if count != 0 {
		t.Fatalf("expected legacy DB record removed, got count %d", count)
	}
}

func TestDeleteStrmByFullPathReturnsDBCleanupError(t *testing.T) {
	tDB := setupMediaFileTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())

	root := t.TempDir()
	withMediaConfig(t, &models.MediaConfig{StoragePath: root})

	relPath := "/movies/db-error.strm"
	fullPath := filepath.Join(root, relPath)

	if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
		t.Fatalf("create strm dir: %v", err)
	}

	if err := os.WriteFile(fullPath, []byte("http://example.test"), 0o644); err != nil {
		t.Fatalf("write strm: %v", err)
	}

	createMediaFile(t, tDB.db, 1006, relPath)

	sqlDB, err := tDB.db.DB()
	if err != nil {
		t.Fatalf("get sql db: %v", err)
	}

	if err := sqlDB.Close(); err != nil {
		t.Fatalf("close sql db: %v", err)
	}

	err = svc.DeleteStrmByFullPath(ctx, fullPath)
	if err == nil {
		t.Fatal("expected DB cleanup error")
	}

	if _, err := os.Stat(fullPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected strm file removed before DB cleanup error, got %v", err)
	}
}

func TestDeleteStrmSkipsUnsafeDBPathOutsideRoot(t *testing.T) {
	tDB := setupMediaFileTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())
	root := t.TempDir()
	outsideDir := t.TempDir()
	outsidePath := filepath.Join(outsideDir, "escape.strm")

	if err := os.WriteFile(outsidePath, []byte("http://example.test"), 0o644); err != nil {
		t.Fatalf("write outside strm: %v", err)
	}

	file := createMediaFile(t, tDB.db, 1005, "../"+filepath.Base(outsideDir)+"/escape.strm")

	if err := svc.DeleteStrm(ctx, file.FID, root); err != nil {
		t.Fatalf("delete unsafe strm: %v", err)
	}

	if _, err := os.Stat(outsidePath); err != nil {
		t.Fatalf("expected outside strm file to remain, got %v", err)
	}

	var count int64
	if err := tDB.db.Model(&models.MediaFile{}).Where("id = ?", file.ID).Count(&count).Error; err != nil {
		t.Fatalf("count media file: %v", err)
	}

	if count != 0 {
		t.Fatalf("expected unsafe DB record removed, got count %d", count)
	}
}

func TestClearRejectsPathOutsideConfiguredMediaRoot(t *testing.T) {
	tDB := setupMediaFileTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())
	root := t.TempDir()
	outsideRoot := t.TempDir()
	outsideFile := filepath.Join(outsideRoot, "outside.strm")

	withMediaConfig(t, &models.MediaConfig{StoragePath: root})

	if err := os.WriteFile(outsideFile, []byte("http://example.test"), 0o644); err != nil {
		t.Fatalf("write outside media file: %v", err)
	}

	if err := svc.Clear(ctx, outsideRoot); err == nil {
		t.Fatal("expected clear to reject outside media root")
	}

	if _, err := os.Stat(outsideFile); err != nil {
		t.Fatalf("expected outside media file to remain, got %v", err)
	}
}

func TestClearRemovesConfiguredMediaRootChildrenOnly(t *testing.T) {
	tDB := setupMediaFileTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())
	root := t.TempDir()
	childDir := filepath.Join(root, "movies")
	childFile := filepath.Join(childDir, "movie.strm")

	withMediaConfig(t, &models.MediaConfig{StoragePath: root})

	if err := os.MkdirAll(childDir, 0o755); err != nil {
		t.Fatalf("create media dir: %v", err)
	}

	if err := os.WriteFile(childFile, []byte("http://example.test"), 0o644); err != nil {
		t.Fatalf("write media file: %v", err)
	}

	createMediaFile(t, tDB.db, 1006, "/movies/movie.strm")

	if err := svc.Clear(ctx, root); err != nil {
		t.Fatalf("clear media root: %v", err)
	}

	if _, err := os.Stat(root); err != nil {
		t.Fatalf("expected media root to remain, got %v", err)
	}

	if _, err := os.Stat(childDir); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected child directory removed, got %v", err)
	}

	var count int64
	if err := tDB.db.Model(&models.MediaFile{}).Count(&count).Error; err != nil {
		t.Fatalf("count media files: %v", err)
	}

	if count != 0 {
		t.Fatalf("expected media DB records cleared, got count %d", count)
	}
}
