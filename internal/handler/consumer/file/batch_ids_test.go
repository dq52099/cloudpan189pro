package file

import (
	stdctx "context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/xxcheng123/cloudpan189-share/internal/bootstrap"
	"github.com/xxcheng123/cloudpan189-share/internal/consts"
	appContext "github.com/xxcheng123/cloudpan189-share/internal/framework/context"
	"github.com/xxcheng123/cloudpan189-share/internal/framework/taskcontext"
	"github.com/xxcheng123/cloudpan189-share/internal/pkgs/taskengine"
	"github.com/xxcheng123/cloudpan189-share/internal/pkgs/utils"
	"github.com/xxcheng123/cloudpan189-share/internal/repository/models"
	"github.com/xxcheng123/cloudpan189-share/internal/services/filetasklog"
	group2fileSvi "github.com/xxcheng123/cloudpan189-share/internal/services/group2file"
	mountPointSvi "github.com/xxcheng123/cloudpan189-share/internal/services/mountpoint"
	userMountPointTokenSvi "github.com/xxcheng123/cloudpan189-share/internal/services/userMountPointToken"
	"github.com/xxcheng123/cloudpan189-share/internal/shared"
	"github.com/xxcheng123/cloudpan189-share/internal/types/topic"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type batchDeleteTaskLogTestDB struct {
	db *gorm.DB
}

func (t *batchDeleteTaskLogTestDB) GetDB(ctx appContext.Context) *gorm.DB {
	return t.db.WithContext(ctx)
}

func (t *batchDeleteTaskLogTestDB) GetDBWithoutContext() *gorm.DB {
	return t.db
}

func (t *batchDeleteTaskLogTestDB) GetLogger(name string, fields ...zap.Field) *zap.Logger {
	return zap.NewNop()
}

func (t *batchDeleteTaskLogTestDB) Close() {}

func (t *batchDeleteTaskLogTestDB) GetPort() int {
	return 9999
}

func (t *batchDeleteTaskLogTestDB) GetTaskEngine() taskengine.TaskEngine {
	return nil
}

func (t *batchDeleteTaskLogTestDB) GetHTTPEngine() *gin.Engine {
	return nil
}

var _ bootstrap.ServiceContext = (*batchDeleteTaskLogTestDB)(nil)

type mockBatchModifyTokenMountPointService struct {
	mountPointSvi.Service
	mountPoints        map[int64]*models.MountPoint
	queryErrByID       map[int64]error
	batchDeleteErrByID map[int64]error
}

func (m *mockBatchModifyTokenMountPointService) Query(ctx appContext.Context, fileID int64) (*models.MountPoint, error) {
	if err := m.queryErrByID[fileID]; err != nil {
		return nil, err
	}

	mountPoint, ok := m.mountPoints[fileID]
	if !ok {
		return nil, gorm.ErrRecordNotFound
	}

	return mountPoint, nil
}

func (m *mockBatchModifyTokenMountPointService) BatchDelete(ctx appContext.Context, req *mountPointSvi.BatchDeleteRequest) error {
	for _, id := range req.FileIds {
		if err := m.batchDeleteErrByID[id]; err != nil {
			return err
		}
	}

	return nil
}

func (m *mockBatchModifyTokenMountPointService) UpdateRefreshTime(ctx appContext.Context, fileID int64) error {
	return nil
}

func (m *mockBatchModifyTokenMountPointService) UpdateLastState(ctx appContext.Context, fileID int64, state string) error {
	return nil
}

type mockBatchModifyTokenUserMountPointTokenService struct {
	userMountPointTokenSvi.Service
	bindErrByMountPointID     map[int64]error
	unbindErrByMountPointID   map[int64]error
	unbindDeletedByMountPoint map[int64]bool
	boundMountPointIDs        []int64
	unboundMountPointIDs      []int64
}

func (m *mockBatchModifyTokenUserMountPointTokenService) BindToken(
	ctx appContext.Context,
	userID int64,
	mountPointID int64,
	tokenID int64,
) error {
	if err := m.bindErrByMountPointID[mountPointID]; err != nil {
		return err
	}

	m.boundMountPointIDs = append(m.boundMountPointIDs, mountPointID)

	return nil
}

func (m *mockBatchModifyTokenUserMountPointTokenService) UnbindTokenWithResult(
	ctx appContext.Context,
	userID int64,
	mountPointID int64,
) (bool, error) {
	if err := m.unbindErrByMountPointID[mountPointID]; err != nil {
		return false, err
	}

	if deleted, ok := m.unbindDeletedByMountPoint[mountPointID]; ok {
		if deleted {
			m.unboundMountPointIDs = append(m.unboundMountPointIDs, mountPointID)
		}

		return deleted, nil
	}

	m.unboundMountPointIDs = append(m.unboundMountPointIDs, mountPointID)

	return true, nil
}

type mockBatchModifyTokenGroup2FileService struct {
	group2fileSvi.Service
	bindFileIDs  []int64
	bindFilesErr error
}

func (m *mockBatchModifyTokenGroup2FileService) GetBindFiles(ctx appContext.Context, groupID int64) ([]int64, error) {
	return m.bindFileIDs, m.bindFilesErr
}

type failingFailedFileTaskLogService struct {
	filetasklog.Service
	failedErr error
}

func (s *failingFailedFileTaskLogService) Failed(
	ctx appContext.Context,
	key filetasklog.LogKey,
	opts ...utils.Field,
) error {
	return s.failedErr
}

type failingCompletedFileTaskLogService struct {
	filetasklog.Service
	completedErr error
}

func (s *failingCompletedFileTaskLogService) Completed(
	ctx appContext.Context,
	key filetasklog.LogKey,
	opts ...utils.Field,
) error {
	return s.completedErr
}

func setupBatchDeleteTaskLogTestDB(t *testing.T) *batchDeleteTaskLogTestDB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}

	if err := db.AutoMigrate(&models.FileTaskLog{}); err != nil {
		t.Fatalf("migrate test db: %v", err)
	}

	return &batchDeleteTaskLogTestDB{db: db}
}

func TestClearFileReturnsCompletedStatusError(t *testing.T) {
	tDB := setupBatchDeleteTaskLogTestDB(t)
	statusErr := errors.New("completed status write failed")
	logService := &failingCompletedFileTaskLogService{
		Service:      filetasklog.NewService(tDB),
		completedErr: statusErr,
	}
	virtualFileService := &mockBatchDeleteVirtualFileService{
		filesByID: map[int64]*models.VirtualFile{
			10: {ID: 10, TopId: 10, Name: "dir", IsDir: true},
		},
		childrenByParent: map[int64][]*models.VirtualFile{},
		deleteErrByID:    map[int64]error{},
	}

	oldMediaConfig := shared.MediaConfig
	shared.MediaConfig = nil

	defer func() {
		shared.MediaConfig = oldMediaConfig
	}()

	handler := NewHandler(
		zap.NewNop(),
		virtualFileService,
		nil,
		nil,
		&mockBatchModifyTokenMountPointService{},
		logService,
		nil,
		nil,
		nil,
		nil,
	)
	req := topic.FileClearFileRequest{FileId: 10}

	payload, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}

	processor := taskcontext.NewHandlerFuncWrapper(zap.NewNop()).Wrap(handler.ClearFile())
	err = processor.Process(stdctx.Background(), payload)

	if !errors.Is(err, statusErr) {
		t.Fatalf("expected completed status error, got %v", err)
	}
}

func TestClearFileJoinsBusinessAndFailedStatusErrors(t *testing.T) {
	tDB := setupBatchDeleteTaskLogTestDB(t)
	statusErr := errors.New("clear failed status write failed")
	logService := &failingFailedFileTaskLogService{
		Service:   filetasklog.NewService(tDB),
		failedErr: statusErr,
	}
	clearErr := errors.New("clear mount files failed")
	virtualFileService := &mockBatchDeleteVirtualFileService{
		listErr: clearErr,
		filesByID: map[int64]*models.VirtualFile{
			10: {ID: 10, TopId: 10, Name: "dir", IsDir: true},
		},
		childrenByParent: map[int64][]*models.VirtualFile{},
		deleteErrByID:    map[int64]error{},
	}

	oldMediaConfig := shared.MediaConfig
	shared.MediaConfig = nil

	defer func() {
		shared.MediaConfig = oldMediaConfig
	}()

	handler := NewHandler(
		zap.NewNop(),
		virtualFileService,
		nil,
		nil,
		&mockBatchModifyTokenMountPointService{},
		logService,
		nil,
		nil,
		nil,
		nil,
	)
	req := topic.FileClearFileRequest{FileId: 10}

	payload, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}

	processor := taskcontext.NewHandlerFuncWrapper(zap.NewNop()).Wrap(handler.ClearFile())
	err = processor.Process(stdctx.Background(), payload)

	if !errors.Is(err, clearErr) {
		t.Fatalf("expected clear business error, got %v", err)
	}

	if !errors.Is(err, statusErr) {
		t.Fatalf("expected clear failed status error, got %v", err)
	}
}

func TestScanFileReturnsCompletedStatusError(t *testing.T) {
	tDB := setupBatchDeleteTaskLogTestDB(t)
	statusErr := errors.New("scan completed status write failed")
	logService := &failingCompletedFileTaskLogService{
		Service:      filetasklog.NewService(tDB),
		completedErr: statusErr,
	}
	virtualFileService := &mockBatchDeleteVirtualFileService{
		childrenByParent: map[int64][]*models.VirtualFile{},
		deleteErrByID:    map[int64]error{},
		filesByID:        map[int64]*models.VirtualFile{},
	}

	oldMediaConfig := shared.MediaConfig
	shared.MediaConfig = nil

	defer func() {
		shared.MediaConfig = oldMediaConfig
	}()

	handler := NewHandler(
		zap.NewNop(),
		virtualFileService,
		nil,
		nil,
		&mockBatchModifyTokenMountPointService{},
		logService,
		nil,
		nil,
		nil,
		nil,
	)
	req := topic.FileScanFileRequest{FileId: 0}

	payload, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}

	processor := taskcontext.NewHandlerFuncWrapper(zap.NewNop()).Wrap(handler.ScanFile())
	err = processor.Process(stdctx.Background(), payload)

	if !errors.Is(err, statusErr) {
		t.Fatalf("expected scan completed status error, got %v", err)
	}
}

func TestScanFileJoinsBusinessAndFailedStatusErrors(t *testing.T) {
	tDB := setupBatchDeleteTaskLogTestDB(t)
	statusErr := errors.New("scan failed status write failed")
	logService := &failingFailedFileTaskLogService{
		Service:   filetasklog.NewService(tDB),
		failedErr: statusErr,
	}
	virtualFileService := &mockBatchDeleteVirtualFileService{
		childrenByParent: map[int64][]*models.VirtualFile{},
		deleteErrByID:    map[int64]error{},
		filesByID: map[int64]*models.VirtualFile{
			10: {ID: 10, TopId: 100, Name: "unsupported", IsDir: true, OsType: "unsupported"},
		},
	}

	oldMediaConfig := shared.MediaConfig
	shared.MediaConfig = nil

	defer func() {
		shared.MediaConfig = oldMediaConfig
	}()

	handler := NewHandler(
		zap.NewNop(),
		virtualFileService,
		nil,
		nil,
		&mockBatchModifyTokenMountPointService{},
		logService,
		nil,
		nil,
		nil,
		nil,
	)
	req := topic.FileScanFileRequest{FileId: 10}

	payload, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}

	processor := taskcontext.NewHandlerFuncWrapper(zap.NewNop()).Wrap(handler.ScanFile())
	err = processor.Process(stdctx.Background(), payload)

	if !errors.Is(err, statusErr) {
		t.Fatalf("expected scan failed status error, got %v", err)
	}

	if err == nil || !strings.Contains(err.Error(), "不支持的文件类型") {
		t.Fatalf("expected scan business error to be preserved, got %v", err)
	}
}

func TestHandleBatchDeleteSkipsEmptyIDList(t *testing.T) {
	handler := NewHandler(
		zap.NewNop(),
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
	)
	req := topic.FileBatchDeleteRequest{IDs: []int64{}}

	payload, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}

	processor := taskcontext.NewHandlerFuncWrapper(zap.NewNop()).Wrap(handler.HandleBatchDelete())
	if err := processor.Process(stdctx.Background(), payload); err != nil {
		t.Fatalf("expected empty batch delete to be ignored, got %v", err)
	}
}

func TestHandleBatchDeleteRejectsInvalidID(t *testing.T) {
	handler := NewHandler(
		zap.NewNop(),
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
	)
	req := topic.FileBatchDeleteRequest{IDs: []int64{10, 0}}

	payload, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}

	processor := taskcontext.NewHandlerFuncWrapper(zap.NewNop()).Wrap(handler.HandleBatchDelete())

	err = processor.Process(stdctx.Background(), payload)
	if !errors.Is(err, errInvalidFileTaskID) {
		t.Fatalf("expected invalid file task id, got %v", err)
	}
}

func TestHandleBatchDeleteRejectsMalformedJSON(t *testing.T) {
	handler := NewHandler(
		zap.NewNop(),
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
	)

	processor := taskcontext.NewHandlerFuncWrapper(zap.NewNop()).Wrap(handler.HandleBatchDelete())
	if err := processor.Process(stdctx.Background(), []byte(`{"ids":`)); err == nil {
		t.Fatal("expected malformed batch delete payload to return error")
	}
}

func TestHandleBatchDeleteReturnsCompletedStatusError(t *testing.T) {
	tDB := setupBatchDeleteTaskLogTestDB(t)
	statusErr := errors.New("batch delete completed status write failed")
	logService := &failingCompletedFileTaskLogService{
		Service:      filetasklog.NewService(tDB),
		completedErr: statusErr,
	}
	virtualFileService := &mockBatchDeleteVirtualFileService{
		filesByID: map[int64]*models.VirtualFile{
			10: {ID: 10, TopId: 100, Name: "ok.mkv"},
		},
		childrenByParent: map[int64][]*models.VirtualFile{},
		deleteErrByID:    map[int64]error{},
	}

	oldMediaConfig := shared.MediaConfig
	shared.MediaConfig = nil

	defer func() {
		shared.MediaConfig = oldMediaConfig
	}()

	handler := NewHandler(
		zap.NewNop(),
		virtualFileService,
		nil,
		nil,
		nil,
		logService,
		nil,
		nil,
		nil,
		nil,
	)
	req := topic.FileBatchDeleteRequest{IDs: []int64{10}}

	payload, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}

	processor := taskcontext.NewHandlerFuncWrapper(zap.NewNop()).Wrap(handler.HandleBatchDelete())
	err = processor.Process(stdctx.Background(), payload)

	if !errors.Is(err, statusErr) {
		t.Fatalf("expected batch delete completed status error, got %v", err)
	}
}

func TestHandleBatchDeleteReturnsFailedStatusError(t *testing.T) {
	tDB := setupBatchDeleteTaskLogTestDB(t)
	statusErr := errors.New("batch delete failed status write failed")
	logService := &failingFailedFileTaskLogService{
		Service:   filetasklog.NewService(tDB),
		failedErr: statusErr,
	}
	deleteErr := errors.New("delete failed")
	virtualFileService := &mockBatchDeleteVirtualFileService{
		filesByID: map[int64]*models.VirtualFile{
			10: {ID: 10, TopId: 100, Name: "broken.mkv"},
		},
		childrenByParent: map[int64][]*models.VirtualFile{},
		deleteErrByID:    map[int64]error{10: deleteErr},
	}

	oldMediaConfig := shared.MediaConfig
	shared.MediaConfig = nil

	defer func() {
		shared.MediaConfig = oldMediaConfig
	}()

	handler := NewHandler(
		zap.NewNop(),
		virtualFileService,
		nil,
		nil,
		nil,
		logService,
		nil,
		nil,
		nil,
		nil,
	)
	req := topic.FileBatchDeleteRequest{IDs: []int64{10}}

	payload, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}

	processor := taskcontext.NewHandlerFuncWrapper(zap.NewNop()).Wrap(handler.HandleBatchDelete())
	err = processor.Process(stdctx.Background(), payload)

	if !errors.Is(err, statusErr) {
		t.Fatalf("expected batch delete failed status error, got %v", err)
	}
}

func TestHandleBatchDeleteRecordsPartialFailure(t *testing.T) {
	tDB := setupBatchDeleteTaskLogTestDB(t)
	logService := filetasklog.NewService(tDB)
	deleteErr := errors.New("delete failed")
	virtualFileService := &mockBatchDeleteVirtualFileService{
		filesByID: map[int64]*models.VirtualFile{
			10: {ID: 10, TopId: 100, Name: "ok.mkv"},
			11: {ID: 11, TopId: 100, Name: "broken.mkv"},
		},
		childrenByParent: map[int64][]*models.VirtualFile{},
		deleteErrByID:    map[int64]error{11: deleteErr},
	}

	oldMediaConfig := shared.MediaConfig
	shared.MediaConfig = nil

	defer func() {
		shared.MediaConfig = oldMediaConfig
	}()

	handler := NewHandler(
		zap.NewNop(),
		virtualFileService,
		nil,
		nil,
		nil,
		logService,
		nil,
		nil,
		nil,
		nil,
	)
	req := topic.FileBatchDeleteRequest{IDs: []int64{10, 11}}

	payload, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}

	processor := taskcontext.NewHandlerFuncWrapper(zap.NewNop()).Wrap(handler.HandleBatchDelete())
	if err := processor.Process(stdctx.Background(), payload); err != nil {
		t.Fatalf("process batch delete: %v", err)
	}

	var log models.FileTaskLog
	if err := tDB.db.First(&log).Error; err != nil {
		t.Fatalf("query task log: %v", err)
	}

	if log.Status != models.StatusFailed {
		t.Fatalf("expected task status failed, got %q", log.Status)
	}

	if log.Completed != 1 || log.Total != 2 || log.Failed != 1 {
		t.Fatalf("expected completed=1 total=2 failed=1, got completed=%d total=%d failed=%d", log.Completed, log.Total, log.Failed)
	}
}

func TestHandleBatchDeleteRecordsMountPointDeleteFailure(t *testing.T) {
	tDB := setupBatchDeleteTaskLogTestDB(t)
	logService := filetasklog.NewService(tDB)
	deleteErr := errors.New("mount point delete failed")
	virtualFileService := &mockBatchDeleteVirtualFileService{
		filesByID: map[int64]*models.VirtualFile{
			10: {ID: 10, TopId: 10, IsTop: true, Name: "top"},
		},
		childrenByParent: map[int64][]*models.VirtualFile{},
		deleteErrByID:    map[int64]error{},
	}

	oldMediaConfig := shared.MediaConfig
	shared.MediaConfig = nil

	defer func() {
		shared.MediaConfig = oldMediaConfig
	}()

	handler := NewHandler(
		zap.NewNop(),
		virtualFileService,
		nil,
		nil,
		&mockBatchModifyTokenMountPointService{
			batchDeleteErrByID: map[int64]error{10: deleteErr},
		},
		logService,
		nil,
		nil,
		nil,
		nil,
	)
	req := topic.FileBatchDeleteRequest{IDs: []int64{10}}

	payload, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}

	processor := taskcontext.NewHandlerFuncWrapper(zap.NewNop()).Wrap(handler.HandleBatchDelete())
	if err := processor.Process(stdctx.Background(), payload); err != nil {
		t.Fatalf("batch delete keeps processing item failures without retrying whole batch: %v", err)
	}

	var log models.FileTaskLog
	if err := tDB.db.First(&log).Error; err != nil {
		t.Fatalf("query task log: %v", err)
	}

	if log.Status != models.StatusFailed {
		t.Fatalf("expected task status failed, got %q", log.Status)
	}

	if log.Completed != 0 || log.Total != 1 || log.Failed != 1 {
		t.Fatalf("expected completed=0 total=1 failed=1, got completed=%d total=%d failed=%d", log.Completed, log.Total, log.Failed)
	}
}

func TestHandleDeleteRejectsMalformedJSON(t *testing.T) {
	handler := NewHandler(
		zap.NewNop(),
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
	)

	processor := taskcontext.NewHandlerFuncWrapper(zap.NewNop()).Wrap(handler.HandleDelete())
	if err := processor.Process(stdctx.Background(), []byte(`{"fileId":`)); err == nil {
		t.Fatal("expected malformed delete payload to return error")
	}
}

func TestHandleDeleteReturnsCompletedStatusError(t *testing.T) {
	tDB := setupBatchDeleteTaskLogTestDB(t)
	statusErr := errors.New("delete completed status write failed")
	logService := &failingCompletedFileTaskLogService{
		Service:      filetasklog.NewService(tDB),
		completedErr: statusErr,
	}
	virtualFileService := &mockBatchDeleteVirtualFileService{
		filesByID: map[int64]*models.VirtualFile{
			10: {ID: 10, TopId: 100, Name: "ok.mkv"},
		},
		childrenByParent: map[int64][]*models.VirtualFile{},
		deleteErrByID:    map[int64]error{},
	}

	oldMediaConfig := shared.MediaConfig
	shared.MediaConfig = nil

	defer func() {
		shared.MediaConfig = oldMediaConfig
	}()

	handler := NewHandler(
		zap.NewNop(),
		virtualFileService,
		nil,
		nil,
		nil,
		logService,
		nil,
		nil,
		nil,
		nil,
	)
	req := topic.FileDeleteRequest{FileId: 10}

	payload, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}

	processor := taskcontext.NewHandlerFuncWrapper(zap.NewNop()).Wrap(handler.HandleDelete())
	err = processor.Process(stdctx.Background(), payload)

	if !errors.Is(err, statusErr) {
		t.Fatalf("expected delete completed status error, got %v", err)
	}
}

func TestHandleDeleteJoinsBusinessAndFailedStatusErrors(t *testing.T) {
	tDB := setupBatchDeleteTaskLogTestDB(t)
	statusErr := errors.New("delete failed status write failed")
	logService := &failingFailedFileTaskLogService{
		Service:   filetasklog.NewService(tDB),
		failedErr: statusErr,
	}
	deleteErr := errors.New("delete failed")
	virtualFileService := &mockBatchDeleteVirtualFileService{
		filesByID: map[int64]*models.VirtualFile{
			10: {ID: 10, TopId: 100, Name: "broken.mkv"},
		},
		childrenByParent: map[int64][]*models.VirtualFile{},
		deleteErrByID:    map[int64]error{10: deleteErr},
	}

	handler := NewHandler(
		zap.NewNop(),
		virtualFileService,
		nil,
		nil,
		nil,
		logService,
		nil,
		nil,
		nil,
		nil,
	)
	req := topic.FileDeleteRequest{FileId: 10}

	payload, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}

	processor := taskcontext.NewHandlerFuncWrapper(zap.NewNop()).Wrap(handler.HandleDelete())
	err = processor.Process(stdctx.Background(), payload)

	if !errors.Is(err, deleteErr) {
		t.Fatalf("expected delete business error, got %v", err)
	}

	if !errors.Is(err, statusErr) {
		t.Fatalf("expected delete failed status error, got %v", err)
	}
}

func TestHandleDeleteReturnsMountPointDeleteFailure(t *testing.T) {
	tDB := setupBatchDeleteTaskLogTestDB(t)
	logService := filetasklog.NewService(tDB)
	deleteErr := errors.New("mount point delete failed")
	virtualFileService := &mockBatchDeleteVirtualFileService{
		filesByID: map[int64]*models.VirtualFile{
			10: {ID: 10, TopId: 10, IsTop: true, Name: "top"},
		},
		childrenByParent: map[int64][]*models.VirtualFile{},
		deleteErrByID:    map[int64]error{},
	}

	handler := NewHandler(
		zap.NewNop(),
		virtualFileService,
		nil,
		nil,
		&mockBatchModifyTokenMountPointService{
			batchDeleteErrByID: map[int64]error{10: deleteErr},
		},
		logService,
		nil,
		nil,
		nil,
		nil,
	)
	req := topic.FileDeleteRequest{FileId: 10}

	payload, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}

	processor := taskcontext.NewHandlerFuncWrapper(zap.NewNop()).Wrap(handler.HandleDelete())
	err = processor.Process(stdctx.Background(), payload)

	if !errors.Is(err, deleteErr) {
		t.Fatalf("expected mount point delete error, got %v", err)
	}

	var log models.FileTaskLog
	if err := tDB.db.First(&log).Error; err != nil {
		t.Fatalf("query task log: %v", err)
	}

	if log.Status != models.StatusFailed {
		t.Fatalf("expected task status failed, got %q", log.Status)
	}

	if log.Total != 1 || log.Completed != 0 {
		t.Fatalf("expected total=1 completed=0, got total=%d completed=%d", log.Total, log.Completed)
	}
}

func TestHandleDeleteReturnsClearMountFilesFailureWhenFileMissing(t *testing.T) {
	tDB := setupBatchDeleteTaskLogTestDB(t)
	logService := filetasklog.NewService(tDB)
	clearErr := errors.New("clear mount files failed")
	virtualFileService := &mockBatchDeleteVirtualFileService{
		listErr:          clearErr,
		filesByID:        map[int64]*models.VirtualFile{},
		childrenByParent: map[int64][]*models.VirtualFile{},
		deleteErrByID:    map[int64]error{},
	}

	handler := NewHandler(
		zap.NewNop(),
		virtualFileService,
		nil,
		nil,
		&mockBatchModifyTokenMountPointService{},
		logService,
		nil,
		nil,
		nil,
		nil,
	)
	req := topic.FileDeleteRequest{FileId: 10}

	payload, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}

	processor := taskcontext.NewHandlerFuncWrapper(zap.NewNop()).Wrap(handler.HandleDelete())
	err = processor.Process(stdctx.Background(), payload)

	if !errors.Is(err, clearErr) {
		t.Fatalf("expected clear mount files error, got %v", err)
	}

	var log models.FileTaskLog
	if err := tDB.db.First(&log).Error; err != nil {
		t.Fatalf("query task log: %v", err)
	}

	if log.Status != models.StatusFailed {
		t.Fatalf("expected task status failed, got %q", log.Status)
	}
}

func TestHandleBatchDeleteCompletesParentTracker(t *testing.T) {
	tDB := setupBatchDeleteTaskLogTestDB(t)
	logService := filetasklog.NewService(tDB)
	ctx := appContext.NewContext(stdctx.Background())

	parentTracker, err := logService.Create(ctx, "批量删除", "批量删除挂载点")
	if err != nil {
		t.Fatalf("create parent task log: %v", err)
	}

	if err := logService.Running(ctx, parentTracker); err != nil {
		t.Fatalf("mark parent running: %v", err)
	}

	if err := logService.FlushCount(ctx, parentTracker, filetasklog.WithTotalCounter(1)); err != nil {
		t.Fatalf("flush parent total: %v", err)
	}

	virtualFileService := &mockBatchDeleteVirtualFileService{
		filesByID: map[int64]*models.VirtualFile{
			10: {ID: 10, TopId: 100, Name: "ok"},
		},
		childrenByParent: map[int64][]*models.VirtualFile{},
		deleteErrByID:    map[int64]error{},
	}

	oldMediaConfig := shared.MediaConfig
	shared.MediaConfig = nil

	defer func() {
		shared.MediaConfig = oldMediaConfig
	}()

	handler := NewHandler(
		zap.NewNop(),
		virtualFileService,
		nil,
		nil,
		&mockBatchModifyTokenMountPointService{},
		logService,
		nil,
		nil,
		nil,
		nil,
	)
	req := topic.FileBatchDeleteRequest{IDs: []int64{10}}

	payload, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}

	parentCtx := ctx.WithValue(consts.CtxKeyTaskTracker, parentTracker)

	processor := taskcontext.NewHandlerFuncWrapper(zap.NewNop()).Wrap(handler.HandleBatchDelete())
	if err := processor.Process(parentCtx, payload); err != nil {
		t.Fatalf("process batch delete: %v", err)
	}

	var parentLog models.FileTaskLog
	if err := tDB.db.First(&parentLog, parentTracker.GetID()).Error; err != nil {
		t.Fatalf("query parent task log: %v", err)
	}

	if parentLog.Status != models.StatusCompleted {
		t.Fatalf("expected parent task completed, got %q", parentLog.Status)
	}

	if parentLog.Total != 1 || parentLog.Completed != 1 || parentLog.Failed != 0 {
		t.Fatalf("expected parent total=1 completed=1 failed=0, got total=%d completed=%d failed=%d", parentLog.Total, parentLog.Completed, parentLog.Failed)
	}
}

func TestHandleBatchModifyTokenSkipsEmptyIDList(t *testing.T) {
	handler := NewHandler(
		zap.NewNop(),
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
	)
	req := topic.FileBatchModifyTokenRequest{IDs: []int64{}}

	payload, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}

	processor := taskcontext.NewHandlerFuncWrapper(zap.NewNop()).Wrap(handler.HandleBatchModifyToken())
	if err := processor.Process(stdctx.Background(), payload); err != nil {
		t.Fatalf("expected empty batch modify token to be ignored, got %v", err)
	}
}

func TestHandleBatchModifyTokenRejectsInvalidID(t *testing.T) {
	handler := NewHandler(
		zap.NewNop(),
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
	)
	req := topic.FileBatchModifyTokenRequest{IDs: []int64{10, -1}}

	payload, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}

	processor := taskcontext.NewHandlerFuncWrapper(zap.NewNop()).Wrap(handler.HandleBatchModifyToken())

	err = processor.Process(stdctx.Background(), payload)
	if !errors.Is(err, errInvalidFileTaskID) {
		t.Fatalf("expected invalid file task id, got %v", err)
	}
}

func TestHandleBatchModifyTokenRecordsPartialFailureWithoutRetry(t *testing.T) {
	tDB := setupBatchDeleteTaskLogTestDB(t)
	logService := filetasklog.NewService(tDB)
	bindErr := errors.New("bind failed")
	userMountPointTokenService := &mockBatchModifyTokenUserMountPointTokenService{
		bindErrByMountPointID: map[int64]error{202: bindErr},
	}

	handler := NewHandler(
		zap.NewNop(),
		nil,
		nil,
		nil,
		&mockBatchModifyTokenMountPointService{
			mountPoints: map[int64]*models.MountPoint{
				10: {ID: 101, FileId: 10, CreatorUserID: 1, Name: "ok"},
				20: {ID: 202, FileId: 20, CreatorUserID: 1, Name: "fail"},
			},
			queryErrByID: map[int64]error{},
		},
		logService,
		nil,
		nil,
		nil,
		userMountPointTokenService,
	)
	req := topic.FileBatchModifyTokenRequest{
		IDs:     []int64{10, 20},
		TokenID: 99,
		UserID:  7,
		IsAdmin: true,
	}

	payload, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}

	processor := taskcontext.NewHandlerFuncWrapper(zap.NewNop()).Wrap(handler.HandleBatchModifyToken())
	if err := processor.Process(stdctx.Background(), payload); err != nil {
		t.Fatalf("expected partial failure not to retry whole batch, got %v", err)
	}

	if len(userMountPointTokenService.boundMountPointIDs) != 1 || userMountPointTokenService.boundMountPointIDs[0] != 101 {
		t.Fatalf("expected only successful mount point to be bound, got %v", userMountPointTokenService.boundMountPointIDs)
	}

	var log models.FileTaskLog
	if err := tDB.db.First(&log).Error; err != nil {
		t.Fatalf("query task log: %v", err)
	}

	if log.Status != models.StatusFailed {
		t.Fatalf("expected task status failed, got %q", log.Status)
	}

	if log.Total != 2 || log.Completed != 2 || log.Failed != 1 {
		t.Fatalf("expected total=2 completed=2 failed=1, got total=%d completed=%d failed=%d", log.Total, log.Completed, log.Failed)
	}

	if log.Result == "" || !strings.Contains(log.Result, "成功 1 个，失败 1 个") {
		t.Fatalf("expected partial failure result, got %q", log.Result)
	}
}

func TestHandleBatchModifyTokenReturnsFinalFailedStatusError(t *testing.T) {
	tDB := setupBatchDeleteTaskLogTestDB(t)
	statusErr := errors.New("failed status write failed")
	logService := &failingFailedFileTaskLogService{
		Service:   filetasklog.NewService(tDB),
		failedErr: statusErr,
	}
	bindErr := errors.New("bind failed")
	userMountPointTokenService := &mockBatchModifyTokenUserMountPointTokenService{
		bindErrByMountPointID: map[int64]error{202: bindErr},
	}

	handler := NewHandler(
		zap.NewNop(),
		nil,
		nil,
		nil,
		&mockBatchModifyTokenMountPointService{
			mountPoints: map[int64]*models.MountPoint{
				20: {ID: 202, FileId: 20, CreatorUserID: 1, Name: "fail"},
			},
			queryErrByID: map[int64]error{},
		},
		logService,
		nil,
		nil,
		nil,
		userMountPointTokenService,
	)
	req := topic.FileBatchModifyTokenRequest{
		IDs:     []int64{20},
		TokenID: 99,
		UserID:  7,
		IsAdmin: true,
	}

	payload, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}

	processor := taskcontext.NewHandlerFuncWrapper(zap.NewNop()).Wrap(handler.HandleBatchModifyToken())
	err = processor.Process(stdctx.Background(), payload)

	if !errors.Is(err, statusErr) {
		t.Fatalf("expected final failed status error, got %v", err)
	}
}

func TestHandleBatchModifyTokenJoinsBindFilesAndFailedStatusErrors(t *testing.T) {
	tDB := setupBatchDeleteTaskLogTestDB(t)
	statusErr := errors.New("failed status write failed")
	logService := &failingFailedFileTaskLogService{
		Service:   filetasklog.NewService(tDB),
		failedErr: statusErr,
	}
	bindFilesErr := errors.New("get bind files failed")

	handler := NewHandler(
		zap.NewNop(),
		nil,
		nil,
		nil,
		&mockBatchModifyTokenMountPointService{},
		logService,
		nil,
		nil,
		&mockBatchModifyTokenGroup2FileService{
			bindFilesErr: bindFilesErr,
		},
		nil,
	)
	req := topic.FileBatchModifyTokenRequest{
		IDs:         []int64{20},
		TokenID:     99,
		UserID:      7,
		UserGroupID: 5,
		IsAdmin:     false,
	}

	payload, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}

	processor := taskcontext.NewHandlerFuncWrapper(zap.NewNop()).Wrap(handler.HandleBatchModifyToken())
	err = processor.Process(stdctx.Background(), payload)

	if !errors.Is(err, bindFilesErr) {
		t.Fatalf("expected bind files business error, got %v", err)
	}

	if !errors.Is(err, statusErr) {
		t.Fatalf("expected failed status error, got %v", err)
	}
}

func TestHandleBatchModifyTokenReportsMissingUnbindSeparately(t *testing.T) {
	tDB := setupBatchDeleteTaskLogTestDB(t)
	logService := filetasklog.NewService(tDB)
	userMountPointTokenService := &mockBatchModifyTokenUserMountPointTokenService{
		unbindDeletedByMountPoint: map[int64]bool{
			101: true,
			202: false,
		},
	}

	handler := NewHandler(
		zap.NewNop(),
		nil,
		nil,
		nil,
		&mockBatchModifyTokenMountPointService{
			mountPoints: map[int64]*models.MountPoint{
				10: {ID: 101, FileId: 10, CreatorUserID: 1, Name: "bound"},
				20: {ID: 202, FileId: 20, CreatorUserID: 1, Name: "already-unbound"},
			},
			queryErrByID: map[int64]error{},
		},
		logService,
		nil,
		nil,
		nil,
		userMountPointTokenService,
	)
	req := topic.FileBatchModifyTokenRequest{
		IDs:     []int64{10, 20},
		TokenID: 0,
		UserID:  7,
		IsAdmin: true,
	}

	payload, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}

	processor := taskcontext.NewHandlerFuncWrapper(zap.NewNop()).Wrap(handler.HandleBatchModifyToken())
	if err := processor.Process(stdctx.Background(), payload); err != nil {
		t.Fatalf("process batch unbind token: %v", err)
	}

	if len(userMountPointTokenService.unboundMountPointIDs) != 1 ||
		userMountPointTokenService.unboundMountPointIDs[0] != 101 {
		t.Fatalf("expected only existing binding to be unbound, got %v", userMountPointTokenService.unboundMountPointIDs)
	}

	var log models.FileTaskLog
	if err := tDB.db.First(&log).Error; err != nil {
		t.Fatalf("query task log: %v", err)
	}

	if log.Status != models.StatusCompleted {
		t.Fatalf("expected task status completed, got %q", log.Status)
	}

	if log.Total != 2 || log.Completed != 2 || log.Failed != 0 {
		t.Fatalf("expected total=2 completed=2 failed=0, got total=%d completed=%d failed=%d", log.Total, log.Completed, log.Failed)
	}

	if log.Result == "" || !strings.Contains(log.Result, "成功 1 个，已无绑定 1 个") {
		t.Fatalf("expected unchanged unbind count in result, got %q", log.Result)
	}
}

func TestNormalizeFileTaskIDsDeduplicatesIDs(t *testing.T) {
	ids, err := normalizeFileTaskIDs([]int64{3, 3, 2})
	if err != nil {
		t.Fatalf("normalize file task ids: %v", err)
	}

	want := []int64{3, 2}
	if len(ids) != len(want) {
		t.Fatalf("expected %v, got %v", want, ids)
	}

	for i := range want {
		if ids[i] != want[i] {
			t.Fatalf("expected %v, got %v", want, ids)
		}
	}
}
