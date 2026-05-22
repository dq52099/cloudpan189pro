package media

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
	mediaconfigSvc "github.com/xxcheng123/cloudpan189-share/internal/services/mediaconfig"
	"go.uber.org/zap"
)

type mockConfigInitMediaConfigService struct {
	mediaconfigSvc.Service
	req *mediaconfigSvc.InitRequest
}

func (m *mockConfigInitMediaConfigService) Init(ctx appContext.Context, req *mediaconfigSvc.InitRequest) error {
	m.req = req

	return nil
}

func (m *mockConfigInitMediaConfigService) Query(ctx appContext.Context) (*models.MediaConfig, error) {
	return nil, nil
}

func performConfigInitRequest(t *testing.T, service *mockConfigInitMediaConfigService, body string) *httptest.ResponseRecorder {
	t.Helper()

	gin.SetMode(gin.TestMode)

	router := gin.New()
	wrapper := httpcontext.NewHandlerFuncWrapper(zap.NewNop())
	router.POST("/config/init", wrapper.Wrap(NewHandler(service, nil, nil, nil, nil, nil, nil).ConfigInit()))

	req := httptest.NewRequestWithContext(stdctx.Background(), http.MethodPost, "/config/init", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	return recorder
}

func TestConfigInitAcceptsExplicitDisableAutoClean(t *testing.T) {
	service := &mockConfigInitMediaConfigService{}

	recorder := performConfigInitRequest(t, service, `{"enable":true,"storagePath":"/media","autoClean":false,"baseURL":"http://example.test"}`)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected success, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	if service.req == nil {
		t.Fatal("expected media config init service to be called")
	}

	if service.req.AutoClean {
		t.Fatal("expected autoClean false to be passed to service")
	}
}

func TestConfigInitRejectsMissingAutoClean(t *testing.T) {
	service := &mockConfigInitMediaConfigService{}

	recorder := performConfigInitRequest(t, service, `{"enable":true,"storagePath":"/media","baseURL":"http://example.test"}`)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected bad request, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	if service.req != nil {
		t.Fatal("expected missing autoClean not to call media config init service")
	}
}
