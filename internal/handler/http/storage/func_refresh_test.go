package storage

import (
	stdctx "context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/xxcheng123/cloudpan189-share/internal/consts"
	"github.com/xxcheng123/cloudpan189-share/internal/framework/httpcontext"
	"github.com/xxcheng123/cloudpan189-share/internal/repository/models"
	"github.com/xxcheng123/cloudpan189-share/internal/types/topic"
	"go.uber.org/zap"
)

func TestRefreshRejectsMountPointOwnedByOtherUser(t *testing.T) {
	gin.SetMode(gin.TestMode)

	taskEngine := &mockBatchDeleteTaskEngine{}
	mountPointService := &mockBatchDeleteMountPointService{
		mountPoints: map[int64]*models.MountPoint{
			11: {FileId: 11, FullPath: "/other", CreatorUserID: 200},
		},
	}

	router := gin.New()
	router.Use(func(ctx *gin.Context) {
		ctx.Set(consts.CtxKeyUserId, int64(100))
		ctx.Set(consts.CtxKeyIsAdmin, false)
	})

	wrapper := httpcontext.NewHandlerFuncWrapper(zap.NewNop())
	router.POST("/refresh", wrapper.Wrap(NewHandler(
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
	).Refresh()))

	req := httptest.NewRequestWithContext(stdctx.Background(), http.MethodPost, "/refresh", strings.NewReader(`{"id":11}`))
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code == http.StatusOK {
		t.Fatalf("expected unauthorized refresh to fail, got body=%s", recorder.Body.String())
	}

	if len(taskEngine.payloads) != 0 {
		t.Fatalf("expected no refresh task for unauthorized mount point, got %d", len(taskEngine.payloads))
	}
}

func TestRefreshRejectsInvalidIDBeforeQuerying(t *testing.T) {
	gin.SetMode(gin.TestMode)

	taskEngine := &mockBatchDeleteTaskEngine{}
	mountPointService := &mockBatchDeleteMountPointService{
		mountPoints: map[int64]*models.MountPoint{},
	}

	router := gin.New()
	router.Use(func(ctx *gin.Context) {
		ctx.Set(consts.CtxKeyUserId, int64(100))
		ctx.Set(consts.CtxKeyIsAdmin, false)
	})

	wrapper := httpcontext.NewHandlerFuncWrapper(zap.NewNop())
	router.POST("/refresh", wrapper.Wrap(NewHandler(
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
	).Refresh()))

	req := httptest.NewRequestWithContext(stdctx.Background(), http.MethodPost, "/refresh", strings.NewReader(`{"id":-1}`))
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
		t.Fatalf("expected no refresh task for invalid request, got %d", len(taskEngine.payloads))
	}
}

func TestRefreshAllowsMountPointOwner(t *testing.T) {
	gin.SetMode(gin.TestMode)

	taskEngine := &mockBatchDeleteTaskEngine{}
	mountPointService := &mockBatchDeleteMountPointService{
		mountPoints: map[int64]*models.MountPoint{
			11: {FileId: 11, FullPath: "/mine", CreatorUserID: 100},
		},
	}

	router := gin.New()
	router.Use(func(ctx *gin.Context) {
		ctx.Set(consts.CtxKeyUserId, int64(100))
		ctx.Set(consts.CtxKeyIsAdmin, false)
	})

	wrapper := httpcontext.NewHandlerFuncWrapper(zap.NewNop())
	router.POST("/refresh", wrapper.Wrap(NewHandler(
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
	).Refresh()))

	req := httptest.NewRequestWithContext(stdctx.Background(), http.MethodPost, "/refresh", strings.NewReader(`{"id":11,"deep":true}`))
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected owner refresh to succeed, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	if len(taskEngine.payloads) != 1 {
		t.Fatalf("expected one refresh task, got %d", len(taskEngine.payloads))
	}

	if taskEngine.paths[0] != "/mine" {
		t.Fatalf("expected refresh task path /mine, got %v", taskEngine.paths[0])
	}
}

func TestRefreshQueuesMountPointFileID(t *testing.T) {
	gin.SetMode(gin.TestMode)

	taskEngine := &mockBatchDeleteTaskEngine{}
	mountPointService := &mockBatchDeleteMountPointService{
		mountPoints: map[int64]*models.MountPoint{
			11: {ID: 11, FileId: 99, FullPath: "/mine", CreatorUserID: 100},
		},
	}

	router := gin.New()
	router.Use(func(ctx *gin.Context) {
		ctx.Set(consts.CtxKeyUserId, int64(100))
		ctx.Set(consts.CtxKeyIsAdmin, false)
	})

	wrapper := httpcontext.NewHandlerFuncWrapper(zap.NewNop())
	router.POST("/refresh", wrapper.Wrap(NewHandler(
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
	).Refresh()))

	req := httptest.NewRequestWithContext(stdctx.Background(), http.MethodPost, "/refresh", strings.NewReader(`{"id":11,"deep":true}`))
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected refresh to succeed, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	if len(taskEngine.payloads) != 1 {
		t.Fatalf("expected one refresh task, got %d", len(taskEngine.payloads))
	}

	var task topic.FileScanFileRequest
	if err := json.Unmarshal(taskEngine.payloads[0], &task); err != nil {
		t.Fatal(err)
	}

	if task.FileId != 99 || !task.Deep {
		t.Fatalf("expected queued virtual file id 99 with deep=true, got %+v", task)
	}
}
