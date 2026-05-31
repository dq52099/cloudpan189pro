package storage

import (
	stdctx "context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/xxcheng123/cloudpan189-share/internal/consts"
	appContext "github.com/xxcheng123/cloudpan189-share/internal/framework/context"
	"github.com/xxcheng123/cloudpan189-share/internal/framework/httpcontext"
	"github.com/xxcheng123/cloudpan189-share/internal/repository/models"
	"github.com/xxcheng123/cloudpan189-share/internal/services/cloudbridge"
	"github.com/xxcheng123/cloudpan189-share/internal/services/storagefacade"
	"github.com/xxcheng123/cloudpan189-share/internal/types/topic"
	"go.uber.org/zap"
)

type mockBatchAddStorageFacade struct {
	storagefacade.Service
	createID int64
	req      *storagefacade.CreateStorageRequest
}

func (m *mockBatchAddStorageFacade) CreateStorage(
	ctx appContext.Context,
	req *storagefacade.CreateStorageRequest,
) (int64, error) {
	m.req = req

	return m.createID, nil
}

type mockBatchAddCloudBridge struct {
	cloudbridge.Service
}

func (m *mockBatchAddCloudBridge) CheckSubscribeUser(ctx appContext.Context, subscribeUser string) (string, error) {
	return subscribeUser, nil
}

func TestBatchAddRejectsExistingPathOwnedByOtherUser(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mountPointService := &mockBatchDeleteMountPointService{
		mountPointsByPath: map[string]*models.MountPoint{
			"/taken": {FileId: 11, FullPath: "/taken", CreatorUserID: 200},
		},
	}

	router := gin.New()
	router.Use(func(ctx *gin.Context) {
		ctx.Set(consts.CtxKeyUserId, int64(100))
		ctx.Set(consts.CtxKeyIsAdmin, false)
	})

	wrapper := httpcontext.NewHandlerFuncWrapper(zap.NewNop())
	router.POST("/batch_add", wrapper.Wrap(NewHandler(
		nil,
		nil,
		nil,
		nil,
		mountPointService,
		nil,
		nil,
		nil,
		nil,
		nil,
	).BatchAdd()))

	req := httptest.NewRequestWithContext(
		stdctx.Background(),
		http.MethodPost,
		"/batch_add",
		strings.NewReader(`{"items":[{"localPath":"/taken","osType":"subscribe"}]}`),
	)
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected batch result response, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	var response struct {
		Code int              `json:"code"`
		Data batchAddResponse `json:"data"`
	}

	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}

	if response.Data.SuccessCount != 0 || response.Data.FailCount != 1 {
		t.Fatalf("expected failed item only, got %+v", response.Data)
	}

	if len(response.Data.Results) != 1 || response.Data.Results[0].Success {
		t.Fatalf("expected failed result item, got %+v", response.Data.Results)
	}

	if !strings.Contains(response.Data.Results[0].Error, "其他用户") {
		t.Fatalf("expected ownership error, got %q", response.Data.Results[0].Error)
	}
}

func TestBatchAddReportsScanDispatchFailureSeparately(t *testing.T) {
	gin.SetMode(gin.TestMode)

	taskEngine := &mockBatchDeleteTaskEngine{pushErr: errors.New("queue unavailable")}
	mountPointService := &mockBatchDeleteMountPointService{
		mountPoints: map[int64]*models.MountPoint{
			11: {FileId: 11, FullPath: "/subscribed", CreatorUserID: 100},
		},
		mountPointsByPath: map[string]*models.MountPoint{},
	}
	storageFacade := &mockBatchAddStorageFacade{createID: 11}

	router := newBatchAddTestRouter(taskEngine, mountPointService, storageFacade)
	req := httptest.NewRequestWithContext(
		stdctx.Background(),
		http.MethodPost,
		"/batch_add",
		strings.NewReader(`{"items":[{"localPath":"/subscribed","osType":"subscribe","subscribeUser":"up-user"}]}`),
	)
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected ok, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	var response struct {
		Code int              `json:"code"`
		Data batchAddResponse `json:"data"`
	}

	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}

	if response.Data.SuccessCount != 1 || response.Data.FailCount != 0 {
		t.Fatalf("expected mount success only, got %+v", response.Data)
	}

	if response.Data.ScanQueuedCount != 0 || response.Data.ScanFailedCount != 1 {
		t.Fatalf("expected scan dispatch failure only, got %+v", response.Data)
	}

	if len(response.Data.Results) != 1 {
		t.Fatalf("expected one result item, got %+v", response.Data.Results)
	}

	result := response.Data.Results[0]
	if !result.Success {
		t.Fatalf("expected successful mount result, got %+v", result)
	}

	if result.ScanQueued {
		t.Fatalf("expected scanQueued false, got %+v", result)
	}

	if !strings.Contains(result.ScanError, "queue unavailable") {
		t.Fatalf("expected scan error to contain queue unavailable, got %q", result.ScanError)
	}

	if got := mountPointService.lastStateUpdates[11]; !strings.Contains(got, "初始化扫描任务入队失败") ||
		!strings.Contains(got, "queue unavailable") {
		t.Fatalf("expected failed initial scan state, got %q", got)
	}
}

func TestBatchAddReportsScanDispatchSuccess(t *testing.T) {
	gin.SetMode(gin.TestMode)

	taskEngine := &mockBatchDeleteTaskEngine{}
	mountPointService := &mockBatchDeleteMountPointService{
		mountPoints: map[int64]*models.MountPoint{
			11: {FileId: 11, FullPath: "/subscribed", CreatorUserID: 100},
		},
		mountPointsByPath: map[string]*models.MountPoint{},
	}
	storageFacade := &mockBatchAddStorageFacade{createID: 11}

	router := newBatchAddTestRouter(taskEngine, mountPointService, storageFacade)
	req := httptest.NewRequestWithContext(
		stdctx.Background(),
		http.MethodPost,
		"/batch_add",
		strings.NewReader(`{"items":[{"localPath":"/subscribed","osType":"subscribe","subscribeUser":"up-user"}]}`),
	)
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected ok, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	var response struct {
		Code int              `json:"code"`
		Data batchAddResponse `json:"data"`
	}

	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}

	if response.Data.SuccessCount != 1 || response.Data.FailCount != 0 {
		t.Fatalf("expected mount success only, got %+v", response.Data)
	}

	if response.Data.ScanQueuedCount != 1 || response.Data.ScanFailedCount != 0 {
		t.Fatalf("expected scan dispatch success only, got %+v", response.Data)
	}

	if len(response.Data.Results) != 1 {
		t.Fatalf("expected one result item, got %+v", response.Data.Results)
	}

	result := response.Data.Results[0]
	if !result.Success || !result.ScanQueued {
		t.Fatalf("expected successful mount and queued scan, got %+v", result)
	}

	if result.ScanError != "" {
		t.Fatalf("expected empty scan error, got %q", result.ScanError)
	}

	if len(taskEngine.payloads) != 1 {
		t.Fatalf("expected one queued scan task, got %d", len(taskEngine.payloads))
	}

	var taskReq topic.FileScanFileRequest
	if err := json.Unmarshal(taskEngine.payloads[0], &taskReq); err != nil {
		t.Fatal(err)
	}

	if taskReq.FileId != 11 || !taskReq.Deep {
		t.Fatalf("expected deep scan for file 11, got %+v", taskReq)
	}

	if taskReq.ExpectedUserID != 100 || taskReq.TriggeredByAdmin {
		t.Fatalf("expected queued scan user snapshot user=100 admin=false, got %+v", taskReq)
	}
}

func TestBatchAddDeduplicatesInitialScanTasksByFileID(t *testing.T) {
	gin.SetMode(gin.TestMode)

	taskEngine := &mockBatchDeleteTaskEngine{}
	mountPointService := &mockBatchDeleteMountPointService{
		mountPoints: map[int64]*models.MountPoint{
			11: {FileId: 11, FullPath: "/subscribed", CreatorUserID: 100},
		},
		mountPointsByPath: map[string]*models.MountPoint{},
	}
	storageFacade := &mockBatchAddStorageFacade{createID: 11}

	router := newBatchAddTestRouter(taskEngine, mountPointService, storageFacade)
	req := httptest.NewRequestWithContext(
		stdctx.Background(),
		http.MethodPost,
		"/batch_add",
		strings.NewReader(`{"items":[{"localPath":"/subscribed","osType":"subscribe","subscribeUser":"up-user"},{"localPath":"/subscribed","osType":"subscribe","subscribeUser":"up-user"}]}`),
	)
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected ok, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	var response struct {
		Code int              `json:"code"`
		Data batchAddResponse `json:"data"`
	}

	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}

	if response.Data.SuccessCount != 2 || response.Data.FailCount != 0 {
		t.Fatalf("expected two successful mount results, got %+v", response.Data)
	}

	if response.Data.ScanQueuedCount != 2 || response.Data.ScanFailedCount != 0 {
		t.Fatalf("expected both results covered by one queued scan, got %+v", response.Data)
	}

	if len(taskEngine.payloads) != 1 {
		t.Fatalf("expected one queued scan task for duplicate file id, got %d", len(taskEngine.payloads))
	}

	if got, want := mountPointService.queries, []int64{11}; !int64SlicesEqual(got, want) {
		t.Fatalf("expected one mount point query for duplicate file id %v, got %v", want, got)
	}

	for _, result := range response.Data.Results {
		if !result.Success || !result.ScanQueued || result.ID != 11 {
			t.Fatalf("expected duplicate result covered by queued scan, got %+v", result)
		}
	}
}

func newBatchAddTestRouter(
	taskEngine *mockBatchDeleteTaskEngine,
	mountPointService *mockBatchDeleteMountPointService,
	storageFacade *mockBatchAddStorageFacade,
) *gin.Engine {
	router := gin.New()
	router.Use(func(ctx *gin.Context) {
		ctx.Set(consts.CtxKeyUserId, int64(100))
		ctx.Set(consts.CtxKeyIsAdmin, false)
	})

	wrapper := httpcontext.NewHandlerFuncWrapper(zap.NewNop())
	router.POST("/batch_add", wrapper.Wrap(NewHandler(
		taskEngine,
		nil,
		&mockBatchAddCloudBridge{},
		nil,
		mountPointService,
		nil,
		storageFacade,
		nil,
		nil,
		nil,
	).BatchAdd()))

	return router
}
