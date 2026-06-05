package setting

import (
	stdctx "context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	appContext "github.com/xxcheng123/cloudpan189-share/internal/framework/context"
	"github.com/xxcheng123/cloudpan189-share/internal/framework/httpcontext"
	"github.com/xxcheng123/cloudpan189-share/internal/repository/models"
	settingSvc "github.com/xxcheng123/cloudpan189-share/internal/services/setting"
	"go.uber.org/zap"
)

type mockInfoSettingService struct {
	settingSvc.Service
	setting *models.Setting
}

func (m *mockInfoSettingService) Query(appContext.Context) (*models.Setting, error) {
	return m.setting, nil
}

func TestInfoReturnsQueryFailedWhenSettingQueryReturnsNil(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	wrapper := httpcontext.NewHandlerFuncWrapper(zap.NewNop())
	router.GET("/info", wrapper.Wrap(NewHandler(nil, &mockInfoSettingService{}, nil).Info()))

	req := httptest.NewRequestWithContext(stdctx.Background(), http.MethodGet, "/info", nil)
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
