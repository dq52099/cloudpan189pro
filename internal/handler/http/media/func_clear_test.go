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
	"go.uber.org/zap"
)

func TestClearReturnsMediaNotEnabledWhenConfigQueryReturnsNil(t *testing.T) {
	gin.SetMode(gin.TestMode)

	taskEngine := &mockRebuildTaskEngine{}
	router := gin.New()
	wrapper := httpcontext.NewHandlerFuncWrapper(zap.NewNop())
	router.POST("/clear", wrapper.Wrap(NewHandler(
		&mockRebuildMediaConfigService{returnNil: true},
		nil,
		nil,
		nil,
		nil,
		nil,
		taskEngine,
	).Clear()))

	req := httptest.NewRequestWithContext(stdctx.Background(), http.MethodPost, "/clear", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected bad request, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	var response httpcontext.Response
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if response.Code != codeMediaNotEnabled.GetCode() {
		t.Fatalf("expected business code %d, got %d", codeMediaNotEnabled.GetCode(), response.Code)
	}

	if len(taskEngine.payloads) != 0 {
		t.Fatalf("expected no clear task, got %d", len(taskEngine.payloads))
	}
}
