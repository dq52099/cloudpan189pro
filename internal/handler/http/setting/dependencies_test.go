package setting

import (
	stdctx "context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/xxcheng123/cloudpan189-share/internal/framework/httpcontext"
	"go.uber.org/zap"
)

func assertSettingHTTPError(t *testing.T, recorder *httptest.ResponseRecorder, expectedStatus int, expectedErr httpcontext.BusinessError) {
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

func TestSettingRoutesReturnErrorWhenSettingServiceMissing(t *testing.T) {
	gin.SetMode(gin.TestMode)

	wrapper := httpcontext.NewHandlerFuncWrapper(zap.NewNop())
	userService := &mockInitUserService{}
	handler := NewHandler(userService, nil, nil)

	tests := []struct {
		name        string
		method      string
		path        string
		body        string
		register    func(*gin.Engine)
		expectedErr httpcontext.BusinessError
	}{
		{
			name:        "init system",
			method:      http.MethodPost,
			path:        "/init_system",
			body:        `{"title":"CloudPan","enableAuth":true,"baseURL":"http://example.test","superUsername":"admin","superPassword":"123456"}`,
			register:    func(router *gin.Engine) { router.POST("/init_system", wrapper.Wrap(handler.InitSystem())) },
			expectedErr: codeInitSettingErr,
		},
		{
			name:        "info",
			method:      http.MethodGet,
			path:        "/info",
			register:    func(router *gin.Engine) { router.GET("/info", wrapper.Wrap(handler.Info())) },
			expectedErr: codeQueryFailed,
		},
		{
			name:        "addition",
			method:      http.MethodGet,
			path:        "/addition",
			register:    func(router *gin.Engine) { router.GET("/addition", wrapper.Wrap(handler.Addition())) },
			expectedErr: codeQueryFailed,
		},
		{
			name:        "modify addition",
			method:      http.MethodPost,
			path:        "/modify_addition",
			body:        `{}`,
			register:    func(router *gin.Engine) { router.POST("/modify_addition", wrapper.Wrap(handler.ModifyAddition())) },
			expectedErr: codeQueryFailed,
		},
		{
			name:        "modify title",
			method:      http.MethodPost,
			path:        "/modify_title",
			body:        `{"title":"CloudPan"}`,
			register:    func(router *gin.Engine) { router.POST("/modify_title", wrapper.Wrap(handler.ModifyTitle())) },
			expectedErr: codeModifyTitleFailed,
		},
		{
			name:        "modify base url",
			method:      http.MethodPost,
			path:        "/modify_base_url",
			body:        `{"baseURL":"http://example.test"}`,
			register:    func(router *gin.Engine) { router.POST("/modify_base_url", wrapper.Wrap(handler.ModifyBaseURL())) },
			expectedErr: codeModifyBaseURLFailed,
		},
		{
			name:        "toggle enable auth",
			method:      http.MethodPost,
			path:        "/toggle_enable_auth",
			body:        `{"enableAuth":false}`,
			register:    func(router *gin.Engine) { router.POST("/toggle_enable_auth", wrapper.Wrap(handler.ToggleEnableAuth())) },
			expectedErr: codeToggleEnableAuthFailed,
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

			assertSettingHTTPError(t, recorder, http.StatusBadRequest, tt.expectedErr)
		})
	}

	if userService.req != nil {
		t.Fatalf("expected missing setting service to stop before user creation, got %+v", userService.req)
	}
}

func TestInfoReturnsErrorWhenSettingServiceTypedNil(t *testing.T) {
	gin.SetMode(gin.TestMode)

	var settingService *mockInfoSettingService

	router := gin.New()
	wrapper := httpcontext.NewHandlerFuncWrapper(zap.NewNop())
	router.GET("/info", wrapper.Wrap(NewHandler(nil, settingService, nil).Info()))

	req := httptest.NewRequestWithContext(stdctx.Background(), http.MethodGet, "/info", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	assertSettingHTTPError(t, recorder, http.StatusBadRequest, codeQueryFailed)
}

func TestInitSystemReturnsErrorWhenUserServiceMissing(t *testing.T) {
	gin.SetMode(gin.TestMode)

	settingService := &mockInitSettingService{}
	router := gin.New()
	wrapper := httpcontext.NewHandlerFuncWrapper(zap.NewNop())
	router.POST("/init_system", wrapper.Wrap(NewHandler(nil, settingService, nil).InitSystem()))

	req := httptest.NewRequestWithContext(
		stdctx.Background(),
		http.MethodPost,
		"/init_system",
		strings.NewReader(`{"title":"CloudPan","enableAuth":true,"baseURL":"http://example.test","superUsername":"admin","superPassword":"123456"}`),
	)
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	assertSettingHTTPError(t, recorder, http.StatusBadRequest, codeInitSuperUserErr)

	if settingService.runTransactionCalls != 0 {
		t.Fatalf("expected missing user service to stop before transaction, got %d calls", settingService.runTransactionCalls)
	}

	if settingService.req != nil {
		t.Fatalf("expected missing user service not to initialize setting, got %+v", settingService.req)
	}
}

func TestInitSystemReturnsErrorWhenUserServiceTypedNil(t *testing.T) {
	gin.SetMode(gin.TestMode)

	var userService *mockInitUserService

	settingService := &mockInitSettingService{}

	router := gin.New()
	wrapper := httpcontext.NewHandlerFuncWrapper(zap.NewNop())
	router.POST("/init_system", wrapper.Wrap(NewHandler(userService, settingService, nil).InitSystem()))

	req := httptest.NewRequestWithContext(
		stdctx.Background(),
		http.MethodPost,
		"/init_system",
		strings.NewReader(`{"title":"CloudPan","enableAuth":true,"baseURL":"http://example.test","superUsername":"admin","superPassword":"123456"}`),
	)
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	assertSettingHTTPError(t, recorder, http.StatusBadRequest, codeInitSuperUserErr)

	if settingService.runTransactionCalls != 0 {
		t.Fatalf("expected typed-nil user service to stop before transaction, got %d calls", settingService.runTransactionCalls)
	}
}
