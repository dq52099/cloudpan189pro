package media

import (
	stdctx "context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	appContext "github.com/xxcheng123/cloudpan189-share/internal/framework/context"
	"github.com/xxcheng123/cloudpan189-share/internal/framework/httpcontext"
	"github.com/xxcheng123/cloudpan189-share/internal/repository/models"
	mediaconfigSvc "github.com/xxcheng123/cloudpan189-share/internal/services/mediaconfig"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type mockConfigToggleMediaConfigService struct {
	mediaconfigSvc.Service
	enable *bool
	err    error
}

func (m *mockConfigToggleMediaConfigService) Toggle(ctx appContext.Context, enable bool) error {
	if m.err != nil {
		return m.err
	}

	m.enable = &enable

	return nil
}

func (m *mockConfigToggleMediaConfigService) Query(ctx appContext.Context) (*models.MediaConfig, error) {
	return nil, nil
}

func performConfigToggleRequest(t *testing.T, service *mockConfigToggleMediaConfigService, body string) *httptest.ResponseRecorder {
	t.Helper()

	gin.SetMode(gin.TestMode)

	router := gin.New()
	wrapper := httpcontext.NewHandlerFuncWrapper(zap.NewNop())
	router.POST("/config/toggle", wrapper.Wrap(NewHandler(service, nil, nil, nil, nil, nil, nil).ConfigToggle()))

	req := httptest.NewRequestWithContext(stdctx.Background(), http.MethodPost, "/config/toggle", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	return recorder
}

func TestConfigToggleRejectsMissingEnable(t *testing.T) {
	service := &mockConfigToggleMediaConfigService{}

	recorder := performConfigToggleRequest(t, service, `{}`)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected bad request, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	if service.enable != nil {
		t.Fatalf("expected missing enable not to toggle, got %v", *service.enable)
	}
}

func TestConfigToggleAcceptsExplicitDisable(t *testing.T) {
	service := &mockConfigToggleMediaConfigService{}

	recorder := performConfigToggleRequest(t, service, `{"enable":false}`)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected success, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	if service.enable == nil {
		t.Fatal("expected media config toggle service to be called")
	}

	if *service.enable {
		t.Fatal("expected explicit false to be passed to service")
	}
}

func TestConfigToggleReturnsNotFoundWhenConfigMissing(t *testing.T) {
	service := &mockConfigToggleMediaConfigService{err: gorm.ErrRecordNotFound}

	recorder := performConfigToggleRequest(t, service, `{"enable":true}`)

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("expected not found, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	var response httpcontext.Response
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}

	if response.Code != codeConfigNotInit.GetCode() {
		t.Fatalf("expected business code %d, got %d", codeConfigNotInit.GetCode(), response.Code)
	}
}
