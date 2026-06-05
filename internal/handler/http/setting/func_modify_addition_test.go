package setting

import (
	stdctx "context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	appContext "github.com/xxcheng123/cloudpan189-share/internal/framework/context"
	"github.com/xxcheng123/cloudpan189-share/internal/framework/httpcontext"
	"github.com/xxcheng123/cloudpan189-share/internal/pkgs/taskengine"
	"github.com/xxcheng123/cloudpan189-share/internal/pkgs/utils"
	"github.com/xxcheng123/cloudpan189-share/internal/repository/models"
	settingSvc "github.com/xxcheng123/cloudpan189-share/internal/services/setting"
	"go.uber.org/zap"
)

type mockModifyAdditionSettingService struct {
	settingSvc.Service
	updateErr error
	queryErr  error
	returnNil bool
	fields    []utils.Field
	addition  *models.SettingAddition
}

func (m *mockModifyAdditionSettingService) Query(appContext.Context) (*models.Setting, error) {
	if m.queryErr != nil {
		return nil, m.queryErr
	}

	if m.returnNil {
		return nil, nil
	}

	addition := models.SettingAddition{}
	if m.addition != nil {
		addition = *m.addition
	} else {
		addition.WorkerCount = 4
	}

	addition.ApplyDefaultsForWrite()

	return &models.Setting{Addition: addition}, nil
}

func (m *mockModifyAdditionSettingService) Update(_ appContext.Context, fields ...utils.Field) error {
	m.fields = fields

	return m.updateErr
}

type mockModifyAdditionTaskEngine struct {
	taskengine.TaskEngine
	setWorkerCountCalls int
	workerCount         int
}

func (m *mockModifyAdditionTaskEngine) Start() error {
	return nil
}

func (m *mockModifyAdditionTaskEngine) Stop() error {
	return nil
}

func (m *mockModifyAdditionTaskEngine) IsRunning() bool {
	return false
}

func (m *mockModifyAdditionTaskEngine) RegisterProcessor(taskengine.Topic, taskengine.MessageProcessor) error {
	return nil
}

func (m *mockModifyAdditionTaskEngine) PushMessage(stdctx.Context, taskengine.Topic, []byte) error {
	return nil
}

func (m *mockModifyAdditionTaskEngine) GetStats() taskengine.TaskStats {
	return taskengine.TaskStats{}
}

func (m *mockModifyAdditionTaskEngine) GetRunningTasks() []*taskengine.TaskInfo {
	return nil
}

func (m *mockModifyAdditionTaskEngine) GetPendingTasks() []*taskengine.TaskInfo {
	return nil
}

func (m *mockModifyAdditionTaskEngine) SetWorkerCount(count int) error {
	m.setWorkerCountCalls++
	m.workerCount = count

	return nil
}

func performModifyAdditionRequest(
	t *testing.T,
	settingService settingSvc.Service,
	taskEngine taskengine.TaskEngine,
	body string,
) *httptest.ResponseRecorder {
	t.Helper()

	gin.SetMode(gin.TestMode)

	router := gin.New()
	wrapper := httpcontext.NewHandlerFuncWrapper(zap.NewNop())
	router.POST("/modify_addition", wrapper.Wrap(NewHandler(nil, settingService, taskEngine).ModifyAddition()))

	req := httptest.NewRequestWithContext(
		stdctx.Background(),
		http.MethodPost,
		"/modify_addition",
		strings.NewReader(body),
	)
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	return recorder
}

func TestModifyAdditionDoesNotHotUpdateWorkerCountWhenPersistFails(t *testing.T) {
	settingService := &mockModifyAdditionSettingService{updateErr: errors.New("db down")}
	taskEngine := &mockModifyAdditionTaskEngine{}

	recorder := performModifyAdditionRequest(t, settingService, taskEngine, `{"workerCount":8}`)

	if recorder.Code == http.StatusOK {
		t.Fatalf("expected update failure, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	if taskEngine.setWorkerCountCalls != 0 {
		t.Fatalf("expected worker count not to hot update on persist failure, got %d calls", taskEngine.setWorkerCountCalls)
	}
}

func TestModifyAdditionReturnsQueryFailedWhenSettingQueryReturnsNil(t *testing.T) {
	settingService := &mockModifyAdditionSettingService{returnNil: true}
	taskEngine := &mockModifyAdditionTaskEngine{}

	recorder := performModifyAdditionRequest(t, settingService, taskEngine, `{"workerCount":8}`)

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

	if len(settingService.fields) != 0 {
		t.Fatalf("expected missing setting not to persist fields, got %#v", settingService.fields)
	}

	if taskEngine.setWorkerCountCalls != 0 {
		t.Fatalf("expected missing setting not to hot update worker count, got %d calls", taskEngine.setWorkerCountCalls)
	}
}

func TestModifyAdditionHotUpdatesWorkerCountAfterPersistSucceeds(t *testing.T) {
	settingService := &mockModifyAdditionSettingService{}
	taskEngine := &mockModifyAdditionTaskEngine{}

	recorder := performModifyAdditionRequest(t, settingService, taskEngine, `{"workerCount":8}`)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected success, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	if len(settingService.fields) != 1 || settingService.fields[0].Key != "addition" {
		t.Fatalf("expected addition update field, got %#v", settingService.fields)
	}

	if taskEngine.setWorkerCountCalls != 1 || taskEngine.workerCount != 8 {
		t.Fatalf("expected worker count hot update to 8, calls=%d count=%d", taskEngine.setWorkerCountCalls, taskEngine.workerCount)
	}
}

func TestModifyAdditionSkipsHotUpdateWhenTaskEngineTypedNil(t *testing.T) {
	settingService := &mockModifyAdditionSettingService{}

	var taskEngine *mockModifyAdditionTaskEngine

	recorder := performModifyAdditionRequest(t, settingService, taskEngine, `{"workerCount":8}`)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected success, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	if len(settingService.fields) != 1 || settingService.fields[0].Key != "addition" {
		t.Fatalf("expected addition update field, got %#v", settingService.fields)
	}

	addition, ok := settingService.fields[0].Value.(models.SettingAddition)
	if !ok {
		t.Fatalf("expected SettingAddition update field, got %T", settingService.fields[0].Value)
	}

	if addition.WorkerCount != 8 {
		t.Fatalf("expected persisted worker count 8, got %d", addition.WorkerCount)
	}
}

func TestModifyAdditionUpdatesLocalProxyURL(t *testing.T) {
	settingService := &mockModifyAdditionSettingService{}

	recorder := performModifyAdditionRequest(
		t,
		settingService,
		nil,
		`{"localProxyURL":" http://127.0.0.1:7890 "}`,
	)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected success, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	if len(settingService.fields) != 1 || settingService.fields[0].Key != "addition" {
		t.Fatalf("expected addition update field, got %#v", settingService.fields)
	}

	addition, ok := settingService.fields[0].Value.(models.SettingAddition)
	if !ok {
		t.Fatalf("expected SettingAddition update field, got %T", settingService.fields[0].Value)
	}

	if addition.LocalProxyURL != "http://127.0.0.1:7890" {
		t.Fatalf("expected trimmed local proxy URL, got %q", addition.LocalProxyURL)
	}
}

func TestModifyAdditionClearsLocalProxyURL(t *testing.T) {
	settingService := &mockModifyAdditionSettingService{
		addition: &models.SettingAddition{
			LocalProxyURL: "http://127.0.0.1:7890",
		},
	}

	recorder := performModifyAdditionRequest(
		t,
		settingService,
		nil,
		`{"localProxyURL":"   "}`,
	)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected success, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	addition, ok := settingService.fields[0].Value.(models.SettingAddition)
	if !ok {
		t.Fatalf("expected SettingAddition update field, got %T", settingService.fields[0].Value)
	}

	if addition.LocalProxyURL != "" {
		t.Fatalf("expected local proxy URL cleared, got %q", addition.LocalProxyURL)
	}
}

func TestModifyAdditionRejectsInvalidLocalProxyURL(t *testing.T) {
	settingService := &mockModifyAdditionSettingService{}
	rawProxyURL := "http://proxy-user:proxy-pass@%zz?token=secret-token#session=secret-session"

	recorder := performModifyAdditionRequest(
		t,
		settingService,
		nil,
		`{"localProxyURL":`+strconv.Quote(rawProxyURL)+`}`,
	)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected bad request, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	if len(settingService.fields) != 0 {
		t.Fatalf("expected invalid local proxy URL not to persist fields, got %#v", settingService.fields)
	}

	for _, leaked := range []string{"proxy-user", "proxy-pass", "secret-token", "secret-session"} {
		if strings.Contains(recorder.Body.String(), leaked) {
			t.Fatalf("expected %q not to leak in response body %q", leaked, recorder.Body.String())
		}
	}
}

func TestModifyAdditionRejectsOversizedMultipleStreamChunkSize(t *testing.T) {
	settingService := &mockModifyAdditionSettingService{}

	recorder := performModifyAdditionRequest(
		t,
		settingService,
		nil,
		`{"multipleStreamChunkSize":67108865}`,
	)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected bad request, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	if len(settingService.fields) != 0 {
		t.Fatalf("expected invalid chunk size not to persist fields, got %#v", settingService.fields)
	}
}

func TestModifyAdditionPreservesLocalProxyURLWhenRedactedPlaceholderSubmitted(t *testing.T) {
	rawProxyURL := "http://proxy-user:proxy-pass@example.test:8080/proxy?token=secret-token#session=secret-fragment"
	settingService := &mockModifyAdditionSettingService{
		addition: &models.SettingAddition{
			LocalProxy:    true,
			LocalProxyURL: rawProxyURL,
		},
	}

	recorder := performModifyAdditionRequest(
		t,
		settingService,
		nil,
		`{"localProxyURL":`+strconv.Quote(utils.RedactURLForLog(rawProxyURL))+`,"workerCount":8}`,
	)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected success, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	if len(settingService.fields) != 1 || settingService.fields[0].Key != "addition" {
		t.Fatalf("expected addition update field, got %#v", settingService.fields)
	}

	addition, ok := settingService.fields[0].Value.(models.SettingAddition)
	if !ok {
		t.Fatalf("expected SettingAddition update field, got %T", settingService.fields[0].Value)
	}

	if addition.LocalProxyURL != rawProxyURL {
		t.Fatalf("expected local proxy URL preserved, got %q", addition.LocalProxyURL)
	}

	if addition.WorkerCount != 8 {
		t.Fatalf("expected other fields still updated, got workerCount=%d", addition.WorkerCount)
	}
}
