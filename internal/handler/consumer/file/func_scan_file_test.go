package file

import (
	stdctx "context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/xxcheng123/cloudpan189-share/internal/consts"
	appContext "github.com/xxcheng123/cloudpan189-share/internal/framework/context"
	"github.com/xxcheng123/cloudpan189-share/internal/framework/taskcontext"
	"github.com/xxcheng123/cloudpan189-share/internal/pkgs/datatypes"
	"github.com/xxcheng123/cloudpan189-share/internal/repository/models"
	cloudbridgeSvi "github.com/xxcheng123/cloudpan189-share/internal/services/cloudbridge"
	"github.com/xxcheng123/cloudpan189-share/internal/services/filetasklog"
	"github.com/xxcheng123/cloudpan189-share/internal/shared"
	"github.com/xxcheng123/cloudpan189-share/internal/types/converter"
	"github.com/xxcheng123/cloudpan189-share/internal/types/topic"
	"go.uber.org/zap"
)

type fetchSubscribeShareCall struct {
	upUserID   string
	shareID    int64
	fileID     string
	isFolder   bool
	shareMode  int
	accessCode string
}

type fetchShareCall struct {
	shareID    int64
	fileID     string
	shareMode  int
	accessCode string
	isFolder   bool
}

type mockFetchSubscribeShareCloudBridgeService struct {
	cloudbridgeSvi.Service
	subscribeCalls []fetchSubscribeShareCall
	shareCalls     []fetchShareCall
}

func (m *mockFetchSubscribeShareCloudBridgeService) GetSubscribeShareFiles(
	ctx appContext.Context,
	upUserID string,
	shareID int64,
	fileID string,
	isFolder bool,
	shareMode int,
	accessCode string,
) ([]converter.VirtualFileConverter, error) {
	m.subscribeCalls = append(m.subscribeCalls, fetchSubscribeShareCall{
		upUserID:   upUserID,
		shareID:    shareID,
		fileID:     fileID,
		isFolder:   isFolder,
		shareMode:  shareMode,
		accessCode: accessCode,
	})

	return nil, nil
}

func (m *mockFetchSubscribeShareCloudBridgeService) GetShareFiles(
	ctx appContext.Context,
	shareID int64,
	fileID string,
	shareMode int,
	accessCode string,
	isFolder bool,
) ([]converter.VirtualFileConverter, error) {
	m.shareCalls = append(m.shareCalls, fetchShareCall{
		shareID:    shareID,
		fileID:     fileID,
		shareMode:  shareMode,
		accessCode: accessCode,
		isFolder:   isFolder,
	})

	return nil, nil
}

func processScanFileForDependencyTest(t *testing.T, handler Handler, req topic.FileScanFileRequest) error {
	t.Helper()

	payload, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}

	processor := taskcontext.NewHandlerFuncWrapper(zap.NewNop()).Wrap(handler.ScanFile())

	return processor.Process(stdctx.Background(), payload)
}

func assertScanDependencyError(t *testing.T, err error, want error) {
	t.Helper()

	if !errors.Is(err, want) {
		t.Fatalf("expected %v, got %v", want, err)
	}

	if strings.Contains(err.Error(), "task processor panic") {
		t.Fatalf("expected direct dependency error, got panic recovery error %v", err)
	}
}

func TestScanFileReturnsMissingVirtualFileServiceWhenTypedNil(t *testing.T) {
	var virtualFileService *mockBatchDeleteVirtualFileService

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

	err := processScanFileForDependencyTest(t, handler, topic.FileScanFileRequest{FileId: 0})

	assertScanDependencyError(t, err, errVirtualFileServiceNotInitialized)
}

func TestScanFileReturnsMissingFileTaskLogServiceBeforeCreatingTracker(t *testing.T) {
	virtualFileService := &mockBatchDeleteVirtualFileService{
		childrenByParent: map[int64][]*models.VirtualFile{},
		deleteErrByID:    map[int64]error{},
		filesByID:        map[int64]*models.VirtualFile{},
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

	err := processScanFileForDependencyTest(t, handler, topic.FileScanFileRequest{FileId: 0})

	assertScanDependencyError(t, err, errFileTaskLogServiceNotInitialized)
}

func TestScanFileReturnsMissingCloudBridgeServiceWhenScanningShareFolder(t *testing.T) {
	tDB := setupBatchDeleteTaskLogTestDB(t)
	logService := filetasklog.NewService(tDB)
	virtualFileService := &mockBatchDeleteVirtualFileService{
		childrenByParent: map[int64][]*models.VirtualFile{},
		deleteErrByID:    map[int64]error{},
		filesByID: map[int64]*models.VirtualFile{
			10: {
				ID:     10,
				TopId:  10,
				IsTop:  true,
				Name:   "share",
				IsDir:  true,
				OsType: models.OsTypeShareFolder,
				Addition: datatypes.JSONMap{
					consts.FileAdditionKeyShareId:   int64(12345),
					consts.FileAdditionKeyShareMode: 1,
					consts.FileAdditionKeyIsFolder:  true,
				},
			},
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

	err := processScanFileForDependencyTest(t, handler, topic.FileScanFileRequest{FileId: 10})

	assertScanDependencyError(t, err, errCloudBridgeServiceNotInitialized)
}

func TestScanFileReturnsMissingCloudTokenServiceWhenScanningPersonFolder(t *testing.T) {
	tDB := setupBatchDeleteTaskLogTestDB(t)
	logService := filetasklog.NewService(tDB)
	virtualFileService := &mockBatchDeleteVirtualFileService{
		childrenByParent: map[int64][]*models.VirtualFile{},
		deleteErrByID:    map[int64]error{},
		filesByID: map[int64]*models.VirtualFile{
			10: {
				ID:      10,
				TopId:   10,
				IsTop:   true,
				Name:    "person",
				IsDir:   true,
				OsType:  models.OsTypePersonFolder,
				CloudId: "cloud-file-id",
			},
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
		&mockBatchModifyTokenMountPointService{
			mountPoints: map[int64]*models.MountPoint{
				10: {ID: 101, FileId: 10, TokenId: 99, Name: "person"},
			},
		},
		logService,
		nil,
		nil,
		nil,
		nil,
	)

	err := processScanFileForDependencyTest(t, handler, topic.FileScanFileRequest{FileId: 10})

	assertScanDependencyError(t, err, errCloudTokenServiceNotInitialized)
}

func TestScanFileSkipsMissingMediaFileServiceWhenAutoCleanEnabled(t *testing.T) {
	tDB := setupBatchDeleteTaskLogTestDB(t)
	logService := filetasklog.NewService(tDB)
	virtualFileService := &mockBatchDeleteVirtualFileService{
		childrenByParent: map[int64][]*models.VirtualFile{},
		deleteErrByID:    map[int64]error{},
		filesByID:        map[int64]*models.VirtualFile{},
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
		nil,
		logService,
		nil,
		nil,
		nil,
		nil,
	)

	err := processScanFileForDependencyTest(t, handler, topic.FileScanFileRequest{FileId: 0})
	if err != nil {
		t.Fatalf("expected scan to skip missing media file service during auto clean, got %v", err)
	}
}

func TestFetchSubscribeShareFileConvertersUsesSubscribeShareWhenUpUserIDExists(t *testing.T) {
	cloudBridgeService := &mockFetchSubscribeShareCloudBridgeService{}
	h := &handler{
		logger:             zap.NewNop(),
		cloudBridgeService: cloudBridgeService,
	}
	inputFile := &models.VirtualFile{
		ID:      10,
		CloudId: "cloud-file-id",
		Addition: datatypes.JSONMap{
			consts.FileAdditionKeyShareId:    int64(12345),
			consts.FileAdditionKeyIsFolder:   true,
			consts.FileAdditionKeyUpUserId:   "up-user-id",
			consts.FileAdditionKeyShareMode:  2,
			consts.FileAdditionKeyAccessCode: "code",
		},
	}

	_, err := h.fetchSubscribeShareFileConverters(appContext.NewContext(stdctx.Background()), inputFile)
	if err != nil {
		t.Fatalf("fetch subscribe share file converters: %v", err)
	}

	if len(cloudBridgeService.shareCalls) != 0 {
		t.Fatalf("expected no GetShareFiles calls, got %d", len(cloudBridgeService.shareCalls))
	}

	if len(cloudBridgeService.subscribeCalls) != 1 {
		t.Fatalf("expected one GetSubscribeShareFiles call, got %d", len(cloudBridgeService.subscribeCalls))
	}

	got := cloudBridgeService.subscribeCalls[0]

	want := fetchSubscribeShareCall{
		upUserID:   "up-user-id",
		shareID:    int64(12345),
		fileID:     "cloud-file-id",
		isFolder:   true,
		shareMode:  2,
		accessCode: "code",
	}

	if got != want {
		t.Fatalf("expected subscribe call %+v, got %+v", want, got)
	}
}

func TestFetchSubscribeShareFileConvertersFallsBackToShareMetadataWithoutUpUserID(t *testing.T) {
	cloudBridgeService := &mockFetchSubscribeShareCloudBridgeService{}
	h := &handler{
		logger:             zap.NewNop(),
		cloudBridgeService: cloudBridgeService,
	}
	inputFile := &models.VirtualFile{
		ID:      20,
		CloudId: "share-file-id",
		Addition: datatypes.JSONMap{
			consts.FileAdditionKeyShareId:    int64(67890),
			consts.FileAdditionKeyIsFolder:   false,
			consts.FileAdditionKeyShareMode:  3,
			consts.FileAdditionKeyAccessCode: "abcd",
		},
	}

	_, err := h.fetchSubscribeShareFileConverters(appContext.NewContext(stdctx.Background()), inputFile)
	if err != nil {
		t.Fatalf("fetch subscribe share file converters: %v", err)
	}

	if len(cloudBridgeService.subscribeCalls) != 0 {
		t.Fatalf("expected no GetSubscribeShareFiles calls, got %d", len(cloudBridgeService.subscribeCalls))
	}

	if len(cloudBridgeService.shareCalls) != 1 {
		t.Fatalf("expected one GetShareFiles call, got %d", len(cloudBridgeService.shareCalls))
	}

	got := cloudBridgeService.shareCalls[0]

	want := fetchShareCall{
		shareID:    int64(67890),
		fileID:     "share-file-id",
		shareMode:  3,
		accessCode: "abcd",
		isFolder:   false,
	}

	if got != want {
		t.Fatalf("expected share call %+v, got %+v", want, got)
	}
}
