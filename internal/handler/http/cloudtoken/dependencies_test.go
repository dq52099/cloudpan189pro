package cloudtoken

import (
	stdctx "context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/xxcheng123/cloudpan189-share/internal/consts"
	"github.com/xxcheng123/cloudpan189-share/internal/framework/httpcontext"
	cloudtokenSvi "github.com/xxcheng123/cloudpan189-share/internal/services/cloudtoken"
	mountpointSvi "github.com/xxcheng123/cloudpan189-share/internal/services/mountpoint"
	userMountPointTokenSvi "github.com/xxcheng123/cloudpan189-share/internal/services/userMountPointToken"
	"go.uber.org/zap"
)

func assertCloudTokenHTTPError(t *testing.T, recorder *httptest.ResponseRecorder, expectedStatus int, expectedErr httpcontext.BusinessError) {
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

func newCloudTokenDependencyRouter(cloudTokenService cloudtokenSvi.Service, mountPointService mountpointSvi.Service) *gin.Engine {
	return newCloudTokenDependencyRouterWithUserMountPointToken(cloudTokenService, mountPointService, nil)
}

func newCloudTokenDependencyRouterWithUserMountPointToken(
	cloudTokenService cloudtokenSvi.Service,
	mountPointService mountpointSvi.Service,
	userMountPointTokenService userMountPointTokenSvi.Service,
) *gin.Engine {
	router := gin.New()
	router.Use(func(ctx *gin.Context) {
		ctx.Set(consts.CtxKeyUserId, int64(88))
		ctx.Set(consts.CtxKeyIsAdmin, false)
	})

	wrapper := httpcontext.NewHandlerFuncWrapper(zap.NewNop())
	handler := NewHandler(cloudTokenService, mountPointService, userMountPointTokenService)

	router.POST("/init_qrcode", wrapper.Wrap(handler.InitQrcode()))
	router.POST("/check_qrcode", wrapper.Wrap(handler.CheckQrcode()))
	router.POST("/modify_name", wrapper.Wrap(handler.ModifyName()))
	router.POST("/delete", wrapper.Wrap(handler.Delete()))
	router.GET("/list", wrapper.Wrap(handler.List()))
	router.GET("/query/:id", wrapper.Wrap(handler.Query()))
	router.POST("/username_login", wrapper.Wrap(handler.UsernameLogin()))

	return router
}

func TestCloudTokenDependencyGuardsRejectTypedNilCloudTokenService(t *testing.T) {
	gin.SetMode(gin.TestMode)

	var cloudTokenService *mockListCloudTokenService

	cases := []struct {
		name              string
		method            string
		path              string
		body              string
		mountPointService mountpointSvi.Service
		expectedErr       httpcontext.BusinessError
	}{
		{
			name:        "init qrcode",
			method:      http.MethodPost,
			path:        "/init_qrcode",
			expectedErr: codeInitQrcodeFailed,
		},
		{
			name:        "list",
			method:      http.MethodGet,
			path:        "/list?currentPage=1&pageSize=10",
			expectedErr: codeListFailed,
		},
		{
			name:        "query",
			method:      http.MethodGet,
			path:        "/query/123",
			expectedErr: codeQueryFailed,
		},
		{
			name:        "username login stored credentials",
			method:      http.MethodPost,
			path:        "/username_login",
			body:        `{"id":123}`,
			expectedErr: codeQueryFailed,
		},
		{
			name:        "username login new credentials",
			method:      http.MethodPost,
			path:        "/username_login",
			body:        `{"username":"user","password":"pass"}`,
			expectedErr: codeUsernameLoginFailed,
		},
		{
			name:              "delete",
			method:            http.MethodPost,
			path:              "/delete",
			body:              `{"id":123}`,
			mountPointService: &mockDeleteMountPointService{},
			expectedErr:       codeDeleteFailed,
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			body := strings.NewReader(tt.body)

			req := httptest.NewRequestWithContext(stdctx.Background(), tt.method, tt.path, body)
			if tt.body != "" {
				req.Header.Set("Content-Type", "application/json")
			}

			recorder := httptest.NewRecorder()
			newCloudTokenDependencyRouter(cloudTokenService, tt.mountPointService).ServeHTTP(recorder, req)

			assertCloudTokenHTTPError(t, recorder, http.StatusBadRequest, tt.expectedErr)
		})
	}
}

func TestDeleteReturnsQueryErrorWhenTypedNilMountPointService(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cloudTokenService := &mockDeleteCloudTokenService{}

	var mountPointService *mockDeleteMountPointService

	recorder := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(stdctx.Background(), http.MethodPost, "/delete", strings.NewReader(`{"id":123}`))
	req.Header.Set("Content-Type", "application/json")
	newCloudTokenDependencyRouter(cloudTokenService, mountPointService).ServeHTTP(recorder, req)

	assertCloudTokenHTTPError(t, recorder, http.StatusBadRequest, codeQueryFailed)

	if cloudTokenService.req != nil {
		t.Fatalf("expected typed nil mount point service to stop before delete, got %+v", cloudTokenService.req)
	}
}

func TestDeleteSkipsTypedNilOptionalUserMountPointTokenService(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cloudTokenService := &mockDeleteCloudTokenService{}
	mountPointService := &mockDeleteMountPointService{}

	var userMountPointTokenService *mockDeleteUserMountPointTokenService

	recorder := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(stdctx.Background(), http.MethodPost, "/delete", strings.NewReader(`{"id":123}`))
	req.Header.Set("Content-Type", "application/json")
	newCloudTokenDependencyRouterWithUserMountPointToken(
		cloudTokenService,
		mountPointService,
		userMountPointTokenService,
	).ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected ok, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	if mountPointService.req == nil {
		t.Fatal("expected mount point usage check")
	}

	if cloudTokenService.req == nil {
		t.Fatal("expected delete service call")
	}
}

func TestInitQrcodeReturnsErrorWhenCloudTokenServiceMissing(t *testing.T) {
	gin.SetMode(gin.TestMode)

	recorder := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(stdctx.Background(), http.MethodPost, "/init_qrcode", nil)
	newCloudTokenDependencyRouter(nil, nil).ServeHTTP(recorder, req)

	assertCloudTokenHTTPError(t, recorder, http.StatusBadRequest, codeInitQrcodeFailed)
}

func TestCheckQrcodeReturnsErrorWhenCloudTokenServiceMissing(t *testing.T) {
	gin.SetMode(gin.TestMode)

	recorder := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(stdctx.Background(), http.MethodPost, "/check_qrcode", strings.NewReader(`{"id":123,"uuid":"uuid"}`))
	req.Header.Set("Content-Type", "application/json")
	newCloudTokenDependencyRouter(nil, nil).ServeHTTP(recorder, req)

	assertCloudTokenHTTPError(t, recorder, http.StatusBadRequest, codeCheckQrcodeFailed)
}

func TestModifyNameReturnsErrorWhenCloudTokenServiceMissing(t *testing.T) {
	gin.SetMode(gin.TestMode)

	recorder := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(stdctx.Background(), http.MethodPost, "/modify_name", strings.NewReader(`{"id":123,"name":"new-name"}`))
	req.Header.Set("Content-Type", "application/json")
	newCloudTokenDependencyRouter(nil, nil).ServeHTTP(recorder, req)

	assertCloudTokenHTTPError(t, recorder, http.StatusBadRequest, codeModifyNameFailed)
}

func TestListReturnsErrorWhenCloudTokenServiceMissing(t *testing.T) {
	gin.SetMode(gin.TestMode)

	recorder := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(stdctx.Background(), http.MethodGet, "/list?currentPage=1&pageSize=10", nil)
	newCloudTokenDependencyRouter(nil, nil).ServeHTTP(recorder, req)

	assertCloudTokenHTTPError(t, recorder, http.StatusBadRequest, codeListFailed)
}

func TestQueryReturnsErrorWhenCloudTokenServiceMissing(t *testing.T) {
	gin.SetMode(gin.TestMode)

	recorder := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(stdctx.Background(), http.MethodGet, "/query/123", nil)
	newCloudTokenDependencyRouter(nil, nil).ServeHTTP(recorder, req)

	assertCloudTokenHTTPError(t, recorder, http.StatusBadRequest, codeQueryFailed)
}

func TestUsernameLoginReturnsQueryErrorWhenCloudTokenServiceMissingForStoredCredentials(t *testing.T) {
	gin.SetMode(gin.TestMode)

	recorder := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(stdctx.Background(), http.MethodPost, "/username_login", strings.NewReader(`{"id":123}`))
	req.Header.Set("Content-Type", "application/json")
	newCloudTokenDependencyRouter(nil, nil).ServeHTTP(recorder, req)

	assertCloudTokenHTTPError(t, recorder, http.StatusBadRequest, codeQueryFailed)
}

func TestUsernameLoginReturnsLoginErrorWhenCloudTokenServiceMissingForNewCredentials(t *testing.T) {
	gin.SetMode(gin.TestMode)

	recorder := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(stdctx.Background(), http.MethodPost, "/username_login", strings.NewReader(`{"username":"user","password":"pass"}`))
	req.Header.Set("Content-Type", "application/json")
	newCloudTokenDependencyRouter(nil, nil).ServeHTTP(recorder, req)

	assertCloudTokenHTTPError(t, recorder, http.StatusBadRequest, codeUsernameLoginFailed)
}

func TestDeleteReturnsQueryErrorWhenMountPointServiceMissing(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cloudTokenService := &mockDeleteCloudTokenService{}

	recorder := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(stdctx.Background(), http.MethodPost, "/delete", strings.NewReader(`{"id":123}`))
	req.Header.Set("Content-Type", "application/json")
	newCloudTokenDependencyRouter(cloudTokenService, nil).ServeHTTP(recorder, req)

	assertCloudTokenHTTPError(t, recorder, http.StatusBadRequest, codeQueryFailed)

	if cloudTokenService.req != nil {
		t.Fatalf("expected missing mount point service to stop before delete, got %+v", cloudTokenService.req)
	}
}

func TestDeleteReturnsDeleteErrorWhenCloudTokenServiceMissing(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mountPointService := &mockDeleteMountPointService{}

	recorder := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(stdctx.Background(), http.MethodPost, "/delete", strings.NewReader(`{"id":123}`))
	req.Header.Set("Content-Type", "application/json")
	newCloudTokenDependencyRouter(nil, mountPointService).ServeHTTP(recorder, req)

	assertCloudTokenHTTPError(t, recorder, http.StatusBadRequest, codeDeleteFailed)

	if mountPointService.req == nil {
		t.Fatal("expected mount point usage check before delete service guard")
	}
}
