package file

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
	mountpointSvi "github.com/xxcheng123/cloudpan189-share/internal/services/mountpoint"
	virtualfileSvi "github.com/xxcheng123/cloudpan189-share/internal/services/virtualfile"
	"github.com/xxcheng123/cloudpan189-share/internal/types/topic"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type mockBatchDeleteTaskEngine struct {
	taskengine.TaskEngine
	err      error
	payloads [][]byte
}

func (m *mockBatchDeleteTaskEngine) PushMessage(ctx stdctx.Context, taskTopic taskengine.Topic, payload []byte) error {
	if m.err != nil {
		return m.err
	}

	m.payloads = append(m.payloads, payload)

	return nil
}

type mockBatchDeleteMountPointService struct {
	mountpointSvi.Service
	mountPoints map[int64]*models.MountPoint
	queries     []int64
	request     *mountpointSvi.BatchDeleteRequest
}

func (m *mockBatchDeleteMountPointService) Query(ctx appContext.Context, fileID int64) (*models.MountPoint, error) {
	m.queries = append(m.queries, fileID)

	mountPoint, ok := m.mountPoints[fileID]
	if !ok {
		return nil, gorm.ErrRecordNotFound
	}

	return mountPoint, nil
}

func (m *mockBatchDeleteMountPointService) BatchDelete(ctx appContext.Context, req *mountpointSvi.BatchDeleteRequest) error {
	copiedIDs := append([]int64(nil), req.FileIds...)
	m.request = &mountpointSvi.BatchDeleteRequest{
		FileIds:       copiedIDs,
		CreatorUserID: req.CreatorUserID,
		IsAdmin:       req.IsAdmin,
	}

	return nil
}

type mockBatchDeleteVirtualFileService struct {
	virtualfileSvi.Service
	files map[int64]*models.VirtualFile
}

func (m *mockBatchDeleteVirtualFileService) Query(ctx appContext.Context, fileID int64) (*models.VirtualFile, error) {
	file, ok := m.files[fileID]
	if !ok {
		return nil, gorm.ErrRecordNotFound
	}

	return file, nil
}

func newBatchDeleteTestRouter(
	taskEngine taskengine.TaskEngine,
	virtualFileService virtualfileSvi.Service,
	mountPointService mountpointSvi.Service,
	userID int64,
	isAdmin bool,
) *gin.Engine {
	router := gin.New()
	router.Use(func(ctx *gin.Context) {
		ctx.Set(consts.CtxKeyUserId, userID)
		ctx.Set(consts.CtxKeyIsAdmin, isAdmin)
	})

	wrapper := httpcontext.NewHandlerFuncWrapper(zap.NewNop())
	router.POST("/batch_delete", wrapper.Wrap(NewHandler(
		virtualFileService,
		nil,
		nil,
		nil,
		mountPointService,
		nil,
		nil,
		taskEngine,
	).BatchDelete()))

	return router
}

func TestBatchDeleteDeduplicatesIDsBeforeQueueing(t *testing.T) {
	gin.SetMode(gin.TestMode)

	taskEngine := &mockBatchDeleteTaskEngine{}
	virtualFileService := &mockBatchDeleteVirtualFileService{
		files: map[int64]*models.VirtualFile{
			11: {ID: 11, TopId: 1000, Name: "a.mkv"},
			22: {ID: 22, TopId: 1000, Name: "b.mkv"},
		},
	}
	mountPointService := &mockBatchDeleteMountPointService{
		mountPoints: map[int64]*models.MountPoint{
			1000: {FileId: 1000, CreatorUserID: 100},
		},
	}

	router := newBatchDeleteTestRouter(taskEngine, virtualFileService, mountPointService, 100, false)

	req := httptest.NewRequestWithContext(stdctx.Background(), http.MethodPost, "/batch_delete", strings.NewReader(`{"ids":[11,11,22]}`))
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected ok, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	if got, want := mountPointService.queries, []int64{1000}; !int64SlicesEqual(got, want) {
		t.Fatalf("expected deduplicated mount point queries %v, got %v", want, got)
	}

	if mountPointService.request != nil {
		t.Fatalf("did not expect immediate mount point batch delete, got %+v", mountPointService.request)
	}

	if len(taskEngine.payloads) != 1 {
		t.Fatalf("expected 1 queued task, got %d", len(taskEngine.payloads))
	}

	var taskReq topic.FileBatchDeleteRequest
	if err := json.Unmarshal(taskEngine.payloads[0], &taskReq); err != nil {
		t.Fatal(err)
	}

	if got, want := taskReq.IDs, []int64{11, 22}; !int64SlicesEqual(got, want) {
		t.Fatalf("expected queued IDs %v, got %v", want, got)
	}

	if taskReq.ExpectedUserID != 100 || taskReq.TriggeredByAdmin {
		t.Fatalf("expected queued user snapshot user=100 admin=false, got %+v", taskReq)
	}
}

func TestBatchDeleteAllowsDuplicateIDsBeyondRawLimit(t *testing.T) {
	gin.SetMode(gin.TestMode)

	taskEngine := &mockBatchDeleteTaskEngine{}
	virtualFileService := &mockBatchDeleteVirtualFileService{
		files: map[int64]*models.VirtualFile{
			11: {ID: 11, TopId: 1000, Name: "a.mkv"},
		},
	}
	mountPointService := &mockBatchDeleteMountPointService{
		mountPoints: map[int64]*models.MountPoint{
			1000: {FileId: 1000, CreatorUserID: 100},
		},
	}

	router := newBatchDeleteTestRouter(taskEngine, virtualFileService, mountPointService, 100, false)

	ids := make([]string, maxBatchDeleteIDs+1)
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

	if got, want := mountPointService.queries, []int64{1000}; !int64SlicesEqual(got, want) {
		t.Fatalf("expected one deduplicated mount point query %v, got %v", want, got)
	}

	if len(taskEngine.payloads) != 1 {
		t.Fatalf("expected 1 queued task, got %d", len(taskEngine.payloads))
	}

	var taskReq topic.FileBatchDeleteRequest
	if err := json.Unmarshal(taskEngine.payloads[0], &taskReq); err != nil {
		t.Fatal(err)
	}

	if got, want := taskReq.IDs, []int64{11}; !int64SlicesEqual(got, want) {
		t.Fatalf("expected queued IDs %v, got %v", want, got)
	}
}

func TestBatchDeleteRejectsTooManyUniqueIDsBeforeQuerying(t *testing.T) {
	gin.SetMode(gin.TestMode)

	taskEngine := &mockBatchDeleteTaskEngine{}
	virtualFileService := &mockBatchDeleteVirtualFileService{}
	mountPointService := &mockBatchDeleteMountPointService{}

	router := newBatchDeleteTestRouter(taskEngine, virtualFileService, mountPointService, 100, false)

	ids := make([]string, maxBatchDeleteIDs+1)
	for i := range ids {
		ids[i] = fmt.Sprintf("%d", i+1)
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

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected bad request, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	if len(mountPointService.queries) != 0 {
		t.Fatalf("expected no mount point queries for oversized request, got %v", mountPointService.queries)
	}

	if len(taskEngine.payloads) != 0 {
		t.Fatalf("expected no queued tasks for oversized request, got %d", len(taskEngine.payloads))
	}
}

func TestBatchDeleteRejectsInvalidIDBeforeDeleteAndQueueing(t *testing.T) {
	gin.SetMode(gin.TestMode)

	taskEngine := &mockBatchDeleteTaskEngine{}
	virtualFileService := &mockBatchDeleteVirtualFileService{}
	mountPointService := &mockBatchDeleteMountPointService{}

	router := newBatchDeleteTestRouter(taskEngine, virtualFileService, mountPointService, 100, false)

	req := httptest.NewRequestWithContext(stdctx.Background(), http.MethodPost, "/batch_delete", strings.NewReader(`{"ids":[0,11]}`))
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected bad request, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	if mountPointService.request != nil {
		t.Fatalf("expected invalid request to stop before batch delete, got %+v", mountPointService.request)
	}

	if len(taskEngine.payloads) != 0 {
		t.Fatalf("expected no queued tasks for invalid request, got %d", len(taskEngine.payloads))
	}
}

func TestBatchDeleteReturnsNotFoundWhenFileMissing(t *testing.T) {
	gin.SetMode(gin.TestMode)

	taskEngine := &mockBatchDeleteTaskEngine{}
	virtualFileService := &mockBatchDeleteVirtualFileService{files: map[int64]*models.VirtualFile{}}
	mountPointService := &mockBatchDeleteMountPointService{}

	router := newBatchDeleteTestRouter(taskEngine, virtualFileService, mountPointService, 100, false)

	req := httptest.NewRequestWithContext(stdctx.Background(), http.MethodPost, "/batch_delete", strings.NewReader(`{"ids":[11]}`))
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

	if response.Code != busCodeFileNotFound.GetCode() {
		t.Fatalf("expected business code %d, got %d", busCodeFileNotFound.GetCode(), response.Code)
	}

	if len(taskEngine.payloads) != 0 {
		t.Fatalf("expected no queued tasks for missing file, got %d", len(taskEngine.payloads))
	}
}

func TestBatchDeleteDoesNotMutateDataWhenQueueFails(t *testing.T) {
	gin.SetMode(gin.TestMode)

	taskEngine := &mockBatchDeleteTaskEngine{err: errors.New("queue failed")}
	virtualFileService := &mockBatchDeleteVirtualFileService{
		files: map[int64]*models.VirtualFile{
			11: {ID: 11, TopId: 1000, Name: "a.mkv"},
		},
	}
	mountPointService := &mockBatchDeleteMountPointService{
		mountPoints: map[int64]*models.MountPoint{
			1000: {FileId: 1000, CreatorUserID: 100},
		},
	}

	router := newBatchDeleteTestRouter(taskEngine, virtualFileService, mountPointService, 100, false)

	req := httptest.NewRequestWithContext(stdctx.Background(), http.MethodPost, "/batch_delete", strings.NewReader(`{"ids":[11]}`))
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected bad request, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	if mountPointService.request != nil {
		t.Fatalf("did not expect mount point batch delete when queueing fails, got %+v", mountPointService.request)
	}

	if len(taskEngine.payloads) != 0 {
		t.Fatalf("expected no queued tasks for queue failure, got %d", len(taskEngine.payloads))
	}
}

func TestBatchDeleteRejectsNonOwnerWithoutQueueingTask(t *testing.T) {
	gin.SetMode(gin.TestMode)

	taskEngine := &mockBatchDeleteTaskEngine{}
	virtualFileService := &mockBatchDeleteVirtualFileService{
		files: map[int64]*models.VirtualFile{
			11: {ID: 11, TopId: 1000, Name: "a.mkv"},
		},
	}
	mountPointService := &mockBatchDeleteMountPointService{
		mountPoints: map[int64]*models.MountPoint{
			1000: {FileId: 1000, CreatorUserID: 200},
		},
	}

	router := newBatchDeleteTestRouter(taskEngine, virtualFileService, mountPointService, 100, false)

	req := httptest.NewRequestWithContext(stdctx.Background(), http.MethodPost, "/batch_delete", strings.NewReader(`{"ids":[11]}`))
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusForbidden {
		t.Fatalf("expected forbidden, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	if len(taskEngine.payloads) != 0 {
		t.Fatalf("expected no queued task, got %d", len(taskEngine.payloads))
	}

	if mountPointService.request != nil {
		t.Fatalf("did not expect mount point batch delete for non-owner, got %+v", mountPointService.request)
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
