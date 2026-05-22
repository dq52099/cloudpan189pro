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

type mockDeleteErrorLogsService struct {
	autoingestlogSvi.Service
	allCalled    bool
	planIDCalled *int64
}

func (m *mockDeleteErrorLogsService) DeleteAllErrorLogs(ctx appContext.Context) (int64, error) {
	m.allCalled = true

	return 7, nil
}

func (m *mockDeleteErrorLogsService) DeleteErrorLogsByPlanId(ctx appContext.Context, planID int64) (int64, error) {
	m.planIDCalled = &planID

	return 3, nil
}

func newDeleteErrorLogsRouter(logService autoingestlogSvi.Service) *gin.Engine {
	router := gin.New()
	wrapper := httpcontext.NewHandlerFuncWrapper(zap.NewNop())
	router.POST("/delete_error", wrapper.Wrap(NewHandler(nil, nil, logService, nil).DeleteErrorLogs()))

	return router
}

func TestDeleteErrorLogsAllowsEmptyBodyAsDeleteAll(t *testing.T) {
	gin.SetMode(gin.TestMode)

	logService := &mockDeleteErrorLogsService{}
	router := newDeleteErrorLogsRouter(logService)

	req := httptest.NewRequestWithContext(stdctx.Background(), http.MethodPost, "/delete_error", nil)
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected ok, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	if !logService.allCalled {
		t.Fatal("expected all error logs to be deleted")
	}

	if logService.planIDCalled != nil {
		t.Fatalf("did not expect plan-scoped delete, got %d", *logService.planIDCalled)
	}
}

func TestDeleteErrorLogsRejectsMalformedJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)

	logService := &mockDeleteErrorLogsService{}
	router := newDeleteErrorLogsRouter(logService)

	req := httptest.NewRequestWithContext(stdctx.Background(), http.MethodPost, "/delete_error", strings.NewReader("{"))
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected bad request, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	if logService.allCalled || logService.planIDCalled != nil {
		t.Fatal("expected malformed request to skip deletion")
	}
}

func TestDeleteErrorLogsRejectsInvalidPlanIDWithoutDeleting(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name string
		body string
	}{
		{name: "zero", body: `{"planId":0}`},
		{name: "negative", body: `{"planId":-1}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logService := &mockDeleteErrorLogsService{}
			router := newDeleteErrorLogsRouter(logService)

			req := httptest.NewRequestWithContext(stdctx.Background(), http.MethodPost, "/delete_error", strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/json")

			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, req)

			if recorder.Code != http.StatusBadRequest {
				t.Fatalf("expected bad request, got %d body=%s", recorder.Code, recorder.Body.String())
			}

			if logService.allCalled || logService.planIDCalled != nil {
				t.Fatal("expected invalid plan id to skip deletion")
			}
		})
	}
}

func TestDeleteErrorLogsWithPlanIDDeletesPlanErrors(t *testing.T) {
	gin.SetMode(gin.TestMode)

	logService := &mockDeleteErrorLogsService{}
	router := newDeleteErrorLogsRouter(logService)

	req := httptest.NewRequestWithContext(stdctx.Background(), http.MethodPost, "/delete_error", strings.NewReader(`{"planId":42}`))
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected ok, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	if logService.planIDCalled == nil || *logService.planIDCalled != 42 {
		t.Fatalf("expected plan 42 delete, got %v", logService.planIDCalled)
	}

	if logService.allCalled {
		t.Fatal("did not expect all error logs to be deleted")
	}
}
