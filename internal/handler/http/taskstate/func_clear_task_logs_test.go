package taskstate

import (
	stdctx "context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	appContext "github.com/xxcheng123/cloudpan189-share/internal/framework/context"
	"github.com/xxcheng123/cloudpan189-share/internal/framework/httpcontext"
	"github.com/xxcheng123/cloudpan189-share/internal/pkgs/utils"
	"github.com/xxcheng123/cloudpan189-share/internal/repository/models"
	filetasklogSvc "github.com/xxcheng123/cloudpan189-share/internal/services/filetasklog"
	"go.uber.org/zap"
)

type mockClearTaskLogsService struct {
	clearCalls           int
	clearByDurationCalls int
	lastDuration         string
	clearCount           int64
	clearByDurationCount int64
}

func (m *mockClearTaskLogsService) Create(appContext.Context, string, string, ...filetasklogSvc.NewOptionFunc) (*filetasklogSvc.Tracker, error) {
	return nil, nil
}

func (m *mockClearTaskLogsService) FlushCount(appContext.Context, filetasklogSvc.LogKey, ...filetasklogSvc.Counter) error {
	return nil
}

func (m *mockClearTaskLogsService) ToggleStatus(appContext.Context, filetasklogSvc.LogKey, string, ...utils.Field) error {
	return nil
}

func (m *mockClearTaskLogsService) Pending(appContext.Context, filetasklogSvc.LogKey, ...utils.Field) error {
	return nil
}

func (m *mockClearTaskLogsService) Running(appContext.Context, filetasklogSvc.LogKey, ...utils.Field) error {
	return nil
}

func (m *mockClearTaskLogsService) Completed(appContext.Context, filetasklogSvc.LogKey, ...utils.Field) error {
	return nil
}

func (m *mockClearTaskLogsService) Failed(appContext.Context, filetasklogSvc.LogKey, ...utils.Field) error {
	return nil
}

func (m *mockClearTaskLogsService) CompleteIfProgressDone(appContext.Context, filetasklogSvc.LogKey, ...utils.Field) error {
	return nil
}

func (m *mockClearTaskLogsService) CompletedWithProgress(appContext.Context, filetasklogSvc.LogKey, int, int) error {
	return nil
}

func (m *mockClearTaskLogsService) FailedWithReason(appContext.Context, filetasklogSvc.LogKey, string) error {
	return nil
}

func (m *mockClearTaskLogsService) List(appContext.Context, *filetasklogSvc.ListRequest) ([]*models.FileTaskLog, error) {
	return nil, nil
}

func (m *mockClearTaskLogsService) Count(appContext.Context, *filetasklogSvc.ListRequest) (int64, error) {
	return 0, nil
}

func (m *mockClearTaskLogsService) ListLatestFileIDsByStatus(appContext.Context, string) ([]int64, error) {
	return nil, nil
}

func (m *mockClearTaskLogsService) FindStaleTasksByDuration(appContext.Context, time.Duration) ([]*models.FileTaskLog, error) {
	return nil, nil
}

func (m *mockClearTaskLogsService) FindByFileID(appContext.Context, int64) ([]*models.FileTaskLog, error) {
	return nil, nil
}

func (m *mockClearTaskLogsService) WithError(appContext.Context, filetasklogSvc.LogKey, error) error {
	return nil
}

func (m *mockClearTaskLogsService) WithErrorAndFail(appContext.Context, filetasklogSvc.LogKey, error) error {
	return nil
}

func (m *mockClearTaskLogsService) ClearError(appContext.Context, filetasklogSvc.LogKey) error {
	return nil
}

func (m *mockClearTaskLogsService) Clear(appContext.Context) (int64, error) {
	m.clearCalls++

	return m.clearCount, nil
}

func (m *mockClearTaskLogsService) ClearByDuration(_ appContext.Context, duration string) (int64, error) {
	m.clearByDurationCalls++
	m.lastDuration = duration

	return m.clearByDurationCount, nil
}

func newClearTaskLogsRouter(service filetasklogSvc.Service) *gin.Engine {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	wrapper := httpcontext.NewHandlerFuncWrapper(zap.NewNop())
	router.POST("/clear", wrapper.Wrap(NewHandler(nil, service).ClearTaskLogs()))

	return router
}

func performClearTaskLogsRequest(t *testing.T, router *gin.Engine, body string) *httptest.ResponseRecorder {
	t.Helper()

	req, err := http.NewRequestWithContext(stdctx.Background(), http.MethodPost, "/clear", strings.NewReader(body))
	if err != nil {
		t.Fatalf("create request: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	return recorder
}

func TestClearTaskLogsRejectsMalformedJSONWithoutClearing(t *testing.T) {
	service := &mockClearTaskLogsService{}
	router := newClearTaskLogsRouter(service)

	recorder := performClearTaskLogsRequest(t, router, `{"duration":`)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected bad request, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	if service.clearCalls != 0 || service.clearByDurationCalls != 0 {
		t.Fatalf("expected no clear calls, got clear=%d clearByDuration=%d", service.clearCalls, service.clearByDurationCalls)
	}
}

func TestClearTaskLogsAllowsEmptyBody(t *testing.T) {
	service := &mockClearTaskLogsService{clearCount: 3}
	router := newClearTaskLogsRouter(service)

	recorder := performClearTaskLogsRequest(t, router, "")

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected ok, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	if service.clearCalls != 1 {
		t.Fatalf("expected one clear call, got %d", service.clearCalls)
	}

	if service.clearByDurationCalls != 0 {
		t.Fatalf("expected no duration clear calls, got %d", service.clearByDurationCalls)
	}

	if !strings.Contains(recorder.Body.String(), `"data":3`) {
		t.Fatalf("expected response to include delete count, got body=%s", recorder.Body.String())
	}
}

func TestClearTaskLogsUsesDurationFromJSON(t *testing.T) {
	service := &mockClearTaskLogsService{clearByDurationCount: 2}
	router := newClearTaskLogsRouter(service)

	recorder := performClearTaskLogsRequest(t, router, `{"duration":"7d"}`)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected ok, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	if service.clearByDurationCalls != 1 {
		t.Fatalf("expected one duration clear call, got %d", service.clearByDurationCalls)
	}

	if service.lastDuration != "7d" {
		t.Fatalf("expected duration 7d, got %q", service.lastDuration)
	}

	if !strings.Contains(recorder.Body.String(), `"data":2`) {
		t.Fatalf("expected response to include delete count, got body=%s", recorder.Body.String())
	}
}
