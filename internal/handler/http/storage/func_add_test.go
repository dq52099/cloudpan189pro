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
	return newAddTestRouterWithCloudTokenAndMountPoint(taskEngine, storageFacade, nil, mountPointService)
}

func newAddTestRouterWithCloudTokenAndMountPoint(
	taskEngine *mockBatchDeleteTaskEngine,
	storageFacade *mockBatchAddStorageFacade,
	cloudTokenService cloudtokenSvi.Service,
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
		&mockBatchAddCloudBridge{},
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
