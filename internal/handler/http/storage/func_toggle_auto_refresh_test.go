package storage

import (
	stdctx "context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/xxcheng123/cloudpan189-share/internal/consts"
	"github.com/xxcheng123/cloudpan189-share/internal/framework/httpcontext"
	"github.com/xxcheng123/cloudpan189-share/internal/repository/models"
	"go.uber.org/zap"
)

func TestToggleAutoRefreshRejectsMountPointOwnedByOtherUser(t *testing.T) {
	gin.SetMode(gin.TestMode)

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
	router.POST("/toggle_auto_refresh", wrapper.Wrap(NewHandler(
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
	).ToggleAutoRefresh()))

	req := httptest.NewRequestWithContext(stdctx.Background(), http.MethodPost, "/toggle_auto_refresh", strings.NewReader(`{"id":11,"enableAutoRefresh":true,"refreshInterval":30}`))
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code == http.StatusOK {
		t.Fatalf("expected unauthorized toggle to fail, got body=%s", recorder.Body.String())
	}

	if len(mountPointService.updateRefreshConfigCalls) != 0 || len(mountPointService.enableAutoRefreshCalls) != 0 {
		t.Fatalf("expected no auto-refresh updates for unauthorized mount point")
	}
}

func TestToggleAutoRefreshAllowsMountPointOwner(t *testing.T) {
	gin.SetMode(gin.TestMode)

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
	router.POST("/toggle_auto_refresh", wrapper.Wrap(NewHandler(
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
	).ToggleAutoRefresh()))

	req := httptest.NewRequestWithContext(stdctx.Background(), http.MethodPost, "/toggle_auto_refresh", strings.NewReader(`{"id":11,"enableAutoRefresh":true,"refreshInterval":30}`))
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected owner toggle to succeed, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	if len(mountPointService.updateRefreshConfigCalls) != 1 || mountPointService.updateRefreshConfigCalls[0] != 11 {
		t.Fatalf("expected refresh config update for mount point 11, got %v", mountPointService.updateRefreshConfigCalls)
	}

	if len(mountPointService.enableAutoRefreshCalls) != 1 || mountPointService.enableAutoRefreshCalls[0] != 11 {
		t.Fatalf("expected auto refresh enable for mount point 11, got %v", mountPointService.enableAutoRefreshCalls)
	}
}

func TestToggleAutoRefreshRejectsMissingEnableAutoRefresh(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mountPointService := &mockBatchDeleteMountPointService{
		mountPoints: map[int64]*models.MountPoint{
			11: {FileId: 11, FullPath: "/mine", CreatorUserID: 100},
		},
	}

	router := newToggleAutoRefreshTestRouter(mountPointService)
	req := httptest.NewRequestWithContext(
		stdctx.Background(),
		http.MethodPost,
		"/toggle_auto_refresh",
		strings.NewReader(`{"id":11,"refreshInterval":60}`),
	)
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected bad request, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	if len(mountPointService.updateRefreshConfigCalls) != 0 || len(mountPointService.enableAutoRefreshCalls) != 0 {
		t.Fatalf("expected missing enableAutoRefresh not to update config, got config=%v enable=%v",
			mountPointService.updateRefreshConfigCalls, mountPointService.enableAutoRefreshCalls)
	}
}

func TestToggleAutoRefreshAcceptsExplicitDisable(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mountPointService := &mockBatchDeleteMountPointService{
		mountPoints: map[int64]*models.MountPoint{
			11: {FileId: 11, FullPath: "/mine", CreatorUserID: 100},
		},
	}

	router := newToggleAutoRefreshTestRouter(mountPointService)
	req := httptest.NewRequestWithContext(
		stdctx.Background(),
		http.MethodPost,
		"/toggle_auto_refresh",
		strings.NewReader(`{"id":11,"enableAutoRefresh":false}`),
	)
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected success, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	if len(mountPointService.updateRefreshConfigCalls) != 0 {
		t.Fatalf("expected disable not to update refresh config, got %v", mountPointService.updateRefreshConfigCalls)
	}

	if len(mountPointService.enableAutoRefreshCalls) != 1 || mountPointService.enableAutoRefreshCalls[0] != 11 {
		t.Fatalf("expected explicit auto refresh disable for mount point 11, got %v", mountPointService.enableAutoRefreshCalls)
	}
}

func newToggleAutoRefreshTestRouter(mountPointService *mockBatchDeleteMountPointService) *gin.Engine {
	router := gin.New()
	router.Use(func(ctx *gin.Context) {
		ctx.Set(consts.CtxKeyUserId, int64(100))
		ctx.Set(consts.CtxKeyIsAdmin, false)
	})

	wrapper := httpcontext.NewHandlerFuncWrapper(zap.NewNop())
	router.POST("/toggle_auto_refresh", wrapper.Wrap(NewHandler(
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
	).ToggleAutoRefresh()))

	return router
}
