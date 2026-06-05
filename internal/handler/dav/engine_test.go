package dav

import (
	stdctx "context"
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
	mountpointSvi "github.com/xxcheng123/cloudpan189-share/internal/services/mountpoint"
	virtualfileSvi "github.com/xxcheng123/cloudpan189-share/internal/services/virtualfile"
	"go.uber.org/zap"
)

type mockDAVVirtualFileService struct {
	virtualfileSvi.Service
	children   []*models.VirtualFile
	pathByID   map[int64]string
	fileByPath map[string]*models.VirtualFile
	queryPaths []string
}

func (m *mockDAVVirtualFileService) List(ctx appContext.Context, req *virtualfileSvi.ListRequest) ([]*models.VirtualFile, error) {
	return m.children, nil
}

func (m *mockDAVVirtualFileService) CalFullPath(ctx appContext.Context, id int64) (string, error) {
	if path, ok := m.pathByID[id]; ok {
		return path, nil
	}

	return "/", nil
}

func (m *mockDAVVirtualFileService) QueryByPath(ctx appContext.Context, path string) (*models.VirtualFile, error) {
	m.queryPaths = append(m.queryPaths, path)

	if file, ok := m.fileByPath[path]; ok {
		return file, nil
	}

	return nil, errors.New("not found")
}

type mockDAVMountPointService struct {
	mountpointSvi.Service
	accessibleIDs []int64
}

func (m *mockDAVMountPointService) GetAccessibleMountPointIDs(
	ctx appContext.Context,
	userID int64,
	isAdmin bool,
	groupFileIDs []int64,
) ([]int64, error) {
	return m.accessibleIDs, nil
}

func TestDAVVirtualFileVisibleAllowsAncestorDirectoryWithNonZeroTopID(t *testing.T) {
	file := &models.VirtualFile{
		ID:     10,
		TopId:  10,
		Name:   "collection",
		IsDir:  true,
		IsTop:  true,
		OsType: models.OsTypeFolder,
	}

	if !isVirtualFileVisible(file, "/collection", []int64{100}, []string{"/collection/movies"}) {
		t.Fatal("expected ancestor directory with non-zero top id to be visible")
	}
}

func TestDAVOpenUsesRawEscapedPathForLookup(t *testing.T) {
	gin.SetMode(gin.TestMode)

	virtualFileService := &mockDAVVirtualFileService{
		fileByPath: map[string]*models.VirtualFile{
			"/literal/a%252Fb": {
				ID:       12,
				TopId:    12,
				Name:     "a%2Fb",
				IsDir:    true,
				IsTop:    true,
				OsType:   models.OsTypeFolder,
				ParentId: 0,
			},
		},
		pathByID: map[int64]string{
			12: "/literal/a%2Fb",
		},
	}

	router := gin.New()
	router.Use(func(ctx *gin.Context) {
		ctx.Set(consts.CtxKeyUserId, int64(100))
		ctx.Set(consts.CtxKeyIsAdmin, true)
	})

	engine := &workEngine{
		virtualFileService: virtualFileService,
		mountPointService:  &mockDAVMountPointService{accessibleIDs: []int64{12}},
	}
	wrapper := httpcontext.NewHandlerFuncWrapper(zap.NewNop())
	router.Handle("PROPFIND", "/dav/*path", wrapper.Wrap(engine.Open()))

	req := httptest.NewRequestWithContext(stdctx.Background(), "PROPFIND", "/dav/literal/a%252Fb", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusMultiStatus {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusMultiStatus, recorder.Code, recorder.Body.String())
	}

	if got, want := virtualFileService.queryPaths, []string{"/literal/a%252Fb"}; len(got) != len(want) || got[0] != want[0] {
		t.Fatalf("expected query path %v, got %v", want, got)
	}

	body := recorder.Body.String()
	if !strings.Contains(body, "<D:href>/dav/literal/a%252Fb/</D:href>") {
		t.Fatalf("expected escaped DAV href for literal %%2F name, got body=%s", body)
	}

	if !strings.Contains(body, "<D:displayname>a%2Fb</D:displayname>") {
		t.Fatalf("expected display name to remain literal %%2F text, got body=%s", body)
	}
}

func TestDAVOpenRejectsEncodedSeparatorPath(t *testing.T) {
	gin.SetMode(gin.TestMode)

	virtualFileService := &mockDAVVirtualFileService{}

	router := gin.New()
	router.Use(func(ctx *gin.Context) {
		ctx.Set(consts.CtxKeyUserId, int64(100))
		ctx.Set(consts.CtxKeyIsAdmin, true)
	})

	engine := &workEngine{
		virtualFileService: virtualFileService,
		mountPointService:  &mockDAVMountPointService{},
	}
	wrapper := httpcontext.NewHandlerFuncWrapper(zap.NewNop())
	router.Handle("PROPFIND", "/dav/*path", wrapper.Wrap(engine.Open()))

	req := httptest.NewRequestWithContext(stdctx.Background(), "PROPFIND", "/dav/literal/a%2Fb", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusBadRequest, recorder.Code, recorder.Body.String())
	}

	if len(virtualFileService.queryPaths) > 0 {
		t.Fatalf("expected invalid path to be rejected before query, got query paths %v", virtualFileService.queryPaths)
	}
}
