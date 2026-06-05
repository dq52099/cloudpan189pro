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
	"github.com/xxcheng123/cloudpan189-share/internal/pkgs/utils"
	"github.com/xxcheng123/cloudpan189-share/internal/repository/models"
	cloudtokenSvi "github.com/xxcheng123/cloudpan189-share/internal/services/cloudtoken"
	"github.com/xxcheng123/cloudpan189-share/internal/types/topic"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

var errQueueUnavailable = errors.New("queue unavailable")

type mockAddCloudTokenService struct {
	cloudtokenSvi.Service
	err     error
	queries []int64
}

func (m *mockAddCloudTokenService) QueryAccessible(ctx appContext.Context, id, userID int64, isAdmin bool) (*models.CloudToken, error) {
	m.queries = append(m.queries, id)

	if m.err != nil {
		return nil, m.err
	}

	return &models.CloudToken{ID: id, AccessToken: "access-token", ExpiresIn: 3600}, nil
}

func TestAddReturnsCreatedStorageWhenInitialScanDispatchFails(t *testing.T) {
	gin.SetMode(gin.TestMode)

	taskEngine := &mockBatchDeleteTaskEngine{pushErr: errQueueUnavailable}
	storageFacade := &mockBatchAddStorageFacade{createID: 11}
	mountPointService := &mockBatchDeleteMountPointService{}
	router := newAddTestRouterWithMountPoint(taskEngine, storageFacade, mountPointService)

	req := httptest.NewRequestWithContext(
		stdctx.Background(),
		http.MethodPost,
		"/add",
		strings.NewReader(`{"localPath":"/subscribed","osType":"subscribe","subscribeUser":"up-user"}`),
	)
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected ok, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	var response struct {
		Code int         `json:"code"`
		Data addResponse `json:"data"`
	}

	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}

	if response.Code != http.StatusOK {
		t.Fatalf("expected business code 200, got %d", response.Code)
	}

	if response.Data.ID != 11 || response.Data.Path != "/subscribed" {
		t.Fatalf("expected created storage response, got %+v", response.Data)
	}

	if response.Data.ScanQueued {
		t.Fatalf("expected scanQueued false, got %+v", response.Data)
	}

	if !strings.Contains(response.Data.ScanError, errQueueUnavailable.Error()) {
		t.Fatalf("expected scan error to contain queue failure, got %q", response.Data.ScanError)
	}

	if storageFacade.req == nil || storageFacade.req.LocalPath != "/subscribed" {
		t.Fatalf("expected storage facade to be called, got %+v", storageFacade.req)
	}

	if got := mountPointService.lastStateUpdates[11]; !strings.Contains(got, "初始化扫描任务入队失败") ||
		!strings.Contains(got, errQueueUnavailable.Error()) {
		t.Fatalf("expected failed initial scan state, got %q", got)
	}
}

func TestAddReturnsCreatedStorageWhenTaskEngineMissing(t *testing.T) {
	gin.SetMode(gin.TestMode)

	storageFacade := &mockBatchAddStorageFacade{createID: 11}
	mountPointService := &mockBatchDeleteMountPointService{}

	router := gin.New()
	router.Use(func(ctx *gin.Context) {
		ctx.Set(consts.CtxKeyUserId, int64(100))
		ctx.Set(consts.CtxKeyIsAdmin, false)
	})

	wrapper := httpcontext.NewHandlerFuncWrapper(zap.NewNop())
	router.POST("/add", wrapper.Wrap(NewHandler(
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
	).Add()))

	req := httptest.NewRequestWithContext(
		stdctx.Background(),
		http.MethodPost,
		"/add",
		strings.NewReader(`{"localPath":"/subscribed","osType":"subscribe","subscribeUser":"up-user"}`),
	)
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected ok, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	var response struct {
		Data addResponse `json:"data"`
	}

	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}

	if response.Data.ID != 11 || response.Data.ScanQueued {
		t.Fatalf("expected created storage without queued scan, got %+v", response.Data)
	}

	if !strings.Contains(response.Data.ScanError, "任务引擎未初始化") {
		t.Fatalf("expected task engine error, got %q", response.Data.ScanError)
	}

	if got := mountPointService.lastStateUpdates[11]; !strings.Contains(got, "初始化扫描任务入队失败") ||
		!strings.Contains(got, "任务引擎未初始化") {
		t.Fatalf("expected failed initial scan state, got %q", got)
	}
}

func TestAddReturnsCreatedStorageWhenTaskEngineTypedNil(t *testing.T) {
	gin.SetMode(gin.TestMode)

	var taskEngine *mockBatchDeleteTaskEngine

	storageFacade := &mockBatchAddStorageFacade{createID: 11}
	mountPointService := &mockBatchDeleteMountPointService{}
	router := newAddTestRouterWithMountPoint(taskEngine, storageFacade, mountPointService)

	req := httptest.NewRequestWithContext(
		stdctx.Background(),
		http.MethodPost,
		"/add",
		strings.NewReader(`{"localPath":"/subscribed","osType":"subscribe","subscribeUser":"up-user"}`),
	)
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected ok, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	var response struct {
		Data addResponse `json:"data"`
	}

	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}

	if response.Data.ID != 11 || response.Data.ScanQueued {
		t.Fatalf("expected created storage without queued scan, got %+v", response.Data)
	}

	if !strings.Contains(response.Data.ScanError, "任务引擎未初始化") {
		t.Fatalf("expected task engine error, got %q", response.Data.ScanError)
	}
}

func TestAddReturnsFailureWhenStorageFacadeTypedNil(t *testing.T) {
	gin.SetMode(gin.TestMode)

	var storageFacade *mockBatchAddStorageFacade

	taskEngine := &mockBatchDeleteTaskEngine{}
	router := newAddTestRouter(taskEngine, storageFacade)

	req := httptest.NewRequestWithContext(
		stdctx.Background(),
		http.MethodPost,
		"/add",
		strings.NewReader(`{"localPath":"/subscribed","osType":"subscribe","subscribeUser":"up-user"}`),
	)
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected bad request, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	var response httpcontext.Response
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}

	if response.Code != busCodeStorageAddMountPointFailed.GetCode() {
		t.Fatalf("expected business code %d, got %d", busCodeStorageAddMountPointFailed.GetCode(), response.Code)
	}

	if len(taskEngine.payloads) != 0 {
		t.Fatalf("expected no scan task, got %d", len(taskEngine.payloads))
	}
}

func TestAddRedactsInitialScanDispatchFailure(t *testing.T) {
	gin.SetMode(gin.TestMode)

	rawErr := `queue failed: https://proxy-user:proxy-pass@example.test/scan?access_token=query-secret&filename=private-name.mkv#token=fragment-secret accessCode=abcd Authorization: Bearer secret-token`
	taskEngine := &mockBatchDeleteTaskEngine{pushErr: errors.New(rawErr)}
	storageFacade := &mockBatchAddStorageFacade{createID: 11}
	mountPointService := &mockBatchDeleteMountPointService{}
	router := newAddTestRouterWithMountPoint(taskEngine, storageFacade, mountPointService)

	req := httptest.NewRequestWithContext(
		stdctx.Background(),
		http.MethodPost,
		"/add",
		strings.NewReader(`{"localPath":"/subscribed","osType":"subscribe","subscribeUser":"up-user"}`),
	)
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected ok, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	var response struct {
		Code int         `json:"code"`
		Data addResponse `json:"data"`
	}

	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}

	assertStorageVisibleTextRedacted(t, response.Data.ScanError, "proxy-user", "proxy-pass", "query-secret", "private-name.mkv", "fragment-secret", "abcd", "secret-token")
	assertStorageVisibleTextRedacted(t, mountPointService.lastStateUpdates[11], "proxy-user", "proxy-pass", "query-secret", "private-name.mkv", "fragment-secret", "abcd", "secret-token")
}

func TestAddQueuesInitialScanTask(t *testing.T) {
	gin.SetMode(gin.TestMode)

	taskEngine := &mockBatchDeleteTaskEngine{}
	storageFacade := &mockBatchAddStorageFacade{createID: 11}
	router := newAddTestRouter(taskEngine, storageFacade)

	req := httptest.NewRequestWithContext(
		stdctx.Background(),
		http.MethodPost,
		"/add",
		strings.NewReader(`{"localPath":"/subscribed","osType":"subscribe","subscribeUser":"up-user"}`),
	)
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected ok, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	var response struct {
		Code int         `json:"code"`
		Data addResponse `json:"data"`
	}

	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}

	if !response.Data.ScanQueued || response.Data.ScanError != "" {
		t.Fatalf("expected queued scan without error, got %+v", response.Data)
	}

	if len(taskEngine.payloads) != 1 {
		t.Fatalf("expected one queued task, got %d", len(taskEngine.payloads))
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

	if taskEngine.paths[0] != "/subscribed" {
		t.Fatalf("expected queued full path /subscribed, got %v", taskEngine.paths[0])
	}
}

func TestAddNormalizesLocalPathBeforeCreateAndScan(t *testing.T) {
	gin.SetMode(gin.TestMode)

	taskEngine := &mockBatchDeleteTaskEngine{}
	storageFacade := &mockBatchAddStorageFacade{createID: 11}
	router := newAddTestRouter(taskEngine, storageFacade)

	req := httptest.NewRequestWithContext(
		stdctx.Background(),
		http.MethodPost,
		"/add",
		strings.NewReader(`{"localPath":"/media//%E4%B8%AD%E6%96%87/","osType":"subscribe","subscribeUser":"up-user"}`),
	)
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected ok, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	var response struct {
		Code int         `json:"code"`
		Data addResponse `json:"data"`
	}

	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}

	if response.Data.Path != "/media/中文" {
		t.Fatalf("expected normalized response path, got %q", response.Data.Path)
	}

	if storageFacade.req == nil || storageFacade.req.LocalPath != "/media/中文" {
		t.Fatalf("expected normalized storage facade path, got %+v", storageFacade.req)
	}

	if len(taskEngine.paths) != 1 || taskEngine.paths[0] != "/media/中文" {
		t.Fatalf("expected normalized queued full path, got %v", taskEngine.paths)
	}
}

func TestAddPassesCanonicalPathToFacadeAfterNormalizingResponse(t *testing.T) {
	gin.SetMode(gin.TestMode)

	taskEngine := &mockBatchDeleteTaskEngine{}
	storageFacade := &mockBatchAddStorageFacade{createID: 11}
	router := newAddTestRouter(taskEngine, storageFacade)

	req := httptest.NewRequestWithContext(
		stdctx.Background(),
		http.MethodPost,
		"/add",
		strings.NewReader(`{"localPath":"/literal/a%252Fb","osType":"subscribe","subscribeUser":"up-user"}`),
	)
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected ok, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	var response struct {
		Code int         `json:"code"`
		Data addResponse `json:"data"`
	}

	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}

	if response.Data.Path != "/literal/a%252Fb" {
		t.Fatalf("expected normalized response path, got %q", response.Data.Path)
	}

	if storageFacade.req == nil || storageFacade.req.LocalPath != "/literal/a%252Fb" {
		t.Fatalf("expected canonical storage facade path, got %+v", storageFacade.req)
	}

	if len(taskEngine.paths) != 1 || taskEngine.paths[0] != "/literal/a%252Fb" {
		t.Fatalf("expected normalized queued full path, got %v", taskEngine.paths)
	}
}

func TestAddNormalizesSubscribeUserLink(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cloudBridge := &mockBatchAddCloudBridge{}
	taskEngine := &mockBatchDeleteTaskEngine{}
	storageFacade := &mockBatchAddStorageFacade{createID: 11}
	router := newAddTestRouterWithCloudBridge(taskEngine, storageFacade, cloudBridge)

	req := httptest.NewRequestWithContext(
		stdctx.Background(),
		http.MethodPost,
		"/add",
		strings.NewReader(`{"localPath":"/subscribed-link","osType":"subscribe","subscribeUser":"https://content.21cn.com/h5/subscrip/?uuid=encoded%2Duser%5F9。"}`),
	)
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected ok, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	if got, want := cloudBridge.subscribeUsers, []string{"encoded-user_9"}; !stringSlicesEqual(got, want) {
		t.Fatalf("expected normalized subscribe user lookup %v, got %v", want, got)
	}

	if storageFacade.req == nil {
		t.Fatal("expected storage facade to be called")
	}

	assertBatchAddAdditionValue(t, storageFacade.req.Addition, consts.FileAdditionKeyUpUserId, "encoded-user_9")
}

func TestAddRejectsLookalikeSubscribeUserLinkBeforeLookup(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cloudBridge := &mockBatchAddCloudBridge{}
	taskEngine := &mockBatchDeleteTaskEngine{}
	storageFacade := &mockBatchAddStorageFacade{createID: 11}
	router := newAddTestRouterWithCloudBridge(taskEngine, storageFacade, cloudBridge)

	req := httptest.NewRequestWithContext(
		stdctx.Background(),
		http.MethodPost,
		"/add",
		strings.NewReader(`{"localPath":"/bad-subscribe-link","osType":"subscribe","subscribeUser":"note https://content.21cn.com.evil.test/h5/subscrip/?uuid=bad-user"}`),
	)
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected bad request, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	var response httpcontext.Response
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}

	if response.Code != busCodeStorageSubscribeUserEmpty.GetCode() {
		t.Fatalf("expected business code %d, got %d", busCodeStorageSubscribeUserEmpty.GetCode(), response.Code)
	}

	if len(cloudBridge.subscribeUsers) != 0 {
		t.Fatalf("expected subscribe lookup not to be called, got %v", cloudBridge.subscribeUsers)
	}

	if storageFacade.req != nil {
		t.Fatalf("expected storage facade not to be called, got %+v", storageFacade.req)
	}
}

func TestAddNormalizesSubscribeShareUserLink(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cloudBridge := &mockBatchAddCloudBridge{}
	taskEngine := &mockBatchDeleteTaskEngine{}
	storageFacade := &mockBatchAddStorageFacade{createID: 11}
	router := newAddTestRouterWithCloudBridge(taskEngine, storageFacade, cloudBridge)

	req := httptest.NewRequestWithContext(
		stdctx.Background(),
		http.MethodPost,
		"/add",
		strings.NewReader(`{"localPath":"/subscribed-share-link","osType":"subscribe_share_folder","subscribeUser":"123 https://content.21cn.com/h5/subscrip/?uuid=number-prefix-user","shareCode":"abcDEF","shareAccessCode":"wxyz"}`),
	)
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected ok, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	if got, want := cloudBridge.subscribeShareUsers, []string{"number-prefix-user"}; !stringSlicesEqual(got, want) {
		t.Fatalf("expected normalized subscribe share user lookup %v, got %v", want, got)
	}

	if storageFacade.req == nil {
		t.Fatal("expected storage facade to be called")
	}

	assertBatchAddAdditionValue(t, storageFacade.req.Addition, consts.FileAdditionKeyUpUserId, "number-prefix-user")
}

func TestAddReturnsNotFoundWhenPersonalCloudTokenMissing(t *testing.T) {
	gin.SetMode(gin.TestMode)

	taskEngine := &mockBatchDeleteTaskEngine{}
	storageFacade := &mockBatchAddStorageFacade{}
	cloudTokenService := &mockAddCloudTokenService{err: gorm.ErrRecordNotFound}
	router := newAddTestRouterWithCloudToken(taskEngine, storageFacade, cloudTokenService)

	req := httptest.NewRequestWithContext(
		stdctx.Background(),
		http.MethodPost,
		"/add",
		strings.NewReader(`{"localPath":"/personal","osType":"person_folder","cloudToken":99,"fileId":"file-1"}`),
	)
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("expected not found, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	var response httpcontext.Response
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}

	if response.Code != busCodeStorageCloudTokenNotExist.GetCode() {
		t.Fatalf("expected business code %d, got %d", busCodeStorageCloudTokenNotExist.GetCode(), response.Code)
	}

	if got, want := cloudTokenService.queries, []int64{99}; !int64SlicesEqual(got, want) {
		t.Fatalf("expected cloud token query %v, got %v", want, got)
	}

	if storageFacade.req != nil {
		t.Fatalf("expected storage facade not to be called, got %+v", storageFacade.req)
	}

	if len(taskEngine.payloads) != 0 {
		t.Fatalf("expected no scan task, got %d", len(taskEngine.payloads))
	}
}

func TestAddRejectsBlankPersonalFileIDBeforeTokenLookup(t *testing.T) {
	gin.SetMode(gin.TestMode)

	taskEngine := &mockBatchDeleteTaskEngine{}
	storageFacade := &mockBatchAddStorageFacade{}
	cloudTokenService := &mockAddCloudTokenService{}
	cloudBridge := &mockBatchAddCloudBridge{}
	router := newAddTestRouterWithCloudTokenCloudBridgeAndMountPoint(
		taskEngine,
		storageFacade,
		cloudTokenService,
		cloudBridge,
		&mockBatchDeleteMountPointService{},
	)

	req := httptest.NewRequestWithContext(
		stdctx.Background(),
		http.MethodPost,
		"/add",
		strings.NewReader(`{"localPath":"/personal-blank","osType":"person_folder","cloudToken":99,"fileId":" \t "}`),
	)
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected bad request, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	var response httpcontext.Response
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}

	if response.Code != busCodeStoragePersonParamsIncomplete.GetCode() {
		t.Fatalf("expected business code %d, got %d", busCodeStoragePersonParamsIncomplete.GetCode(), response.Code)
	}

	if len(cloudTokenService.queries) != 0 {
		t.Fatalf("expected cloud token not to be queried, got %v", cloudTokenService.queries)
	}

	if len(cloudBridge.personFileIDs) != 0 {
		t.Fatalf("expected cloud bridge not to be called, got %v", cloudBridge.personFileIDs)
	}

	if storageFacade.req != nil {
		t.Fatalf("expected storage facade not to be called, got %+v", storageFacade.req)
	}
}

func TestAddTrimsPersonalFileIDBeforeLookupAndCreate(t *testing.T) {
	gin.SetMode(gin.TestMode)

	taskEngine := &mockBatchDeleteTaskEngine{}
	storageFacade := &mockBatchAddStorageFacade{createID: 11}
	cloudTokenService := &mockAddCloudTokenService{}
	cloudBridge := &mockBatchAddCloudBridge{}
	router := newAddTestRouterWithCloudTokenCloudBridgeAndMountPoint(
		taskEngine,
		storageFacade,
		cloudTokenService,
		cloudBridge,
		&mockBatchDeleteMountPointService{},
	)

	req := httptest.NewRequestWithContext(
		stdctx.Background(),
		http.MethodPost,
		"/add",
		strings.NewReader(`{"localPath":"/personal-trim","osType":"person_folder","cloudToken":99,"fileId":" file-1 "}`),
	)
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected ok, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	if got, want := cloudBridge.personFileIDs, []string{"file-1"}; !stringSlicesEqual(got, want) {
		t.Fatalf("expected personal lookup file IDs %v, got %v", want, got)
	}

	if storageFacade.req == nil || storageFacade.req.FileId != "file-1" {
		t.Fatalf("expected trimmed file id in storage facade request, got %+v", storageFacade.req)
	}
}

func TestAddTrimsFamilyIDsBeforeLookupAndCreate(t *testing.T) {
	gin.SetMode(gin.TestMode)

	taskEngine := &mockBatchDeleteTaskEngine{}
	storageFacade := &mockBatchAddStorageFacade{createID: 11}
	cloudTokenService := &mockAddCloudTokenService{}
	cloudBridge := &mockBatchAddCloudBridge{}
	router := newAddTestRouterWithCloudTokenCloudBridgeAndMountPoint(
		taskEngine,
		storageFacade,
		cloudTokenService,
		cloudBridge,
		&mockBatchDeleteMountPointService{},
	)

	req := httptest.NewRequestWithContext(
		stdctx.Background(),
		http.MethodPost,
		"/add",
		strings.NewReader(`{"localPath":"/family-trim","osType":"family_folder","cloudToken":99,"familyId":" family-1 ","fileId":" file-1 "}`),
	)
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected ok, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	if got, want := cloudBridge.familyIDs, []string{"family-1"}; !stringSlicesEqual(got, want) {
		t.Fatalf("expected family lookup family IDs %v, got %v", want, got)
	}

	if got, want := cloudBridge.familyFileIDs, []string{"file-1"}; !stringSlicesEqual(got, want) {
		t.Fatalf("expected family lookup file IDs %v, got %v", want, got)
	}

	if storageFacade.req == nil || storageFacade.req.FileId != "file-1" {
		t.Fatalf("expected trimmed file id in storage facade request, got %+v", storageFacade.req)
	}

	assertBatchAddAdditionValue(t, storageFacade.req.Addition, consts.FileAdditionKeyFamilyId, "family-1")
}

func newAddTestRouter(
	taskEngine *mockBatchDeleteTaskEngine,
	storageFacade *mockBatchAddStorageFacade,
) *gin.Engine {
	return newAddTestRouterWithMountPoint(taskEngine, storageFacade, &mockBatchDeleteMountPointService{})
}

func newAddTestRouterWithCloudToken(
	taskEngine *mockBatchDeleteTaskEngine,
	storageFacade *mockBatchAddStorageFacade,
	cloudTokenService cloudtokenSvi.Service,
) *gin.Engine {
	return newAddTestRouterWithCloudTokenAndMountPoint(
		taskEngine,
		storageFacade,
		cloudTokenService,
		&mockBatchDeleteMountPointService{},
	)
}

func newAddTestRouterWithMountPoint(
	taskEngine *mockBatchDeleteTaskEngine,
	storageFacade *mockBatchAddStorageFacade,
	mountPointService *mockBatchDeleteMountPointService,
) *gin.Engine {
	return newAddTestRouterWithCloudTokenCloudBridgeAndMountPoint(taskEngine, storageFacade, nil, &mockBatchAddCloudBridge{}, mountPointService)
}

func newAddTestRouterWithCloudBridge(
	taskEngine *mockBatchDeleteTaskEngine,
	storageFacade *mockBatchAddStorageFacade,
	cloudBridge *mockBatchAddCloudBridge,
) *gin.Engine {
	return newAddTestRouterWithCloudTokenCloudBridgeAndMountPoint(taskEngine, storageFacade, nil, cloudBridge, &mockBatchDeleteMountPointService{})
}

func newAddTestRouterWithCloudTokenAndMountPoint(
	taskEngine *mockBatchDeleteTaskEngine,
	storageFacade *mockBatchAddStorageFacade,
	cloudTokenService cloudtokenSvi.Service,
	mountPointService *mockBatchDeleteMountPointService,
) *gin.Engine {
	return newAddTestRouterWithCloudTokenCloudBridgeAndMountPoint(taskEngine, storageFacade, cloudTokenService, &mockBatchAddCloudBridge{}, mountPointService)
}

func newAddTestRouterWithCloudTokenCloudBridgeAndMountPoint(
	taskEngine *mockBatchDeleteTaskEngine,
	storageFacade *mockBatchAddStorageFacade,
	cloudTokenService cloudtokenSvi.Service,
	cloudBridge *mockBatchAddCloudBridge,
	mountPointService *mockBatchDeleteMountPointService,
) *gin.Engine {
	router := gin.New()
	router.Use(func(ctx *gin.Context) {
		ctx.Set(consts.CtxKeyUserId, int64(100))
		ctx.Set(consts.CtxKeyIsAdmin, false)
	})

	wrapper := httpcontext.NewHandlerFuncWrapper(zap.NewNop())
	router.POST("/add", wrapper.Wrap(NewHandler(
		taskEngine,
		nil,
		cloudBridge,
		cloudTokenService,
		mountPointService,
		nil,
		storageFacade,
		nil,
		nil,
		nil,
	).Add()))

	return router
}

func assertStorageVisibleTextRedacted(t *testing.T, text string, leakedValues ...string) {
	t.Helper()

	for _, leaked := range leakedValues {
		if strings.Contains(text, leaked) {
			t.Fatalf("expected %q to be redacted from %q", leaked, text)
		}
	}

	if !strings.Contains(text, utils.RedactedSecret) {
		t.Fatalf("expected redacted marker in %q", text)
	}
}
