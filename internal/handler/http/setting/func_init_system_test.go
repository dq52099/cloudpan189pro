package setting

import (
	stdctx "context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	appContext "github.com/xxcheng123/cloudpan189-share/internal/framework/context"
	"github.com/xxcheng123/cloudpan189-share/internal/framework/httpcontext"
	"github.com/xxcheng123/cloudpan189-share/internal/repository/models"
	settingSvc "github.com/xxcheng123/cloudpan189-share/internal/services/setting"
	userSvc "github.com/xxcheng123/cloudpan189-share/internal/services/user"
	"go.uber.org/zap"
)

type mockInitSettingService struct {
	settingSvc.Service
	req *settingSvc.InitSystemRequest
}

func (m *mockInitSettingService) InitSystem(ctx appContext.Context, req *settingSvc.InitSystemRequest) error {
	m.req = req

	return nil
}

type mockInitUserService struct {
	userSvc.Service
	req *userSvc.AddRequest
}

func (m *mockInitUserService) Add(ctx appContext.Context, req *userSvc.AddRequest, opts ...userSvc.AddOptionFunc) (*userSvc.AddResponse, error) {
	m.req = req

	user := &models.User{}
	for _, opt := range opts {
		opt(user)
	}

	return &userSvc.AddResponse{ID: 1}, nil
}

func performInitSystemRequest(t *testing.T, settingService *mockInitSettingService, userService *mockInitUserService, body string) *httptest.ResponseRecorder {
	t.Helper()

	gin.SetMode(gin.TestMode)

	router := gin.New()
	wrapper := httpcontext.NewHandlerFuncWrapper(zap.NewNop())
	router.POST("/init_system", wrapper.Wrap(NewHandler(userService, settingService, nil).InitSystem()))

	req := httptest.NewRequestWithContext(stdctx.Background(), http.MethodPost, "/init_system", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	return recorder
}

func TestInitSystemAcceptsExplicitDisableAuth(t *testing.T) {
	settingService := &mockInitSettingService{}
	userService := &mockInitUserService{}

	recorder := performInitSystemRequest(t, settingService, userService, `{"title":"CloudPan","enableAuth":false,"baseURL":"http://example.test","superUsername":"admin","superPassword":"123456"}`)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected success, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	if settingService.req == nil {
		t.Fatal("expected init system service to be called")
	}

	if settingService.req.EnableAuth {
		t.Fatal("expected enableAuth false to be passed to service")
	}

	if userService.req == nil {
		t.Fatal("expected super user to be created")
	}
}

func TestInitSystemRejectsMissingEnableAuth(t *testing.T) {
	settingService := &mockInitSettingService{}
	userService := &mockInitUserService{}

	recorder := performInitSystemRequest(t, settingService, userService, `{"title":"CloudPan","baseURL":"http://example.test","superUsername":"admin","superPassword":"123456"}`)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected bad request, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	if settingService.req != nil {
		t.Fatal("expected missing enableAuth not to call init system service")
	}

	if userService.req != nil {
		t.Fatal("expected missing enableAuth not to create super user")
	}
}
