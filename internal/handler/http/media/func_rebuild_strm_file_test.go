package media

import (
	stdctx "context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	appContext "github.com/xxcheng123/cloudpan189-share/internal/framework/context"
	"github.com/xxcheng123/cloudpan189-share/internal/framework/httpcontext"
	"github.com/xxcheng123/cloudpan189-share/internal/pkgs/taskengine"
	"github.com/xxcheng123/cloudpan189-share/internal/repository/models"
	mediaconfigSvi "github.com/xxcheng123/cloudpan189-share/internal/services/mediaconfig"
	mountpointSvi "github.com/xxcheng123/cloudpan189-share/internal/services/mountpoint"
	"github.com/xxcheng123/cloudpan189-share/internal/types/topic"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type mockRebuildMediaConfigService struct {
	mediaconfigSvi.Service
	cfg       *models.MediaConfig
	err       error
	returnNil bool
}

func (m *mockRebuildMediaConfigService) Query(ctx appContext.Context) (*models.MediaConfig, error) {
	if m.err != nil {
		return nil, m.err
	}

	if m.returnNil {
		return nil, nil
	}

	if m.cfg != nil {
		return m.cfg, nil
	}

	return &models.MediaConfig{Enable: true}, nil
}

type mockRebuildMountPointService struct {
	mountpointSvi.Service
	list []*models.MountPoint
	req  *mountpointSvi.ListRequest
}

func (m *mockRebuildMountPointService) List(ctx appContext.Context, req *mountpointSvi.ListRequest) ([]*models.MountPoint, error) {
	m.req = req

	return m.list, nil
}

type mockRebuildTaskEngine struct {
	taskengine.TaskEngine
	payloads [][]byte
}

func (m *mockRebuildTaskEngine) PushMessage(ctx stdctx.Context, topic taskengine.Topic, payload []byte) error {
	m.payloads = append(m.payloads, payload)

	return nil
}

func TestRebuildStrmFileRejectsMalformedJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	wrapper := httpcontext.NewHandlerFuncWrapper(zap.NewNop())
	router.POST("/rebuild", wrapper.Wrap(NewHandler(nil, nil, nil, nil, nil, nil, nil).RebuildStrmFile()))

	req := httptest.NewRequestWithContext(stdctx.Background(), http.MethodPost, "/rebuild", strings.NewReader("{"))
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected bad request, got %d", recorder.Code)
	}
}

func TestRebuildStrmFileRejectsInvalidMountPointID(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	wrapper := httpcontext.NewHandlerFuncWrapper(zap.NewNop())
	router.POST("/rebuild", wrapper.Wrap(NewHandler(
		&mockRebuildMediaConfigService{},
		nil,
		nil,
		nil,
		nil,
		nil,
		&mockRebuildTaskEngine{},
	).RebuildStrmFile()))

	req := httptest.NewRequestWithContext(stdctx.Background(), http.MethodPost, "/rebuild", strings.NewReader(`{"mountPointIds":[0]}`))
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected bad request, got %d body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestRebuildStrmFileReturnsNotFoundWhenConfigMissing(t *testing.T) {
	gin.SetMode(gin.TestMode)

	taskEngine := &mockRebuildTaskEngine{}
	router := gin.New()
	wrapper := httpcontext.NewHandlerFuncWrapper(zap.NewNop())
	router.POST("/rebuild", wrapper.Wrap(NewHandler(
		&mockRebuildMediaConfigService{err: gorm.ErrRecordNotFound},
		nil,
		nil,
		nil,
		nil,
		nil,
		taskEngine,
	).RebuildStrmFile()))

	req := httptest.NewRequestWithContext(stdctx.Background(), http.MethodPost, "/rebuild", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("expected not found, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	var response httpcontext.Response
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if response.Code != codeConfigNotInit.GetCode() {
		t.Fatalf("expected business code %d, got %d", codeConfigNotInit.GetCode(), response.Code)
	}

	if len(taskEngine.payloads) != 0 {
		t.Fatalf("expected no rebuild task, got %d", len(taskEngine.payloads))
	}
}

func TestRebuildStrmFileReturnsNotFoundWhenConfigQueryReturnsNil(t *testing.T) {
	gin.SetMode(gin.TestMode)

	taskEngine := &mockRebuildTaskEngine{}
	router := gin.New()
	wrapper := httpcontext.NewHandlerFuncWrapper(zap.NewNop())
	router.POST("/rebuild", wrapper.Wrap(NewHandler(
		&mockRebuildMediaConfigService{returnNil: true},
		nil,
		nil,
		nil,
		nil,
		nil,
		taskEngine,
	).RebuildStrmFile()))

	req := httptest.NewRequestWithContext(stdctx.Background(), http.MethodPost, "/rebuild", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("expected not found, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	var response httpcontext.Response
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if response.Code != codeConfigNotInit.GetCode() {
		t.Fatalf("expected business code %d, got %d", codeConfigNotInit.GetCode(), response.Code)
	}

	if len(taskEngine.payloads) != 0 {
		t.Fatalf("expected no rebuild task, got %d", len(taskEngine.payloads))
	}
}

func TestRebuildStrmFileReturnsQueryFailedWhenConfigQueryFails(t *testing.T) {
	gin.SetMode(gin.TestMode)

	taskEngine := &mockRebuildTaskEngine{}
	router := gin.New()
	wrapper := httpcontext.NewHandlerFuncWrapper(zap.NewNop())
	router.POST("/rebuild", wrapper.Wrap(NewHandler(
		&mockRebuildMediaConfigService{err: errors.New("db unavailable")},
		nil,
		nil,
		nil,
		nil,
		nil,
		taskEngine,
	).RebuildStrmFile()))

	req := httptest.NewRequestWithContext(stdctx.Background(), http.MethodPost, "/rebuild", strings.NewReader(`{}`))
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

	if response.Code != codeConfigQueryFailed.GetCode() {
		t.Fatalf("expected business code %d, got %d", codeConfigQueryFailed.GetCode(), response.Code)
	}

	if len(taskEngine.payloads) != 0 {
		t.Fatalf("expected no rebuild task, got %d", len(taskEngine.payloads))
	}
}

func TestRebuildStrmFileReturnsMediaNotEnabledWhenConfigDisabled(t *testing.T) {
	gin.SetMode(gin.TestMode)

	taskEngine := &mockRebuildTaskEngine{}
	router := gin.New()
	wrapper := httpcontext.NewHandlerFuncWrapper(zap.NewNop())
	router.POST("/rebuild", wrapper.Wrap(NewHandler(
		&mockRebuildMediaConfigService{cfg: &models.MediaConfig{Enable: false}},
		nil,
		nil,
		nil,
		nil,
		nil,
		taskEngine,
	).RebuildStrmFile()))

	req := httptest.NewRequestWithContext(stdctx.Background(), http.MethodPost, "/rebuild", strings.NewReader(`{}`))
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
		t.Fatalf("expected no rebuild task, got %d", len(taskEngine.payloads))
	}
}

func TestRebuildStrmFileEmptyMountPointIDsDoesNotTriggerFullRebuild(t *testing.T) {
	gin.SetMode(gin.TestMode)

	taskEngine := &mockRebuildTaskEngine{}
	mountPointService := &mockRebuildMountPointService{list: []*models.MountPoint{
		{ID: 7, FileId: 11, FullPath: "/movie"},
	}}
	router := gin.New()
	wrapper := httpcontext.NewHandlerFuncWrapper(zap.NewNop())
	router.POST("/rebuild", wrapper.Wrap(NewHandler(
		&mockRebuildMediaConfigService{},
		nil,
		mountPointService,
		nil,
		nil,
		nil,
		taskEngine,
	).RebuildStrmFile()))

	req := httptest.NewRequestWithContext(stdctx.Background(), http.MethodPost, "/rebuild", strings.NewReader(`{"mountPointIds":[]}`))
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected ok, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	var response struct {
		Data rebuildStrmResponse `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if response.Data.Total != 0 || response.Data.Success != 0 || response.Data.Failed != 0 {
		t.Fatalf("expected empty scoped rebuild response, got %+v", response.Data)
	}

	if len(taskEngine.payloads) != 0 {
		t.Fatalf("expected no rebuild task for explicit empty mountPointIds, got %d", len(taskEngine.payloads))
	}

	if mountPointService.req != nil {
		t.Fatalf("expected explicit empty mountPointIds to skip mount point query, got %+v", mountPointService.req)
	}
}

func TestRebuildStrmFileMissingMountPointIDsTriggersFullRebuild(t *testing.T) {
	gin.SetMode(gin.TestMode)

	taskEngine := &mockRebuildTaskEngine{}
	router := gin.New()
	wrapper := httpcontext.NewHandlerFuncWrapper(zap.NewNop())
	router.POST("/rebuild", wrapper.Wrap(NewHandler(
		&mockRebuildMediaConfigService{},
		nil,
		nil,
		nil,
		nil,
		nil,
		taskEngine,
	).RebuildStrmFile()))

	req := httptest.NewRequestWithContext(stdctx.Background(), http.MethodPost, "/rebuild", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected ok, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	var response struct {
		Data rebuildStrmResponse `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if response.Data.Total != 1 || response.Data.Success != 1 || response.Data.Failed != 0 {
		t.Fatalf("expected full rebuild response, got %+v", response.Data)
	}

	if len(taskEngine.payloads) != 1 {
		t.Fatalf("expected one full rebuild task, got %d", len(taskEngine.payloads))
	}

	var payload topic.MediaRebuildStrmFileRequest
	if err := json.Unmarshal(taskEngine.payloads[0], &payload); err != nil {
		t.Fatalf("decode full rebuild payload: %v", err)
	}
}

func TestRebuildStrmFileCountsMissingMountPointsAsFailed(t *testing.T) {
	gin.SetMode(gin.TestMode)

	taskEngine := &mockRebuildTaskEngine{}
	mountPointService := &mockRebuildMountPointService{list: []*models.MountPoint{
		{ID: 7, FileId: 11, FullPath: "/movie"},
	}}
	router := gin.New()
	wrapper := httpcontext.NewHandlerFuncWrapper(zap.NewNop())
	router.POST("/rebuild", wrapper.Wrap(NewHandler(
		&mockRebuildMediaConfigService{},
		nil,
		mountPointService,
		nil,
		nil,
		nil,
		taskEngine,
	).RebuildStrmFile()))

	req := httptest.NewRequestWithContext(stdctx.Background(), http.MethodPost, "/rebuild", strings.NewReader(`{"mountPointIds":[7,22,7]}`))
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected ok, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	var response struct {
		Data rebuildStrmResponse `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if response.Data.Total != 2 || response.Data.Success != 1 || response.Data.Failed != 1 {
		t.Fatalf("expected total=2 success=1 failed=1, got %+v", response.Data)
	}

	if len(taskEngine.payloads) != 1 {
		t.Fatalf("expected one rebuild task, got %d", len(taskEngine.payloads))
	}

	var payload topic.MediaRebuildStrmFileByMountPointRequest
	if err := json.Unmarshal(taskEngine.payloads[0], &payload); err != nil {
		t.Fatalf("decode task payload: %v", err)
	}

	if payload.MountPointFileId != 11 || payload.MountPointPath != "/movie" {
		t.Fatalf("expected payload to use mount point file id and path, got %+v", payload)
	}

	if mountPointService.req == nil || !mountPointService.req.NoPaginate || !mountPointService.req.IsAdmin {
		t.Fatalf("expected admin no-paginate mount point request, got %+v", mountPointService.req)
	}
}

func TestRebuildStrmFileSkipsNilMountPointsInScopedRebuild(t *testing.T) {
	gin.SetMode(gin.TestMode)

	taskEngine := &mockRebuildTaskEngine{}
	mountPointService := &mockRebuildMountPointService{list: []*models.MountPoint{
		nil,
		{ID: 7, FileId: 11, FullPath: "/movie"},
	}}
	router := gin.New()
	wrapper := httpcontext.NewHandlerFuncWrapper(zap.NewNop())
	router.POST("/rebuild", wrapper.Wrap(NewHandler(
		&mockRebuildMediaConfigService{},
		nil,
		mountPointService,
		nil,
		nil,
		nil,
		taskEngine,
	).RebuildStrmFile()))

	req := httptest.NewRequestWithContext(stdctx.Background(), http.MethodPost, "/rebuild", strings.NewReader(`{"mountPointIds":[7,22]}`))
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected ok, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	var response struct {
		Data rebuildStrmResponse `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if response.Data.Total != 2 || response.Data.Success != 1 || response.Data.Failed != 1 {
		t.Fatalf("expected total=2 success=1 failed=1, got %+v", response.Data)
	}

	if len(taskEngine.payloads) != 1 {
		t.Fatalf("expected one rebuild task, got %d", len(taskEngine.payloads))
	}

	var payload topic.MediaRebuildStrmFileByMountPointRequest
	if err := json.Unmarshal(taskEngine.payloads[0], &payload); err != nil {
		t.Fatalf("decode task payload: %v", err)
	}

	if payload.MountPointFileId != 11 || payload.MountPointPath != "/movie" {
		t.Fatalf("expected payload to use mount point file id and path, got %+v", payload)
	}
}

func TestRebuildStrmFileReturnsNotFoundWhenAllScopedMountPointsMissing(t *testing.T) {
	gin.SetMode(gin.TestMode)

	taskEngine := &mockRebuildTaskEngine{}
	mountPointService := &mockRebuildMountPointService{list: []*models.MountPoint{
		{ID: 7, FileId: 11, FullPath: "/movie"},
	}}
	router := gin.New()
	wrapper := httpcontext.NewHandlerFuncWrapper(zap.NewNop())
	router.POST("/rebuild", wrapper.Wrap(NewHandler(
		&mockRebuildMediaConfigService{},
		nil,
		mountPointService,
		nil,
		nil,
		nil,
		taskEngine,
	).RebuildStrmFile()))

	req := httptest.NewRequestWithContext(stdctx.Background(), http.MethodPost, "/rebuild", strings.NewReader(`{"mountPointIds":[22,33,22]}`))
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("expected not found, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	var response httpcontext.Response
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if response.Code != codeRebuildMountPointNotFound.GetCode() {
		t.Fatalf("expected business code %d, got %d", codeRebuildMountPointNotFound.GetCode(), response.Code)
	}

	if len(taskEngine.payloads) != 0 {
		t.Fatalf("expected no rebuild task, got %d", len(taskEngine.payloads))
	}

	if mountPointService.req == nil || !mountPointService.req.NoPaginate || !mountPointService.req.IsAdmin {
		t.Fatalf("expected admin no-paginate mount point request, got %+v", mountPointService.req)
	}
}
