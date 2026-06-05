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

	recorder := performConfigInitRequest(
		t,
		service,
		`{"enable":true,"storagePath":"/media","autoClean":false,"baseURL":"http://example.test","autoRebuildEnable":true,"autoRebuildCron":"0 4 * * *"}`,
	)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected success, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	if service.req == nil {
		t.Fatal("expected media config init service to be called")
	}

	if service.req.AutoClean {
		t.Fatal("expected autoClean false to be passed to service")
	}

	if service.req.AutoRebuildCron != "0 4 * * *" {
		t.Fatalf("expected autoRebuildCron to be passed to service, got %q", service.req.AutoRebuildCron)
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

func TestConfigInitRejectsInvalidBaseURL(t *testing.T) {
	tests := []struct {
		name    string
		baseURL string
	}{
		{name: "blank", baseURL: "   "},
		{name: "relative", baseURL: "/media"},
		{name: "unsupported scheme", baseURL: "ftp://example.test"},
		{name: "missing host", baseURL: "https:///media"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := &mockConfigInitMediaConfigService{}
			recorder := performConfigInitRequest(
				t,
				service,
				`{"enable":true,"storagePath":"/media","autoClean":true,"baseURL":"`+tt.baseURL+`"}`,
			)

			if recorder.Code != http.StatusBadRequest {
				t.Fatalf("expected bad request, got %d body=%s", recorder.Code, recorder.Body.String())
			}

			if service.req != nil {
				t.Fatal("expected invalid baseURL not to call media config init service")
			}
		})
	}
}

func TestConfigInitTrimsBaseURL(t *testing.T) {
	service := &mockConfigInitMediaConfigService{}

	recorder := performConfigInitRequest(
		t,
		service,
		`{"enable":true,"storagePath":"/media","autoClean":true,"baseURL":" https://example.test/media "}`,
	)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected success, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	if service.req == nil {
		t.Fatal("expected media config init service to be called")
	}

	if service.req.BaseURL != "https://example.test/media" {
		t.Fatalf("expected trimmed baseURL, got %q", service.req.BaseURL)
	}
}
