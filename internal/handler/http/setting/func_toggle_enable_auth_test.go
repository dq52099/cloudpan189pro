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
	"github.com/xxcheng123/cloudpan189-share/internal/pkgs/utils"
	settingSvc "github.com/xxcheng123/cloudpan189-share/internal/services/setting"
	"go.uber.org/zap"
)

type mockToggleEnableAuthSettingService struct {
	settingSvc.Service
	fields []utils.Field
}

func (m *mockToggleEnableAuthSettingService) Update(ctx appContext.Context, fields ...utils.Field) error {
	m.fields = fields

	return nil
}

func performToggleEnableAuthRequest(
	t *testing.T,
	service *mockToggleEnableAuthSettingService,
	body string,
) *httptest.ResponseRecorder {
	t.Helper()

	gin.SetMode(gin.TestMode)

	router := gin.New()
	wrapper := httpcontext.NewHandlerFuncWrapper(zap.NewNop())
	router.POST("/toggle_enable_auth", wrapper.Wrap(NewHandler(nil, service, nil).ToggleEnableAuth()))

	req := httptest.NewRequestWithContext(
		stdctx.Background(),
		http.MethodPost,
		"/toggle_enable_auth",
		strings.NewReader(body),
	)
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	return recorder
}

func TestToggleEnableAuthRejectsMissingEnableAuth(t *testing.T) {
	service := &mockToggleEnableAuthSettingService{}

	recorder := performToggleEnableAuthRequest(t, service, `{}`)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected bad request, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	if len(service.fields) != 0 {
		t.Fatalf("expected missing enableAuth not to update setting, got %#v", service.fields)
	}
}

func TestToggleEnableAuthAcceptsExplicitDisable(t *testing.T) {
	service := &mockToggleEnableAuthSettingService{}

	recorder := performToggleEnableAuthRequest(t, service, `{"enableAuth":false}`)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected success, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	if len(service.fields) != 1 {
		t.Fatalf("expected one update field, got %#v", service.fields)
	}

	if service.fields[0].Key != "enable_auth" || service.fields[0].Value != false {
		t.Fatalf("expected explicit false enable_auth field, got %#v", service.fields[0])
	}
}
