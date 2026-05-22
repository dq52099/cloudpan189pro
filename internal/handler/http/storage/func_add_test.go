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
	"github.com/xxcheng123/cloudpan189-share/internal/framework/httpcontext"
	"github.com/xxcheng123/cloudpan189-share/internal/types/topic"
	"go.uber.org/zap"
)

var errQueueUnavailable = errors.New("queue unavailable")

func TestAddReturnsCreatedStorageWhenInitialScanDispatchFails(t *testing.T) {
	gin.SetMode(gin.TestMode)

	taskEngine := &mockBatchDeleteTaskEngine{pushErr: errQueueUnavailable}
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

	if taskEngine.paths[0] != "/subscribed" {
		t.Fatalf("expected queued full path /subscribed, got %v", taskEngine.paths[0])
	}
}

func newAddTestRouter(
	taskEngine *mockBatchDeleteTaskEngine,
	storageFacade *mockBatchAddStorageFacade,
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
		nil,
		nil,
		nil,
		storageFacade,
		nil,
		nil,
		nil,
	).Add()))

	return router
}
