package cloudtoken

import (
	stdctx "context"
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
)

type mockDeleteCloudTokenService struct {
	cloudtokenSvi.Service
	req *cloudtokenSvi.DeleteRequest
}

func (m *mockDeleteCloudTokenService) Delete(ctx appContext.Context, req *cloudtokenSvi.DeleteRequest) error {
	m.req = req

	return nil
}

type mockDeleteMountPointService struct {
	mountpointSvi.Service
	req *mountpointSvi.ListRequest
}

func (m *mockDeleteMountPointService) Count(ctx appContext.Context, req *mountpointSvi.ListRequest) (int64, error) {
	m.req = req

	return 0, nil
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
