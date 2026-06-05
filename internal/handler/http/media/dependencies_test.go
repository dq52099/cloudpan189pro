package media

import (
	stdctx "context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/xxcheng123/cloudpan189-share/internal/framework/httpcontext"
	"github.com/xxcheng123/cloudpan189-share/internal/repository/models"
	"go.uber.org/zap"
)

func assertMediaHTTPError(t *testing.T, recorder *httptest.ResponseRecorder, expectedStatus int, expectedErr httpcontext.BusinessError) {
	t.Helper()

	if recorder.Code != expectedStatus {
		t.Fatalf("expected HTTP %d, got %d body=%s", expectedStatus, recorder.Code, recorder.Body.String())
	}

	var response httpcontext.Response
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if response.Code != expectedErr.GetCode() {
		t.Fatalf("expected business code %d, got %d", expectedErr.GetCode(), response.Code)
	}
}

func TestMediaConfigRoutesReturnErrorWhenConfigServiceMissing(t *testing.T) {
	gin.SetMode(gin.TestMode)

	wrapper := httpcontext.NewHandlerFuncWrapper(zap.NewNop())
	handler := NewHandler(nil, nil, nil, nil, nil, nil, &mockRebuildTaskEngine{})

	tests := []struct {
		name        string
		method      string
		path        string
		body        string
		register    func(*gin.Engine)
		expectedErr httpcontext.BusinessError
	}{
		{
			name:        "config info",
			method:      http.MethodGet,
			path:        "/config/info",
			register:    func(router *gin.Engine) { router.GET("/config/info", wrapper.Wrap(handler.ConfigInfo())) },
			expectedErr: codeConfigQueryFailed,
		},
		{
			name:        "config init",
			method:      http.MethodPost,
			path:        "/config/init",
			body:        `{"enable":true,"storagePath":"/media","autoClean":true,"baseURL":"http://example.test"}`,
			register:    func(router *gin.Engine) { router.POST("/config/init", wrapper.Wrap(handler.ConfigInit())) },
			expectedErr: codeConfigInitFailed,
		},
		{
			name:        "config update",
			method:      http.MethodPost,
			path:        "/config/update",
			body:        `{"enable":true}`,
			register:    func(router *gin.Engine) { router.POST("/config/update", wrapper.Wrap(handler.ConfigUpdate())) },
			expectedErr: codeConfigUpdateFailed,
		},
		{
			name:        "config toggle",
			method:      http.MethodPost,
			path:        "/config/toggle",
			body:        `{"enable":true}`,
			register:    func(router *gin.Engine) { router.POST("/config/toggle", wrapper.Wrap(handler.ConfigToggle())) },
			expectedErr: codeConfigToggleFailed,
		},
		{
			name:        "clear",
			method:      http.MethodPost,
			path:        "/clear",
			body:        `{}`,
			register:    func(router *gin.Engine) { router.POST("/clear", wrapper.Wrap(handler.Clear())) },
			expectedErr: codeMediaNotEnabled,
		},
		{
			name:        "rebuild",
			method:      http.MethodPost,
			path:        "/rebuild",
			body:        `{}`,
			register:    func(router *gin.Engine) { router.POST("/rebuild", wrapper.Wrap(handler.RebuildStrmFile())) },
			expectedErr: codeConfigQueryFailed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := gin.New()
			tt.register(router)

			req := httptest.NewRequestWithContext(stdctx.Background(), tt.method, tt.path, strings.NewReader(tt.body))
			if tt.body != "" {
				req.Header.Set("Content-Type", "application/json")
			}

			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, req)

			assertMediaHTTPError(t, recorder, http.StatusBadRequest, tt.expectedErr)
		})
	}
}

func TestConfigInfoReturnsErrorWhenConfigServiceTypedNil(t *testing.T) {
	gin.SetMode(gin.TestMode)

	var mediaConfigService *mockRebuildMediaConfigService

	router := gin.New()
	wrapper := httpcontext.NewHandlerFuncWrapper(zap.NewNop())
	router.GET("/config/info", wrapper.Wrap(NewHandler(mediaConfigService, nil, nil, nil, nil, nil, nil).ConfigInfo()))

	req := httptest.NewRequestWithContext(stdctx.Background(), http.MethodGet, "/config/info", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	assertMediaHTTPError(t, recorder, http.StatusBadRequest, codeConfigQueryFailed)
}

func TestClearReturnsErrorWhenTaskEngineMissing(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	wrapper := httpcontext.NewHandlerFuncWrapper(zap.NewNop())
	router.POST("/clear", wrapper.Wrap(NewHandler(
		&mockRebuildMediaConfigService{cfg: &models.MediaConfig{Enable: true}},
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
	).Clear()))

	req := httptest.NewRequestWithContext(stdctx.Background(), http.MethodPost, "/clear", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	assertMediaHTTPError(t, recorder, http.StatusBadRequest, codeClearFailed)
}

func TestClearReturnsErrorWhenTaskEngineTypedNil(t *testing.T) {
	gin.SetMode(gin.TestMode)

	var taskEngine *mockRebuildTaskEngine

	router := gin.New()
	wrapper := httpcontext.NewHandlerFuncWrapper(zap.NewNop())
	router.POST("/clear", wrapper.Wrap(NewHandler(
		&mockRebuildMediaConfigService{cfg: &models.MediaConfig{Enable: true}},
		nil,
		nil,
		nil,
		nil,
		nil,
		taskEngine,
	).Clear()))

	req := httptest.NewRequestWithContext(stdctx.Background(), http.MethodPost, "/clear", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	assertMediaHTTPError(t, recorder, http.StatusBadRequest, codeClearFailed)
}

func TestRebuildStrmFileReturnsErrorWhenTaskEngineMissingForFullRebuild(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	wrapper := httpcontext.NewHandlerFuncWrapper(zap.NewNop())
	router.POST("/rebuild", wrapper.Wrap(NewHandler(
		&mockRebuildMediaConfigService{},
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
	).RebuildStrmFile()))

	req := httptest.NewRequestWithContext(stdctx.Background(), http.MethodPost, "/rebuild", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	assertMediaHTTPError(t, recorder, http.StatusBadRequest, codeRebuildFailed)
}

func TestRebuildStrmFileReturnsErrorWhenMountPointServiceTypedNilForScopedRebuild(t *testing.T) {
	gin.SetMode(gin.TestMode)

	var mountPointService *mockRebuildMountPointService

	taskEngine := &mockRebuildTaskEngine{}

	router := gin.New()
	wrapper := httpcontext.NewHandlerFuncWrapper(zap.NewNop())
	router.POST("/rebuild", wrapper.Wrap(NewHandler(
		&mockRebuildMediaConfigService{},
		nil,
		mountPointService,
		nil,
		nil,
		nil,
		taskEngine,
	).RebuildStrmFile()))

	req := httptest.NewRequestWithContext(stdctx.Background(), http.MethodPost, "/rebuild", strings.NewReader(`{"mountPointIds":[7]}`))
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	assertMediaHTTPError(t, recorder, http.StatusBadRequest, codeRebuildFailed)

	if len(taskEngine.payloads) != 0 {
		t.Fatalf("expected typed-nil mount point service not to dispatch rebuild tasks, got %d", len(taskEngine.payloads))
	}
}

func TestRebuildStrmFileReturnsErrorWhenMountPointServiceMissingForScopedRebuild(t *testing.T) {
	gin.SetMode(gin.TestMode)

	taskEngine := &mockRebuildTaskEngine{}
	router := gin.New()
	wrapper := httpcontext.NewHandlerFuncWrapper(zap.NewNop())
	router.POST("/rebuild", wrapper.Wrap(NewHandler(
		&mockRebuildMediaConfigService{},
		nil,
		nil,
		nil,
		nil,
		nil,
		taskEngine,
	).RebuildStrmFile()))

	req := httptest.NewRequestWithContext(stdctx.Background(), http.MethodPost, "/rebuild", strings.NewReader(`{"mountPointIds":[7]}`))
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	assertMediaHTTPError(t, recorder, http.StatusBadRequest, codeRebuildFailed)

	if len(taskEngine.payloads) != 0 {
		t.Fatalf("expected missing mount point service not to dispatch rebuild tasks, got %d", len(taskEngine.payloads))
	}
}

func TestRebuildStrmFileReturnsErrorWhenTaskEngineMissingForScopedRebuild(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mountPointService := &mockRebuildMountPointService{list: []*models.MountPoint{
		{ID: 7, FileId: 11, FullPath: "/movie"},
	}}
	router := gin.New()
	wrapper := httpcontext.NewHandlerFuncWrapper(zap.NewNop())
	router.POST("/rebuild", wrapper.Wrap(NewHandler(
		&mockRebuildMediaConfigService{},
		nil,
		mountPointService,
		nil,
		nil,
		nil,
		nil,
	).RebuildStrmFile()))

	req := httptest.NewRequestWithContext(stdctx.Background(), http.MethodPost, "/rebuild", strings.NewReader(`{"mountPointIds":[7]}`))
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	assertMediaHTTPError(t, recorder, http.StatusBadRequest, codeRebuildFailed)

	if mountPointService.req != nil {
		t.Fatalf("expected missing task engine to stop before mount point query, got %+v", mountPointService.req)
	}
}
