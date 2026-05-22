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
	children []*models.VirtualFile
}

func (m *mockOpenVirtualFileService) List(ctx appContext.Context, req *virtualfileSvi.ListRequest) ([]*models.VirtualFile, error) {
	return m.children, nil
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
}
