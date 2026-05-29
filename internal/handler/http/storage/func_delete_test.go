package storage

import (
	stdctx "context"
	"encoding/json"
	"errors"
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
	mountPointSvi "github.com/xxcheng123/cloudpan189-share/internal/services/mountpoint"
	virtualfileSvi "github.com/xxcheng123/cloudpan189-share/internal/services/virtualfile"
	"github.com/xxcheng123/cloudpan189-share/internal/types/topic"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type mockDeleteTaskEngine struct {
	taskengine.TaskEngine
	err      error
	payloads [][]byte
	paths    []any
}

func (m *mockDeleteTaskEngine) PushMessage(ctx stdctx.Context, taskTopic taskengine.Topic, payload []byte) error {
	if m.err != nil {
		return m.err
	}

	m.payloads = append(m.payloads, payload)
	m.paths = append(m.paths, ctx.Value(consts.CtxKeyFullPath))

	return nil
}

type mockDeleteMountPointService struct {
	mountPointSvi.Service
	mountPoints  map[int64]*models.MountPoint
	queries      []int64
	deleteCalled bool
}

func (m *mockDeleteMountPointService) Query(ctx appContext.Context, fileID int64) (*models.MountPoint, error) {
	mountPoint, ok := m.mountPoints[fileID]
	if !ok {
		return nil, gorm.ErrRecordNotFound
	}

	return mountPoint, nil
}

func (m *mockDeleteMountPointService) QueryByID(ctx appContext.Context, id int64) (*models.MountPoint, error) {
	m.queries = append(m.queries, id)

	if mountPoint, ok := m.mountPoints[id]; ok {
		return mountPoint, nil
	}

	for _, mountPoint := range m.mountPoints {
		if mountPoint.ID == id {
			return mountPoint, nil
		}
	}

	return nil, gorm.ErrRecordNotFound
}

func (m *mockDeleteMountPointService) Delete(ctx appContext.Context, req *mountPointSvi.DeleteRequest) error {
	m.deleteCalled = true

	return nil
}

type mockDeleteVirtualFileService struct {
	virtualfileSvi.Service
	clearUnusedCalled bool
}

func (m *mockDeleteVirtualFileService) ClearUnusedAncestorFolder(ctx appContext.Context, subId int64) error {
	m.clearUnusedCalled = true

	return nil
}

func newDeleteTestRouter(
	taskEngine taskengine.TaskEngine,
	virtualFileService virtualfileSvi.Service,
	mountPointService mountPointSvi.Service,
) *gin.Engine {
	return newDeleteTestRouterWithAuth(taskEngine, virtualFileService, mountPointService, 100, false)
}

func newDeleteTestRouterWithAuth(
	taskEngine taskengine.TaskEngine,
	virtualFileService virtualfileSvi.Service,
	mountPointService mountPointSvi.Service,
	userID int64,
	isAdmin bool,
) *gin.Engine {
	router := gin.New()
	router.Use(func(ctx *gin.Context) {
		ctx.Set(consts.CtxKeyUserId, userID)
		ctx.Set(consts.CtxKeyIsAdmin, isAdmin)
	})

	wrapper := httpcontext.NewHandlerFuncWrapper(zap.NewNop())
	router.POST("/delete", wrapper.Wrap(NewHandler(
		taskEngine,
		virtualFileService,
		nil,
		nil,
		mountPointService,
		nil,
		nil,
		nil,
		nil,
		nil,
	).Delete()))

	return router
}

func TestDeleteQueuesTaskWithoutImmediateDataDeletion(t *testing.T) {
	gin.SetMode(gin.TestMode)

	taskEngine := &mockDeleteTaskEngine{}
	virtualFileService := &mockDeleteVirtualFileService{}
	mountPointService := &mockDeleteMountPointService{
		mountPoints: map[int64]*models.MountPoint{
			77: {ID: 77, FileId: 7700, FullPath: "/movies", CreatorUserID: 100},
		},
	}
	router := newDeleteTestRouter(taskEngine, virtualFileService, mountPointService)

	req := httptest.NewRequestWithContext(stdctx.Background(), http.MethodPost, "/delete", strings.NewReader(`{"id":77}`))
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected ok, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	if mountPointService.deleteCalled {
		t.Fatal("did not expect mount point to be deleted before the consumer task runs")
	}

	if virtualFileService.clearUnusedCalled {
		t.Fatal("did not expect ancestor cleanup before the consumer deletes the virtual file")
	}

	if len(taskEngine.payloads) != 1 {
		t.Fatalf("expected 1 queued task, got %d", len(taskEngine.payloads))
	}

	var taskReq topic.FileBatchDeleteRequest
	if err := json.Unmarshal(taskEngine.payloads[0], &taskReq); err != nil {
		t.Fatal(err)
	}

	if got, want := taskReq.IDs, []int64{7700}; !int64SlicesEqual(got, want) {
		t.Fatalf("expected queued task IDs %v, got %v", want, got)
	}

	if taskReq.ExpectedUserID != 100 || taskReq.TriggeredByAdmin {
		t.Fatalf("expected queued user snapshot user=100 admin=false, got %+v", taskReq)
	}

	if taskEngine.paths[0] != "/movies" {
		t.Fatalf("expected queued full path /movies, got %v", taskEngine.paths[0])
	}
}

func TestDeleteRejectsInvalidIDBeforeQuerying(t *testing.T) {
	gin.SetMode(gin.TestMode)

	taskEngine := &mockDeleteTaskEngine{}
	virtualFileService := &mockDeleteVirtualFileService{}
	mountPointService := &mockDeleteMountPointService{mountPoints: map[int64]*models.MountPoint{}}
	router := newDeleteTestRouter(taskEngine, virtualFileService, mountPointService)

	req := httptest.NewRequestWithContext(stdctx.Background(), http.MethodPost, "/delete", strings.NewReader(`{"id":-1}`))
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected bad request, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	if len(mountPointService.queries) != 0 {
		t.Fatalf("expected invalid request to stop before querying mount points, got %v", mountPointService.queries)
	}

	if len(taskEngine.payloads) != 0 {
		t.Fatalf("expected no queued task for invalid request, got %d", len(taskEngine.payloads))
	}
}

func TestDeleteReturnsNotFoundWhenMountPointMissing(t *testing.T) {
	gin.SetMode(gin.TestMode)

	taskEngine := &mockDeleteTaskEngine{}
	virtualFileService := &mockDeleteVirtualFileService{}
	mountPointService := &mockDeleteMountPointService{mountPoints: map[int64]*models.MountPoint{}}
	router := newDeleteTestRouter(taskEngine, virtualFileService, mountPointService)

	req := httptest.NewRequestWithContext(stdctx.Background(), http.MethodPost, "/delete", strings.NewReader(`{"id":77}`))
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("expected not found, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	var response httpcontext.Response
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}

	if response.Code != busCodeStorageMountPointNotFound.GetCode() {
		t.Fatalf("expected business code %d, got %d", busCodeStorageMountPointNotFound.GetCode(), response.Code)
	}

	if len(taskEngine.payloads) != 0 {
		t.Fatalf("expected no queued task for missing mount point, got %d", len(taskEngine.payloads))
	}
}

func TestDeleteDoesNotMutateDataWhenQueueFails(t *testing.T) {
	gin.SetMode(gin.TestMode)

	taskEngine := &mockDeleteTaskEngine{err: errors.New("queue failed")}
	virtualFileService := &mockDeleteVirtualFileService{}
	mountPointService := &mockDeleteMountPointService{
		mountPoints: map[int64]*models.MountPoint{
			77: {FileId: 77, FullPath: "/movies", CreatorUserID: 100},
		},
	}
	router := newDeleteTestRouter(taskEngine, virtualFileService, mountPointService)

	req := httptest.NewRequestWithContext(stdctx.Background(), http.MethodPost, "/delete", strings.NewReader(`{"id":77}`))
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected bad request, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	if mountPointService.deleteCalled {
		t.Fatal("did not expect mount point to be deleted when queueing fails")
	}

	if virtualFileService.clearUnusedCalled {
		t.Fatal("did not expect ancestor cleanup when queueing fails")
	}
}

func TestDeleteRejectsNonOwnerWithoutQueueingTask(t *testing.T) {
	gin.SetMode(gin.TestMode)

	taskEngine := &mockDeleteTaskEngine{}
	virtualFileService := &mockDeleteVirtualFileService{}
	mountPointService := &mockDeleteMountPointService{
		mountPoints: map[int64]*models.MountPoint{
			77: {FileId: 77, FullPath: "/movies", CreatorUserID: 200},
		},
	}
	router := newDeleteTestRouterWithAuth(taskEngine, virtualFileService, mountPointService, 100, false)

	req := httptest.NewRequestWithContext(stdctx.Background(), http.MethodPost, "/delete", strings.NewReader(`{"id":77}`))
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusForbidden {
		t.Fatalf("expected forbidden, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	if len(taskEngine.payloads) != 0 {
		t.Fatalf("expected no queued task, got %d", len(taskEngine.payloads))
	}

	if mountPointService.deleteCalled {
		t.Fatal("did not expect mount point delete for non-owner")
	}

	if virtualFileService.clearUnusedCalled {
		t.Fatal("did not expect ancestor cleanup for non-owner")
	}
}
