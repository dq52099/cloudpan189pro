package storage

import (
	stdctx "context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	appContext "github.com/xxcheng123/cloudpan189-share/internal/framework/context"
	"github.com/xxcheng123/cloudpan189-share/internal/framework/httpcontext"
	"github.com/xxcheng123/cloudpan189-share/internal/repository/models"
	mediafileSvi "github.com/xxcheng123/cloudpan189-share/internal/services/mediafile"
	mountPointSvi "github.com/xxcheng123/cloudpan189-share/internal/services/mountpoint"
	virtualfileSvi "github.com/xxcheng123/cloudpan189-share/internal/services/virtualfile"
	"github.com/xxcheng123/cloudpan189-share/internal/shared"
	"go.uber.org/zap"
)

type mockClearAllMountPointService struct {
	mountPointSvi.Service
	called bool
}

func (m *mockClearAllMountPointService) ClearAll(ctx appContext.Context) (int64, error) {
	m.called = true

	return 3, nil
}

type mockClearAllVirtualFileService struct {
	virtualfileSvi.Service
	called bool
}

func (m *mockClearAllVirtualFileService) ClearAll(ctx appContext.Context) error {
	m.called = true

	return nil
}

type mockClearAllMediaFileService struct {
	mediafileSvi.Service
	clearAllCalled bool
	clearCalled    bool
	clearPath      string
}

func (m *mockClearAllMediaFileService) Clear(ctx appContext.Context, rootPath string) error {
	m.clearCalled = true
	m.clearPath = rootPath

	return nil
}

func (m *mockClearAllMediaFileService) ClearAll(ctx appContext.Context) error {
	m.clearAllCalled = true

	return nil
}

func newClearAllTestRouter(
	mountPointService mountPointSvi.Service,
	virtualFileService virtualfileSvi.Service,
	mediaFileService mediafileSvi.Service,
) *gin.Engine {
	router := gin.New()
	wrapper := httpcontext.NewHandlerFuncWrapper(zap.NewNop())
	router.POST("/clear_all", wrapper.Wrap(NewHandler(
		nil,
		virtualFileService,
		nil,
		nil,
		mountPointService,
		nil,
		nil,
		mediaFileService,
		nil,
		nil,
	).ClearAll()))

	return router
}

func TestClearAllRejectsMalformedJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := newClearAllTestRouter(nil, nil, nil)

	req := httptest.NewRequestWithContext(stdctx.Background(), http.MethodPost, "/clear_all", strings.NewReader("{"))
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected bad request, got %d", recorder.Code)
	}
}

func TestClearAllWithoutDeleteFilesOnlyClearsMediaDB(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mountPointService := &mockClearAllMountPointService{}
	virtualFileService := &mockClearAllVirtualFileService{}
	mediaFileService := &mockClearAllMediaFileService{}
	router := newClearAllTestRouter(mountPointService, virtualFileService, mediaFileService)

	req := httptest.NewRequestWithContext(stdctx.Background(), http.MethodPost, "/clear_all", strings.NewReader("{}"))
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected ok, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	if !mountPointService.called {
		t.Fatal("expected mount points to be cleared")
	}

	if !virtualFileService.called {
		t.Fatal("expected virtual files to be cleared")
	}

	if !mediaFileService.clearAllCalled {
		t.Fatal("expected media DB clear to be called")
	}

	if mediaFileService.clearCalled {
		t.Fatal("did not expect local media files to be deleted")
	}
}

func TestClearAllWithDeleteFilesClearsConfiguredMediaPath(t *testing.T) {
	gin.SetMode(gin.TestMode)

	oldMediaConfig := shared.MediaConfig
	shared.MediaConfig = &models.MediaConfig{StoragePath: "/tmp/cloudpan-media"}

	t.Cleanup(func() {
		shared.MediaConfig = oldMediaConfig
	})

	mountPointService := &mockClearAllMountPointService{}
	virtualFileService := &mockClearAllVirtualFileService{}
	mediaFileService := &mockClearAllMediaFileService{}
	router := newClearAllTestRouter(mountPointService, virtualFileService, mediaFileService)

	req := httptest.NewRequestWithContext(stdctx.Background(), http.MethodPost, "/clear_all", strings.NewReader(`{"deleteFiles":true}`))
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected ok, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	if !mediaFileService.clearCalled {
		t.Fatal("expected local media files to be cleared")
	}

	if mediaFileService.clearPath != "/tmp/cloudpan-media" {
		t.Fatalf("expected configured media path, got %q", mediaFileService.clearPath)
	}

	if mediaFileService.clearAllCalled {
		t.Fatal("did not expect media DB-only clear when deleteFiles is true")
	}
}

func TestClearAllWithDeleteFilesRejectsMissingMediaPath(t *testing.T) {
	gin.SetMode(gin.TestMode)

	oldMediaConfig := shared.MediaConfig
	shared.MediaConfig = nil

	t.Cleanup(func() {
		shared.MediaConfig = oldMediaConfig
	})

	mountPointService := &mockClearAllMountPointService{}
	virtualFileService := &mockClearAllVirtualFileService{}
	mediaFileService := &mockClearAllMediaFileService{}
	router := newClearAllTestRouter(mountPointService, virtualFileService, mediaFileService)

	req := httptest.NewRequestWithContext(stdctx.Background(), http.MethodPost, "/clear_all", strings.NewReader(`{"deleteFiles":true}`))
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected bad request, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	if mountPointService.called || virtualFileService.called || mediaFileService.clearAllCalled || mediaFileService.clearCalled {
		t.Fatal("expected no destructive cleanup when media path is missing")
	}
}
