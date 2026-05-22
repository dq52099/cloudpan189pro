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
	"github.com/xxcheng123/cloudpan189-share/internal/repository/models"
	group2fileSvi "github.com/xxcheng123/cloudpan189-share/internal/services/group2file"
	userMountPointTokenSvi "github.com/xxcheng123/cloudpan189-share/internal/services/userMountPointToken"
	"github.com/xxcheng123/cloudpan189-share/internal/types/topic"
	"go.uber.org/zap"
)

type mockBatchModifyGroup2FileService struct {
	group2fileSvi.Service
	fileIDs []int64
	err     error
}

func (m *mockBatchModifyGroup2FileService) GetBindFiles(ctx appContext.Context, groupID int64) ([]int64, error) {
	return m.fileIDs, m.err
}

type mockBatchModifyUserMountPointTokenService struct {
	userMountPointTokenSvi.Service
	tokens  map[int64]int64
	unbound []int64
}

func (m *mockBatchModifyUserMountPointTokenService) GetTokenID(ctx appContext.Context, userID, mountPointID int64) (int64, error) {
	return m.tokens[mountPointID], nil
}

func (m *mockBatchModifyUserMountPointTokenService) UnbindToken(ctx appContext.Context, userID, mountPointID int64) error {
	m.unbound = append(m.unbound, mountPointID)

	return nil
}

func TestBatchModifyTokenDeduplicatesIDsBeforePermissionCheckAndQueueing(t *testing.T) {
	gin.SetMode(gin.TestMode)

	taskEngine := &mockBatchDeleteTaskEngine{}
	mountPointService := &mockBatchDeleteMountPointService{
		mountPoints: map[int64]*models.MountPoint{
			11: {ID: 101, FileId: 11, FullPath: "/movies", CreatorUserID: 100},
			22: {ID: 202, FileId: 22, FullPath: "/series", CreatorUserID: 100},
		},
	}

	router := gin.New()
	router.Use(func(ctx *gin.Context) {
		ctx.Set(consts.CtxKeyUserId, int64(100))
		ctx.Set(consts.CtxKeyIsAdmin, false)
	})

	wrapper := httpcontext.NewHandlerFuncWrapper(zap.NewNop())
	router.POST("/batch_modify_token", wrapper.Wrap(NewHandler(
		taskEngine,
		nil,
		nil,
		nil,
		mountPointService,
		nil,
		nil,
		nil,
		nil,
		nil,
	).BatchModifyToken()))

	req := httptest.NewRequestWithContext(stdctx.Background(), http.MethodPost, "/batch_modify_token", strings.NewReader(`{"ids":[11,11,22],"tokenId":0}`))
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected ok, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	if got, want := mountPointService.queries, []int64{11, 22}; !int64SlicesEqual(got, want) {
		t.Fatalf("expected deduplicated permission queries %v, got %v", want, got)
	}

	if len(taskEngine.payloads) != 1 {
		t.Fatalf("expected 1 queued task, got %d", len(taskEngine.payloads))
	}

	var taskReq topic.FileBatchModifyTokenRequest
	if err := json.Unmarshal(taskEngine.payloads[0], &taskReq); err != nil {
		t.Fatal(err)
	}

	if got, want := taskReq.IDs, []int64{11, 22}; !int64SlicesEqual(got, want) {
		t.Fatalf("expected queued task IDs %v, got %v", want, got)
	}

	if taskReq.TokenID != 0 || taskReq.UserID != 100 || taskReq.IsAdmin {
		t.Fatalf("unexpected queued task: %+v", taskReq)
	}

	var response struct {
		Code int    `json:"code"`
		Data string `json:"data"`
	}

	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}

	if response.Code != http.StatusOK {
		t.Fatalf("expected business code 200, got %d", response.Code)
	}

	if !strings.Contains(response.Data, "共 2 个挂载点") {
		t.Fatalf("unexpected response data: %q", response.Data)
	}
}

func TestBatchModifyTokenAllowsUserBoundMountPoint(t *testing.T) {
	gin.SetMode(gin.TestMode)

	taskEngine := &mockBatchDeleteTaskEngine{}
	mountPointService := &mockBatchDeleteMountPointService{
		mountPoints: map[int64]*models.MountPoint{
			11: {ID: 101, FileId: 11, FullPath: "/shared", CreatorUserID: 200},
		},
	}
	userTokenService := &mockBatchModifyUserMountPointTokenService{
		tokens: map[int64]int64{101: 88},
	}

	router := gin.New()
	router.Use(func(ctx *gin.Context) {
		ctx.Set(consts.CtxKeyUserId, int64(100))
		ctx.Set(consts.CtxKeyIsAdmin, false)
	})

	wrapper := httpcontext.NewHandlerFuncWrapper(zap.NewNop())
	router.POST("/batch_modify_token", wrapper.Wrap(NewHandler(
		taskEngine,
		nil,
		nil,
		nil,
		mountPointService,
		nil,
		nil,
		nil,
		nil,
		userTokenService,
	).BatchModifyToken()))

	req := httptest.NewRequestWithContext(stdctx.Background(), http.MethodPost, "/batch_modify_token", strings.NewReader(`{"ids":[11],"tokenId":0}`))
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected ok, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	if len(taskEngine.payloads) != 1 {
		t.Fatalf("expected 1 queued task, got %d", len(taskEngine.payloads))
	}
}

func TestBatchModifyTokenFailsWhenGroupBindingLookupFails(t *testing.T) {
	gin.SetMode(gin.TestMode)

	taskEngine := &mockBatchDeleteTaskEngine{}
	groupErr := errors.New("group lookup failed")

	router := gin.New()
	router.Use(func(ctx *gin.Context) {
		ctx.Set(consts.CtxKeyUserId, int64(100))
		ctx.Set(consts.CtxKeyIsAdmin, false)
		ctx.Set(consts.CtxKeyUserGroupId, int64(7))
	})

	wrapper := httpcontext.NewHandlerFuncWrapper(zap.NewNop())
	router.POST("/batch_modify_token", wrapper.Wrap(NewHandler(
		taskEngine,
		nil,
		nil,
		nil,
		&mockBatchDeleteMountPointService{},
		nil,
		nil,
		nil,
		&mockBatchModifyGroup2FileService{err: groupErr},
		nil,
	).BatchModifyToken()))

	req := httptest.NewRequestWithContext(stdctx.Background(), http.MethodPost, "/batch_modify_token", strings.NewReader(`{"ids":[11],"tokenId":0}`))
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code == http.StatusOK {
		t.Fatalf("expected failure, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	if len(taskEngine.payloads) != 0 {
		t.Fatalf("expected no queued task, got %d", len(taskEngine.payloads))
	}
}

func TestModifyTokenAllowsUserBoundMountPoint(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mountPointService := &mockBatchDeleteMountPointService{
		mountPoints: map[int64]*models.MountPoint{
			11: {ID: 101, FileId: 11, FullPath: "/shared", CreatorUserID: 200},
		},
	}
	userTokenService := &mockBatchModifyUserMountPointTokenService{
		tokens: map[int64]int64{101: 88},
	}

	router := gin.New()
	router.Use(func(ctx *gin.Context) {
		ctx.Set(consts.CtxKeyUserId, int64(100))
		ctx.Set(consts.CtxKeyIsAdmin, false)
	})

	wrapper := httpcontext.NewHandlerFuncWrapper(zap.NewNop())
	router.POST("/modify_token", wrapper.Wrap(NewHandler(
		nil,
		nil,
		nil,
		nil,
		mountPointService,
		nil,
		nil,
		nil,
		nil,
		userTokenService,
	).ModifyToken()))

	req := httptest.NewRequestWithContext(stdctx.Background(), http.MethodPost, "/modify_token", strings.NewReader(`{"id":11,"tokenId":0}`))
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected ok, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	if got, want := userTokenService.unbound, []int64{101}; !int64SlicesEqual(got, want) {
		t.Fatalf("expected unbound mount point IDs %v, got %v", want, got)
	}
}
