package storage

import (
	stdctx "context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/xxcheng123/cloudpan189-share/internal/consts"
	appContext "github.com/xxcheng123/cloudpan189-share/internal/framework/context"
	"github.com/xxcheng123/cloudpan189-share/internal/framework/httpcontext"
	"github.com/xxcheng123/cloudpan189-share/internal/pkgs/taskengine"
	"github.com/xxcheng123/cloudpan189-share/internal/repository/models"
	filetasklogSvi "github.com/xxcheng123/cloudpan189-share/internal/services/filetasklog"
	"github.com/xxcheng123/cloudpan189-share/internal/types/topic"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type storageTaskLogTestDB struct {
	db *gorm.DB
}

func (t *storageTaskLogTestDB) GetDB(ctx appContext.Context) *gorm.DB {
	return t.db.WithContext(ctx)
}

func (t *storageTaskLogTestDB) GetDBWithoutContext() *gorm.DB {
	return t.db
}

func (t *storageTaskLogTestDB) GetLogger(name string, fields ...zap.Field) *zap.Logger {
	return zap.NewNop()
}

func (t *storageTaskLogTestDB) Close() {}

func (t *storageTaskLogTestDB) GetPort() int {
	return 9999
}

func (t *storageTaskLogTestDB) GetTaskEngine() taskengine.TaskEngine {
	return nil
}

func (t *storageTaskLogTestDB) GetHTTPEngine() *gin.Engine {
	return nil
}

func setupStorageTaskLogTestDB(t *testing.T) *storageTaskLogTestDB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}

	if err := db.AutoMigrate(&models.FileTaskLog{}); err != nil {
		t.Fatalf("migrate task log table: %v", err)
	}

	return &storageTaskLogTestDB{db: db}
}

func TestBatchRefreshDeduplicatesIDsBeforeQueueingTasks(t *testing.T) {
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
	router.POST("/batch_refresh", wrapper.Wrap(NewHandler(
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
	).BatchRefresh()))

	req := httptest.NewRequestWithContext(stdctx.Background(), http.MethodPost, "/batch_refresh", strings.NewReader(`{"ids":[11,11,22,33],"deep":true}`))
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

	var firstTask topic.FileScanFileRequest
	if err := json.Unmarshal(taskEngine.payloads[0], &firstTask); err != nil {
		t.Fatal(err)
	}

	if firstTask.FileId != 11 || !firstTask.Deep {
		t.Fatalf("unexpected first queued task: %+v", firstTask)
	}

	var response struct {
		Code int                  `json:"code"`
		Data batchRefreshResponse `json:"data"`
	}

	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}

	if response.Data.Total != 3 || response.Data.Success != 2 || response.Data.Failed != 1 {
		t.Fatalf("unexpected response data: %+v", response.Data)
	}

	if taskEngine.paths[0] != "/movies" {
		t.Fatalf("expected first task path /movies, got %v", taskEngine.paths[0])
	}

	if response.Code != http.StatusOK {
		t.Fatalf("expected business code 200, got %d", response.Code)
	}
}

func TestBatchRefreshRejectsInvalidIDBeforeQuerying(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mountPointService := &mockBatchDeleteMountPointService{
		mountPoints: map[int64]*models.MountPoint{},
	}

	router := gin.New()
	router.Use(func(ctx *gin.Context) {
		ctx.Set(consts.CtxKeyUserId, int64(100))
		ctx.Set(consts.CtxKeyIsAdmin, false)
	})

	wrapper := httpcontext.NewHandlerFuncWrapper(zap.NewNop())
	router.POST("/batch_refresh", wrapper.Wrap(NewHandler(
		&mockBatchDeleteTaskEngine{},
		nil,
		nil,
		nil,
		mountPointService,
		&mockBatchDeleteFileTaskLogService{},
		nil,
		nil,
		nil,
		nil,
	).BatchRefresh()))

	req := httptest.NewRequestWithContext(stdctx.Background(), http.MethodPost, "/batch_refresh", strings.NewReader(`{"ids":[0,11]}`))
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected bad request, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	if len(mountPointService.queries) != 0 {
		t.Fatalf("expected invalid request to stop before querying mount points, got %v", mountPointService.queries)
	}
}

func TestBatchRefreshSkipsMountPointsOwnedByOtherUsers(t *testing.T) {
	gin.SetMode(gin.TestMode)

	taskEngine := &mockBatchDeleteTaskEngine{}
	mountPointService := &mockBatchDeleteMountPointService{
		mountPoints: map[int64]*models.MountPoint{
			11: {FileId: 11, FullPath: "/movies", CreatorUserID: 100},
			22: {FileId: 22, FullPath: "/other", CreatorUserID: 200},
		},
	}

	router := gin.New()
	router.Use(func(ctx *gin.Context) {
		ctx.Set(consts.CtxKeyUserId, int64(100))
		ctx.Set(consts.CtxKeyIsAdmin, false)
	})

	wrapper := httpcontext.NewHandlerFuncWrapper(zap.NewNop())
	router.POST("/batch_refresh", wrapper.Wrap(NewHandler(
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
	).BatchRefresh()))

	req := httptest.NewRequestWithContext(stdctx.Background(), http.MethodPost, "/batch_refresh", strings.NewReader(`{"ids":[11,22]}`))
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected ok, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	if len(taskEngine.payloads) != 1 {
		t.Fatalf("expected only owned mount point to be queued, got %d", len(taskEngine.payloads))
	}

	if taskEngine.paths[0] != "/movies" {
		t.Fatalf("expected owned mount point path /movies, got %v", taskEngine.paths[0])
	}

	var response struct {
		Data batchRefreshResponse `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}

	if response.Data.Total != 2 || response.Data.Success != 1 || response.Data.Failed != 1 {
		t.Fatalf("unexpected response data: %+v", response.Data)
	}
}

func TestBatchRefreshTaskLogKeepsDispatchFailureCounts(t *testing.T) {
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
	router.POST("/batch_refresh", wrapper.Wrap(NewHandler(
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
	).BatchRefresh()))

	req := httptest.NewRequestWithContext(stdctx.Background(), http.MethodPost, "/batch_refresh", strings.NewReader(`{"ids":[11,22,33]}`))
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected ok, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	var log models.FileTaskLog
	if err := taskLogDB.db.First(&log).Error; err != nil {
		t.Fatalf("query task log: %v", err)
	}

	if log.Total != 3 || log.Completed != 1 || log.Failed != 2 {
		t.Fatalf("expected total=3 completed=1 failed=2, got total=%d completed=%d failed=%d", log.Total, log.Completed, log.Failed)
	}

	if log.Status != models.StatusFailed {
		t.Fatalf("expected dispatch failure log status %q, got %q", models.StatusFailed, log.Status)
	}
}

func TestBatchRefreshTaskLogCompletesWhenAllTasksDispatched(t *testing.T) {
	gin.SetMode(gin.TestMode)

	taskEngine := &mockBatchDeleteTaskEngine{}
	mountPointService := &mockBatchDeleteMountPointService{
		mountPoints: map[int64]*models.MountPoint{
			11: {FileId: 1101, FullPath: "/movies", CreatorUserID: 100},
			22: {FileId: 2201, FullPath: "/series", CreatorUserID: 100},
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
	router.POST("/batch_refresh", wrapper.Wrap(NewHandler(
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
	).BatchRefresh()))

	req := httptest.NewRequestWithContext(stdctx.Background(), http.MethodPost, "/batch_refresh", strings.NewReader(`{"ids":[11,22]}`))
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected ok, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	var log models.FileTaskLog
	if err := taskLogDB.db.First(&log).Error; err != nil {
		t.Fatalf("query task log: %v", err)
	}

	if log.Total != 2 || log.Completed != 2 || log.Failed != 0 {
		t.Fatalf("expected total=2 completed=2 failed=0, got total=%d completed=%d failed=%d", log.Total, log.Completed, log.Failed)
	}

	if log.Status != models.StatusCompleted {
		t.Fatalf("expected dispatch success log status %q, got %q", models.StatusCompleted, log.Status)
	}
}
