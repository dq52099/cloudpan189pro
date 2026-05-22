package storage

import (
	stdctx "context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/xxcheng123/cloudpan189-share/internal/consts"
	appContext "github.com/xxcheng123/cloudpan189-share/internal/framework/context"
	"github.com/xxcheng123/cloudpan189-share/internal/framework/httpcontext"
	"github.com/xxcheng123/cloudpan189-share/internal/pkgs/taskengine"
	"github.com/xxcheng123/cloudpan189-share/internal/repository/models"
	filetasklogSvi "github.com/xxcheng123/cloudpan189-share/internal/services/filetasklog"
	mountPointSvi "github.com/xxcheng123/cloudpan189-share/internal/services/mountpoint"
	"github.com/xxcheng123/cloudpan189-share/internal/types/topic"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type mockBatchDeleteTaskEngine struct {
	taskengine.TaskEngine
	payloads [][]byte
	paths    []any
	pushErr  error
}

func (m *mockBatchDeleteTaskEngine) PushMessage(ctx stdctx.Context, taskTopic taskengine.Topic, payload []byte) error {
	if m.pushErr != nil {
		return m.pushErr
	}

	m.payloads = append(m.payloads, payload)
	m.paths = append(m.paths, ctx.Value(consts.CtxKeyFullPath))

	return nil
}

type mockBatchDeleteMountPointService struct {
	mountPointSvi.Service
	mountPoints              map[int64]*models.MountPoint
	mountPointsByPath        map[string]*models.MountPoint
	queries                  []int64
	enableAutoRefreshCalls   []int64
	updateRefreshConfigCalls []int64
}

func (m *mockBatchDeleteMountPointService) Query(ctx appContext.Context, fileID int64) (*models.MountPoint, error) {
	m.queries = append(m.queries, fileID)

	mountPoint, ok := m.mountPoints[fileID]
	if !ok {
		return nil, gorm.ErrRecordNotFound
	}

	return mountPoint, nil
}

func (m *mockBatchDeleteMountPointService) QueryByPath(ctx appContext.Context, fullPath string) (*models.MountPoint, error) {
	mountPoint, ok := m.mountPointsByPath[fullPath]
	if !ok {
		return nil, nil
	}

	return mountPoint, nil
}

func (m *mockBatchDeleteMountPointService) EnableAutoRefresh(ctx appContext.Context, fileID int64, enable bool) error {
	m.enableAutoRefreshCalls = append(m.enableAutoRefreshCalls, fileID)

	return nil
}

func (m *mockBatchDeleteMountPointService) UpdateRefreshConfig(ctx appContext.Context, fileID int64, config mountPointSvi.RefreshConfig) error {
	m.updateRefreshConfigCalls = append(m.updateRefreshConfigCalls, fileID)

	return nil
}

type mockBatchDeleteFileTaskLogService struct {
	filetasklogSvi.Service
}

func (m *mockBatchDeleteFileTaskLogService) Create(
	ctx appContext.Context,
	typ string,
	title string,
	opts ...filetasklogSvi.NewOptionFunc,
) (*filetasklogSvi.Tracker, error) {
	return nil, nil
}

func TestBatchDeleteDeduplicatesIDsBeforeQueueingTasks(t *testing.T) {
	gin.SetMode(gin.TestMode)

	taskEngine := &mockBatchDeleteTaskEngine{}
	mountPointService := &mockBatchDeleteMountPointService{
		mountPoints: map[int64]*models.MountPoint{
			11: {FileId: 11, FullPath: "/movies", CreatorUserID: 100},
			22: {FileId: 22, FullPath: "/series", CreatorUserID: 100},
		},
	}

	router := gin.New()
	router.Use(func(ctx *gin.Context) {
		ctx.Set(consts.CtxKeyUserId, int64(100))
		ctx.Set(consts.CtxKeyIsAdmin, false)
	})

	wrapper := httpcontext.NewHandlerFuncWrapper(zap.NewNop())
	router.POST("/batch_delete", wrapper.Wrap(NewHandler(
		taskEngine,
		nil,
		nil,
		nil,
		mountPointService,
		&mockBatchDeleteFileTaskLogService{},
		nil,
		nil,
		nil,
		nil,
	).BatchDelete()))

	req := httptest.NewRequestWithContext(stdctx.Background(), http.MethodPost, "/batch_delete", strings.NewReader(`{"ids":[11,11,22,33]}`))
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected ok, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	if got, want := mountPointService.queries, []int64{11, 22, 33}; !int64SlicesEqual(got, want) {
		t.Fatalf("expected deduplicated queries %v, got %v", want, got)
	}

	if len(taskEngine.payloads) != 2 {
		t.Fatalf("expected 2 queued tasks, got %d", len(taskEngine.payloads))
	}

	var firstTask topic.FileBatchDeleteRequest
	if err := json.Unmarshal(taskEngine.payloads[0], &firstTask); err != nil {
		t.Fatal(err)
	}

	if got, want := firstTask.IDs, []int64{11}; !int64SlicesEqual(got, want) {
		t.Fatalf("expected first queued task IDs %v, got %v", want, got)
	}

	var response struct {
		Code int                 `json:"code"`
		Data batchDeleteResponse `json:"data"`
	}

	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}

	if response.Data.Total != 3 || response.Data.Success != 2 || response.Data.Failed != 1 {
		t.Fatalf("unexpected response data: %+v", response.Data)
	}
}

func TestBatchDeleteAllowsDuplicateIDsBeyondRawLimit(t *testing.T) {
	gin.SetMode(gin.TestMode)

	taskEngine := &mockBatchDeleteTaskEngine{}
	mountPointService := &mockBatchDeleteMountPointService{
		mountPoints: map[int64]*models.MountPoint{
			11: {FileId: 11, FullPath: "/movies", CreatorUserID: 100},
		},
	}

	router := gin.New()
	router.Use(func(ctx *gin.Context) {
		ctx.Set(consts.CtxKeyUserId, int64(100))
		ctx.Set(consts.CtxKeyIsAdmin, false)
	})

	wrapper := httpcontext.NewHandlerFuncWrapper(zap.NewNop())
	router.POST("/batch_delete", wrapper.Wrap(NewHandler(
		taskEngine,
		nil,
		nil,
		nil,
		mountPointService,
		&mockBatchDeleteFileTaskLogService{},
		nil,
		nil,
		nil,
		nil,
	).BatchDelete()))

	ids := make([]string, maxBatchIDs+1)
	for i := range ids {
		ids[i] = "11"
	}

	req := httptest.NewRequestWithContext(
		stdctx.Background(),
		http.MethodPost,
		"/batch_delete",
		strings.NewReader(fmt.Sprintf(`{"ids":[%s]}`, strings.Join(ids, ","))),
	)
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected ok, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	if got, want := mountPointService.queries, []int64{11}; !int64SlicesEqual(got, want) {
		t.Fatalf("expected one deduplicated query %v, got %v", want, got)
	}

	var response struct {
		Data batchDeleteResponse `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}

	if response.Data.Total != 1 || response.Data.Success != 1 || response.Data.Failed != 0 {
		t.Fatalf("unexpected response data: %+v", response.Data)
	}
}

func TestBatchDeleteTaskLogRecordsDispatchFailures(t *testing.T) {
	gin.SetMode(gin.TestMode)

	taskEngine := &mockBatchDeleteTaskEngine{pushErr: errors.New("queue unavailable")}
	mountPointService := &mockBatchDeleteMountPointService{
		mountPoints: map[int64]*models.MountPoint{
			11: {FileId: 1101, FullPath: "/movies", CreatorUserID: 100},
			22: {FileId: 2201, FullPath: "/series", CreatorUserID: 200},
		},
	}
	taskLogDB := setupStorageTaskLogTestDB(t)
	fileTaskLogService := filetasklogSvi.NewService(taskLogDB)

	router := gin.New()
	router.Use(func(ctx *gin.Context) {
		ctx.Set(consts.CtxKeyUserId, int64(100))
		ctx.Set(consts.CtxKeyIsAdmin, false)
	})

	wrapper := httpcontext.NewHandlerFuncWrapper(zap.NewNop())
	router.POST("/batch_delete", wrapper.Wrap(NewHandler(
		taskEngine,
		nil,
		nil,
		nil,
		mountPointService,
		fileTaskLogService,
		nil,
		nil,
		nil,
		nil,
	).BatchDelete()))

	req := httptest.NewRequestWithContext(stdctx.Background(), http.MethodPost, "/batch_delete", strings.NewReader(`{"ids":[11,22,33]}`))
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected ok, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	var response struct {
		Data batchDeleteResponse `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}

	if response.Data.Total != 3 || response.Data.Success != 0 || response.Data.Failed != 3 {
		t.Fatalf("unexpected response data: %+v", response.Data)
	}

	var log models.FileTaskLog
	if err := taskLogDB.db.First(&log).Error; err != nil {
		t.Fatalf("query task log: %v", err)
	}

	if log.Total != 3 || log.Completed != 0 || log.Failed != 3 {
		t.Fatalf("expected requested total=3 completed=0 failed=3, got total=%d completed=%d failed=%d", log.Total, log.Completed, log.Failed)
	}

	if log.Status != models.StatusFailed {
		t.Fatalf("expected failed parent task status, got %q", log.Status)
	}
}

func TestBatchDeleteTaskLogStaysRunningUntilQueuedTasksFinishPartialFailure(t *testing.T) {
	gin.SetMode(gin.TestMode)

	taskEngine := &mockBatchDeleteTaskEngine{}
	mountPointService := &mockBatchDeleteMountPointService{
		mountPoints: map[int64]*models.MountPoint{
			11: {FileId: 1101, FullPath: "/movies", CreatorUserID: 100},
			22: {FileId: 2201, FullPath: "/series", CreatorUserID: 200},
		},
	}
	taskLogDB := setupStorageTaskLogTestDB(t)
	fileTaskLogService := filetasklogSvi.NewService(taskLogDB)

	router := gin.New()
	router.Use(func(ctx *gin.Context) {
		ctx.Set(consts.CtxKeyUserId, int64(100))
		ctx.Set(consts.CtxKeyIsAdmin, false)
	})

	wrapper := httpcontext.NewHandlerFuncWrapper(zap.NewNop())
	router.POST("/batch_delete", wrapper.Wrap(NewHandler(
		taskEngine,
		nil,
		nil,
		nil,
		mountPointService,
		fileTaskLogService,
		nil,
		nil,
		nil,
		nil,
	).BatchDelete()))

	req := httptest.NewRequestWithContext(stdctx.Background(), http.MethodPost, "/batch_delete", strings.NewReader(`{"ids":[11,22,33]}`))
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected ok, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	if len(taskEngine.payloads) != 1 {
		t.Fatalf("expected 1 queued task, got %d", len(taskEngine.payloads))
	}

	var response struct {
		Data batchDeleteResponse `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}

	if response.Data.Total != 3 || response.Data.Success != 1 || response.Data.Failed != 2 {
		t.Fatalf("unexpected response data: %+v", response.Data)
	}

	var log models.FileTaskLog
	if err := taskLogDB.db.First(&log).Error; err != nil {
		t.Fatalf("query task log: %v", err)
	}

	if log.Total != 3 || log.Completed != 0 || log.Failed != 2 {
		t.Fatalf("expected total=3 completed=0 failed=2 before queued task finishes, got total=%d completed=%d failed=%d", log.Total, log.Completed, log.Failed)
	}

	if log.Status != models.StatusRunning {
		t.Fatalf("expected parent task to stay %q before queued task finishes, got %q", models.StatusRunning, log.Status)
	}

	appCtx := appContext.NewContext(stdctx.Background())

	logKey := filetasklogSvi.NewLogID(log.ID)
	if err := fileTaskLogService.FlushCount(appCtx, logKey, filetasklogSvi.WithCompletedCounter(1)); err != nil {
		t.Fatalf("flush child completion: %v", err)
	}

	if err := fileTaskLogService.CompleteIfProgressDone(appCtx, logKey); err != nil {
		t.Fatalf("complete parent task: %v", err)
	}

	if err := taskLogDB.db.First(&log, log.ID).Error; err != nil {
		t.Fatalf("query updated task log: %v", err)
	}

	if log.Total != 3 || log.Completed != 1 || log.Failed != 2 {
		t.Fatalf("expected total=3 completed=1 failed=2 after queued task finishes, got total=%d completed=%d failed=%d", log.Total, log.Completed, log.Failed)
	}

	if log.Status != models.StatusFailed {
		t.Fatalf("expected parent task to become %q after partial failure finishes, got %q", models.StatusFailed, log.Status)
	}
}

func int64SlicesEqual(a []int64, b []int64) bool {
	if len(a) != len(b) {
		return false
	}

	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}

	return true
}
