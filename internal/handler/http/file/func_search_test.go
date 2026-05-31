package file

import (
	stdctx "context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
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

type mockSearchVirtualFileService struct {
	virtualfileSvi.Service
	listReq  *virtualfileSvi.ListRequest
	countReq *virtualfileSvi.ListRequest
	list     []*models.VirtualFile
	count    int64
}

func (m *mockSearchVirtualFileService) List(ctx appContext.Context, req *virtualfileSvi.ListRequest) ([]*models.VirtualFile, error) {
	m.listReq = req

	return m.list, nil
}

func (m *mockSearchVirtualFileService) Count(ctx appContext.Context, req *virtualfileSvi.ListRequest) (int64, error) {
	m.countReq = req

	return m.count, nil
}

func (m *mockSearchVirtualFileService) CalFullPath(ctx appContext.Context, fid int64) (string, error) {
	return "/visible", nil
}

type mockSearchMountPointService struct {
	mountpointSvi.Service
	userID        int64
	isAdmin       bool
	groupFileIDs  []int64
	accessibleIDs []int64
}

func (m *mockSearchMountPointService) GetAccessibleMountPointIDs(
	ctx appContext.Context,
	userID int64,
	isAdmin bool,
	groupFileIDs []int64,
) ([]int64, error) {
	m.userID = userID
	m.isAdmin = isAdmin
	m.groupFileIDs = groupFileIDs

	return m.accessibleIDs, nil
}

func TestSearchRestrictsResultsToAccessibleMountPoints(t *testing.T) {
	gin.SetMode(gin.TestMode)

	virtualFileService := &mockSearchVirtualFileService{
		list: []*models.VirtualFile{
			{ID: 11, TopId: 200, ParentId: 50, Name: "movie.mp4"},
		},
		count: 1,
	}
	mountPointService := &mockSearchMountPointService{accessibleIDs: []int64{200}}

	router := newSearchTestRouter(virtualFileService, mountPointService, 100, false, 0)

	req := httptest.NewRequestWithContext(stdctx.Background(), http.MethodGet, "/search?keyword=movie&pageSize=10&currentPage=1&global=true", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected ok, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	if mountPointService.userID != 100 || mountPointService.isAdmin {
		t.Fatalf("expected accessible lookup for non-admin user 100, got userID=%d isAdmin=%v", mountPointService.userID, mountPointService.isAdmin)
	}

	if virtualFileService.listReq == nil {
		t.Fatal("expected virtual file list to be called")
	}

	if !reflect.DeepEqual(virtualFileService.listReq.TopIdList, []int64{200}) {
		t.Fatalf("expected search top id filter [200], got %v", virtualFileService.listReq.TopIdList)
	}

	if virtualFileService.countReq == nil {
		t.Fatal("expected virtual file count to be called")
	}

	if !reflect.DeepEqual(virtualFileService.countReq.TopIdList, []int64{200}) {
		t.Fatalf("expected count top id filter [200], got %v", virtualFileService.countReq.TopIdList)
	}

	var response struct {
		Code int            `json:"code"`
		Data searchResponse `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}

	if response.Data.Total != 1 || len(response.Data.Data) != 1 {
		t.Fatalf("expected one filtered result, got total=%d len=%d", response.Data.Total, len(response.Data.Data))
	}

	if !strings.HasSuffix(response.Data.Data[0].FullPath, "movie.mp4") {
		t.Fatalf("expected full path to include file name, got %q", response.Data.Data[0].FullPath)
	}
}

func TestSearchReturnsEmptyWhenUserHasNoAccessibleMountPoints(t *testing.T) {
	gin.SetMode(gin.TestMode)

	virtualFileService := &mockSearchVirtualFileService{}
	mountPointService := &mockSearchMountPointService{accessibleIDs: []int64{}}

	router := newSearchTestRouter(virtualFileService, mountPointService, 100, false, 0)

	req := httptest.NewRequestWithContext(stdctx.Background(), http.MethodGet, "/search?keyword=movie&pageSize=10&currentPage=1&global=true", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected ok, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	if virtualFileService.listReq != nil {
		t.Fatal("did not expect virtual file list to be called without accessible mount points")
	}

	var response struct {
		Code int            `json:"code"`
		Data searchResponse `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}

	if response.Data.Total != 0 || len(response.Data.Data) != 0 {
		t.Fatalf("expected empty search response, got total=%d len=%d", response.Data.Total, len(response.Data.Data))
	}
}

func TestSearchAllowsRootParentID(t *testing.T) {
	gin.SetMode(gin.TestMode)

	virtualFileService := &mockSearchVirtualFileService{
		list: []*models.VirtualFile{
			{ID: 12, TopId: 200, ParentId: 0, Name: "root-file.mp4"},
		},
		count: 1,
	}
	mountPointService := &mockSearchMountPointService{accessibleIDs: []int64{200}}

	router := newSearchTestRouter(virtualFileService, mountPointService, 100, false, 0)

	req := httptest.NewRequestWithContext(stdctx.Background(), http.MethodGet, "/search?keyword=root&pageSize=10&currentPage=1&pid=0", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected ok, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	if virtualFileService.listReq == nil {
		t.Fatal("expected virtual file list to be called")
	}

	if virtualFileService.listReq.ParentId == nil || *virtualFileService.listReq.ParentId != 0 {
		t.Fatalf("expected root parent id 0, got %#v", virtualFileService.listReq.ParentId)
	}

	if virtualFileService.countReq == nil {
		t.Fatal("expected virtual file count to be called")
	}

	if virtualFileService.countReq.ParentId == nil || *virtualFileService.countReq.ParentId != 0 {
		t.Fatalf("expected count root parent id 0, got %#v", virtualFileService.countReq.ParentId)
	}
}

func newSearchTestRouter(
	virtualFileService virtualfileSvi.Service,
	mountPointService mountpointSvi.Service,
	userID int64,
	isAdmin bool,
	userGroupID int64,
) *gin.Engine {
	router := gin.New()
	router.Use(func(ctx *gin.Context) {
		ctx.Set(consts.CtxKeyUserId, userID)
		ctx.Set(consts.CtxKeyIsAdmin, isAdmin)
		ctx.Set(consts.CtxKeyUserGroupId, userGroupID)
	})

	wrapper := httpcontext.NewHandlerFuncWrapper(zap.NewNop())
	router.GET("/search", wrapper.Wrap(NewHandler(
		virtualFileService,
		nil,
		nil,
		nil,
		mountPointService,
		nil,
		nil,
		nil,
	).Search()))

	return router
}
