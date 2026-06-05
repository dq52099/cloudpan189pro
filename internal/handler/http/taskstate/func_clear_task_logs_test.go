package taskstate

import (
	stdctx "context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	appContext "github.com/xxcheng123/cloudpan189-share/internal/framework/context"
	"github.com/xxcheng123/cloudpan189-share/internal/framework/httpcontext"
	"github.com/xxcheng123/cloudpan189-share/internal/pkgs/taskengine"
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
	list                 []*models.FileTaskLog
	count                int64
}

type mockTaskStateTaskEngine struct {
	taskengine.TaskEngine
	stats        taskengine.TaskStats
	runningTasks []*taskengine.TaskInfo
	pendingTasks []*taskengine.TaskInfo
	running      bool
}

func (m *mockTaskStateTaskEngine) GetStats() taskengine.TaskStats {
	return m.stats
}

func (m *mockTaskStateTaskEngine) GetRunningTasks() []*taskengine.TaskInfo {
	return m.runningTasks
}

func (m *mockTaskStateTaskEngine) GetPendingTasks() []*taskengine.TaskInfo {
	return m.pendingTasks
}

func (m *mockTaskStateTaskEngine) IsRunning() bool {
	return m.running
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
	return m.list, nil
}

func (m *mockClearTaskLogsService) Count(appContext.Context, *filetasklogSvc.ListRequest) (int64, error) {
	return m.count, nil
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
	handler := NewHandler(nil, service)

	router.GET("/list", wrapper.Wrap(handler.FileLogList()))
	router.GET("/task_engine", wrapper.Wrap(handler.TaskEngineList()))
	router.POST("/clear", wrapper.Wrap(handler.ClearTaskLogs()))

	return router
}

func newTaskEngineListRouter(taskEngine taskengine.TaskEngine) *gin.Engine {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	wrapper := httpcontext.NewHandlerFuncWrapper(zap.NewNop())
	router.GET("/task_engine", wrapper.Wrap(NewHandler(taskEngine, nil).TaskEngineList()))

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

func assertTaskStateHTTPError(t *testing.T, recorder *httptest.ResponseRecorder, expectedStatus int, expectedErr httpcontext.BusinessError) {
	t.Helper()

	if recorder.Code != expectedStatus {
		t.Fatalf("expected HTTP %d, got %d body=%s", expectedStatus, recorder.Code, recorder.Body.String())
	}

	var response httpcontext.Response
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if response.Code != expectedErr.GetCode() {
		t.Fatalf("expected business code %d, got %d", expectedErr.GetCode(), response.Code)
	}
}

func TestFileLogListReturnsErrorWhenFileTaskLogServiceMissing(t *testing.T) {
	router := newClearTaskLogsRouter(nil)

	req := httptest.NewRequestWithContext(stdctx.Background(), http.MethodGet, "/list?currentPage=1&pageSize=10", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	assertTaskStateHTTPError(t, recorder, http.StatusBadRequest, codeListTasksFailed)
}

func TestFileLogListReturnsErrorWhenFileTaskLogServiceTypedNil(t *testing.T) {
	var service *mockClearTaskLogsService

	router := newClearTaskLogsRouter(service)

	req := httptest.NewRequestWithContext(stdctx.Background(), http.MethodGet, "/list?currentPage=1&pageSize=10", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	assertTaskStateHTTPError(t, recorder, http.StatusBadRequest, codeListTasksFailed)
}

func TestTaskEngineListReturnsErrorWhenTaskEngineMissing(t *testing.T) {
	router := newTaskEngineListRouter(nil)

	req := httptest.NewRequestWithContext(stdctx.Background(), http.MethodGet, "/task_engine", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	assertTaskStateHTTPError(t, recorder, http.StatusBadRequest, codeGetTaskEngineStatsFailed)
}

func TestTaskEngineListReturnsErrorWhenTaskEngineTypedNil(t *testing.T) {
	var taskEngine *mockTaskStateTaskEngine

	router := newTaskEngineListRouter(taskEngine)

	req := httptest.NewRequestWithContext(stdctx.Background(), http.MethodGet, "/task_engine", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	assertTaskStateHTTPError(t, recorder, http.StatusBadRequest, codeGetTaskEngineStatsFailed)
}

func TestTaskEngineListReturnsStatsWhenTaskEngineAvailable(t *testing.T) {
	taskEngine := &mockTaskStateTaskEngine{
		stats: taskengine.TaskStats{
			PendingTasks:   1,
			RunningTasks:   2,
			FailedTasks:    3,
			CompletedTasks: 4,
		},
		runningTasks: []*taskengine.TaskInfo{{ID: "running-1", Status: taskengine.TaskStatusRunning}},
		pendingTasks: []*taskengine.TaskInfo{{ID: "pending-1", Status: taskengine.TaskStatusPending}},
		running:      true,
	}
	router := newTaskEngineListRouter(taskEngine)

	req := httptest.NewRequestWithContext(stdctx.Background(), http.MethodGet, "/task_engine", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected ok, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	var response struct {
		Code int                    `json:"code"`
		Data taskEngineListResponse `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if response.Code != http.StatusOK {
		t.Fatalf("expected business code 200, got %d", response.Code)
	}

	if response.Data.Stats.PendingTasks != 1 || response.Data.Stats.RunningTasks != 2 ||
		response.Data.Stats.FailedTasks != 3 || response.Data.Stats.CompletedTasks != 4 {
		t.Fatalf("expected task stats in response, got %+v", response.Data.Stats)
	}

	if !response.Data.IsRunning {
		t.Fatal("expected task engine running state to be true")
	}

	if len(response.Data.RunningTasks) != 1 || response.Data.RunningTasks[0].ID != "running-1" {
		t.Fatalf("expected one running task, got %+v", response.Data.RunningTasks)
	}

	if len(response.Data.PendingTasks) != 1 || response.Data.PendingTasks[0].ID != "pending-1" {
		t.Fatalf("expected one pending task, got %+v", response.Data.PendingTasks)
	}
}

func TestTaskEngineListRedactsTaskPayloadAndProcessorErrors(t *testing.T) {
	runningPayload := []byte(`{"path":"/movie","accessCode":"abcd","shareAccessCode":"efgh","url":"https://user:pass@example.test/path?token=secret#frag","tokenId":123}`)
	pendingPayload := []byte(`accessToken=plain-secret&filename=public-name`)
	originalRunningPayload := string(runningPayload)
	originalPendingPayload := string(pendingPayload)
	taskEngine := &mockTaskStateTaskEngine{
		runningTasks: []*taskengine.TaskInfo{
			{
				ID:      "running-sensitive",
				Payload: runningPayload,
				Results: []taskengine.ProcessorResult{
					{
						ProcessorID: "processor-sensitive",
						Error:       "failed https://error-user:error-pass@example.test/cb?token=result-secret#frag accessCode=wxyz",
					},
				},
			},
		},
		pendingTasks: []*taskengine.TaskInfo{
			{
				ID:      "pending-sensitive",
				Payload: pendingPayload,
			},
		},
	}
	router := newTaskEngineListRouter(taskEngine)

	req := httptest.NewRequestWithContext(stdctx.Background(), http.MethodGet, "/task_engine", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected ok, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	var response struct {
		Code int                    `json:"code"`
		Data taskEngineListResponse `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if len(response.Data.RunningTasks) != 1 || len(response.Data.PendingTasks) != 1 {
		t.Fatalf("expected one running and one pending task, got running=%d pending=%d",
			len(response.Data.RunningTasks), len(response.Data.PendingTasks))
	}

	runningText := string(response.Data.RunningTasks[0].Payload)
	pendingText := string(response.Data.PendingTasks[0].Payload)

	resultError := response.Data.RunningTasks[0].Results[0].Error
	for name, text := range map[string]string{
		"running payload": runningText,
		"pending payload": pendingText,
		"processor error": resultError,
	} {
		for _, leaked := range []string{
			"abcd",
			"efgh",
			"secret",
			"user:pass",
			"plain-secret",
			"result-secret",
			"wxyz",
			"error-user",
			"error-pass",
			"frag",
		} {
			if strings.Contains(text, leaked) {
				t.Fatalf("expected %s to redact %q, got %s", name, leaked, text)
			}
		}

		if !strings.Contains(text, utils.RedactedSecret) {
			t.Fatalf("expected %s to contain redaction marker, got %s", name, text)
		}
	}

	if !strings.Contains(runningText, `"/movie"`) || !strings.Contains(runningText, `"tokenId":123`) {
		t.Fatalf("expected non-sensitive payload fields to remain, got %s", runningText)
	}

	var redactedPayload map[string]interface{}
	if err := json.Unmarshal([]byte(runningText), &redactedPayload); err != nil {
		t.Fatalf("expected redacted JSON payload to stay valid, got %v payload=%s", err, runningText)
	}

	if redactedPayload["path"] != "/movie" {
		t.Fatalf("expected non-sensitive path to remain, got %#v", redactedPayload["path"])
	}

	if redactedPayload["accessCode"] != utils.RedactedSecret ||
		redactedPayload["shareAccessCode"] != utils.RedactedSecret {
		t.Fatalf("expected access codes to be redacted, got %#v", redactedPayload)
	}

	if redactedPayload["tokenId"] != float64(123) {
		t.Fatalf("expected tokenId metadata to remain, got %#v", redactedPayload["tokenId"])
	}

	if !strings.Contains(pendingText, "filename=public-name") {
		t.Fatalf("expected non-sensitive pending payload value to remain, got %s", pendingText)
	}

	if string(taskEngine.runningTasks[0].Payload) != originalRunningPayload {
		t.Fatalf("expected original running payload not to be mutated, got %s", string(taskEngine.runningTasks[0].Payload))
	}

	if string(taskEngine.pendingTasks[0].Payload) != originalPendingPayload {
		t.Fatalf("expected original pending payload not to be mutated, got %s", string(taskEngine.pendingTasks[0].Payload))
	}

	if taskEngine.runningTasks[0].Results[0].Error == resultError {
		t.Fatal("expected processor error response to be redacted without mutating original task")
	}
}

func TestTaskEngineListRedactsBinaryPayload(t *testing.T) {
	payload := []byte{0xff, 0xfe, 0xfd}
	taskEngine := &mockTaskStateTaskEngine{
		runningTasks: []*taskengine.TaskInfo{
			{
				ID:      "running-binary",
				Payload: payload,
			},
		},
	}
	router := newTaskEngineListRouter(taskEngine)

	req := httptest.NewRequestWithContext(stdctx.Background(), http.MethodGet, "/task_engine", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected ok, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	var response struct {
		Data taskEngineListResponse `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if got := string(response.Data.RunningTasks[0].Payload); got != utils.RedactedSecret {
		t.Fatalf("expected binary payload to be redacted, got %q", got)
	}

	if string(taskEngine.runningTasks[0].Payload) != string(payload) {
		t.Fatalf("expected original binary payload not to be mutated, got %#v", taskEngine.runningTasks[0].Payload)
	}
}

func TestClearTaskLogsReturnsErrorWhenFileTaskLogServiceMissing(t *testing.T) {
	router := newClearTaskLogsRouter(nil)

	recorder := performClearTaskLogsRequest(t, router, "")

	assertTaskStateHTTPError(t, recorder, http.StatusBadRequest, codeClearTaskLogsFailed)
}

func TestFileLogListSkipsNilRowsBeforeNormalizing(t *testing.T) {
	gin.SetMode(gin.TestMode)

	beginAt := time.Now().Add(-2 * time.Second)
	service := &mockClearTaskLogsService{
		count: 3,
		list: []*models.FileTaskLog{
			nil,
			{
				ID:        1,
				Title:     "scan",
				Type:      "file_scan",
				BeginAt:   beginAt,
				Status:    models.StatusCompleted,
				Completed: 1,
				Total:     3,
			},
			nil,
		},
	}

	router := gin.New()
	wrapper := httpcontext.NewHandlerFuncWrapper(zap.NewNop())
	router.GET("/list", wrapper.Wrap(NewHandler(nil, service).FileLogList()))

	req := httptest.NewRequestWithContext(stdctx.Background(), http.MethodGet, "/list", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected ok, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	var response struct {
		Data fileLogListResponse `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if response.Data.Total != 3 || len(response.Data.Data) != 1 {
		t.Fatalf("expected paginated total and one non-nil task log, got total=%d len=%d", response.Data.Total, len(response.Data.Data))
	}

	got := response.Data.Data[0]
	if got.ID != 1 {
		t.Fatalf("expected task log id 1, got %d", got.ID)
	}

	if got.Completed != got.Total {
		t.Fatalf("expected completed count normalized to total, got completed=%d total=%d", got.Completed, got.Total)
	}

	if got.Duration <= 0 {
		t.Fatalf("expected duration to be filled, got %d", got.Duration)
	}
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
