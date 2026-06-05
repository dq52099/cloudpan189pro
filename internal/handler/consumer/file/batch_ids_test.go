package file

import (
	stdctx "context"
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/xxcheng123/cloudpan189-share/internal/bootstrap"
	"github.com/xxcheng123/cloudpan189-share/internal/consts"
	appContext "github.com/xxcheng123/cloudpan189-share/internal/framework/context"
	"github.com/xxcheng123/cloudpan189-share/internal/framework/taskcontext"
	"github.com/xxcheng123/cloudpan189-share/internal/pkgs/datatypes"
	"github.com/xxcheng123/cloudpan189-share/internal/pkgs/taskengine"
	"github.com/xxcheng123/cloudpan189-share/internal/pkgs/utils"
	"github.com/xxcheng123/cloudpan189-share/internal/repository/models"
	cloudtokenSvi "github.com/xxcheng123/cloudpan189-share/internal/services/cloudtoken"
	"github.com/xxcheng123/cloudpan189-share/internal/services/filetasklog"
	group2fileSvi "github.com/xxcheng123/cloudpan189-share/internal/services/group2file"
	mountPointSvi "github.com/xxcheng123/cloudpan189-share/internal/services/mountpoint"
	userMountPointTokenSvi "github.com/xxcheng123/cloudpan189-share/internal/services/userMountPointToken"
	"github.com/xxcheng123/cloudpan189-share/internal/shared"
	"github.com/xxcheng123/cloudpan189-share/internal/types/topic"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
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
	mountPoints            map[int64]*models.MountPoint
	mountPointsByPrimaryID map[int64]*models.MountPoint
	queryErrByID           map[int64]error
	queryByPrimaryErrByID  map[int64]error
	queriedFileIDs         []int64
	queriedMountPointIDs   []int64
	batchDeleteErrByID     map[int64]error
	batchDeleteRequests    []*mountPointSvi.BatchDeleteRequest
	refreshTimeUpdates     []int64
	lastStateUpdates       []int64
	callLog                *[]string
}

func (m *mockBatchModifyTokenMountPointService) Query(ctx appContext.Context, fileID int64) (*models.MountPoint, error) {
	m.queriedFileIDs = append(m.queriedFileIDs, fileID)

	if err := m.queryErrByID[fileID]; err != nil {
		return nil, err
	}

	mountPoint, ok := m.mountPoints[fileID]
	if !ok {
		return nil, gorm.ErrRecordNotFound
	}

	return mountPoint, nil
}

func (m *mockBatchModifyTokenMountPointService) QueryByID(ctx appContext.Context, id int64) (*models.MountPoint, error) {
	m.queriedMountPointIDs = append(m.queriedMountPointIDs, id)

	if err := m.queryByPrimaryErrByID[id]; err != nil {
		return nil, err
	}

	mountPoint, ok := m.mountPointsByPrimaryID[id]
	if !ok {
		return nil, gorm.ErrRecordNotFound
	}

	return mountPoint, nil
}

func (m *mockBatchModifyTokenMountPointService) BatchDelete(ctx appContext.Context, req *mountPointSvi.BatchDeleteRequest) error {
	appendTestCall(m.callLog, "mount-delete")

	copiedReq := &mountPointSvi.BatchDeleteRequest{
		FileIds:       append([]int64(nil), req.FileIds...),
		CreatorUserID: req.CreatorUserID,
		IsAdmin:       req.IsAdmin,
	}
	m.batchDeleteRequests = append(m.batchDeleteRequests, copiedReq)

	for _, id := range req.FileIds {
		if err := m.batchDeleteErrByID[id]; err != nil {
			return err
		}
	}

	return nil
}

func (m *mockBatchModifyTokenMountPointService) UpdateRefreshTime(ctx appContext.Context, fileID int64) error {
	m.refreshTimeUpdates = append(m.refreshTimeUpdates, fileID)

	return nil
}

func (m *mockBatchModifyTokenMountPointService) UpdateLastState(ctx appContext.Context, fileID int64, state string) error {
	m.lastStateUpdates = append(m.lastStateUpdates, fileID)

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

type mockBatchModifyTokenCloudTokenService struct {
	cloudtokenSvi.Service
	err         error
	queryErr    error
	nilQueryIDs map[int64]bool
	queries     []int64
	queryCalls  []int64
}

func (m *mockBatchModifyTokenCloudTokenService) Query(ctx appContext.Context, id int64) (*models.CloudToken, error) {
	m.queryCalls = append(m.queryCalls, id)

	if m.queryErr != nil {
		return nil, m.queryErr
	}

	if m.nilQueryIDs[id] {
		return nil, nil
	}

	return &models.CloudToken{ID: id, AccessToken: "access-token"}, nil
}

func (m *mockBatchModifyTokenCloudTokenService) QueryAccessible(ctx appContext.Context, id, userID int64, isAdmin bool) (*models.CloudToken, error) {
	m.queries = append(m.queries, id)

	if m.err != nil {
		return nil, m.err
	}

	if m.nilQueryIDs[id] {
		return nil, nil
	}

	return &models.CloudToken{ID: id, UserID: userID}, nil
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

func TestClearFileIgnoresTerminalCompletedStatus(t *testing.T) {
	tDB := setupBatchDeleteTaskLogTestDB(t)
	logService := &failingCompletedFileTaskLogService{
		Service:      filetasklog.NewService(tDB),
		completedErr: filetasklog.ErrFileTaskLogTerminalState,
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
	if err = processor.Process(stdctx.Background(), payload); err != nil {
		t.Fatalf("expected terminal completed status to be ignored, got %v", err)
	}
}

func TestClearFileSkipsMissingMediaFileServiceWhenAutoCleanEnabled(t *testing.T) {
	tDB := setupBatchDeleteTaskLogTestDB(t)
	logService := filetasklog.NewService(tDB)
	virtualFileService := &mockBatchDeleteVirtualFileService{
		filesByID: map[int64]*models.VirtualFile{
			10: {ID: 10, TopId: 10, Name: "dir", IsDir: true},
		},
		childrenByParent: map[int64][]*models.VirtualFile{},
		deleteErrByID:    map[int64]error{},
	}

	oldMediaConfig := shared.GetMediaConfig()

	shared.SetMediaConfig(&models.MediaConfig{
		Enable:      true,
		AutoClean:   true,
		StoragePath: t.TempDir(),
	})

	defer shared.SetMediaConfig(oldMediaConfig)

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
	if err = processor.Process(stdctx.Background(), payload); err != nil {
		t.Fatalf("expected clear to skip missing media file service during auto clean, got %v", err)
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

func TestClearFileReturnsNotFoundWhenFileQueryReturnsNil(t *testing.T) {
	virtualFileService := &mockBatchDeleteVirtualFileService{
		filesByID: map[int64]*models.VirtualFile{
			10: nil,
		},
	}
	handler := NewHandler(
		zap.NewNop(),
		virtualFileService,
		nil,
		nil,
		nil,
		nil,
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
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("expected record not found, got %v", err)
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

func TestScanFileIgnoresTerminalCompletedStatus(t *testing.T) {
	tDB := setupBatchDeleteTaskLogTestDB(t)
	logService := &failingCompletedFileTaskLogService{
		Service:      filetasklog.NewService(tDB),
		completedErr: filetasklog.ErrFileTaskLogTerminalState,
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
	if err = processor.Process(stdctx.Background(), payload); err != nil {
		t.Fatalf("expected terminal completed status to be ignored, got %v", err)
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

func TestScanFileReturnsNotFoundWhenFileQueryReturnsNil(t *testing.T) {
	virtualFileService := &mockBatchDeleteVirtualFileService{
		filesByID: map[int64]*models.VirtualFile{
			10: nil,
		},
	}
	handler := NewHandler(
		zap.NewNop(),
		virtualFileService,
		nil,
		nil,
		nil,
		nil,
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
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("expected record not found, got %v", err)
	}
}

func TestScanFileSkipsWhenMountPointOwnerChanged(t *testing.T) {
	tDB := setupBatchDeleteTaskLogTestDB(t)
	logService := filetasklog.NewService(tDB)
	virtualFileService := &mockBatchDeleteVirtualFileService{
		childrenByParent: map[int64][]*models.VirtualFile{},
		deleteErrByID:    map[int64]error{},
		filesByID: map[int64]*models.VirtualFile{
			10: {ID: 10, TopId: 10, IsTop: true, Name: "top", IsDir: true, OsType: models.OsTypeFolder},
		},
	}
	mountPointService := &mockBatchModifyTokenMountPointService{
		mountPoints: map[int64]*models.MountPoint{
			10: {ID: 1001, FileId: 10, CreatorUserID: 200, Name: "changed-owner"},
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
		mountPointService,
		logService,
		nil,
		nil,
		nil,
		nil,
	)
	req := topic.FileScanFileRequest{
		FileId:         10,
		ExpectedUserID: 100,
	}

	payload, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}

	processor := taskcontext.NewHandlerFuncWrapper(zap.NewNop()).Wrap(handler.ScanFile())
	if err := processor.Process(stdctx.Background(), payload); err != nil {
		t.Fatalf("owner-changed scan should be skipped without retrying queue item: %v", err)
	}

	var logCount int64
	if err := tDB.db.Model(&models.FileTaskLog{}).Count(&logCount).Error; err != nil {
		t.Fatalf("count task logs: %v", err)
	}

	if logCount != 0 {
		t.Fatalf("expected skipped scan not to create task log, got %d", logCount)
	}

	if len(mountPointService.refreshTimeUpdates) != 0 || len(mountPointService.lastStateUpdates) != 0 {
		t.Fatalf("expected skipped scan not to update mount point state, got refresh=%v state=%v", mountPointService.refreshTimeUpdates, mountPointService.lastStateUpdates)
	}
}

func TestScanFileReturnsMountPointOwnerLookupError(t *testing.T) {
	tDB := setupBatchDeleteTaskLogTestDB(t)
	logService := filetasklog.NewService(tDB)
	virtualFileService := &mockBatchDeleteVirtualFileService{
		childrenByParent: map[int64][]*models.VirtualFile{},
		deleteErrByID:    map[int64]error{},
		filesByID: map[int64]*models.VirtualFile{
			10: {ID: 10, TopId: 10, IsTop: true, Name: "top", IsDir: true, OsType: models.OsTypeFolder},
		},
	}
	lookupErr := errors.New("db unavailable")
	mountPointService := &mockBatchModifyTokenMountPointService{
		queryErrByID: map[int64]error{
			10: lookupErr,
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
		mountPointService,
		logService,
		nil,
		nil,
		nil,
		nil,
	)
	req := topic.FileScanFileRequest{
		FileId:         10,
		ExpectedUserID: 100,
	}

	payload, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}

	processor := taskcontext.NewHandlerFuncWrapper(zap.NewNop()).Wrap(handler.ScanFile())
	if err := processor.Process(stdctx.Background(), payload); !errors.Is(err, lookupErr) {
		t.Fatalf("expected owner lookup error %v, got %v", lookupErr, err)
	}

	var logCount int64
	if err := tDB.db.Model(&models.FileTaskLog{}).Count(&logCount).Error; err != nil {
		t.Fatalf("count task logs: %v", err)
	}

	if logCount != 0 {
		t.Fatalf("expected owner lookup failure before task log creation, got %d", logCount)
	}
}

func TestScanFileSkipsWhenMountPointOwnerLookupReturnsNil(t *testing.T) {
	tDB := setupBatchDeleteTaskLogTestDB(t)
	logService := filetasklog.NewService(tDB)
	virtualFileService := &mockBatchDeleteVirtualFileService{
		childrenByParent: map[int64][]*models.VirtualFile{},
		deleteErrByID:    map[int64]error{},
		filesByID: map[int64]*models.VirtualFile{
			10: {ID: 10, TopId: 10, IsTop: true, Name: "top", IsDir: true, OsType: models.OsTypeFolder},
		},
	}
	mountPointService := &mockBatchModifyTokenMountPointService{
		mountPoints: map[int64]*models.MountPoint{
			10: nil,
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
		mountPointService,
		logService,
		nil,
		nil,
		nil,
		nil,
	)
	req := topic.FileScanFileRequest{
		FileId:         10,
		ExpectedUserID: 100,
	}

	payload, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}

	processor := taskcontext.NewHandlerFuncWrapper(zap.NewNop()).Wrap(handler.ScanFile())
	if err := processor.Process(stdctx.Background(), payload); err != nil {
		t.Fatalf("nil owner mount point should be skipped without retrying queue item: %v", err)
	}

	var logCount int64
	if err := tDB.db.Model(&models.FileTaskLog{}).Count(&logCount).Error; err != nil {
		t.Fatalf("count task logs: %v", err)
	}

	if logCount != 0 {
		t.Fatalf("expected skipped scan not to create task log, got %d", logCount)
	}

	if len(mountPointService.refreshTimeUpdates) != 0 || len(mountPointService.lastStateUpdates) != 0 {
		t.Fatalf("expected skipped scan not to update mount point state, got refresh=%v state=%v", mountPointService.refreshTimeUpdates, mountPointService.lastStateUpdates)
	}
}

func TestScanFileReturnsNotFoundWhenPersonFolderMountPointIsNil(t *testing.T) {
	tDB := setupBatchDeleteTaskLogTestDB(t)
	logService := filetasklog.NewService(tDB)
	virtualFileService := &mockBatchDeleteVirtualFileService{
		childrenByParent: map[int64][]*models.VirtualFile{},
		deleteErrByID:    map[int64]error{},
		filesByID: map[int64]*models.VirtualFile{
			10: {ID: 10, TopId: 10, IsTop: true, Name: "person", IsDir: true, OsType: models.OsTypePersonFolder, CloudId: "cloud-folder-id"},
		},
	}
	mountPointService := &mockBatchModifyTokenMountPointService{
		mountPoints: map[int64]*models.MountPoint{
			10: nil,
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
		&mockBatchModifyTokenCloudTokenService{},
		mountPointService,
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
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("expected nil person mount point to return record not found, got %v", err)
	}

	if len(mountPointService.lastStateUpdates) != 1 || mountPointService.lastStateUpdates[0] != 10 {
		t.Fatalf("expected failed scan to update mount point state, got %v", mountPointService.lastStateUpdates)
	}
}

func TestScanFileLogsPersonFolderCloudTokenQueryError(t *testing.T) {
	tDB := setupBatchDeleteTaskLogTestDB(t)
	logService := filetasklog.NewService(tDB)
	virtualFileService := &mockBatchDeleteVirtualFileService{
		childrenByParent: map[int64][]*models.VirtualFile{},
		deleteErrByID:    map[int64]error{},
		filesByID: map[int64]*models.VirtualFile{
			10: {ID: 10, TopId: 10, IsTop: true, Name: "person", IsDir: true, OsType: models.OsTypePersonFolder, CloudId: "cloud-folder-id"},
		},
	}
	mountPointService := &mockBatchModifyTokenMountPointService{
		mountPoints: map[int64]*models.MountPoint{
			10: {ID: 1001, FileId: 10, TokenId: 88, CreatorUserID: 1, Name: "person"},
		},
	}
	queryErr := errors.New(`cloud token query failed: https://proxy-user:proxy-pass@example.test/token?access_token=query-secret&filename=private-name.mkv#token=fragment-secret accessCode=abcd Authorization: Bearer secret-token`)
	cloudTokenService := &mockBatchModifyTokenCloudTokenService{queryErr: queryErr}

	oldMediaConfig := shared.MediaConfig
	shared.MediaConfig = nil

	defer func() {
		shared.MediaConfig = oldMediaConfig
	}()

	core, logs := observer.New(zap.ErrorLevel)
	logger := zap.New(core)
	handler := NewHandler(
		logger,
		virtualFileService,
		nil,
		cloudTokenService,
		mountPointService,
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

	processor := taskcontext.NewHandlerFuncWrapper(logger).Wrap(handler.ScanFile())

	err = processor.Process(stdctx.Background(), payload)
	if err == nil || !strings.Contains(err.Error(), "获取云盘令牌失败") {
		t.Fatalf("expected scan token query failure, got %v", err)
	}

	for _, leaked := range []string{"proxy-user", "proxy-pass", "query-secret", "private-name.mkv", "fragment-secret", "abcd", "secret-token"} {
		if strings.Contains(err.Error(), leaked) {
			t.Fatalf("expected %q to be redacted from returned error %q", leaked, err.Error())
		}
	}

	if !strings.Contains(err.Error(), utils.RedactedSecret) {
		t.Fatalf("expected returned error to include redacted marker, got %q", err.Error())
	}

	if len(cloudTokenService.queryCalls) != 1 || cloudTokenService.queryCalls[0] != 88 {
		t.Fatalf("expected token 88 to be queried, got %v", cloudTokenService.queryCalls)
	}

	entries := logs.FilterMessage("获取云盘令牌失败").All()
	if len(entries) != 1 {
		t.Fatalf("expected one cloud token query failure log, got %d", len(entries))
	}

	logText := entries[0].Message + entries[0].ContextMap()["error"].(string)
	if !strings.Contains(logText, "cloud token query failed") {
		t.Fatalf("expected log to include safe query failure reason, got %s", logText)
	}

	for _, leaked := range []string{"proxy-user", "proxy-pass", "query-secret", "private-name.mkv", "fragment-secret", "abcd", "secret-token"} {
		if strings.Contains(logText, leaked) {
			t.Fatalf("expected %q to be redacted from log %s", leaked, logText)
		}
	}

	if !strings.Contains(logText, utils.RedactedSecret) {
		t.Fatalf("expected log to include redacted marker, got %s", logText)
	}

	var log models.FileTaskLog
	if err := tDB.db.First(&log).Error; err != nil {
		t.Fatalf("query task log: %v", err)
	}

	if !strings.Contains(log.Result, "cloud token query failed") {
		t.Fatalf("expected task log result to keep failure reason, got %q", log.Result)
	}

	for _, leaked := range []string{"proxy-user", "proxy-pass", "query-secret", "private-name.mkv", "fragment-secret", "abcd", "secret-token"} {
		if strings.Contains(log.Result, leaked) {
			t.Fatalf("expected %q to be redacted from task log result %q", leaked, log.Result)
		}
	}

	if !strings.Contains(log.Result, utils.RedactedSecret) {
		t.Fatalf("expected task log result to include redacted marker, got %q", log.Result)
	}
}

func TestScanFileKeepsSafeFamilyFolderCloudTokenQueryFailureReason(t *testing.T) {
	tDB := setupBatchDeleteTaskLogTestDB(t)
	logService := filetasklog.NewService(tDB)
	virtualFileService := &mockBatchDeleteVirtualFileService{
		childrenByParent: map[int64][]*models.VirtualFile{},
		deleteErrByID:    map[int64]error{},
		filesByID: map[int64]*models.VirtualFile{
			20: {
				ID:      20,
				TopId:   20,
				IsTop:   true,
				Name:    "family",
				IsDir:   true,
				OsType:  models.OsTypeFamilyFolder,
				CloudId: "cloud-family-folder-id",
				Addition: datatypes.JSONMap{
					consts.FileAdditionKeyFamilyId: "family-id",
				},
			},
		},
	}
	mountPointService := &mockBatchModifyTokenMountPointService{
		mountPoints: map[int64]*models.MountPoint{
			20: {ID: 1002, FileId: 20, TokenId: 99, CreatorUserID: 1, Name: "family"},
		},
	}
	queryErr := errors.New(`family token query failed: https://proxy-user:proxy-pass@example.test/token?access_token=query-secret#token=fragment-secret accessCode=abcd`)
	cloudTokenService := &mockBatchModifyTokenCloudTokenService{queryErr: queryErr}

	oldMediaConfig := shared.MediaConfig
	shared.MediaConfig = nil

	defer func() {
		shared.MediaConfig = oldMediaConfig
	}()

	handler := NewHandler(
		zap.NewNop(),
		virtualFileService,
		nil,
		cloudTokenService,
		mountPointService,
		logService,
		nil,
		nil,
		nil,
		nil,
	)
	req := topic.FileScanFileRequest{FileId: 20}

	payload, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}

	processor := taskcontext.NewHandlerFuncWrapper(zap.NewNop()).Wrap(handler.ScanFile())

	err = processor.Process(stdctx.Background(), payload)
	if err == nil || !strings.Contains(err.Error(), "family token query failed") {
		t.Fatalf("expected safe family token failure reason, got %v", err)
	}

	if len(cloudTokenService.queryCalls) != 1 || cloudTokenService.queryCalls[0] != 99 {
		t.Fatalf("expected token 99 to be queried, got %v", cloudTokenService.queryCalls)
	}

	for _, leaked := range []string{"proxy-user", "proxy-pass", "query-secret", "fragment-secret", "abcd"} {
		if strings.Contains(err.Error(), leaked) {
			t.Fatalf("expected %q to be redacted from returned error %q", leaked, err.Error())
		}
	}

	if !strings.Contains(err.Error(), utils.RedactedSecret) {
		t.Fatalf("expected returned error to include redacted marker, got %q", err.Error())
	}
}

func TestScanFileReturnsNotFoundWhenFamilyFolderCloudTokenIsNil(t *testing.T) {
	tDB := setupBatchDeleteTaskLogTestDB(t)
	logService := filetasklog.NewService(tDB)
	virtualFileService := &mockBatchDeleteVirtualFileService{
		childrenByParent: map[int64][]*models.VirtualFile{},
		deleteErrByID:    map[int64]error{},
		filesByID: map[int64]*models.VirtualFile{
			20: {
				ID:      20,
				TopId:   20,
				IsTop:   true,
				Name:    "family",
				IsDir:   true,
				OsType:  models.OsTypeFamilyFolder,
				CloudId: "cloud-family-folder-id",
				Addition: datatypes.JSONMap{
					consts.FileAdditionKeyFamilyId: "family-id",
				},
			},
		},
	}
	mountPointService := &mockBatchModifyTokenMountPointService{
		mountPoints: map[int64]*models.MountPoint{
			20: {ID: 1002, FileId: 20, TokenId: 99, CreatorUserID: 1, Name: "family"},
		},
	}
	cloudTokenService := &mockBatchModifyTokenCloudTokenService{
		nilQueryIDs: map[int64]bool{99: true},
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
		cloudTokenService,
		mountPointService,
		logService,
		nil,
		nil,
		nil,
		nil,
	)
	req := topic.FileScanFileRequest{FileId: 20}

	payload, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}

	processor := taskcontext.NewHandlerFuncWrapper(zap.NewNop()).Wrap(handler.ScanFile())

	err = processor.Process(stdctx.Background(), payload)
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("expected nil family cloud token to return record not found, got %v", err)
	}

	if err == nil || !strings.Contains(err.Error(), "获取云盘令牌失败") {
		t.Fatalf("expected nil family cloud token to keep token failure context, got %v", err)
	}

	if len(cloudTokenService.queryCalls) != 1 || cloudTokenService.queryCalls[0] != 99 {
		t.Fatalf("expected token 99 to be queried, got %v", cloudTokenService.queryCalls)
	}
}

func TestHandleBatchDeleteRejectsEmptyIDList(t *testing.T) {
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
	if err := processor.Process(stdctx.Background(), payload); !errors.Is(err, errEmptyFileTaskIDs) {
		t.Fatalf("expected empty batch delete to return %v, got %v", errEmptyFileTaskIDs, err)
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

func TestHandleBatchDeleteSkipsWhenMountPointOwnerChanged(t *testing.T) {
	tDB := setupBatchDeleteTaskLogTestDB(t)
	logService := filetasklog.NewService(tDB)
	virtualFileService := &mockBatchDeleteVirtualFileService{
		filesByID: map[int64]*models.VirtualFile{
			10: {ID: 10, TopId: 10, IsTop: true, Name: "top"},
		},
		childrenByParent: map[int64][]*models.VirtualFile{},
		deleteErrByID:    map[int64]error{},
	}
	mountPointService := &mockBatchModifyTokenMountPointService{
		mountPoints: map[int64]*models.MountPoint{
			10: {ID: 1001, FileId: 10, CreatorUserID: 200, Name: "changed-owner"},
		},
	}

	handler := NewHandler(
		zap.NewNop(),
		virtualFileService,
		nil,
		nil,
		mountPointService,
		logService,
		nil,
		nil,
		nil,
		nil,
	)
	req := topic.FileBatchDeleteRequest{
		IDs:            []int64{10},
		ExpectedUserID: 100,
	}

	payload, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}

	processor := taskcontext.NewHandlerFuncWrapper(zap.NewNop()).Wrap(handler.HandleBatchDelete())
	if err := processor.Process(stdctx.Background(), payload); err != nil {
		t.Fatalf("owner change should be recorded as item failure without retrying whole batch: %v", err)
	}

	if len(mountPointService.batchDeleteRequests) != 0 {
		t.Fatalf("expected no mount point delete after owner changed, got %+v", mountPointService.batchDeleteRequests)
	}

	if len(virtualFileService.deletedIDs) != 0 || len(virtualFileService.batchDeletedIDs) != 0 {
		t.Fatalf("expected no virtual file deletion after owner changed, got deleted=%v batch=%v", virtualFileService.deletedIDs, virtualFileService.batchDeletedIDs)
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

func TestHandleBatchDeleteRecordsFailureWhenOwnerMountPointQueryReturnsNil(t *testing.T) {
	tDB := setupBatchDeleteTaskLogTestDB(t)
	logService := filetasklog.NewService(tDB)
	virtualFileService := &mockBatchDeleteVirtualFileService{
		filesByID: map[int64]*models.VirtualFile{
			10: {ID: 10, TopId: 10, IsTop: true, Name: "top"},
		},
		childrenByParent: map[int64][]*models.VirtualFile{},
		deleteErrByID:    map[int64]error{},
	}
	mountPointService := &mockBatchModifyTokenMountPointService{
		mountPoints: map[int64]*models.MountPoint{
			10: nil,
		},
	}

	handler := NewHandler(
		zap.NewNop(),
		virtualFileService,
		nil,
		nil,
		mountPointService,
		logService,
		nil,
		nil,
		nil,
		nil,
	)
	req := topic.FileBatchDeleteRequest{
		IDs:            []int64{10},
		ExpectedUserID: 100,
	}

	payload, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}

	processor := taskcontext.NewHandlerFuncWrapper(zap.NewNop()).Wrap(handler.HandleBatchDelete())
	if err := processor.Process(stdctx.Background(), payload); err != nil {
		t.Fatalf("nil owner mount point should be recorded as item failure without retrying whole batch: %v", err)
	}

	if len(mountPointService.batchDeleteRequests) != 0 {
		t.Fatalf("expected no mount point delete when owner lookup returns nil, got %+v", mountPointService.batchDeleteRequests)
	}

	if len(virtualFileService.deletedIDs) != 0 || len(virtualFileService.batchDeletedIDs) != 0 {
		t.Fatalf("expected no virtual file deletion when owner lookup returns nil, got deleted=%v batch=%v", virtualFileService.deletedIDs, virtualFileService.batchDeletedIDs)
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

func TestHandleBatchDeleteUsesOwnerScopedMountPointDelete(t *testing.T) {
	tDB := setupBatchDeleteTaskLogTestDB(t)
	logService := filetasklog.NewService(tDB)
	virtualFileService := &mockBatchDeleteVirtualFileService{
		filesByID: map[int64]*models.VirtualFile{
			10: {ID: 10, TopId: 10, IsTop: true, Name: "top"},
		},
		childrenByParent: map[int64][]*models.VirtualFile{},
		deleteErrByID:    map[int64]error{},
	}
	mountPointService := &mockBatchModifyTokenMountPointService{
		mountPoints: map[int64]*models.MountPoint{
			10: {ID: 1001, FileId: 10, CreatorUserID: 100, Name: "owned"},
		},
	}

	handler := NewHandler(
		zap.NewNop(),
		virtualFileService,
		nil,
		nil,
		mountPointService,
		logService,
		nil,
		nil,
		nil,
		nil,
	)
	req := topic.FileBatchDeleteRequest{
		IDs:            []int64{10},
		ExpectedUserID: 100,
	}

	payload, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}

	processor := taskcontext.NewHandlerFuncWrapper(zap.NewNop()).Wrap(handler.HandleBatchDelete())
	if err := processor.Process(stdctx.Background(), payload); err != nil {
		t.Fatalf("process batch delete: %v", err)
	}

	if len(mountPointService.batchDeleteRequests) != 1 {
		t.Fatalf("expected one mount point delete, got %+v", mountPointService.batchDeleteRequests)
	}

	deleteReq := mountPointService.batchDeleteRequests[0]
	if deleteReq.IsAdmin || deleteReq.CreatorUserID != 100 {
		t.Fatalf("expected owner-scoped mount point delete, got %+v", deleteReq)
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

func TestHandleDeleteRejectsZeroFileID(t *testing.T) {
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
	if err := processor.Process(stdctx.Background(), []byte(`{}`)); !errors.Is(err, errInvalidFileTaskID) {
		t.Fatalf("expected zero file id to return %v, got %v", errInvalidFileTaskID, err)
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

func TestHandleDeleteKeepsMountPointWhenTopClearMountFilesFails(t *testing.T) {
	tDB := setupBatchDeleteTaskLogTestDB(t)
	logService := filetasklog.NewService(tDB)
	clearErr := errors.New("clear mount files failed")
	virtualFileService := &mockBatchDeleteVirtualFileService{
		listErr: clearErr,
		filesByID: map[int64]*models.VirtualFile{
			10: {ID: 10, TopId: 10, IsTop: true, Name: "top"},
		},
		childrenByParent: map[int64][]*models.VirtualFile{},
		deleteErrByID:    map[int64]error{},
	}
	mountPointService := &mockBatchModifyTokenMountPointService{}

	handler := NewHandler(
		zap.NewNop(),
		virtualFileService,
		nil,
		nil,
		mountPointService,
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

	if len(mountPointService.batchDeleteRequests) != 0 {
		t.Fatalf("expected mount point kept when virtual file cleanup fails, got %+v", mountPointService.batchDeleteRequests)
	}

	if len(virtualFileService.deletedIDs) != 0 {
		t.Fatalf("expected root virtual file kept when child cleanup fails, got %v", virtualFileService.deletedIDs)
	}
}

func TestHandleDeleteDeletesTopMountPointAfterVirtualFiles(t *testing.T) {
	tDB := setupBatchDeleteTaskLogTestDB(t)
	logService := filetasklog.NewService(tDB)
	callLog := make([]string, 0)
	virtualFileService := &mockBatchDeleteVirtualFileService{
		filesByID: map[int64]*models.VirtualFile{
			10: {ID: 10, TopId: 10, IsTop: true, Name: "top"},
		},
		childrenByParent: map[int64][]*models.VirtualFile{},
		deleteErrByID:    map[int64]error{},
		callLog:          &callLog,
	}
	mountPointService := &mockBatchModifyTokenMountPointService{
		callLog: &callLog,
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
		mountPointService,
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
	if err = processor.Process(stdctx.Background(), payload); err != nil {
		t.Fatalf("process delete: %v", err)
	}

	wantCalls := []string{"virtual-list", "virtual-delete", "mount-delete"}
	if !slices.Equal(callLog, wantCalls) {
		t.Fatalf("expected call order %v, got %v", wantCalls, callLog)
	}

	if !slices.Equal(virtualFileService.deletedIDs, []int64{10}) {
		t.Fatalf("expected root virtual file deleted, got %v", virtualFileService.deletedIDs)
	}

	if len(mountPointService.batchDeleteRequests) != 1 {
		t.Fatalf("expected one mount point delete, got %+v", mountPointService.batchDeleteRequests)
	}
}

func TestHandleDeleteSkipsMissingMediaFileServiceWhenMediaEnabled(t *testing.T) {
	tDB := setupBatchDeleteTaskLogTestDB(t)
	logService := filetasklog.NewService(tDB)
	virtualFileService := &mockBatchDeleteVirtualFileService{
		filesByID: map[int64]*models.VirtualFile{
			10: {ID: 10, TopId: 10, IsTop: true, Name: "top", IsDir: true},
		},
		childrenByParent: map[int64][]*models.VirtualFile{},
		deleteErrByID:    map[int64]error{},
	}
	mountPointService := &mockBatchModifyTokenMountPointService{}

	oldMediaConfig := shared.GetMediaConfig()

	shared.SetMediaConfig(&models.MediaConfig{
		Enable:      true,
		StoragePath: t.TempDir(),
	})

	defer shared.SetMediaConfig(oldMediaConfig)

	handler := NewHandler(
		zap.NewNop(),
		virtualFileService,
		nil,
		nil,
		mountPointService,
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
	if err = processor.Process(stdctx.Background(), payload); err != nil {
		t.Fatalf("expected delete to skip missing media file service during empty dir cleanup, got %v", err)
	}

	if !slices.Equal(virtualFileService.deletedIDs, []int64{10}) {
		t.Fatalf("expected root virtual file deleted, got %v", virtualFileService.deletedIDs)
	}

	if len(mountPointService.batchDeleteRequests) != 1 {
		t.Fatalf("expected one mount point delete, got %+v", mountPointService.batchDeleteRequests)
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
	mountPointService := &mockBatchModifyTokenMountPointService{}

	handler := NewHandler(
		zap.NewNop(),
		virtualFileService,
		nil,
		nil,
		mountPointService,
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

	if len(mountPointService.batchDeleteRequests) != 0 {
		t.Fatalf("expected mount point kept when residual file cleanup fails, got %+v", mountPointService.batchDeleteRequests)
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

func TestHandleBatchModifyTokenRejectsEmptyIDList(t *testing.T) {
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
	if err := processor.Process(stdctx.Background(), payload); !errors.Is(err, errEmptyFileTaskIDs) {
		t.Fatalf("expected empty batch modify token to return %v, got %v", errEmptyFileTaskIDs, err)
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
		&mockBatchModifyTokenCloudTokenService{},
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

func TestHandleBatchModifyTokenUsesMountPointIDsWhenPresent(t *testing.T) {
	tDB := setupBatchDeleteTaskLogTestDB(t)
	logService := filetasklog.NewService(tDB)
	userMountPointTokenService := &mockBatchModifyTokenUserMountPointTokenService{}
	mountPointService := &mockBatchModifyTokenMountPointService{
		mountPoints: map[int64]*models.MountPoint{
			10: {ID: 101, FileId: 10, CreatorUserID: 1, Name: "first"},
		},
		mountPointsByPrimaryID: map[int64]*models.MountPoint{
			202: {ID: 202, FileId: 10, CreatorUserID: 1, Name: "selected"},
		},
	}

	handler := NewHandler(
		zap.NewNop(),
		nil,
		nil,
		&mockBatchModifyTokenCloudTokenService{},
		mountPointService,
		logService,
		nil,
		nil,
		nil,
		userMountPointTokenService,
	)
	req := topic.FileBatchModifyTokenRequest{
		IDs:           []int64{10},
		MountPointIDs: []int64{202},
		TokenID:       99,
		UserID:        7,
		IsAdmin:       true,
	}

	payload, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}

	processor := taskcontext.NewHandlerFuncWrapper(zap.NewNop()).Wrap(handler.HandleBatchModifyToken())
	if err := processor.Process(stdctx.Background(), payload); err != nil {
		t.Fatalf("process batch modify token: %v", err)
	}

	if got, want := mountPointService.queriedMountPointIDs, []int64{202}; !slices.Equal(got, want) {
		t.Fatalf("expected primary-key queries %v, got %v", want, got)
	}

	if len(mountPointService.queriedFileIDs) != 0 {
		t.Fatalf("expected no file-id fallback queries, got %v", mountPointService.queriedFileIDs)
	}

	if got, want := userMountPointTokenService.boundMountPointIDs, []int64{202}; !slices.Equal(got, want) {
		t.Fatalf("expected selected mount point to be bound %v, got %v", want, got)
	}

	var log models.FileTaskLog
	if err := tDB.db.First(&log).Error; err != nil {
		t.Fatalf("query task log: %v", err)
	}

	if log.FileId != 10 {
		t.Fatalf("expected task log to keep file id 10, got %d", log.FileId)
	}
}

func TestHandleBatchModifyTokenKeepsLegacyFileIDPayloadCompatible(t *testing.T) {
	tDB := setupBatchDeleteTaskLogTestDB(t)
	logService := filetasklog.NewService(tDB)
	userMountPointTokenService := &mockBatchModifyTokenUserMountPointTokenService{}
	mountPointService := &mockBatchModifyTokenMountPointService{
		mountPoints: map[int64]*models.MountPoint{
			10: {ID: 101, FileId: 10, CreatorUserID: 1, Name: "legacy"},
		},
		mountPointsByPrimaryID: map[int64]*models.MountPoint{
			10: {ID: 999, FileId: 10, CreatorUserID: 1, Name: "wrong-mode"},
		},
	}

	handler := NewHandler(
		zap.NewNop(),
		nil,
		nil,
		&mockBatchModifyTokenCloudTokenService{},
		mountPointService,
		logService,
		nil,
		nil,
		nil,
		userMountPointTokenService,
	)
	req := topic.FileBatchModifyTokenRequest{
		IDs:     []int64{10},
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
		t.Fatalf("process legacy batch modify token: %v", err)
	}

	if got, want := mountPointService.queriedFileIDs, []int64{10}; !slices.Equal(got, want) {
		t.Fatalf("expected legacy file-id queries %v, got %v", want, got)
	}

	if len(mountPointService.queriedMountPointIDs) != 0 {
		t.Fatalf("expected no primary-key queries for legacy payload, got %v", mountPointService.queriedMountPointIDs)
	}

	if got, want := userMountPointTokenService.boundMountPointIDs, []int64{101}; !slices.Equal(got, want) {
		t.Fatalf("expected legacy mount point to be bound %v, got %v", want, got)
	}
}

func TestHandleBatchModifyTokenRejectsInaccessibleTokenBeforeBinding(t *testing.T) {
	tDB := setupBatchDeleteTaskLogTestDB(t)
	logService := filetasklog.NewService(tDB)
	tokenErr := errors.New("token not accessible")
	cloudTokenService := &mockBatchModifyTokenCloudTokenService{err: tokenErr}
	userMountPointTokenService := &mockBatchModifyTokenUserMountPointTokenService{}

	handler := NewHandler(
		zap.NewNop(),
		nil,
		nil,
		cloudTokenService,
		&mockBatchModifyTokenMountPointService{
			mountPoints: map[int64]*models.MountPoint{
				10: {ID: 101, FileId: 10, CreatorUserID: 7, Name: "owned"},
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
		IDs:     []int64{10},
		TokenID: 99,
		UserID:  7,
		IsAdmin: false,
	}

	payload, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}

	processor := taskcontext.NewHandlerFuncWrapper(zap.NewNop()).Wrap(handler.HandleBatchModifyToken())
	if err := processor.Process(stdctx.Background(), payload); err != nil {
		t.Fatalf("inaccessible token should fail task status without retrying whole queue item, got %v", err)
	}

	if got, want := cloudTokenService.queries, []int64{99}; !slices.Equal(got, want) {
		t.Fatalf("expected token access query %v, got %v", want, got)
	}

	if len(userMountPointTokenService.boundMountPointIDs) != 0 {
		t.Fatalf("expected no binding when token is inaccessible, got %v", userMountPointTokenService.boundMountPointIDs)
	}

	var log models.FileTaskLog
	if err := tDB.db.First(&log).Error; err != nil {
		t.Fatalf("query task log: %v", err)
	}

	if log.Status != models.StatusFailed {
		t.Fatalf("expected task status failed, got %q", log.Status)
	}

	if !strings.Contains(log.Result, "令牌不可用") {
		t.Fatalf("expected token unavailable result, got %q", log.Result)
	}
}

func TestHandleBatchModifyTokenRejectsNilAccessibleTokenBeforeBinding(t *testing.T) {
	tDB := setupBatchDeleteTaskLogTestDB(t)
	logService := filetasklog.NewService(tDB)
	cloudTokenService := &mockBatchModifyTokenCloudTokenService{
		nilQueryIDs: map[int64]bool{99: true},
	}
	userMountPointTokenService := &mockBatchModifyTokenUserMountPointTokenService{}

	handler := NewHandler(
		zap.NewNop(),
		nil,
		nil,
		cloudTokenService,
		&mockBatchModifyTokenMountPointService{
			mountPoints: map[int64]*models.MountPoint{
				10: {ID: 101, FileId: 10, CreatorUserID: 7, Name: "owned"},
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
		IDs:     []int64{10},
		TokenID: 99,
		UserID:  7,
		IsAdmin: false,
	}

	payload, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}

	processor := taskcontext.NewHandlerFuncWrapper(zap.NewNop()).Wrap(handler.HandleBatchModifyToken())
	if err := processor.Process(stdctx.Background(), payload); err != nil {
		t.Fatalf("nil accessible token should fail task status without retrying whole queue item, got %v", err)
	}

	if got, want := cloudTokenService.queries, []int64{99}; !slices.Equal(got, want) {
		t.Fatalf("expected token access query %v, got %v", want, got)
	}

	if len(userMountPointTokenService.boundMountPointIDs) != 0 {
		t.Fatalf("expected no binding when token query returns nil, got %v", userMountPointTokenService.boundMountPointIDs)
	}

	var log models.FileTaskLog
	if err := tDB.db.First(&log).Error; err != nil {
		t.Fatalf("query task log: %v", err)
	}

	if log.Status != models.StatusFailed {
		t.Fatalf("expected task status failed, got %q", log.Status)
	}

	if !strings.Contains(log.Result, "令牌不可用") {
		t.Fatalf("expected token unavailable result, got %q", log.Result)
	}
}

func TestHandleBatchModifyTokenReturnsMissingCloudTokenServiceWhenTypedNil(t *testing.T) {
	tDB := setupBatchDeleteTaskLogTestDB(t)
	logService := filetasklog.NewService(tDB)

	var cloudTokenService *mockBatchModifyTokenCloudTokenService

	handler := NewHandler(
		zap.NewNop(),
		nil,
		nil,
		cloudTokenService,
		&mockBatchModifyTokenMountPointService{},
		logService,
		nil,
		nil,
		nil,
		&mockBatchModifyTokenUserMountPointTokenService{},
	)
	req := topic.FileBatchModifyTokenRequest{
		IDs:     []int64{10},
		TokenID: 99,
		UserID:  7,
		IsAdmin: false,
	}

	payload, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}

	processor := taskcontext.NewHandlerFuncWrapper(zap.NewNop()).Wrap(handler.HandleBatchModifyToken())

	err = processor.Process(stdctx.Background(), payload)
	if err == nil || !strings.Contains(err.Error(), "云盘令牌服务未初始化") {
		t.Fatalf("expected missing cloud token service error, got %v", err)
	}
}

func TestHandleBatchModifyTokenSkipsTypedNilUserTokenAccessCheck(t *testing.T) {
	tDB := setupBatchDeleteTaskLogTestDB(t)
	logService := filetasklog.NewService(tDB)

	var userMountPointTokenService *mockBatchModifyTokenUserMountPointTokenService

	handler := NewHandler(
		zap.NewNop(),
		nil,
		nil,
		&mockBatchModifyTokenCloudTokenService{},
		&mockBatchModifyTokenMountPointService{
			mountPoints: map[int64]*models.MountPoint{
				10: {ID: 101, FileId: 10, CreatorUserID: 8, Name: "not-owned"},
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
		IDs:     []int64{10},
		TokenID: 0,
		UserID:  7,
		IsAdmin: false,
	}

	payload, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}

	processor := taskcontext.NewHandlerFuncWrapper(zap.NewNop()).Wrap(handler.HandleBatchModifyToken())
	if err := processor.Process(stdctx.Background(), payload); err != nil {
		t.Fatalf("typed nil user token service should skip optional access check without retrying, got %v", err)
	}

	var log models.FileTaskLog
	if err := tDB.db.First(&log).Error; err != nil {
		t.Fatalf("query task log: %v", err)
	}

	if log.Status != models.StatusFailed {
		t.Fatalf("expected task status failed, got %q", log.Status)
	}

	if !strings.Contains(log.Result, "失败 1 个") {
		t.Fatalf("expected one failed item in result, got %q", log.Result)
	}
}

func TestHandleBatchModifyTokenRecordsFailureWhenMountPointQueryReturnsNil(t *testing.T) {
	tDB := setupBatchDeleteTaskLogTestDB(t)
	logService := filetasklog.NewService(tDB)
	userMountPointTokenService := &mockBatchModifyTokenUserMountPointTokenService{}

	handler := NewHandler(
		zap.NewNop(),
		nil,
		nil,
		&mockBatchModifyTokenCloudTokenService{},
		&mockBatchModifyTokenMountPointService{
			mountPoints: map[int64]*models.MountPoint{
				10: nil,
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
		IDs:     []int64{10},
		TokenID: 0,
		UserID:  7,
		IsAdmin: false,
	}

	payload, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}

	processor := taskcontext.NewHandlerFuncWrapper(zap.NewNop()).Wrap(handler.HandleBatchModifyToken())
	if err := processor.Process(stdctx.Background(), payload); err != nil {
		t.Fatalf("nil mount point should be recorded as item failure without retrying whole queue item, got %v", err)
	}

	if len(userMountPointTokenService.boundMountPointIDs) != 0 || len(userMountPointTokenService.unboundMountPointIDs) != 0 {
		t.Fatalf("expected no token binding changes, bound=%v unbound=%v",
			userMountPointTokenService.boundMountPointIDs,
			userMountPointTokenService.unboundMountPointIDs)
	}

	var log models.FileTaskLog
	if err := tDB.db.First(&log).Error; err != nil {
		t.Fatalf("query task log: %v", err)
	}

	if log.Status != models.StatusFailed {
		t.Fatalf("expected task status failed, got %q", log.Status)
	}

	if !strings.Contains(log.Result, "失败 1 个") {
		t.Fatalf("expected one failed item in result, got %q", log.Result)
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
		&mockBatchModifyTokenCloudTokenService{},
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
		&mockBatchModifyTokenCloudTokenService{},
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
