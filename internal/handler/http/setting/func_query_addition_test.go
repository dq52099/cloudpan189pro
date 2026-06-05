package setting

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
	"github.com/xxcheng123/cloudpan189-share/internal/pkgs/utils"
	"github.com/xxcheng123/cloudpan189-share/internal/repository/models"
	settingSvc "github.com/xxcheng123/cloudpan189-share/internal/services/setting"
	"go.uber.org/zap"
)

type mockQueryAdditionSettingService struct {
	settingSvc.Service
	setting *models.Setting
}

func (m *mockQueryAdditionSettingService) Query(appContext.Context) (*models.Setting, error) {
	return m.setting, nil
}

func TestAdditionRedactsLocalProxyURLCredentials(t *testing.T) {
	gin.SetMode(gin.TestMode)

	rawProxyURL := "http://proxy-user:proxy-pass@example.test:8080/proxy?token=secret-token#access_token=secret-fragment"
	settingService := &mockQueryAdditionSettingService{
		setting: &models.Setting{
			Addition: models.SettingAddition{
				LocalProxy:    true,
				LocalProxyURL: rawProxyURL,
			},
		},
	}

	router := gin.New()
	wrapper := httpcontext.NewHandlerFuncWrapper(zap.NewNop())
	router.GET("/addition", wrapper.Wrap(NewHandler(nil, settingService, nil).Addition()))

	req := httptest.NewRequestWithContext(stdctx.Background(), http.MethodGet, "/addition", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected success, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	var response struct {
		Data models.SettingAddition `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if !response.Data.LocalProxy {
		t.Fatal("expected non-sensitive localProxy flag to remain")
	}

	for _, leaked := range []string{"proxy-user", "proxy-pass", "secret-token", "secret-fragment"} {
		if strings.Contains(response.Data.LocalProxyURL, leaked) {
			t.Fatalf("expected %q to be redacted from %q", leaked, response.Data.LocalProxyURL)
		}
	}

	if !strings.Contains(response.Data.LocalProxyURL, "example.test:8080/proxy") {
		t.Fatalf("expected proxy host and path to remain, got %q", response.Data.LocalProxyURL)
	}

	if !strings.Contains(response.Data.LocalProxyURL, utils.RedactedSecret) {
		t.Fatalf("expected redacted marker in %q", response.Data.LocalProxyURL)
	}

	if settingService.setting.Addition.LocalProxyURL != rawProxyURL {
		t.Fatalf("expected stored addition to remain unchanged, got %q", settingService.setting.Addition.LocalProxyURL)
	}
}

func TestAdditionReturnsQueryFailedWhenSettingQueryReturnsNil(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	wrapper := httpcontext.NewHandlerFuncWrapper(zap.NewNop())
	router.GET("/addition", wrapper.Wrap(NewHandler(nil, &mockQueryAdditionSettingService{}, nil).Addition()))

	req := httptest.NewRequestWithContext(stdctx.Background(), http.MethodGet, "/addition", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected bad request, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	var response httpcontext.Response
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if response.Code != codeQueryFailed.GetCode() {
		t.Fatalf("expected business code %d, got %d", codeQueryFailed.GetCode(), response.Code)
	}
}
