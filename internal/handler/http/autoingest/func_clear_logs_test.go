package autoingest

import (
	stdctx "context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	appContext "github.com/xxcheng123/cloudpan189-share/internal/framework/context"
	"github.com/xxcheng123/cloudpan189-share/internal/framework/httpcontext"
	autoingestlogSvi "github.com/xxcheng123/cloudpan189-share/internal/services/autoingestlog"
	"go.uber.org/zap"
)

type mockClearLogsService struct {
	autoingestlogSvi.Service
	clearCalled            bool
	clearByDurationCalled  bool
	clearByDurationRequest string
}

func (m *mockClearLogsService) Clear(ctx appContext.Context) (int64, error) {
	m.clearCalled = true

	return 5, nil
}

func (m *mockClearLogsService) ClearByDuration(ctx appContext.Context, duration string) (int64, error) {
	m.clearByDurationCalled = true
	m.clearByDurationRequest = duration

	return 2, nil
}

func newClearLogsRouter(logService autoingestlogSvi.Service) *gin.Engine {
	router := gin.New()
	wrapper := httpcontext.NewHandlerFuncWrapper(zap.NewNop())
	router.POST("/clear", wrapper.Wrap(NewHandler(nil, nil, logService, nil).ClearLogs()))

	return router
}

func TestClearLogsAllowsEmptyBodyAsClearAll(t *testing.T) {
	gin.SetMode(gin.TestMode)

	logService := &mockClearLogsService{}
	router := newClearLogsRouter(logService)

	req := httptest.NewRequestWithContext(stdctx.Background(), http.MethodPost, "/clear", nil)
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected ok, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	if !logService.clearCalled {
		t.Fatal("expected all logs to be cleared")
	}

	if logService.clearByDurationCalled {
		t.Fatal("did not expect duration clear")
	}

	if !strings.Contains(recorder.Body.String(), `"data":5`) {
		t.Fatalf("expected deleted count in response, got %s", recorder.Body.String())
	}
}

func TestClearLogsRejectsMalformedJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)

	logService := &mockClearLogsService{}
	router := newClearLogsRouter(logService)

	req := httptest.NewRequestWithContext(stdctx.Background(), http.MethodPost, "/clear", strings.NewReader("{"))
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected bad request, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	if logService.clearCalled || logService.clearByDurationCalled {
		t.Fatal("expected malformed request to skip deletion")
	}
}

func TestClearLogsWithDurationUsesDurationClear(t *testing.T) {
	gin.SetMode(gin.TestMode)

	logService := &mockClearLogsService{}
	router := newClearLogsRouter(logService)

	req := httptest.NewRequestWithContext(stdctx.Background(), http.MethodPost, "/clear", strings.NewReader(`{"duration":"7d"}`))
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected ok, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	if logService.clearCalled {
		t.Fatal("did not expect clear all")
	}

	if !logService.clearByDurationCalled || logService.clearByDurationRequest != "7d" {
		t.Fatalf("expected duration 7d clear, got called=%t duration=%q", logService.clearByDurationCalled, logService.clearByDurationRequest)
	}

	if !strings.Contains(recorder.Body.String(), `"data":2`) {
		t.Fatalf("expected deleted count in response, got %s", recorder.Body.String())
	}
}
