package file

import (
	stdctx "context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/xxcheng123/cloudpan189-share/internal/consts"
	appContext "github.com/xxcheng123/cloudpan189-share/internal/framework/context"
	"github.com/xxcheng123/cloudpan189-share/internal/framework/httpcontext"
	"github.com/xxcheng123/cloudpan189-share/internal/repository/models"
	group2fileSvi "github.com/xxcheng123/cloudpan189-share/internal/services/group2file"
	mountpointSvi "github.com/xxcheng123/cloudpan189-share/internal/services/mountpoint"
	virtualfileSvi "github.com/xxcheng123/cloudpan189-share/internal/services/virtualfile"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type mockOpenGroup2FileService struct {
	group2fileSvi.Service
	err error
}

func (m *mockOpenGroup2FileService) GetBindFiles(ctx appContext.Context, groupID int64) ([]int64, error) {
	return nil, m.err
}

type mockOpenVirtualFileService struct {
	virtualfileSvi.Service
	children    []*models.VirtualFile
	pathByID    map[int64]string
	fileByPath  map[string]*models.VirtualFile
	queryPaths  []string
	calPathErr  error
	queryByPath error
}

func (m *mockOpenVirtualFileService) List(ctx appContext.Context, req *virtualfileSvi.ListRequest) ([]*models.VirtualFile, error) {
	return m.children, nil
}

func (m *mockOpenVirtualFileService) CalFullPath(ctx appContext.Context, id int64) (string, error) {
	if m.calPathErr != nil {
		return "", m.calPathErr
	}

	if path, ok := m.pathByID[id]; ok {
		return path, nil
	}

	return "/", nil
}

func (m *mockOpenVirtualFileService) QueryByPath(ctx appContext.Context, path string) (*models.VirtualFile, error) {
	m.queryPaths = append(m.queryPaths, path)

	if m.queryByPath != nil {
		return nil, m.queryByPath
	}

	if file, ok := m.fileByPath[path]; ok {
		return file, nil
	}

	return nil, errors.New("not found")
}

type mockOpenMountPointService struct {
	mountpointSvi.Service
	accessibleIDs []int64
}

func (m *mockOpenMountPointService) GetAccessibleMountPointIDs(
	ctx appContext.Context,
	userID int64,
	isAdmin bool,
	groupFileIDs []int64,
) ([]int64, error) {
	return m.accessibleIDs, nil
}

func TestOpenFailsWhenGroupBindingLookupFails(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(func(ctx *gin.Context) {
		ctx.Set(consts.CtxKeyUserId, int64(100))
		ctx.Set(consts.CtxKeyIsAdmin, false)
		ctx.Set(consts.CtxKeyUserGroupId, int64(7))
	})

	wrapper := httpcontext.NewHandlerFuncWrapper(zap.NewNop())
	router.GET("/open/*fullPath", wrapper.Wrap(NewHandler(
		nil,
		nil,
		nil,
		nil,
		nil,
		&mockOpenGroup2FileService{err: errors.New("group lookup failed")},
		nil,
		nil,
	).Open()))

	req := httptest.NewRequestWithContext(stdctx.Background(), http.MethodGet, "/open/", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code == http.StatusOK {
		t.Fatalf("expected failure, got %d body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestOpenReturnsErrorWhenGroupBindingServiceMissing(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(func(ctx *gin.Context) {
		ctx.Set(consts.CtxKeyUserId, int64(100))
		ctx.Set(consts.CtxKeyIsAdmin, false)
		ctx.Set(consts.CtxKeyUserGroupId, int64(7))
	})

	wrapper := httpcontext.NewHandlerFuncWrapper(zap.NewNop())
	router.GET("/open/*fullPath", wrapper.Wrap(NewHandler(
		&mockOpenVirtualFileService{},
		nil,
		nil,
		nil,
		&mockOpenMountPointService{accessibleIDs: []int64{100}},
		nil,
		nil,
		nil,
	).Open()))

	req := httptest.NewRequestWithContext(stdctx.Background(), http.MethodGet, "/open/", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	assertFileHTTPError(t, recorder, http.StatusBadRequest, busCodeQueryTopIdError)
}

func TestOpenReturnsErrorWhenMountPointServiceTypedNil(t *testing.T) {
	gin.SetMode(gin.TestMode)

	var mountPointService *mockOpenMountPointService

	router := gin.New()
	router.Use(func(ctx *gin.Context) {
		ctx.Set(consts.CtxKeyUserId, int64(100))
		ctx.Set(consts.CtxKeyIsAdmin, false)
	})

	wrapper := httpcontext.NewHandlerFuncWrapper(zap.NewNop())
	router.GET("/open/*fullPath", wrapper.Wrap(NewHandler(
		&mockOpenVirtualFileService{},
		nil,
		nil,
		nil,
		mountPointService,
		nil,
		nil,
		nil,
	).Open()))

	req := httptest.NewRequestWithContext(stdctx.Background(), http.MethodGet, "/open/", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	assertFileHTTPError(t, recorder, http.StatusBadRequest, busCodeQueryTopIdError)
}

func TestOpenReturnsErrorWhenVirtualFileServiceMissingForRoot(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(func(ctx *gin.Context) {
		ctx.Set(consts.CtxKeyUserId, int64(100))
		ctx.Set(consts.CtxKeyIsAdmin, false)
	})

	wrapper := httpcontext.NewHandlerFuncWrapper(zap.NewNop())
	router.GET("/open/*fullPath", wrapper.Wrap(NewHandler(
		nil,
		nil,
		nil,
		nil,
		&mockOpenMountPointService{accessibleIDs: []int64{100}},
		nil,
		nil,
		nil,
	).Open()))

	req := httptest.NewRequestWithContext(stdctx.Background(), http.MethodGet, "/open/", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	assertFileHTTPError(t, recorder, http.StatusBadRequest, busCodeFileQueryError)
}

func TestOpenReturnsErrorWhenVirtualFileServiceTypedNilForRoot(t *testing.T) {
	gin.SetMode(gin.TestMode)

	var virtualFileService *mockOpenVirtualFileService

	router := gin.New()
	router.Use(func(ctx *gin.Context) {
		ctx.Set(consts.CtxKeyUserId, int64(100))
		ctx.Set(consts.CtxKeyIsAdmin, false)
	})

	wrapper := httpcontext.NewHandlerFuncWrapper(zap.NewNop())
	router.GET("/open/*fullPath", wrapper.Wrap(NewHandler(
		virtualFileService,
		nil,
		nil,
		nil,
		&mockOpenMountPointService{accessibleIDs: []int64{100}},
		nil,
		nil,
		nil,
	).Open()))

	req := httptest.NewRequestWithContext(stdctx.Background(), http.MethodGet, "/open/", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	assertFileHTTPError(t, recorder, http.StatusBadRequest, busCodeFileQueryError)
}

func TestOpenReturnsNotFoundWhenPathMissing(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(func(ctx *gin.Context) {
		ctx.Set(consts.CtxKeyUserId, int64(100))
		ctx.Set(consts.CtxKeyIsAdmin, false)
	})

	wrapper := httpcontext.NewHandlerFuncWrapper(zap.NewNop())
	router.GET("/open/*fullPath", wrapper.Wrap(NewHandler(
		&mockOpenVirtualFileService{queryByPath: gorm.ErrRecordNotFound},
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
	).Open()))

	req := httptest.NewRequestWithContext(stdctx.Background(), http.MethodGet, "/open/missing", nil)
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
}

func TestOpenChildrenTotalUsesFilteredChildren(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(func(ctx *gin.Context) {
		ctx.Set(consts.CtxKeyUserId, int64(100))
		ctx.Set(consts.CtxKeyIsAdmin, false)
	})

	wrapper := httpcontext.NewHandlerFuncWrapper(zap.NewNop())
	router.GET("/open/*fullPath", wrapper.Wrap(NewHandler(
		&mockOpenVirtualFileService{children: []*models.VirtualFile{
			{ID: 1, TopId: 100, Name: "allowed.mp4", OsType: models.OsTypePersonFile},
			{ID: 2, TopId: 200, Name: "hidden.mp4", OsType: models.OsTypePersonFile},
			{ID: 3, TopId: 0, Name: "folder", OsType: models.OsTypeFolder, IsDir: true},
			{ID: 4, TopId: 0, Name: "other", OsType: models.OsTypeFolder, IsDir: true},
		}, pathByID: map[int64]string{
			100: "/folder/allowed",
		}},
		nil,
		nil,
		nil,
		&mockOpenMountPointService{accessibleIDs: []int64{100}},
		nil,
		nil,
		nil,
	).Open()))

	req := httptest.NewRequestWithContext(stdctx.Background(), http.MethodGet, "/open/", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected ok, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	var response struct {
		Code int          `json:"code"`
		Data openResponse `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}

	if response.Data.ChildrenTotal != 2 {
		t.Fatalf("expected filtered children total 2, got %d", response.Data.ChildrenTotal)
	}

	if len(response.Data.Children) != 2 {
		t.Fatalf("expected 2 visible children, got %d", len(response.Data.Children))
	}

	gotNames := make(map[string]bool, len(response.Data.Children))
	for _, child := range response.Data.Children {
		gotNames[child.Name] = true
	}

	if !gotNames["allowed.mp4"] || !gotNames["folder"] || gotNames["hidden.mp4"] || gotNames["other"] {
		t.Fatalf("unexpected visible children: %#v", gotNames)
	}
}

func TestOpenSkipsNilChildrenRows(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(func(ctx *gin.Context) {
		ctx.Set(consts.CtxKeyUserId, int64(100))
		ctx.Set(consts.CtxKeyIsAdmin, true)
	})

	wrapper := httpcontext.NewHandlerFuncWrapper(zap.NewNop())
	router.GET("/open/*fullPath", wrapper.Wrap(NewHandler(
		&mockOpenVirtualFileService{children: []*models.VirtualFile{
			nil,
			{ID: 1, TopId: 100, Name: "allowed.mp4", OsType: models.OsTypePersonFile},
		}},
		nil,
		nil,
		nil,
		&mockOpenMountPointService{accessibleIDs: []int64{100}},
		nil,
		nil,
		nil,
	).Open()))

	req := httptest.NewRequestWithContext(stdctx.Background(), http.MethodGet, "/open/", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected ok, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	var response struct {
		Code int          `json:"code"`
		Data openResponse `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}

	if response.Data.ChildrenTotal != 1 {
		t.Fatalf("expected one non-nil child, got %d", response.Data.ChildrenTotal)
	}

	if len(response.Data.Children) != 1 {
		t.Fatalf("expected one child DTO, got %d", len(response.Data.Children))
	}

	if response.Data.Children[0].Name != "allowed.mp4" {
		t.Fatalf("expected visible child, got %q", response.Data.Children[0].Name)
	}
}

func TestOpenHandlesPercentFileNamePath(t *testing.T) {
	gin.SetMode(gin.TestMode)

	virtualFileService := &mockOpenVirtualFileService{
		fileByPath: map[string]*models.VirtualFile{
			"/percent/a%25b": {
				ID:       11,
				TopId:    11,
				Name:     "a%b",
				IsDir:    true,
				IsTop:    true,
				OsType:   models.OsTypeFolder,
				ParentId: 0,
			},
		},
		pathByID: map[int64]string{
			11: "/percent/a%b",
		},
	}

	router := gin.New()
	router.Use(func(ctx *gin.Context) {
		ctx.Set(consts.CtxKeyUserId, int64(100))
		ctx.Set(consts.CtxKeyIsAdmin, false)
	})

	wrapper := httpcontext.NewHandlerFuncWrapper(zap.NewNop())
	router.GET("/open/*fullPath", wrapper.Wrap(NewHandler(
		virtualFileService,
		nil,
		nil,
		nil,
		&mockOpenMountPointService{accessibleIDs: []int64{11}},
		nil,
		nil,
		nil,
	).Open()))

	req := httptest.NewRequestWithContext(stdctx.Background(), http.MethodGet, "/open/percent/a%25b", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected ok, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	if got, want := virtualFileService.queryPaths, []string{"/percent/a%25b"}; len(got) != len(want) || got[0] != want[0] {
		t.Fatalf("expected query path %v, got %v", want, got)
	}

	var response struct {
		Code int          `json:"code"`
		Data openResponse `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}

	if response.Data.Href != "/percent/a%25b" {
		t.Fatalf("expected escaped href, got %q", response.Data.Href)
	}

	if response.Data.ApiPath != "/api/file/open/percent/a%25b" {
		t.Fatalf("expected escaped api path, got %q", response.Data.ApiPath)
	}
}

func TestOpenHandlesLiteralEscapedSeparatorFileNamePath(t *testing.T) {
	gin.SetMode(gin.TestMode)

	virtualFileService := &mockOpenVirtualFileService{
		children: []*models.VirtualFile{
			{
				ID:       13,
				TopId:    12,
				Name:     "child%b.txt",
				IsDir:    false,
				IsTop:    false,
				OsType:   models.OsTypePersonFile,
				ParentId: 12,
			},
		},
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

	wrapper := httpcontext.NewHandlerFuncWrapper(zap.NewNop())
	router.GET("/open/*fullPath", wrapper.Wrap(NewHandler(
		virtualFileService,
		nil,
		nil,
		nil,
		&mockOpenMountPointService{accessibleIDs: []int64{12}},
		nil,
		nil,
		nil,
	).Open()))

	req := httptest.NewRequestWithContext(stdctx.Background(), http.MethodGet, "/open/literal/a%252Fb", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected ok, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	if got, want := virtualFileService.queryPaths, []string{"/literal/a%252Fb"}; len(got) != len(want) || got[0] != want[0] {
		t.Fatalf("expected query path %v, got %v", want, got)
	}

	var response struct {
		Code int          `json:"code"`
		Data openResponse `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}

	if response.Data.Href != "/literal/a%252Fb" {
		t.Fatalf("expected escaped href, got %q", response.Data.Href)
	}

	if response.Data.ApiPath != "/api/file/open/literal/a%252Fb" {
		t.Fatalf("expected escaped api path, got %q", response.Data.ApiPath)
	}

	if len(response.Data.Breadcrumbs) != 2 {
		t.Fatalf("expected 2 breadcrumbs, got %d", len(response.Data.Breadcrumbs))
	}

	if response.Data.Breadcrumbs[1].Name != "a%2Fb" {
		t.Fatalf("expected display breadcrumb name, got %q", response.Data.Breadcrumbs[1].Name)
	}

	if response.Data.Breadcrumbs[1].Href != "/api/file/open/literal/a%252Fb" {
		t.Fatalf("expected escaped breadcrumb href, got %q", response.Data.Breadcrumbs[1].Href)
	}

	if len(response.Data.Children) != 1 {
		t.Fatalf("expected one child, got %d", len(response.Data.Children))
	}

	if response.Data.Children[0].Href != "/literal/a%252Fb/child%25b.txt" {
		t.Fatalf("expected child href to be escaped once, got %q", response.Data.Children[0].Href)
	}

	if response.Data.Children[0].ApiPath != "/api/file/open/literal/a%252Fb/child%25b.txt" {
		t.Fatalf("expected child api path to be escaped once, got %q", response.Data.Children[0].ApiPath)
	}
}

func TestOpenShowsAncestorDirectoryWithNonZeroTopID(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(func(ctx *gin.Context) {
		ctx.Set(consts.CtxKeyUserId, int64(100))
		ctx.Set(consts.CtxKeyIsAdmin, false)
	})

	wrapper := httpcontext.NewHandlerFuncWrapper(zap.NewNop())
	router.GET("/open/*fullPath", wrapper.Wrap(NewHandler(
		&mockOpenVirtualFileService{
			children: []*models.VirtualFile{
				{ID: 10, TopId: 10, Name: "collection", OsType: models.OsTypeFolder, IsDir: true, IsTop: true},
			},
			pathByID: map[int64]string{
				100: "/collection/movies",
			},
		},
		nil,
		nil,
		nil,
		&mockOpenMountPointService{accessibleIDs: []int64{100}},
		nil,
		nil,
		nil,
	).Open()))

	req := httptest.NewRequestWithContext(stdctx.Background(), http.MethodGet, "/open/", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected ok, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	var response struct {
		Code int          `json:"code"`
		Data openResponse `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}

	if response.Data.ChildrenTotal != 1 || len(response.Data.Children) != 1 {
		t.Fatalf("expected ancestor directory to be visible, got total=%d children=%d", response.Data.ChildrenTotal, len(response.Data.Children))
	}

	if response.Data.Children[0].Name != "collection" {
		t.Fatalf("expected collection ancestor directory, got %q", response.Data.Children[0].Name)
	}
}

func TestOpenRootReturnsCanonicalRootHref(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(func(ctx *gin.Context) {
		ctx.Set(consts.CtxKeyUserId, int64(100))
		ctx.Set(consts.CtxKeyIsAdmin, true)
	})

	wrapper := httpcontext.NewHandlerFuncWrapper(zap.NewNop())
	router.GET("/open/*fullPath", wrapper.Wrap(NewHandler(
		&mockOpenVirtualFileService{},
		nil,
		nil,
		nil,
		&mockOpenMountPointService{},
		nil,
		nil,
		nil,
	).Open()))

	req := httptest.NewRequestWithContext(stdctx.Background(), http.MethodGet, "/open/", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected ok, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	var response struct {
		Code int          `json:"code"`
		Data openResponse `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}

	if response.Data.Href != "/" {
		t.Fatalf("expected root href '/', got %q", response.Data.Href)
	}

	if response.Data.ApiPath != "/api/file/open" {
		t.Fatalf("expected root api path '/api/file/open', got %q", response.Data.ApiPath)
	}

	if len(response.Data.Breadcrumbs) != 0 {
		t.Fatalf("expected no breadcrumbs for root, got %#v", response.Data.Breadcrumbs)
	}
}

func TestOpenRejectsInaccessibleFolderOutsideVisibleMountPath(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(func(ctx *gin.Context) {
		ctx.Set(consts.CtxKeyUserId, int64(100))
		ctx.Set(consts.CtxKeyIsAdmin, false)
	})

	wrapper := httpcontext.NewHandlerFuncWrapper(zap.NewNop())
	router.GET("/open/*fullPath", wrapper.Wrap(NewHandler(
		&mockOpenVirtualFileService{
			fileByPath: map[string]*models.VirtualFile{
				"/other": {ID: 9, TopId: 0, Name: "other", OsType: models.OsTypeFolder, IsDir: true},
			},
			pathByID: map[int64]string{
				100: "/folder/allowed",
			},
		},
		nil,
		nil,
		nil,
		&mockOpenMountPointService{accessibleIDs: []int64{100}},
		nil,
		nil,
		nil,
	).Open()))

	req := httptest.NewRequestWithContext(stdctx.Background(), http.MethodGet, "/open/other", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code == http.StatusOK {
		t.Fatalf("expected inaccessible folder to fail, got %d body=%s", recorder.Code, recorder.Body.String())
	}
}
