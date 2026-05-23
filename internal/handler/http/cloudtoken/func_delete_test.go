package cloudtoken

import (
	stdctx "context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/xxcheng123/cloudpan189-share/internal/consts"
	appContext "github.com/xxcheng123/cloudpan189-share/internal/framework/context"
	"github.com/xxcheng123/cloudpan189-share/internal/framework/httpcontext"
	cloudtokenSvi "github.com/xxcheng123/cloudpan189-share/internal/services/cloudtoken"
	mountpointSvi "github.com/xxcheng123/cloudpan189-share/internal/services/mountpoint"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type mockDeleteCloudTokenService struct {
	cloudtokenSvi.Service
	req           *cloudtokenSvi.DeleteRequest
	deleteErr     error
	modifyNameErr error
}

func (m *mockDeleteCloudTokenService) Delete(ctx appContext.Context, req *cloudtokenSvi.DeleteRequest) error {
	m.req = req

	return m.deleteErr
}

func (m *mockDeleteCloudTokenService) ModifyName(ctx appContext.Context, req *cloudtokenSvi.ModifyNameRequest) error {
	return m.modifyNameErr
}

type mockDeleteMountPointService struct {
	mountpointSvi.Service
	req *mountpointSvi.ListRequest
}

func (m *mockDeleteMountPointService) Count(ctx appContext.Context, req *mountpointSvi.ListRequest) (int64, error) {
	m.req = req

	return 0, nil
}

func newCloudTokenActionRouter(cloudTokenService cloudtokenSvi.Service, mountPointService mountpointSvi.Service) *gin.Engine {
	router := gin.New()
	router.Use(func(ctx *gin.Context) {
		ctx.Set(consts.CtxKeyUserId, int64(88))
		ctx.Set(consts.CtxKeyIsAdmin, false)
	})

	wrapper := httpcontext.NewHandlerFuncWrapper(zap.NewNop())
	handler := NewHandler(cloudTokenService, mountPointService)
	router.POST("/delete", wrapper.Wrap(handler.Delete()))
	router.POST("/modify_name", wrapper.Wrap(handler.ModifyName()))

	return router
}

func assertCloudTokenNotFoundResponse(t *testing.T, recorder *httptest.ResponseRecorder) {
	t.Helper()

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("expected not found, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	var response httpcontext.Response
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if response.Code != codeTokenNotFound.GetCode() {
		t.Fatalf("expected business code %d, got %d", codeTokenNotFound.GetCode(), response.Code)
	}

	if response.Msg != codeTokenNotFound.GetMessage() {
		t.Fatalf("expected message %q, got %q", codeTokenNotFound.GetMessage(), response.Msg)
	}
}

func TestDeletePassesCurrentUserToMountPointUsageCheck(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cloudTokenService := &mockDeleteCloudTokenService{}
	mountPointService := &mockDeleteMountPointService{}

	router := gin.New()
	router.Use(func(ctx *gin.Context) {
		ctx.Set(consts.CtxKeyUserId, int64(88))
		ctx.Set(consts.CtxKeyIsAdmin, false)
	})

	wrapper := httpcontext.NewHandlerFuncWrapper(zap.NewNop())
	router.POST("/delete", wrapper.Wrap(NewHandler(cloudTokenService, mountPointService).Delete()))

	req := httptest.NewRequestWithContext(stdctx.Background(), http.MethodPost, "/delete", strings.NewReader(`{"id":123}`))
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected ok, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	if mountPointService.req == nil {
		t.Fatal("expected mount point count request")
	}

	if mountPointService.req.TokenId == nil || *mountPointService.req.TokenId != 123 {
		t.Fatalf("expected token id 123, got %+v", mountPointService.req.TokenId)
	}

	if mountPointService.req.UserID != 88 || mountPointService.req.IsAdmin {
		t.Fatalf("expected non-admin user 88 on mount point request, got %+v", mountPointService.req)
	}

	if cloudTokenService.req == nil {
		t.Fatal("expected cloud token delete request")
	}

	if cloudTokenService.req.UserID != 88 || cloudTokenService.req.IsAdmin {
		t.Fatalf("expected non-admin user 88 on delete request, got %+v", cloudTokenService.req)
	}
}

func TestDeleteReturnsNotFoundWhenCloudTokenMissing(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cloudTokenService := &mockDeleteCloudTokenService{deleteErr: gorm.ErrRecordNotFound}
	mountPointService := &mockDeleteMountPointService{}
	router := newCloudTokenActionRouter(cloudTokenService, mountPointService)

	req := httptest.NewRequestWithContext(stdctx.Background(), http.MethodPost, "/delete", strings.NewReader(`{"id":123}`))
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	assertCloudTokenNotFoundResponse(t, recorder)
}

func TestModifyNameReturnsNotFoundWhenCloudTokenMissing(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cloudTokenService := &mockDeleteCloudTokenService{modifyNameErr: gorm.ErrRecordNotFound}
	router := newCloudTokenActionRouter(cloudTokenService, nil)

	req := httptest.NewRequestWithContext(stdctx.Background(), http.MethodPost, "/modify_name", strings.NewReader(`{"id":123,"name":"new-name"}`))
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	assertCloudTokenNotFoundResponse(t, recorder)
}
