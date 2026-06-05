package media

import (
	stdctx "context"
	"encoding/json"
	"errors"
	"net/url"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/xxcheng123/cloudpan189-share/internal/bootstrap"
	"github.com/xxcheng123/cloudpan189-share/internal/framework/context"
	"github.com/xxcheng123/cloudpan189-share/internal/framework/taskcontext"
	"github.com/xxcheng123/cloudpan189-share/internal/pkgs/taskengine"
	"github.com/xxcheng123/cloudpan189-share/internal/pkgs/utils"
	"github.com/xxcheng123/cloudpan189-share/internal/repository/models"
	"github.com/xxcheng123/cloudpan189-share/internal/services/filetasklog"
	mediafileSvi "github.com/xxcheng123/cloudpan189-share/internal/services/mediafile"
	mountpointSvi "github.com/xxcheng123/cloudpan189-share/internal/services/mountpoint"
	verifySvi "github.com/xxcheng123/cloudpan189-share/internal/services/verify"
	virtualfileSvi "github.com/xxcheng123/cloudpan189-share/internal/services/virtualfile"
	"github.com/xxcheng123/cloudpan189-share/internal/shared"
	mediaType "github.com/xxcheng123/cloudpan189-share/internal/types/media"
	"github.com/xxcheng123/cloudpan189-share/internal/types/topic"
	"go.uber.org/zap"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type rebuildStrmTestDB struct {
	db *gorm.DB
}

func (t *rebuildStrmTestDB) GetDB(ctx context.Context) *gorm.DB {
	return t.db.WithContext(ctx)
}

func (t *rebuildStrmTestDB) GetDBWithoutContext() *gorm.DB {
	return t.db
}

func (t *rebuildStrmTestDB) GetLogger(name string, fields ...zap.Field) *zap.Logger {
	return zap.NewNop()
}

func (t *rebuildStrmTestDB) Close() {}

func (t *rebuildStrmTestDB) GetPort() int {
	return 9999
}

func (t *rebuildStrmTestDB) GetTaskEngine() taskengine.TaskEngine {
	return nil
}

func (t *rebuildStrmTestDB) GetHTTPEngine() *gin.Engine {
	return nil
}

var _ bootstrap.ServiceContext = (*rebuildStrmTestDB)(nil)

func setupRebuildStrmTestDB(t *testing.T) *rebuildStrmTestDB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "rebuild-strm.db")), &gorm.Config{})
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get sql db: %v", err)
	}

	sqlDB.SetMaxOpenConns(1)

	if err := db.AutoMigrate(&models.FileTaskLog{}); err != nil {
		t.Fatalf("migrate test db: %v", err)
	}

	return &rebuildStrmTestDB{db: db}
}

type mockRebuildStrmVirtualFileService struct {
	virtualfileSvi.Service

	filesByParent map[int64][]*models.VirtualFile
	errByParent   map[int64]error
}

func (m *mockRebuildStrmVirtualFileService) List(ctx context.Context, req *virtualfileSvi.ListRequest) ([]*models.VirtualFile, error) {
	parentID := int64(0)
	if req != nil && req.ParentId != nil {
		parentID = *req.ParentId
	}

	if err := m.errByParent[parentID]; err != nil {
		return nil, err
	}

	return m.filesByParent[parentID], nil
}

func (m *mockRebuildStrmVirtualFileService) BatchQueryParentFiles(ctx context.Context, id int64) ([]*models.VirtualFile, error) {
	return nil, nil
}

func (m *mockRebuildStrmVirtualFileService) GetMaxId(ctx context.Context) (int64, error) {
	return 0, nil
}

func (m *mockRebuildStrmVirtualFileService) CalFullPath(ctx context.Context, id int64) (string, error) {
	return "", nil
}

func (m *mockRebuildStrmVirtualFileService) CalFilePath(ctx context.Context, id int64) (string, error) {
	return "", nil
}

func (m *mockRebuildStrmVirtualFileService) Count(ctx context.Context, req *virtualfileSvi.ListRequest) (int64, error) {
	return 0, nil
}

func (m *mockRebuildStrmVirtualFileService) Query(ctx context.Context, fid int64) (*models.VirtualFile, error) {
	return nil, nil
}

func (m *mockRebuildStrmVirtualFileService) QueryByPath(ctx context.Context, filePath string) (*models.VirtualFile, error) {
	return nil, nil
}

func (m *mockRebuildStrmVirtualFileService) QueryTop(ctx context.Context, fid int64) (*models.VirtualFile, error) {
	return nil, nil
}

func (m *mockRebuildStrmVirtualFileService) FindOrCreateAncestors(ctx context.Context, filePath string) (int64, error) {
	return 0, nil
}

func (m *mockRebuildStrmVirtualFileService) Create(ctx context.Context, parentId int64, file *models.VirtualFile) (int64, error) {
	return 0, nil
}

func (m *mockRebuildStrmVirtualFileService) CreateTop(ctx context.Context, parentId int64, file *models.VirtualFile) (int64, error) {
	return 0, nil
}

func (m *mockRebuildStrmVirtualFileService) BatchCreate(ctx context.Context, parentId int64, files []*models.VirtualFile, hooks ...virtualfileSvi.BatchCreateHook) (int64, error) {
	return 0, nil
}

func (m *mockRebuildStrmVirtualFileService) Delete(ctx context.Context, id int64, hooks ...virtualfileSvi.DeleteHook) error {
	return nil
}

func (m *mockRebuildStrmVirtualFileService) BatchDelete(ctx context.Context, ids []int64, hooks ...virtualfileSvi.BatchDeleteHook) ([]int64, error) {
	return nil, nil
}

func (m *mockRebuildStrmVirtualFileService) Update(ctx context.Context, id int64, opts []utils.Field, hooks ...virtualfileSvi.UpdateHook) error {
	return nil
}

func (m *mockRebuildStrmVirtualFileService) ModifyAddition(ctx context.Context, id int64, key string, value any) error {
	return nil
}

func (m *mockRebuildStrmVirtualFileService) BatchUpdate(ctx context.Context, filesToUpdate map[int64][]utils.Field) error {
	return nil
}

func (m *mockRebuildStrmVirtualFileService) BatchUpdatePlus(ctx context.Context, values []utils.Field, exps []clause.Expression) error {
	return nil
}

func (m *mockRebuildStrmVirtualFileService) GroupCountByTopId(ctx context.Context, req *virtualfileSvi.GroupCountByTopIdRequest) ([]*virtualfileSvi.GroupCountByTopId, error) {
	return nil, nil
}

func (m *mockRebuildStrmVirtualFileService) ClearUnusedAncestorFolder(ctx context.Context, subId int64) error {
	return nil
}

func (m *mockRebuildStrmVirtualFileService) ClearAll(ctx context.Context) error {
	return nil
}

type mockRebuildStrmVerifyService struct {
	verifySvi.Service

	errByFileID map[int64]error
}

func (m *mockRebuildStrmVerifyService) SignV1(ctx context.Context, fileID int64, opts ...verifySvi.SignV1OptionFunc) (url.Values, error) {
	if err := m.errByFileID[fileID]; err != nil {
		return nil, err
	}

	return url.Values{"sign": []string{"ok"}}, nil
}

type mockRebuildStrmMediaFileService struct {
	mediafileSvi.Service

	mu         sync.Mutex
	writeCalls []rebuildStrmWriteCall
}

type rebuildStrmWriteCall struct {
	fid     int64
	baseURL string
	path    string
	fileURL string
}

func (m *mockRebuildStrmMediaFileService) WriteStrm(ctx context.Context, car mediaType.WriterCar, fid int64, fileURL string) (int64, error) {
	m.mu.Lock()
	m.writeCalls = append(m.writeCalls, rebuildStrmWriteCall{
		fid:     fid,
		baseURL: car.GetBaseURL(),
		path:    car.GetPath(),
		fileURL: fileURL,
	})
	m.mu.Unlock()

	return 1, nil
}

func (m *mockRebuildStrmMediaFileService) WrittenCalls() []rebuildStrmWriteCall {
	m.mu.Lock()
	defer m.mu.Unlock()

	return append([]rebuildStrmWriteCall(nil), m.writeCalls...)
}

func (m *mockRebuildStrmMediaFileService) QueryStrm(ctx context.Context, fid int64) (*models.MediaFile, error) {
	return nil, nil
}

func (m *mockRebuildStrmMediaFileService) QueryByPath(ctx context.Context, filePath string) (*models.MediaFile, error) {
	return nil, nil
}

func (m *mockRebuildStrmMediaFileService) DeleteStrm(ctx context.Context, fid int64, rootPath string) error {
	return nil
}

func (m *mockRebuildStrmMediaFileService) DeleteStrmByFullPath(ctx context.Context, fullPath string) error {
	return nil
}

func (m *mockRebuildStrmMediaFileService) ClearEmptyDir(ctx context.Context, entryPath string) error {
	return nil
}

func (m *mockRebuildStrmMediaFileService) Clear(ctx context.Context, rootPath string) error {
	return nil
}

func (m *mockRebuildStrmMediaFileService) ClearAll(ctx context.Context) error {
	return nil
}

type mockRebuildStrmMountPointService struct {
	mountpointSvi.Service

	items []*models.MountPoint
}

func (m *mockRebuildStrmMountPointService) List(ctx context.Context, req *mountpointSvi.ListRequest) ([]*models.MountPoint, error) {
	return m.items, nil
}

type failingRebuildStatusFileTaskLogService struct {
	filetasklog.Service
	completedErr error
	failedErr    error
}

func (s *failingRebuildStatusFileTaskLogService) Completed(
	ctx context.Context,
	key filetasklog.LogKey,
	opts ...utils.Field,
) error {
	if s.completedErr != nil {
		return s.completedErr
	}

	return s.Service.Completed(ctx, key, opts...)
}

func (s *failingRebuildStatusFileTaskLogService) Failed(
	ctx context.Context,
	key filetasklog.LogKey,
	opts ...utils.Field,
) error {
	if s.failedErr != nil {
		return s.failedErr
	}

	return s.Service.Failed(ctx, key, opts...)
}

func setupRebuildStrmMediaConfig(t *testing.T) {
	t.Helper()

	oldMediaConfig := shared.MediaConfig
	oldBaseURL := shared.BaseURL

	t.Cleanup(func() {
		shared.MediaConfig = oldMediaConfig
		shared.BaseURL = oldBaseURL
	})

	shared.MediaConfig = &models.MediaConfig{
		Enable:         true,
		StoragePath:    t.TempDir(),
		ConflictPolicy: mediaType.FileConflictPolicySkip,
		BaseURL:        "http://media.example.test",
	}
	shared.BaseURL = "http://system.example.test"
}

func assertMediaDependencyError(t *testing.T, err error, want error) {
	t.Helper()

	if !errors.Is(err, want) {
		t.Fatalf("expected %v, got %v", want, err)
	}

	if strings.Contains(err.Error(), "task processor panic") {
		t.Fatalf("expected direct dependency error, got panic recovery error %v", err)
	}
}

func TestClearReturnsMissingMediaFileServiceWhenTypedNil(t *testing.T) {
	setupRebuildStrmMediaConfig(t)

	var mediaFileService *mockRebuildStrmMediaFileService

	handler := NewHandler(
		mediaFileService,
		nil,
		nil,
		nil,
		nil,
	)

	processor := taskcontext.NewHandlerFuncWrapper(zap.NewNop()).Wrap(handler.Clear())
	err := processor.Process(stdctx.Background(), []byte(`{}`))

	assertMediaDependencyError(t, err, errMediaFileServiceNotInitialized)
}

func TestRebuildStrmFileReturnsDependencyErrorsBeforeProcessing(t *testing.T) {
	tests := []struct {
		name string
		want error
		new  func(t *testing.T) Handler
	}{
		{
			name: "missing media file service",
			want: errMediaFileServiceNotInitialized,
			new: func(t *testing.T) Handler {
				t.Helper()

				tDB := setupRebuildStrmTestDB(t)

				var mediaFileService *mockRebuildStrmMediaFileService

				return NewHandler(
					mediaFileService,
					&mockRebuildStrmMountPointService{},
					&mockRebuildStrmVirtualFileService{filesByParent: map[int64][]*models.VirtualFile{}, errByParent: map[int64]error{}},
					&mockRebuildStrmVerifyService{errByFileID: map[int64]error{}},
					filetasklog.NewService(tDB),
				)
			},
		},
		{
			name: "missing mount point service",
			want: errMountPointServiceNotInitialized,
			new: func(t *testing.T) Handler {
				t.Helper()

				tDB := setupRebuildStrmTestDB(t)

				var mountPointService *mockRebuildStrmMountPointService

				return NewHandler(
					&mockRebuildStrmMediaFileService{},
					mountPointService,
					&mockRebuildStrmVirtualFileService{filesByParent: map[int64][]*models.VirtualFile{}, errByParent: map[int64]error{}},
					&mockRebuildStrmVerifyService{errByFileID: map[int64]error{}},
					filetasklog.NewService(tDB),
				)
			},
		},
		{
			name: "missing virtual file service",
			want: errVirtualFileServiceNotInitialized,
			new: func(t *testing.T) Handler {
				t.Helper()

				tDB := setupRebuildStrmTestDB(t)

				var virtualFileService *mockRebuildStrmVirtualFileService

				return NewHandler(
					&mockRebuildStrmMediaFileService{},
					&mockRebuildStrmMountPointService{},
					virtualFileService,
					&mockRebuildStrmVerifyService{errByFileID: map[int64]error{}},
					filetasklog.NewService(tDB),
				)
			},
		},
		{
			name: "missing verify service",
			want: errVerifyServiceNotInitialized,
			new: func(t *testing.T) Handler {
				t.Helper()

				tDB := setupRebuildStrmTestDB(t)

				var verifyService *mockRebuildStrmVerifyService

				return NewHandler(
					&mockRebuildStrmMediaFileService{},
					&mockRebuildStrmMountPointService{},
					&mockRebuildStrmVirtualFileService{filesByParent: map[int64][]*models.VirtualFile{}, errByParent: map[int64]error{}},
					verifyService,
					filetasklog.NewService(tDB),
				)
			},
		},
		{
			name: "missing file task log service",
			want: errFileTaskLogServiceNotInitialized,
			new: func(t *testing.T) Handler {
				t.Helper()

				var fileTaskLogService *failingRebuildStatusFileTaskLogService

				return NewHandler(
					&mockRebuildStrmMediaFileService{},
					&mockRebuildStrmMountPointService{},
					&mockRebuildStrmVirtualFileService{filesByParent: map[int64][]*models.VirtualFile{}, errByParent: map[int64]error{}},
					&mockRebuildStrmVerifyService{errByFileID: map[int64]error{}},
					fileTaskLogService,
				)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setupRebuildStrmMediaConfig(t)

			processor := taskcontext.NewHandlerFuncWrapper(zap.NewNop()).Wrap(tt.new(t).RebuildStrmFile())
			err := processor.Process(stdctx.Background(), []byte(`{}`))

			assertMediaDependencyError(t, err, tt.want)
		})
	}
}

func TestRebuildStrmFileByMountPointDoesNotRequireMountPointService(t *testing.T) {
	tDB := setupRebuildStrmTestDB(t)
	fileTaskLogService := filetasklog.NewService(tDB)

	setupRebuildStrmMediaConfig(t)

	var mountPointService *mockRebuildStrmMountPointService

	handler := NewHandler(
		&mockRebuildStrmMediaFileService{},
		mountPointService,
		&mockRebuildStrmVirtualFileService{
			filesByParent: map[int64][]*models.VirtualFile{
				100: {},
			},
			errByParent: map[int64]error{},
		},
		&mockRebuildStrmVerifyService{errByFileID: map[int64]error{}},
		fileTaskLogService,
	)

	req := topic.MediaRebuildStrmFileByMountPointRequest{
		MountPointFileId: 100,
		MountPointPath:   "/movies",
	}

	body, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}

	processor := taskcontext.NewHandlerFuncWrapper(zap.NewNop()).Wrap(handler.RebuildStrmFileByMountPoint())
	if err := processor.Process(stdctx.Background(), body); err != nil {
		t.Fatalf("single mount point rebuild should not require mount point service, got %v", err)
	}
}

func TestRebuildStrmFileByMountPointRejectsMissingFileID(t *testing.T) {
	tDB := setupRebuildStrmTestDB(t)
	fileTaskLogService := filetasklog.NewService(tDB)

	setupRebuildStrmMediaConfig(t)

	virtualFileService := &mockRebuildStrmVirtualFileService{
		filesByParent: map[int64][]*models.VirtualFile{
			0: {
				{ID: 1, ParentId: 0, Name: "unexpected.mp4", IsDir: false},
			},
		},
		errByParent: map[int64]error{},
	}
	handler := NewHandler(
		&mockRebuildStrmMediaFileService{},
		&mockRebuildStrmMountPointService{},
		virtualFileService,
		&mockRebuildStrmVerifyService{errByFileID: map[int64]error{}},
		fileTaskLogService,
	)

	processor := taskcontext.NewHandlerFuncWrapper(zap.NewNop()).Wrap(handler.RebuildStrmFileByMountPoint())
	err := processor.Process(stdctx.Background(), []byte(`{}`))

	if !errors.Is(err, errInvalidRebuildMountPointFileID) {
		t.Fatalf("expected invalid mount point file id error, got %v", err)
	}

	var count int64
	if err := tDB.db.Model(&models.FileTaskLog{}).Count(&count).Error; err != nil {
		t.Fatalf("count task logs: %v", err)
	}

	if count != 0 {
		t.Fatalf("expected invalid task not to create task log, got %d", count)
	}
}

func TestRebuildStrmFileByMountPointUsesMediaConfigBaseURL(t *testing.T) {
	tDB := setupRebuildStrmTestDB(t)
	fileTaskLogService := filetasklog.NewService(tDB)

	setupRebuildStrmMediaConfig(t)

	mediaFileService := &mockRebuildStrmMediaFileService{}
	handler := NewHandler(
		mediaFileService,
		&mockRebuildStrmMountPointService{},
		&mockRebuildStrmVirtualFileService{
			filesByParent: map[int64][]*models.VirtualFile{
				100: {
					{ID: 200, ParentId: 100, Name: "movie.mp4", IsDir: false},
				},
			},
			errByParent: map[int64]error{},
		},
		&mockRebuildStrmVerifyService{errByFileID: map[int64]error{}},
		fileTaskLogService,
	)

	req := topic.MediaRebuildStrmFileByMountPointRequest{
		MountPointFileId: 100,
		MountPointPath:   "/movies",
	}

	body, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}

	processor := taskcontext.NewHandlerFuncWrapper(zap.NewNop()).Wrap(handler.RebuildStrmFileByMountPoint())
	if err := processor.Process(stdctx.Background(), body); err != nil {
		t.Fatalf("process rebuild strm: %v", err)
	}

	calls := mediaFileService.WrittenCalls()
	if len(calls) != 1 {
		t.Fatalf("expected one STRM write, got %#v", calls)
	}

	if calls[0].baseURL != shared.MediaConfig.BaseURL {
		t.Fatalf("expected media base URL %q, got %q", shared.MediaConfig.BaseURL, calls[0].baseURL)
	}

	if calls[0].baseURL == shared.BaseURL {
		t.Fatalf("expected STRM write not to use system base URL %q", shared.BaseURL)
	}

	if !strings.HasPrefix(calls[0].fileURL, shared.MediaConfig.BaseURL+"/api/file/download/200?") {
		t.Fatalf("expected STRM download URL to use media base URL, got %q", calls[0].fileURL)
	}

	if strings.HasPrefix(calls[0].fileURL, shared.BaseURL) {
		t.Fatalf("expected STRM download URL not to use system base URL %q", shared.BaseURL)
	}
}

func TestRebuildStrmFileUsesMediaConfigBaseURL(t *testing.T) {
	tDB := setupRebuildStrmTestDB(t)
	fileTaskLogService := filetasklog.NewService(tDB)

	setupRebuildStrmMediaConfig(t)

	mediaFileService := &mockRebuildStrmMediaFileService{}
	handler := NewHandler(
		mediaFileService,
		&mockRebuildStrmMountPointService{items: []*models.MountPoint{
			{FileId: 100, FullPath: "/movies"},
		}},
		&mockRebuildStrmVirtualFileService{
			filesByParent: map[int64][]*models.VirtualFile{
				100: {
					{ID: 200, ParentId: 100, Name: "movie.mp4", IsDir: false},
				},
			},
			errByParent: map[int64]error{},
		},
		&mockRebuildStrmVerifyService{errByFileID: map[int64]error{}},
		fileTaskLogService,
	)

	processor := taskcontext.NewHandlerFuncWrapper(zap.NewNop()).Wrap(handler.RebuildStrmFile())
	if err := processor.Process(stdctx.Background(), []byte(`{}`)); err != nil {
		t.Fatalf("process rebuild strm: %v", err)
	}

	calls := mediaFileService.WrittenCalls()
	if len(calls) != 1 {
		t.Fatalf("expected one STRM write, got %#v", calls)
	}

	if calls[0].baseURL != shared.MediaConfig.BaseURL {
		t.Fatalf("expected media base URL %q, got %q", shared.MediaConfig.BaseURL, calls[0].baseURL)
	}

	if calls[0].baseURL == shared.BaseURL {
		t.Fatalf("expected STRM write not to use system base URL %q", shared.BaseURL)
	}

	if !strings.HasPrefix(calls[0].fileURL, shared.MediaConfig.BaseURL+"/api/file/download/200?") {
		t.Fatalf("expected STRM download URL to use media base URL, got %q", calls[0].fileURL)
	}

	if strings.HasPrefix(calls[0].fileURL, shared.BaseURL) {
		t.Fatalf("expected STRM download URL not to use system base URL %q", shared.BaseURL)
	}
}

func TestRebuildStrmFileByMountPointReturnsCompletedStatusError(t *testing.T) {
	tDB := setupRebuildStrmTestDB(t)
	statusErr := errors.New("completed status write failed")
	fileTaskLogService := &failingRebuildStatusFileTaskLogService{
		Service:      filetasklog.NewService(tDB),
		completedErr: statusErr,
	}

	setupRebuildStrmMediaConfig(t)

	handler := NewHandler(
		&mockRebuildStrmMediaFileService{},
		&mockRebuildStrmMountPointService{},
		&mockRebuildStrmVirtualFileService{
			filesByParent: map[int64][]*models.VirtualFile{
				100: {},
			},
			errByParent: map[int64]error{},
		},
		&mockRebuildStrmVerifyService{errByFileID: map[int64]error{}},
		fileTaskLogService,
	)

	req := topic.MediaRebuildStrmFileByMountPointRequest{
		MountPointFileId: 100,
		MountPointPath:   "/movies",
	}

	body, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}

	processor := taskcontext.NewHandlerFuncWrapper(zap.NewNop()).Wrap(handler.RebuildStrmFileByMountPoint())
	err = processor.Process(stdctx.Background(), body)

	if !errors.Is(err, statusErr) {
		t.Fatalf("expected completed status error, got %v", err)
	}
}

func TestRebuildStrmFileByMountPointReturnsFailedStatusError(t *testing.T) {
	tDB := setupRebuildStrmTestDB(t)
	statusErr := errors.New("failed status write failed")
	fileTaskLogService := &failingRebuildStatusFileTaskLogService{
		Service:   filetasklog.NewService(tDB),
		failedErr: statusErr,
	}

	setupRebuildStrmMediaConfig(t)

	handler := NewHandler(
		&mockRebuildStrmMediaFileService{},
		&mockRebuildStrmMountPointService{},
		&mockRebuildStrmVirtualFileService{
			filesByParent: map[int64][]*models.VirtualFile{
				100: {
					{ID: 200, ParentId: 100, Name: "movie.mp4", IsDir: false},
				},
			},
			errByParent: map[int64]error{},
		},
		&mockRebuildStrmVerifyService{errByFileID: map[int64]error{
			200: errors.New("sign failed"),
		}},
		fileTaskLogService,
	)

	req := topic.MediaRebuildStrmFileByMountPointRequest{
		MountPointFileId: 100,
		MountPointPath:   "/movies",
	}

	body, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}

	processor := taskcontext.NewHandlerFuncWrapper(zap.NewNop()).Wrap(handler.RebuildStrmFileByMountPoint())
	err = processor.Process(stdctx.Background(), body)

	if !errors.Is(err, statusErr) {
		t.Fatalf("expected failed status error, got %v", err)
	}
}

func TestRebuildStrmFileByMountPointMarksTaskFailedWhenFilesFail(t *testing.T) {
	tDB := setupRebuildStrmTestDB(t)
	fileTaskLogService := filetasklog.NewService(tDB)
	oldMediaConfig := shared.MediaConfig
	oldBaseURL := shared.BaseURL

	t.Cleanup(func() {
		shared.MediaConfig = oldMediaConfig
		shared.BaseURL = oldBaseURL
	})

	shared.MediaConfig = &models.MediaConfig{
		Enable:         true,
		StoragePath:    t.TempDir(),
		ConflictPolicy: mediaType.FileConflictPolicySkip,
	}
	shared.BaseURL = "http://example.test"

	handler := NewHandler(
		&mockRebuildStrmMediaFileService{},
		&mockRebuildStrmMountPointService{},
		&mockRebuildStrmVirtualFileService{
			filesByParent: map[int64][]*models.VirtualFile{
				100: {
					{ID: 200, ParentId: 100, Name: "movie.mp4", IsDir: false},
				},
			},
			errByParent: map[int64]error{},
		},
		&mockRebuildStrmVerifyService{errByFileID: map[int64]error{
			200: errors.New("sign failed"),
		}},
		fileTaskLogService,
	)

	req := topic.MediaRebuildStrmFileByMountPointRequest{
		MountPointFileId: 100,
		MountPointPath:   "/movies",
	}

	body, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}

	processor := taskcontext.NewHandlerFuncWrapper(zap.NewNop()).Wrap(handler.RebuildStrmFileByMountPoint())
	if err := processor.Process(stdctx.Background(), body); err != nil {
		t.Fatalf("process rebuild strm: %v", err)
	}

	var log models.FileTaskLog
	if err := tDB.db.First(&log).Error; err != nil {
		t.Fatalf("query task log: %v", err)
	}

	if log.Status != models.StatusFailed {
		t.Fatalf("expected failed task status, got %q", log.Status)
	}

	if log.Failed != 1 {
		t.Fatalf("expected one failed mount point, got %d", log.Failed)
	}

	if log.Completed != 0 {
		t.Fatalf("expected no completed mount points, got %d", log.Completed)
	}
}

func TestRebuildStrmFileReturnsCompletedStatusError(t *testing.T) {
	tDB := setupRebuildStrmTestDB(t)
	statusErr := errors.New("full rebuild completed status write failed")
	fileTaskLogService := &failingRebuildStatusFileTaskLogService{
		Service:      filetasklog.NewService(tDB),
		completedErr: statusErr,
	}

	setupRebuildStrmMediaConfig(t)

	handler := NewHandler(
		&mockRebuildStrmMediaFileService{},
		&mockRebuildStrmMountPointService{items: []*models.MountPoint{
			{FileId: 100, FullPath: "/ok"},
		}},
		&mockRebuildStrmVirtualFileService{
			filesByParent: map[int64][]*models.VirtualFile{
				100: {},
			},
			errByParent: map[int64]error{},
		},
		&mockRebuildStrmVerifyService{errByFileID: map[int64]error{}},
		fileTaskLogService,
	)

	processor := taskcontext.NewHandlerFuncWrapper(zap.NewNop()).Wrap(handler.RebuildStrmFile())
	err := processor.Process(stdctx.Background(), []byte(`{}`))

	if !errors.Is(err, statusErr) {
		t.Fatalf("expected full rebuild completed status error, got %v", err)
	}
}

func TestRebuildStrmFileReturnsFailedStatusError(t *testing.T) {
	tDB := setupRebuildStrmTestDB(t)
	statusErr := errors.New("full rebuild failed status write failed")
	fileTaskLogService := &failingRebuildStatusFileTaskLogService{
		Service:   filetasklog.NewService(tDB),
		failedErr: statusErr,
	}

	setupRebuildStrmMediaConfig(t)

	handler := NewHandler(
		&mockRebuildStrmMediaFileService{},
		&mockRebuildStrmMountPointService{items: []*models.MountPoint{
			{FileId: 100, FullPath: "/failed"},
		}},
		&mockRebuildStrmVirtualFileService{
			filesByParent: map[int64][]*models.VirtualFile{
				100: {
					{ID: 200, ParentId: 100, Name: "movie.mp4", IsDir: false},
				},
			},
			errByParent: map[int64]error{},
		},
		&mockRebuildStrmVerifyService{errByFileID: map[int64]error{
			200: errors.New("sign failed"),
		}},
		fileTaskLogService,
	)

	processor := taskcontext.NewHandlerFuncWrapper(zap.NewNop()).Wrap(handler.RebuildStrmFile())
	err := processor.Process(stdctx.Background(), []byte(`{}`))

	if !errors.Is(err, statusErr) {
		t.Fatalf("expected full rebuild failed status error, got %v", err)
	}
}

func TestRebuildStrmFileMarksMixedMountPointsAsFailed(t *testing.T) {
	tDB := setupRebuildStrmTestDB(t)
	fileTaskLogService := filetasklog.NewService(tDB)
	oldMediaConfig := shared.MediaConfig
	oldBaseURL := shared.BaseURL

	t.Cleanup(func() {
		shared.MediaConfig = oldMediaConfig
		shared.BaseURL = oldBaseURL
	})

	shared.MediaConfig = &models.MediaConfig{
		Enable:         true,
		StoragePath:    t.TempDir(),
		ConflictPolicy: mediaType.FileConflictPolicySkip,
	}
	shared.BaseURL = "http://example.test"

	handler := NewHandler(
		&mockRebuildStrmMediaFileService{},
		&mockRebuildStrmMountPointService{items: []*models.MountPoint{
			{FileId: 100, FullPath: "/ok"},
			{FileId: 300, FullPath: "/failed"},
		}},
		&mockRebuildStrmVirtualFileService{
			filesByParent: map[int64][]*models.VirtualFile{
				100: {
					{ID: 200, ParentId: 100, Name: "movie.mp4", IsDir: false},
				},
				300: {
					{ID: 400, ParentId: 300, Name: "bad.mp4", IsDir: false},
				},
			},
			errByParent: map[int64]error{},
		},
		&mockRebuildStrmVerifyService{errByFileID: map[int64]error{
			400: errors.New("sign failed"),
		}},
		fileTaskLogService,
	)

	processor := taskcontext.NewHandlerFuncWrapper(zap.NewNop()).Wrap(handler.RebuildStrmFile())
	if err := processor.Process(stdctx.Background(), []byte(`{}`)); err != nil {
		t.Fatalf("process rebuild strm: %v", err)
	}

	var log models.FileTaskLog
	if err := tDB.db.First(&log).Error; err != nil {
		t.Fatalf("query task log: %v", err)
	}

	if log.Status != models.StatusFailed {
		t.Fatalf("expected failed task status, got %q", log.Status)
	}

	if log.Failed != 1 {
		t.Fatalf("expected one failed mount point, got %d", log.Failed)
	}

	if log.Completed != 1 {
		t.Fatalf("expected one completed mount point, got %d", log.Completed)
	}
}

func TestRebuildStrmFileMarksNilMountPointAsFailed(t *testing.T) {
	tDB := setupRebuildStrmTestDB(t)
	fileTaskLogService := filetasklog.NewService(tDB)

	setupRebuildStrmMediaConfig(t)

	mediaFileService := &mockRebuildStrmMediaFileService{}
	handler := NewHandler(
		mediaFileService,
		&mockRebuildStrmMountPointService{items: []*models.MountPoint{nil}},
		&mockRebuildStrmVirtualFileService{
			filesByParent: map[int64][]*models.VirtualFile{},
			errByParent:   map[int64]error{},
		},
		&mockRebuildStrmVerifyService{errByFileID: map[int64]error{}},
		fileTaskLogService,
	)

	processor := taskcontext.NewHandlerFuncWrapper(zap.NewNop()).Wrap(handler.RebuildStrmFile())
	if err := processor.Process(stdctx.Background(), []byte(`{}`)); err != nil {
		t.Fatalf("process rebuild strm with nil mount point: %v", err)
	}

	if calls := mediaFileService.WrittenCalls(); len(calls) != 0 {
		t.Fatalf("expected nil mount point not to write STRM files, got %#v", calls)
	}

	var log models.FileTaskLog
	if err := tDB.db.First(&log).Error; err != nil {
		t.Fatalf("query task log: %v", err)
	}

	if log.Status != models.StatusFailed {
		t.Fatalf("expected failed task status, got %q", log.Status)
	}

	if log.Failed != 1 || log.Completed != 0 || log.Total != 1 {
		t.Fatalf("expected failed=1 completed=0 total=1, got failed=%d completed=%d total=%d", log.Failed, log.Completed, log.Total)
	}
}

func TestRebuildStrmFileByMountPointMarksNilChildFileAsFailed(t *testing.T) {
	tDB := setupRebuildStrmTestDB(t)
	fileTaskLogService := filetasklog.NewService(tDB)

	setupRebuildStrmMediaConfig(t)

	mediaFileService := &mockRebuildStrmMediaFileService{}
	handler := NewHandler(
		mediaFileService,
		&mockRebuildStrmMountPointService{},
		&mockRebuildStrmVirtualFileService{
			filesByParent: map[int64][]*models.VirtualFile{
				100: {nil},
			},
			errByParent: map[int64]error{},
		},
		&mockRebuildStrmVerifyService{errByFileID: map[int64]error{}},
		fileTaskLogService,
	)

	req := topic.MediaRebuildStrmFileByMountPointRequest{
		MountPointFileId: 100,
		MountPointPath:   "/movies",
	}

	body, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}

	processor := taskcontext.NewHandlerFuncWrapper(zap.NewNop()).Wrap(handler.RebuildStrmFileByMountPoint())
	if err := processor.Process(stdctx.Background(), body); err != nil {
		t.Fatalf("process rebuild strm with nil child file: %v", err)
	}

	if calls := mediaFileService.WrittenCalls(); len(calls) != 0 {
		t.Fatalf("expected nil child file not to write STRM files, got %#v", calls)
	}

	var log models.FileTaskLog
	if err := tDB.db.First(&log).Error; err != nil {
		t.Fatalf("query task log: %v", err)
	}

	if log.Status != models.StatusFailed {
		t.Fatalf("expected failed task status, got %q", log.Status)
	}

	if log.Failed != 1 || log.Completed != 0 || log.Total != 1 {
		t.Fatalf("expected failed=1 completed=0 total=1, got failed=%d completed=%d total=%d", log.Failed, log.Completed, log.Total)
	}
}
