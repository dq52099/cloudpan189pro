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
	subscribeUsers       []string
	subscribeShareUsers  []string
	subscribeShareCodes  []string
	subscribeAccessCodes []string
	shareCodes           []string
	shareAccessCodes     []string
	personFileIDs        []string
	familyIDs            []string
	familyFileIDs        []string
	checkShareResult     *cloudbridge.CheckShareResult
	checkShareErr        error
}

func (m *mockBatchAddCloudBridge) CheckSubscribeUser(ctx appContext.Context, subscribeUser string) (string, error) {
	m.subscribeUsers = append(m.subscribeUsers, subscribeUser)

	return subscribeUser, nil
}

func (m *mockBatchAddCloudBridge) CheckSubscribeShare(ctx appContext.Context, subscribeUser, shareCode, accessCode string) (shareId int64, isFolder bool, fileId string, shareMode int, resolvedAccessCode string, err error) {
	m.subscribeShareUsers = append(m.subscribeShareUsers, subscribeUser)
	m.subscribeShareCodes = append(m.subscribeShareCodes, shareCode)
	m.subscribeAccessCodes = append(m.subscribeAccessCodes, accessCode)

	return 12345, true, "cloud-file-id", 2, accessCode, nil
}

func (m *mockBatchAddCloudBridge) CheckShare(ctx appContext.Context, shareCode string, accessCode string) (*cloudbridge.CheckShareResult, error) {
	m.shareCodes = append(m.shareCodes, shareCode)
	m.shareAccessCodes = append(m.shareAccessCodes, accessCode)

	if m.checkShareErr != nil {
		return nil, m.checkShareErr
	}

	if m.checkShareResult != nil {
		return m.checkShareResult, nil
	}

	return &cloudbridge.CheckShareResult{
		ShareId:    67890,
		IsFolder:   true,
		AccessCode: accessCode,
		ShareMode:  5,
		FileId:     "share-file-id",
	}, nil
}

func (m *mockBatchAddCloudBridge) CheckPerson(ctx appContext.Context, token cloudbridge.AuthToken, fileId string) (string, error) {
	m.personFileIDs = append(m.personFileIDs, fileId)

	return "person-folder", nil
}

func (m *mockBatchAddCloudBridge) CheckFamily(ctx appContext.Context, token cloudbridge.AuthToken, familyId, fileId string) error {
	m.familyIDs = append(m.familyIDs, familyId)
	m.familyFileIDs = append(m.familyFileIDs, fileId)

	return nil
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

func TestBatchAddRejectsInvalidPathBeforeRemoteValidation(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cloudBridge := &mockBatchAddCloudBridge{}
	taskEngine := &mockBatchDeleteTaskEngine{}
	mountPointService := &mockBatchDeleteMountPointService{
		mountPointsByPath: map[string]*models.MountPoint{},
	}
	storageFacade := &mockBatchAddStorageFacade{createID: 11}

	router := newBatchAddTestRouterWithCloudBridge(taskEngine, mountPointService, storageFacade, cloudBridge)
	req := httptest.NewRequestWithContext(
		stdctx.Background(),
		http.MethodPost,
		"/batch_add",
		strings.NewReader(`{"items":[{"localPath":"invalid","osType":"subscribe","subscribeUser":"up-user"}]}`),
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

	if !strings.Contains(response.Data.Results[0].Error, "路径不合法") {
		t.Fatalf("expected invalid path error, got %q", response.Data.Results[0].Error)
	}

	if len(cloudBridge.subscribeUsers) != 0 {
		t.Fatalf("expected remote subscribe validation to be skipped, got %v", cloudBridge.subscribeUsers)
	}

	if storageFacade.req != nil {
		t.Fatalf("expected storage facade to be skipped, got %+v", storageFacade.req)
	}
}

func TestBatchAddReportsStorageFacadeTypedNilAsItemFailure(t *testing.T) {
	gin.SetMode(gin.TestMode)

	var storageFacade *mockBatchAddStorageFacade

	taskEngine := &mockBatchDeleteTaskEngine{}
	mountPointService := &mockBatchDeleteMountPointService{
		mountPointsByPath: map[string]*models.MountPoint{},
	}

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
		t.Fatalf("expected one failed item, got %+v", response.Data)
	}

	if len(response.Data.Results) != 1 || response.Data.Results[0].Success {
		t.Fatalf("expected failed result item, got %+v", response.Data.Results)
	}

	if !strings.Contains(response.Data.Results[0].Error, "存储组合服务未初始化") {
		t.Fatalf("expected storage facade error, got %q", response.Data.Results[0].Error)
	}

	if len(taskEngine.payloads) != 0 {
		t.Fatalf("expected no scan task, got %d", len(taskEngine.payloads))
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

func TestBatchAddReportsMissingTaskEngineAsScanDispatchFailure(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mountPointService := &mockBatchDeleteMountPointService{
		mountPoints: map[int64]*models.MountPoint{
			11: {FileId: 11, FullPath: "/subscribed", CreatorUserID: 100},
		},
		mountPointsByPath: map[string]*models.MountPoint{},
	}
	storageFacade := &mockBatchAddStorageFacade{createID: 11}

	router := gin.New()
	router.Use(func(ctx *gin.Context) {
		ctx.Set(consts.CtxKeyUserId, int64(100))
		ctx.Set(consts.CtxKeyIsAdmin, false)
	})

	wrapper := httpcontext.NewHandlerFuncWrapper(zap.NewNop())
	router.POST("/batch_add", wrapper.Wrap(NewHandler(
		nil,
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
		Data batchAddResponse `json:"data"`
	}

	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}

	if response.Data.SuccessCount != 1 || response.Data.FailCount != 0 ||
		response.Data.ScanQueuedCount != 0 || response.Data.ScanFailedCount != 1 {
		t.Fatalf("unexpected response data: %+v", response.Data)
	}

	result := response.Data.Results[0]
	if !result.Success || result.ScanQueued || !strings.Contains(result.ScanError, "任务引擎未初始化") {
		t.Fatalf("expected successful mount with task engine scan error, got %+v", result)
	}

	if got := mountPointService.lastStateUpdates[11]; !strings.Contains(got, "初始化扫描任务入队失败") ||
		!strings.Contains(got, "任务引擎未初始化") {
		t.Fatalf("expected failed initial scan state, got %q", got)
	}
}

func TestBatchAddReportsNilMountPointScanPreparationFailure(t *testing.T) {
	gin.SetMode(gin.TestMode)

	taskEngine := &mockBatchDeleteTaskEngine{}
	mountPointService := &mockBatchDeleteMountPointService{
		mountPoints: map[int64]*models.MountPoint{
			11: nil,
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
		t.Fatalf("expected scan preparation failure only, got %+v", response.Data)
	}

	if len(response.Data.Results) != 1 {
		t.Fatalf("expected one result item, got %+v", response.Data.Results)
	}

	result := response.Data.Results[0]
	if !result.Success || result.ScanQueued {
		t.Fatalf("expected successful mount result without queued scan, got %+v", result)
	}

	if !strings.Contains(result.ScanError, "挂载点不存在") {
		t.Fatalf("expected scan error to contain missing mount point, got %q", result.ScanError)
	}

	if len(taskEngine.payloads) != 0 {
		t.Fatalf("expected nil mount point not to queue scan task, got %d", len(taskEngine.payloads))
	}

	if got := mountPointService.lastStateUpdates[11]; !strings.Contains(got, "初始化扫描任务准备失败") ||
		!strings.Contains(got, "挂载点不存在") {
		t.Fatalf("expected failed initial scan state, got %q", got)
	}
}

func TestBatchAddRedactsScanDispatchFailure(t *testing.T) {
	gin.SetMode(gin.TestMode)

	rawErr := `queue failed: https://proxy-user:proxy-pass@example.test/scan?access_token=query-secret&filename=private-name.mkv#token=fragment-secret accessCode=abcd Authorization: Bearer secret-token`
	taskEngine := &mockBatchDeleteTaskEngine{pushErr: errors.New(rawErr)}
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

	if len(response.Data.Results) != 1 {
		t.Fatalf("expected one result item, got %+v", response.Data.Results)
	}

	assertStorageVisibleTextRedacted(t, response.Data.Results[0].ScanError, "proxy-user", "proxy-pass", "query-secret", "private-name.mkv", "fragment-secret", "abcd", "secret-token")
	assertStorageVisibleTextRedacted(t, mountPointService.lastStateUpdates[11], "proxy-user", "proxy-pass", "query-secret", "private-name.mkv", "fragment-secret", "abcd", "secret-token")
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

func TestBatchAddNormalizesLocalPathBeforeCreateAndResponse(t *testing.T) {
	gin.SetMode(gin.TestMode)

	taskEngine := &mockBatchDeleteTaskEngine{}
	mountPointService := &mockBatchDeleteMountPointService{
		mountPoints: map[int64]*models.MountPoint{
			11: {FileId: 11, FullPath: "/media/中文", CreatorUserID: 100},
		},
		mountPointsByPath: map[string]*models.MountPoint{},
	}
	storageFacade := &mockBatchAddStorageFacade{createID: 11}

	router := newBatchAddTestRouter(taskEngine, mountPointService, storageFacade)
	req := httptest.NewRequestWithContext(
		stdctx.Background(),
		http.MethodPost,
		"/batch_add",
		strings.NewReader(`{"items":[{"localPath":"/media//%E4%B8%AD%E6%96%87/","osType":"subscribe","subscribeUser":"up-user"}]}`),
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

	if len(response.Data.Results) != 1 || response.Data.Results[0].LocalPath != "/media/中文" {
		t.Fatalf("expected normalized result path, got %+v", response.Data.Results)
	}

	if storageFacade.req == nil || storageFacade.req.LocalPath != "/media/中文" {
		t.Fatalf("expected normalized storage facade path, got %+v", storageFacade.req)
	}

	if len(taskEngine.paths) != 1 || taskEngine.paths[0] != "/media/中文" {
		t.Fatalf("expected normalized queued full path, got %v", taskEngine.paths)
	}
}

func TestBatchAddPassesCanonicalPathToFacadeAfterNormalizingResult(t *testing.T) {
	gin.SetMode(gin.TestMode)

	taskEngine := &mockBatchDeleteTaskEngine{}
	mountPointService := &mockBatchDeleteMountPointService{
		mountPoints: map[int64]*models.MountPoint{
			11: {FileId: 11, FullPath: "/literal/a%252Fb", CreatorUserID: 100},
		},
		mountPointsByPath: map[string]*models.MountPoint{},
	}
	storageFacade := &mockBatchAddStorageFacade{createID: 11}

	router := newBatchAddTestRouter(taskEngine, mountPointService, storageFacade)
	req := httptest.NewRequestWithContext(
		stdctx.Background(),
		http.MethodPost,
		"/batch_add",
		strings.NewReader(`{"items":[{"localPath":"/literal/a%252Fb","osType":"subscribe","subscribeUser":"up-user"}]}`),
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

	if len(response.Data.Results) != 1 || response.Data.Results[0].LocalPath != "/literal/a%252Fb" {
		t.Fatalf("expected normalized result path, got %+v", response.Data.Results)
	}

	if storageFacade.req == nil || storageFacade.req.LocalPath != "/literal/a%252Fb" {
		t.Fatalf("expected canonical storage facade path, got %+v", storageFacade.req)
	}

	if len(taskEngine.paths) != 1 || taskEngine.paths[0] != "/literal/a%252Fb" {
		t.Fatalf("expected normalized queued full path, got %v", taskEngine.paths)
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

func TestBatchAddDeduplicatesInitialScanTasksAfterPathNormalization(t *testing.T) {
	gin.SetMode(gin.TestMode)

	taskEngine := &mockBatchDeleteTaskEngine{}
	mountPointService := &mockBatchDeleteMountPointService{
		mountPoints: map[int64]*models.MountPoint{
			11: {FileId: 11, FullPath: "/media/中文", CreatorUserID: 100},
		},
		mountPointsByPath: map[string]*models.MountPoint{},
	}
	storageFacade := &mockBatchAddStorageFacade{createID: 11}

	router := newBatchAddTestRouter(taskEngine, mountPointService, storageFacade)
	req := httptest.NewRequestWithContext(
		stdctx.Background(),
		http.MethodPost,
		"/batch_add",
		strings.NewReader(`{"items":[{"localPath":"/media/%E4%B8%AD%E6%96%87/","osType":"subscribe","subscribeUser":"up-user"},{"localPath":"/media//中文","osType":"subscribe","subscribeUser":"up-user"}]}`),
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
		t.Fatalf("expected both normalized results covered by one queued scan, got %+v", response.Data)
	}

	if len(taskEngine.payloads) != 1 {
		t.Fatalf("expected one queued scan task for normalized duplicate path, got %d", len(taskEngine.payloads))
	}

	if got, want := mountPointService.queries, []int64{11}; !int64SlicesEqual(got, want) {
		t.Fatalf("expected one mount point query for duplicate file id %v, got %v", want, got)
	}

	for _, result := range response.Data.Results {
		if result.LocalPath != "/media/中文" {
			t.Fatalf("expected normalized result path, got %+v", response.Data.Results)
		}

		if !result.Success || !result.ScanQueued || result.ID != 11 {
			t.Fatalf("expected normalized duplicate result covered by queued scan, got %+v", result)
		}
	}
}

func TestBatchAddNormalizesSubscribeShareURL(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cloudBridge := &mockBatchAddCloudBridge{}
	taskEngine := &mockBatchDeleteTaskEngine{}
	mountPointService := &mockBatchDeleteMountPointService{
		mountPoints: map[int64]*models.MountPoint{
			11: {FileId: 11, FullPath: "/subscribed-share", CreatorUserID: 100},
		},
		mountPointsByPath: map[string]*models.MountPoint{},
	}
	storageFacade := &mockBatchAddStorageFacade{createID: 11}

	router := newBatchAddTestRouterWithCloudBridge(taskEngine, mountPointService, storageFacade, cloudBridge)
	req := httptest.NewRequestWithContext(
		stdctx.Background(),
		http.MethodPost,
		"/batch_add",
		strings.NewReader(`{"items":[{"localPath":"/subscribed-share","osType":"subscribe_share_folder","subscribeUser":"up-user","shareCode":"https://cloud.189.cn/t/abcDEF（访问码：wxyz）"}]}`),
	)
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected ok, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	if got, want := cloudBridge.subscribeShareCodes, []string{"abcDEF"}; !stringSlicesEqual(got, want) {
		t.Fatalf("expected normalized subscribe share code %v, got %v", want, got)
	}

	if got, want := cloudBridge.subscribeAccessCodes, []string{"wxyz"}; !stringSlicesEqual(got, want) {
		t.Fatalf("expected normalized subscribe access code %v, got %v", want, got)
	}

	if storageFacade.req == nil {
		t.Fatal("expected storage facade to be called")
	}

	assertBatchAddAdditionValue(t, storageFacade.req.Addition, consts.FileAdditionKeyUpUserId, "up-user")
	assertBatchAddAdditionValue(t, storageFacade.req.Addition, consts.FileAdditionKeyShareId, int64(12345))
	assertBatchAddAdditionValue(t, storageFacade.req.Addition, consts.FileAdditionKeyIsFolder, true)
	assertBatchAddAdditionValue(t, storageFacade.req.Addition, consts.FileAdditionKeyShareMode, 2)
	assertBatchAddAdditionValue(t, storageFacade.req.Addition, consts.FileAdditionKeyAccessCode, "wxyz")

	if storageFacade.req.FileId != "cloud-file-id" {
		t.Fatalf("expected resolved cloud file id, got %q", storageFacade.req.FileId)
	}
}

func TestBatchAddNormalizesShareURLWithAccessCode(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cloudBridge := &mockBatchAddCloudBridge{}
	taskEngine := &mockBatchDeleteTaskEngine{}
	mountPointService := &mockBatchDeleteMountPointService{
		mountPoints: map[int64]*models.MountPoint{
			11: {FileId: 11, FullPath: "/shared", CreatorUserID: 100},
		},
		mountPointsByPath: map[string]*models.MountPoint{},
	}
	storageFacade := &mockBatchAddStorageFacade{createID: 11}

	router := newBatchAddTestRouterWithCloudBridge(taskEngine, mountPointService, storageFacade, cloudBridge)
	req := httptest.NewRequestWithContext(
		stdctx.Background(),
		http.MethodPost,
		"/batch_add",
		strings.NewReader(`{"items":[{"localPath":"/shared","osType":"share_folder","shareCode":"https://cloud.189.cn/t/abcDEF（访问码：wxyz）"}]}`),
	)
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected ok, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	if got, want := cloudBridge.shareCodes, []string{"abcDEF"}; !stringSlicesEqual(got, want) {
		t.Fatalf("expected normalized share code %v, got %v", want, got)
	}

	if got, want := cloudBridge.shareAccessCodes, []string{"wxyz"}; !stringSlicesEqual(got, want) {
		t.Fatalf("expected normalized share access code %v, got %v", want, got)
	}

	if storageFacade.req == nil {
		t.Fatal("expected storage facade to be called")
	}

	assertBatchAddAdditionValue(t, storageFacade.req.Addition, consts.FileAdditionKeyShareId, int64(67890))
	assertBatchAddAdditionValue(t, storageFacade.req.Addition, consts.FileAdditionKeyIsFolder, true)
	assertBatchAddAdditionValue(t, storageFacade.req.Addition, consts.FileAdditionKeyShareMode, 5)
	assertBatchAddAdditionValue(t, storageFacade.req.Addition, consts.FileAdditionKeyAccessCode, "wxyz")

	if storageFacade.req.FileId != "share-file-id" {
		t.Fatalf("expected resolved share file id, got %q", storageFacade.req.FileId)
	}
}

func TestBatchAddRejectsInvalidShareAccessCodeBeforeRemoteValidation(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cloudBridge := &mockBatchAddCloudBridge{}
	taskEngine := &mockBatchDeleteTaskEngine{}
	mountPointService := &mockBatchDeleteMountPointService{
		mountPointsByPath: map[string]*models.MountPoint{},
	}
	storageFacade := &mockBatchAddStorageFacade{createID: 11}

	router := newBatchAddTestRouterWithCloudBridge(taskEngine, mountPointService, storageFacade, cloudBridge)
	req := httptest.NewRequestWithContext(
		stdctx.Background(),
		http.MethodPost,
		"/batch_add",
		strings.NewReader(`{"items":[{"localPath":"/bad-share","osType":"share_folder","shareCode":"https://cloud.189.cn/t/abcDEF","shareAccessCode":"https://example.com/share?token=secret-token"}]}`),
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
		t.Fatalf("expected one failed item, got %+v", response.Data)
	}

	if len(response.Data.Results) != 1 || response.Data.Results[0].Success {
		t.Fatalf("expected failed result item, got %+v", response.Data.Results)
	}

	if !strings.Contains(response.Data.Results[0].Error, "访问码格式无效") {
		t.Fatalf("expected access code validation error, got %q", response.Data.Results[0].Error)
	}

	if strings.Contains(response.Data.Results[0].Error, "secret-token") ||
		strings.Contains(response.Data.Results[0].Error, "example.com") {
		t.Fatalf("expected invalid input not to be echoed, got %q", response.Data.Results[0].Error)
	}

	if len(cloudBridge.shareCodes) != 0 || len(cloudBridge.shareAccessCodes) != 0 {
		t.Fatalf("expected remote share validation not to be called, got share=%v access=%v", cloudBridge.shareCodes, cloudBridge.shareAccessCodes)
	}

	if storageFacade.req != nil {
		t.Fatalf("expected storage facade not to be called, got %+v", storageFacade.req)
	}
}

func TestBatchAddRejectsInvalidSubscribeShareCodeBeforeRemoteValidation(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cloudBridge := &mockBatchAddCloudBridge{}
	taskEngine := &mockBatchDeleteTaskEngine{}
	mountPointService := &mockBatchDeleteMountPointService{
		mountPointsByPath: map[string]*models.MountPoint{},
	}
	storageFacade := &mockBatchAddStorageFacade{createID: 11}

	router := newBatchAddTestRouterWithCloudBridge(taskEngine, mountPointService, storageFacade, cloudBridge)
	req := httptest.NewRequestWithContext(
		stdctx.Background(),
		http.MethodPost,
		"/batch_add",
		strings.NewReader(`{"items":[{"localPath":"/bad-share","osType":"subscribe_share_folder","subscribeUser":"up-user","shareCode":"https://example.com/share?token=secret-token"}]}`),
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
		t.Fatalf("expected one failed item, got %+v", response.Data)
	}

	if len(response.Data.Results) != 1 || response.Data.Results[0].Success {
		t.Fatalf("expected failed result item, got %+v", response.Data.Results)
	}

	if !strings.Contains(response.Data.Results[0].Error, "订阅分享参数不完整") {
		t.Fatalf("expected subscribe share validation error, got %q", response.Data.Results[0].Error)
	}

	if strings.Contains(response.Data.Results[0].Error, "secret-token") ||
		strings.Contains(response.Data.Results[0].Error, "example.com") {
		t.Fatalf("expected invalid input not to be echoed, got %q", response.Data.Results[0].Error)
	}

	if len(cloudBridge.subscribeShareCodes) != 0 || len(cloudBridge.subscribeAccessCodes) != 0 {
		t.Fatalf("expected remote subscribe share validation not to be called, got share=%v access=%v", cloudBridge.subscribeShareCodes, cloudBridge.subscribeAccessCodes)
	}

	if storageFacade.req != nil {
		t.Fatalf("expected storage facade not to be called, got %+v", storageFacade.req)
	}
}

func TestBatchAddRejectsInvalidSubscribeShareAccessCodeBeforeRemoteValidation(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cloudBridge := &mockBatchAddCloudBridge{}
	taskEngine := &mockBatchDeleteTaskEngine{}
	mountPointService := &mockBatchDeleteMountPointService{
		mountPointsByPath: map[string]*models.MountPoint{},
	}
	storageFacade := &mockBatchAddStorageFacade{createID: 11}

	router := newBatchAddTestRouterWithCloudBridge(taskEngine, mountPointService, storageFacade, cloudBridge)
	req := httptest.NewRequestWithContext(
		stdctx.Background(),
		http.MethodPost,
		"/batch_add",
		strings.NewReader(`{"items":[{"localPath":"/bad-subscribe-share","osType":"subscribe_share_folder","subscribeUser":"up-user","shareCode":"abcDEF","shareAccessCode":"https://example.com/share?token=secret-token"}]}`),
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
		t.Fatalf("expected one failed item, got %+v", response.Data)
	}

	if len(response.Data.Results) != 1 || response.Data.Results[0].Success {
		t.Fatalf("expected failed result item, got %+v", response.Data.Results)
	}

	if !strings.Contains(response.Data.Results[0].Error, "访问码格式无效") {
		t.Fatalf("expected access code validation error, got %q", response.Data.Results[0].Error)
	}

	if strings.Contains(response.Data.Results[0].Error, "secret-token") ||
		strings.Contains(response.Data.Results[0].Error, "example.com") {
		t.Fatalf("expected invalid input not to be echoed, got %q", response.Data.Results[0].Error)
	}

	if len(cloudBridge.subscribeShareCodes) != 0 || len(cloudBridge.subscribeAccessCodes) != 0 {
		t.Fatalf("expected remote subscribe share validation not to be called, got share=%v access=%v", cloudBridge.subscribeShareCodes, cloudBridge.subscribeAccessCodes)
	}

	if storageFacade.req != nil {
		t.Fatalf("expected storage facade not to be called, got %+v", storageFacade.req)
	}
}

func TestNormalizeSubscribeShareParamsHandlesLabeledCodes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		shareCode      string
		shareAccess    string
		wantShareCode  string
		wantAccessCode string
	}{
		{
			name:          "chinese share label only",
			shareCode:     "分享码：abcDEF",
			wantShareCode: "abcDEF",
		},
		{
			name:           "chinese share and access labels",
			shareCode:      "分享码：abcDEF 提取码：wxyz",
			wantShareCode:  "abcDEF",
			wantAccessCode: "wxyz",
		},
		{
			name:           "parenthesized access code",
			shareCode:      "abcDEF（访问码：wxyz）",
			wantShareCode:  "abcDEF",
			wantAccessCode: "wxyz",
		},
		{
			name:           "camel case english labels",
			shareCode:      "shareCode:abc accessCode:wxyz",
			wantShareCode:  "abc",
			wantAccessCode: "wxyz",
		},
		{
			name:           "explicit access fallback",
			shareCode:      "分享码：abcDEF",
			shareAccess:    "zzzz",
			wantShareCode:  "abcDEF",
			wantAccessCode: "zzzz",
		},
		{
			name:           "explicit access overrides embedded access",
			shareCode:      "分享码：abcDEF 提取码：wxyz",
			shareAccess:    "zzzz",
			wantShareCode:  "abcDEF",
			wantAccessCode: "zzzz",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			gotShareCode, gotAccessCode := normalizeSubscribeShareParams(&addRequest{
				ShareCode:       tt.shareCode,
				ShareAccessCode: tt.shareAccess,
			})

			if gotShareCode != tt.wantShareCode || gotAccessCode != tt.wantAccessCode {
				t.Fatalf("expected share/access %q/%q, got %q/%q", tt.wantShareCode, tt.wantAccessCode, gotShareCode, gotAccessCode)
			}
		})
	}
}

func newBatchAddTestRouter(
	taskEngine *mockBatchDeleteTaskEngine,
	mountPointService *mockBatchDeleteMountPointService,
	storageFacade *mockBatchAddStorageFacade,
) *gin.Engine {
	return newBatchAddTestRouterWithCloudBridge(taskEngine, mountPointService, storageFacade, &mockBatchAddCloudBridge{})
}

func newBatchAddTestRouterWithCloudBridge(
	taskEngine *mockBatchDeleteTaskEngine,
	mountPointService *mockBatchDeleteMountPointService,
	storageFacade *mockBatchAddStorageFacade,
	cloudBridge *mockBatchAddCloudBridge,
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
		cloudBridge,
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

func stringSlicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}

	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}

	return true
}

func assertBatchAddAdditionValue(t *testing.T, addition map[string]interface{}, key string, want interface{}) {
	t.Helper()

	got, ok := addition[key]
	if !ok {
		t.Fatalf("expected addition %q to exist, got %#v", key, addition)
	}

	if got != want {
		t.Fatalf("expected addition %q = %#v (%T), got %#v (%T)", key, want, want, got, got)
	}
}
