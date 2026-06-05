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
	"github.com/xxcheng123/cloudpan189-share/internal/repository/models"
	cloudtokenSvi "github.com/xxcheng123/cloudpan189-share/internal/services/cloudtoken"
	group2fileSvi "github.com/xxcheng123/cloudpan189-share/internal/services/group2file"
	userMountPointTokenSvi "github.com/xxcheng123/cloudpan189-share/internal/services/userMountPointToken"
	"github.com/xxcheng123/cloudpan189-share/internal/types/topic"
	"go.uber.org/zap"
	"gorm.io/gorm"
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
	bound   []int64
	unbound []int64
}

func (m *mockBatchModifyUserMountPointTokenService) GetTokenID(ctx appContext.Context, userID, mountPointID int64) (int64, error) {
	return m.tokens[mountPointID], nil
}

func (m *mockBatchModifyUserMountPointTokenService) BindToken(ctx appContext.Context, userID, mountPointID, tokenID int64) error {
	m.bound = append(m.bound, mountPointID)

	return nil
}

func (m *mockBatchModifyUserMountPointTokenService) UnbindToken(ctx appContext.Context, userID, mountPointID int64) error {
	m.unbound = append(m.unbound, mountPointID)

	return nil
}

type mockBatchModifyCloudTokenService struct {
	cloudtokenSvi.Service
	tokens  map[int64]*models.CloudToken
	err     error
	queries []int64
}

func (m *mockBatchModifyCloudTokenService) QueryAccessible(ctx appContext.Context, id, userID int64, isAdmin bool) (*models.CloudToken, error) {
	m.queries = append(m.queries, id)

	if m.err != nil {
		return nil, m.err
	}

	if token, ok := m.tokens[id]; ok {
		return token, nil
	}

	return &models.CloudToken{ID: id, UserID: userID}, nil
}

func TestBatchModifyTokenDeduplicatesIDsBeforePermissionCheckAndQueueing(t *testing.T) {
	gin.SetMode(gin.TestMode)

	taskEngine := &mockBatchDeleteTaskEngine{}
	mountPointService := &mockBatchDeleteMountPointService{
		mountPoints: map[int64]*models.MountPoint{
			101: {ID: 101, FileId: 11, FullPath: "/movies", CreatorUserID: 100},
			202: {ID: 202, FileId: 22, FullPath: "/series", CreatorUserID: 100},
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

	req := httptest.NewRequestWithContext(stdctx.Background(), http.MethodPost, "/batch_modify_token", strings.NewReader(`{"ids":[101,101,202],"tokenId":0}`))
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected ok, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	if got, want := mountPointService.queries, []int64{101, 202}; !int64SlicesEqual(got, want) {
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

	if got, want := taskReq.MountPointIDs, []int64{101, 202}; !int64SlicesEqual(got, want) {
		t.Fatalf("expected queued mount point IDs %v, got %v", want, got)
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

func TestBatchModifyTokenAllowsDuplicateIDsBeyondRawLimit(t *testing.T) {
	gin.SetMode(gin.TestMode)

	taskEngine := &mockBatchDeleteTaskEngine{}
	mountPointService := &mockBatchDeleteMountPointService{
		mountPoints: map[int64]*models.MountPoint{
			101: {ID: 101, FileId: 11, FullPath: "/movies", CreatorUserID: 100},
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

	ids := make([]string, maxBatchModifyTokenIDs+1)
	for i := range ids {
		ids[i] = "101"
	}

	req := httptest.NewRequestWithContext(
		stdctx.Background(),
		http.MethodPost,
		"/batch_modify_token",
		strings.NewReader(fmt.Sprintf(`{"ids":[%s],"tokenId":0}`, strings.Join(ids, ","))),
	)
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected ok, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	if got, want := mountPointService.queries, []int64{101}; !int64SlicesEqual(got, want) {
		t.Fatalf("expected one deduplicated query %v, got %v", want, got)
	}

	var taskReq topic.FileBatchModifyTokenRequest
	if err := json.Unmarshal(taskEngine.payloads[0], &taskReq); err != nil {
		t.Fatal(err)
	}

	if got, want := taskReq.IDs, []int64{11}; !int64SlicesEqual(got, want) {
		t.Fatalf("expected queued IDs %v, got %v", want, got)
	}

	if got, want := taskReq.MountPointIDs, []int64{101}; !int64SlicesEqual(got, want) {
		t.Fatalf("expected queued mount point IDs %v, got %v", want, got)
	}
}

func TestBatchModifyTokenRejectsTooManyUniqueIDs(t *testing.T) {
	gin.SetMode(gin.TestMode)

	taskEngine := &mockBatchDeleteTaskEngine{}
	mountPointService := &mockBatchDeleteMountPointService{}

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

	ids := make([]string, maxBatchModifyTokenIDs+1)
	for i := range ids {
		ids[i] = fmt.Sprintf("%d", i+1)
	}

	req := httptest.NewRequestWithContext(
		stdctx.Background(),
		http.MethodPost,
		"/batch_modify_token",
		strings.NewReader(fmt.Sprintf(`{"ids":[%s],"tokenId":0}`, strings.Join(ids, ","))),
	)
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected bad request, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	if len(mountPointService.queries) != 0 {
		t.Fatalf("expected request to stop before querying mount points, got %v", mountPointService.queries)
	}

	if len(taskEngine.payloads) != 0 {
		t.Fatalf("expected no queued task, got %d", len(taskEngine.payloads))
	}
}

func TestBatchModifyTokenRejectsNegativeTokenIDBeforeQuerying(t *testing.T) {
	gin.SetMode(gin.TestMode)

	taskEngine := &mockBatchDeleteTaskEngine{}
	mountPointService := &mockBatchDeleteMountPointService{}

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

	req := httptest.NewRequestWithContext(stdctx.Background(), http.MethodPost, "/batch_modify_token", strings.NewReader(`{"ids":[11],"tokenId":-1}`))
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected bad request, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	if len(mountPointService.queries) != 0 {
		t.Fatalf("expected request to stop before querying mount points, got %v", mountPointService.queries)
	}

	if len(taskEngine.payloads) != 0 {
		t.Fatalf("expected no queued task, got %d", len(taskEngine.payloads))
	}
}

func TestBatchModifyTokenReturnsNotFoundWhenMountPointMissing(t *testing.T) {
	gin.SetMode(gin.TestMode)

	taskEngine := &mockBatchDeleteTaskEngine{}
	mountPointService := &mockBatchDeleteMountPointService{
		mountPoints: map[int64]*models.MountPoint{
			11: {ID: 101, FileId: 11, FullPath: "/movies", CreatorUserID: 100},
			33: {ID: 303, FileId: 33, FullPath: "/series", CreatorUserID: 100},
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

	req := httptest.NewRequestWithContext(stdctx.Background(), http.MethodPost, "/batch_modify_token", strings.NewReader(`{"ids":[11,22,33],"tokenId":0}`))
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("expected not found, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	if got, want := mountPointService.queries, []int64{11, 22}; !int64SlicesEqual(got, want) {
		t.Fatalf("expected querying to stop at missing mount point %v, got %v", want, got)
	}

	if len(taskEngine.payloads) != 0 {
		t.Fatalf("expected no queued task, got %d", len(taskEngine.payloads))
	}
}

func TestBatchModifyTokenReturnsNotFoundWhenMountPointQueryReturnsNil(t *testing.T) {
	gin.SetMode(gin.TestMode)

	taskEngine := &mockBatchDeleteTaskEngine{}
	mountPointService := &mockBatchDeleteMountPointService{
		mountPoints: map[int64]*models.MountPoint{
			11: {ID: 101, FileId: 11, FullPath: "/movies", CreatorUserID: 100},
			22: nil,
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

	req := httptest.NewRequestWithContext(stdctx.Background(), http.MethodPost, "/batch_modify_token", strings.NewReader(`{"ids":[11,22],"tokenId":0}`))
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("expected not found, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	if got, want := mountPointService.queries, []int64{11, 22}; !int64SlicesEqual(got, want) {
		t.Fatalf("expected querying to stop at nil mount point %v, got %v", want, got)
	}

	if len(taskEngine.payloads) != 0 {
		t.Fatalf("expected no queued task, got %d", len(taskEngine.payloads))
	}
}

func TestBatchModifyTokenReturnsNotFoundWhenCloudTokenNotFound(t *testing.T) {
	gin.SetMode(gin.TestMode)

	taskEngine := &mockBatchDeleteTaskEngine{}
	mountPointService := &mockBatchDeleteMountPointService{
		mountPoints: map[int64]*models.MountPoint{
			11: {ID: 101, FileId: 11, FullPath: "/movies", CreatorUserID: 100},
		},
	}
	cloudTokenService := &mockBatchModifyCloudTokenService{err: gorm.ErrRecordNotFound}

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
		cloudTokenService,
		mountPointService,
		nil,
		nil,
		nil,
		nil,
		nil,
	).BatchModifyToken()))

	req := httptest.NewRequestWithContext(stdctx.Background(), http.MethodPost, "/batch_modify_token", strings.NewReader(`{"ids":[11],"tokenId":99}`))
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("expected not found, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	if got, want := cloudTokenService.queries, []int64{99}; !int64SlicesEqual(got, want) {
		t.Fatalf("expected cloud token query %v, got %v", want, got)
	}

	if len(mountPointService.queries) != 0 {
		t.Fatalf("expected request to stop before querying mount points, got %v", mountPointService.queries)
	}

	if len(taskEngine.payloads) != 0 {
		t.Fatalf("expected no queued task, got %d", len(taskEngine.payloads))
	}
}

func TestBatchModifyTokenReturnsNotFoundWhenCloudTokenQueryReturnsNil(t *testing.T) {
	gin.SetMode(gin.TestMode)

	taskEngine := &mockBatchDeleteTaskEngine{}
	mountPointService := &mockBatchDeleteMountPointService{
		mountPoints: map[int64]*models.MountPoint{
			11: {ID: 101, FileId: 11, FullPath: "/movies", CreatorUserID: 100},
		},
	}
	cloudTokenService := &mockBatchModifyCloudTokenService{
		tokens: map[int64]*models.CloudToken{99: nil},
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
		cloudTokenService,
		mountPointService,
		nil,
		nil,
		nil,
		nil,
		nil,
	).BatchModifyToken()))

	req := httptest.NewRequestWithContext(stdctx.Background(), http.MethodPost, "/batch_modify_token", strings.NewReader(`{"ids":[11],"tokenId":99}`))
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("expected not found, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	if got, want := cloudTokenService.queries, []int64{99}; !int64SlicesEqual(got, want) {
		t.Fatalf("expected cloud token query %v, got %v", want, got)
	}

	if len(mountPointService.queries) != 0 {
		t.Fatalf("expected request to stop before querying mount points, got %v", mountPointService.queries)
	}

	if len(taskEngine.payloads) != 0 {
		t.Fatalf("expected no queued task, got %d", len(taskEngine.payloads))
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

func TestModifyTokenRejectsInvalidIDsBeforeQuerying(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name string
		body string
	}{
		{
			name: "zero mount point id",
			body: `{"id":0,"tokenId":0}`,
		},
		{
			name: "negative token id",
			body: `{"id":11,"tokenId":-1}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mountPointService := &mockBatchDeleteMountPointService{}

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
				nil,
			).ModifyToken()))

			req := httptest.NewRequestWithContext(stdctx.Background(), http.MethodPost, "/modify_token", strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/json")

			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, req)

			if recorder.Code != http.StatusBadRequest {
				t.Fatalf("expected bad request, got %d body=%s", recorder.Code, recorder.Body.String())
			}

			if len(mountPointService.queries) != 0 {
				t.Fatalf("expected request to stop before querying mount points, got %v", mountPointService.queries)
			}
		})
	}
}

func TestModifyTokenReturnsNotFoundWhenMountPointMissing(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mountPointService := &mockBatchDeleteMountPointService{}
	cloudTokenService := &mockBatchModifyCloudTokenService{}
	userTokenService := &mockBatchModifyUserMountPointTokenService{}

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
		cloudTokenService,
		mountPointService,
		nil,
		nil,
		nil,
		nil,
		userTokenService,
	).ModifyToken()))

	req := httptest.NewRequestWithContext(stdctx.Background(), http.MethodPost, "/modify_token", strings.NewReader(`{"id":11,"tokenId":99}`))
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("expected not found, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	if got, want := mountPointService.queries, []int64{11}; !int64SlicesEqual(got, want) {
		t.Fatalf("expected mount point query %v, got %v", want, got)
	}

	if len(cloudTokenService.queries) != 0 {
		t.Fatalf("expected request to stop before querying cloud token, got %v", cloudTokenService.queries)
	}

	if len(userTokenService.bound) != 0 || len(userTokenService.unbound) != 0 {
		t.Fatalf("expected no token binding changes, got bound=%v unbound=%v", userTokenService.bound, userTokenService.unbound)
	}
}

func TestModifyTokenReturnsNotFoundWhenMountPointQueryReturnsNil(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mountPointService := &mockBatchDeleteMountPointService{
		mountPoints: map[int64]*models.MountPoint{
			11: nil,
		},
	}
	cloudTokenService := &mockBatchModifyCloudTokenService{}
	userTokenService := &mockBatchModifyUserMountPointTokenService{}

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
		cloudTokenService,
		mountPointService,
		nil,
		nil,
		nil,
		nil,
		userTokenService,
	).ModifyToken()))

	req := httptest.NewRequestWithContext(stdctx.Background(), http.MethodPost, "/modify_token", strings.NewReader(`{"id":11,"tokenId":99}`))
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("expected not found, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	if got, want := mountPointService.queries, []int64{11}; !int64SlicesEqual(got, want) {
		t.Fatalf("expected mount point query %v, got %v", want, got)
	}

	if len(cloudTokenService.queries) != 0 {
		t.Fatalf("expected request to stop before querying cloud token, got %v", cloudTokenService.queries)
	}

	if len(userTokenService.bound) != 0 || len(userTokenService.unbound) != 0 {
		t.Fatalf("expected no token binding changes, got bound=%v unbound=%v", userTokenService.bound, userTokenService.unbound)
	}
}

func TestModifyTokenReturnsNotFoundWhenCloudTokenNotFound(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mountPointService := &mockBatchDeleteMountPointService{
		mountPoints: map[int64]*models.MountPoint{
			11: {ID: 101, FileId: 11, FullPath: "/movies", CreatorUserID: 100},
		},
	}
	cloudTokenService := &mockBatchModifyCloudTokenService{err: gorm.ErrRecordNotFound}
	userTokenService := &mockBatchModifyUserMountPointTokenService{}

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
		cloudTokenService,
		mountPointService,
		nil,
		nil,
		nil,
		nil,
		userTokenService,
	).ModifyToken()))

	req := httptest.NewRequestWithContext(stdctx.Background(), http.MethodPost, "/modify_token", strings.NewReader(`{"id":11,"tokenId":99}`))
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("expected not found, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	if got, want := mountPointService.queries, []int64{11}; !int64SlicesEqual(got, want) {
		t.Fatalf("expected mount point query %v, got %v", want, got)
	}

	if got, want := cloudTokenService.queries, []int64{99}; !int64SlicesEqual(got, want) {
		t.Fatalf("expected cloud token query %v, got %v", want, got)
	}

	if len(userTokenService.bound) != 0 || len(userTokenService.unbound) != 0 {
		t.Fatalf("expected no token binding changes, got bound=%v unbound=%v", userTokenService.bound, userTokenService.unbound)
	}
}

func TestModifyTokenReturnsNotFoundWhenCloudTokenQueryReturnsNil(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mountPointService := &mockBatchDeleteMountPointService{
		mountPoints: map[int64]*models.MountPoint{
			11: {ID: 101, FileId: 11, FullPath: "/movies", CreatorUserID: 100},
		},
	}
	cloudTokenService := &mockBatchModifyCloudTokenService{
		tokens: map[int64]*models.CloudToken{99: nil},
	}
	userTokenService := &mockBatchModifyUserMountPointTokenService{}

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
		cloudTokenService,
		mountPointService,
		nil,
		nil,
		nil,
		nil,
		userTokenService,
	).ModifyToken()))

	req := httptest.NewRequestWithContext(stdctx.Background(), http.MethodPost, "/modify_token", strings.NewReader(`{"id":11,"tokenId":99}`))
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("expected not found, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	if got, want := mountPointService.queries, []int64{11}; !int64SlicesEqual(got, want) {
		t.Fatalf("expected mount point query %v, got %v", want, got)
	}

	if got, want := cloudTokenService.queries, []int64{99}; !int64SlicesEqual(got, want) {
		t.Fatalf("expected cloud token query %v, got %v", want, got)
	}

	if len(userTokenService.bound) != 0 || len(userTokenService.unbound) != 0 {
		t.Fatalf("expected no token binding changes, got bound=%v unbound=%v", userTokenService.bound, userTokenService.unbound)
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

func TestModifyTokenFailsWhenUserTokenServiceMissing(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mountPointService := &mockBatchDeleteMountPointService{
		mountPoints: map[int64]*models.MountPoint{
			11: {ID: 101, FileId: 11, FullPath: "/movies", CreatorUserID: 100},
		},
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
		nil,
	).ModifyToken()))

	req := httptest.NewRequestWithContext(stdctx.Background(), http.MethodPost, "/modify_token", strings.NewReader(`{"id":11,"tokenId":0}`))
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected bad request, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	var response struct {
		Code int `json:"code"`
	}

	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}

	if response.Code != busCodeStorageModifyTokenFailed.GetCode() {
		t.Fatalf("expected modify token business code %d, got %d", busCodeStorageModifyTokenFailed.GetCode(), response.Code)
	}
}

func TestModifyTokenFailsWhenGroupBindingServiceMissing(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mountPointService := &mockBatchDeleteMountPointService{
		mountPoints: map[int64]*models.MountPoint{
			11: {ID: 101, FileId: 11, FullPath: "/shared", CreatorUserID: 200},
		},
	}

	router := gin.New()
	router.Use(func(ctx *gin.Context) {
		ctx.Set(consts.CtxKeyUserId, int64(100))
		ctx.Set(consts.CtxKeyIsAdmin, false)
		ctx.Set(consts.CtxKeyUserGroupId, int64(7))
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
		&mockBatchModifyUserMountPointTokenService{},
	).ModifyToken()))

	req := httptest.NewRequestWithContext(stdctx.Background(), http.MethodPost, "/modify_token", strings.NewReader(`{"id":11,"tokenId":0}`))
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected bad request, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	var response struct {
		Code int `json:"code"`
	}

	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}

	if response.Code != busCodeStorageQueryMountPointError.GetCode() {
		t.Fatalf("expected query mount point business code %d, got %d", busCodeStorageQueryMountPointError.GetCode(), response.Code)
	}
}

func TestBatchModifyTokenFailsWhenTaskEngineMissing(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mountPointService := &mockBatchDeleteMountPointService{
		mountPoints: map[int64]*models.MountPoint{
			11: {ID: 101, FileId: 11, FullPath: "/movies", CreatorUserID: 100},
		},
	}

	router := gin.New()
	router.Use(func(ctx *gin.Context) {
		ctx.Set(consts.CtxKeyUserId, int64(100))
		ctx.Set(consts.CtxKeyIsAdmin, false)
	})

	wrapper := httpcontext.NewHandlerFuncWrapper(zap.NewNop())
	router.POST("/batch_modify_token", wrapper.Wrap(NewHandler(
		nil,
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

	req := httptest.NewRequestWithContext(stdctx.Background(), http.MethodPost, "/batch_modify_token", strings.NewReader(`{"ids":[11],"tokenId":0}`))
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected bad request, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	var response struct {
		Code int `json:"code"`
	}

	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}

	if response.Code != busCodeStorageSendTaskFail.GetCode() {
		t.Fatalf("expected send task business code %d, got %d", busCodeStorageSendTaskFail.GetCode(), response.Code)
	}
}

func TestBatchModifyTokenFailsWhenGroupBindingServiceMissing(t *testing.T) {
	gin.SetMode(gin.TestMode)

	taskEngine := &mockBatchDeleteTaskEngine{}
	mountPointService := &mockBatchDeleteMountPointService{
		mountPoints: map[int64]*models.MountPoint{
			11: {ID: 101, FileId: 11, FullPath: "/shared", CreatorUserID: 200},
		},
	}

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
		mountPointService,
		nil,
		nil,
		nil,
		nil,
		nil,
	).BatchModifyToken()))

	req := httptest.NewRequestWithContext(stdctx.Background(), http.MethodPost, "/batch_modify_token", strings.NewReader(`{"ids":[11],"tokenId":0}`))
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected bad request, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	var response struct {
		Code int `json:"code"`
	}

	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}

	if response.Code != busCodeStorageQueryMountPointError.GetCode() {
		t.Fatalf("expected query mount point business code %d, got %d", busCodeStorageQueryMountPointError.GetCode(), response.Code)
	}

	if len(taskEngine.payloads) != 0 {
		t.Fatalf("expected no queued task, got %d", len(taskEngine.payloads))
	}
}
