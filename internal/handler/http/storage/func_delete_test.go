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
	deleteCalled bool
}

func (m *mockDeleteMountPointService) Query(ctx appContext.Context, fileID int64) (*models.MountPoint, error) {
	mountPoint, ok := m.mountPoints[fileID]
	if !ok {
		return nil, gorm.ErrRecordNotFound
	}

	return mountPoint, nil
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
			77: {FileId: 77, FullPath: "/movies", CreatorUserID: 100},
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

	if got, want := taskReq.IDs, []int64{77}; !int64SlicesEqual(got, want) {
		t.Fatalf("expected queued task IDs %v, got %v", want, got)
	}

	if taskEngine.paths[0] != "/movies" {
		t.Fatalf("expected queued full path /movies, got %v", taskEngine.paths[0])
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

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected bad request, got %d body=%s", recorder.Code, recorder.Body.String())
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
