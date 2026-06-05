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
	"github.com/xxcheng123/cloudpan189-share/internal/pkgs/utils"
	"github.com/xxcheng123/cloudpan189-share/internal/repository/models"
	"go.uber.org/zap"
)

func TestConfigInfoRedactsBaseURL(t *testing.T) {
	gin.SetMode(gin.TestMode)

	rawBaseURL := "https://media-user:media-pass@example.test/strm?access_token=secret-token#session=secret-session"
	service := &mockConfigUpdateMediaConfigService{
		queryConfig: &models.MediaConfig{
			ID:      1,
			BaseURL: rawBaseURL,
		},
	}
	router := gin.New()
	wrapper := httpcontext.NewHandlerFuncWrapper(zap.NewNop())
	router.GET("/config/info", wrapper.Wrap(NewHandler(service, nil, nil, nil, nil, nil, nil).ConfigInfo()))

	req := httptest.NewRequestWithContext(stdctx.Background(), http.MethodGet, "/config/info", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected success, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	var response struct {
		Code int                `json:"code"`
		Data configInfoResponse `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if response.Data.Config == nil {
		t.Fatal("expected media config in response")
	}

	if response.Data.Config.BaseURL != utils.RedactURLForLog(rawBaseURL) {
		t.Fatalf("expected redacted base URL, got %q", response.Data.Config.BaseURL)
	}

	for _, leaked := range []string{"media-user", "media-pass", "secret-token", "secret-session"} {
		if strings.Contains(recorder.Body.String(), leaked) {
			t.Fatalf("expected %q to be redacted from response %s", leaked, recorder.Body.String())
		}
	}

	if service.queryConfig.BaseURL != rawBaseURL {
		t.Fatalf("expected stored config not to be mutated, got %q", service.queryConfig.BaseURL)
	}
}

func TestConfigInfoTreatsNilConfigAsNotInitialized(t *testing.T) {
	gin.SetMode(gin.TestMode)

	service := &mockConfigUpdateMediaConfigService{}
	router := gin.New()
	wrapper := httpcontext.NewHandlerFuncWrapper(zap.NewNop())
	router.GET("/config/info", wrapper.Wrap(NewHandler(service, nil, nil, nil, nil, nil, nil).ConfigInfo()))

	req := httptest.NewRequestWithContext(stdctx.Background(), http.MethodGet, "/config/info", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected success, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	var response struct {
		Code int                `json:"code"`
		Data configInfoResponse `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if response.Data.Initialized {
		t.Fatalf("expected nil media config to be reported as not initialized, got %+v", response.Data)
	}

	if response.Data.Config != nil {
		t.Fatalf("expected nil config in response, got %+v", response.Data.Config)
	}
}
