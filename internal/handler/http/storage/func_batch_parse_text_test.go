package storage

import (
	stdctx "context"
	"encoding/json"
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
	cloudtokenSvi "github.com/xxcheng123/cloudpan189-share/internal/services/cloudtoken"
	mountpointSvi "github.com/xxcheng123/cloudpan189-share/internal/services/mountpoint"
	"github.com/xxcheng123/cloudpan189-share/internal/types/topic"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type mockBatchParseCloudTokenService struct {
	cloudtokenSvi.Service
	queries []int64
}

func (m *mockBatchParseCloudTokenService) QueryAccessible(ctx appContext.Context, id, userID int64, isAdmin bool) (*models.CloudToken, error) {
	m.queries = append(m.queries, id)

	return &models.CloudToken{ID: id}, nil
}

type mockBatchParseMountPointService struct {
	mountpointSvi.Service
	called      bool
	parseErr    error
	lastContent string
	lastRequest *topic.BatchParseTextRequest
}

func (m *mockBatchParseMountPointService) BatchParseText(ctx appContext.Context, req *topic.BatchParseTextRequest) ([]*topic.BatchParseItem, error) {
	m.called = true
	m.lastContent = req.Content
	copiedReq := *req

	m.lastRequest = &copiedReq
	if m.parseErr != nil {
		return nil, m.parseErr
	}

	return []*topic.BatchParseItem{{Name: "folder", OsType: models.OsTypePersonFolder, FileId: "1"}}, nil
}

func TestBatchParseTextAllowsMissingCloudTokenBeforeServiceCall(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cloudTokenService := &mockBatchParseCloudTokenService{}
	mountPointService := &mockBatchParseMountPointService{}
	router := newBatchParseTextTestRouter(cloudTokenService, mountPointService)

	req := httptest.NewRequestWithContext(
		stdctx.Background(),
		http.MethodPost,
		"/batch_parse_text",
		strings.NewReader(`{"content":"订阅号：up-user"}`),
	)
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected ok, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	if len(cloudTokenService.queries) != 0 {
		t.Fatalf("expected cloud token lookup to be skipped, got %v", cloudTokenService.queries)
	}

	if !mountPointService.called {
		t.Fatal("expected batch parse service to be called")
	}

	if mountPointService.lastRequest == nil || mountPointService.lastRequest.CloudToken != 0 {
		t.Fatalf("expected missing cloud token to be passed as zero, got %+v", mountPointService.lastRequest)
	}
}

func TestBatchParseTextRejectsBlankContentBeforeServiceCalls(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cloudTokenService := &mockBatchParseCloudTokenService{}
	mountPointService := &mockBatchParseMountPointService{}
	router := newBatchParseTextTestRouter(cloudTokenService, mountPointService)

	req := httptest.NewRequestWithContext(
		stdctx.Background(),
		http.MethodPost,
		"/batch_parse_text",
		strings.NewReader(`{"content":" \n\t ","cloudToken":9}`),
	)
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected bad request, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	var response httpcontext.Response
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}

	if response.Code != 99998 {
		t.Fatalf("expected invalid params code 99998, got %d", response.Code)
	}

	if len(cloudTokenService.queries) != 0 {
		t.Fatalf("expected cloud token lookup to be skipped, got %v", cloudTokenService.queries)
	}

	if mountPointService.called {
		t.Fatal("expected batch parse service to be skipped")
	}
}

func TestBatchParseTextReturnsMountPointErrorWhenServiceTypedNil(t *testing.T) {
	gin.SetMode(gin.TestMode)

	var mountPointService *mockBatchParseMountPointService

	router := newBatchParseTextTestRouter(nil, mountPointService)

	req := httptest.NewRequestWithContext(
		stdctx.Background(),
		http.MethodPost,
		"/batch_parse_text",
		strings.NewReader(`{"content":"订阅号：up-user"}`),
	)
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected bad request, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	var response httpcontext.Response
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}

	if response.Code != busCodeStorageQueryMountPointError.GetCode() {
		t.Fatalf("expected business code %d, got %d", busCodeStorageQueryMountPointError.GetCode(), response.Code)
	}
}

func TestBatchParseTextUsesMountPointServiceTokenValidationOnly(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cloudTokenService := &mockBatchParseCloudTokenService{}
	mountPointService := &mockBatchParseMountPointService{}
	router := newBatchParseTextTestRouter(cloudTokenService, mountPointService)

	req := httptest.NewRequestWithContext(
		stdctx.Background(),
		http.MethodPost,
		"/batch_parse_text",
		strings.NewReader(`{"content":"123","cloudToken":9}`),
	)
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected ok, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	if len(cloudTokenService.queries) != 0 {
		t.Fatalf("expected handler token lookup to be skipped, got %v", cloudTokenService.queries)
	}

	if !mountPointService.called {
		t.Fatal("expected batch parse service to be called")
	}
}

func TestBatchParseTextTrimsContentBeforeServiceCall(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cloudTokenService := &mockBatchParseCloudTokenService{}
	mountPointService := &mockBatchParseMountPointService{}
	router := newBatchParseTextTestRouter(cloudTokenService, mountPointService)

	req := httptest.NewRequestWithContext(
		stdctx.Background(),
		http.MethodPost,
		"/batch_parse_text",
		strings.NewReader(`{"content":" \n123\n ","cloudToken":9}`),
	)
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected ok, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	if !mountPointService.called {
		t.Fatal("expected batch parse service to be called")
	}

	if mountPointService.lastContent != "123" {
		t.Fatalf("expected trimmed content %q, got %q", "123", mountPointService.lastContent)
	}
}

func TestBatchParseTextPassesAuthenticatedUserContext(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cloudTokenService := &mockBatchParseCloudTokenService{}
	mountPointService := &mockBatchParseMountPointService{}
	router := newBatchParseTextTestRouterWithIdentity(cloudTokenService, mountPointService, 321, true)

	req := httptest.NewRequestWithContext(
		stdctx.Background(),
		http.MethodPost,
		"/batch_parse_text",
		strings.NewReader(`{"content":" 订阅号：up-user ","cloudToken":9}`),
	)
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected ok, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	if mountPointService.lastRequest == nil {
		t.Fatal("expected batch parse request to be captured")
	}

	if mountPointService.lastRequest.UserID != 321 || !mountPointService.lastRequest.IsAdmin {
		t.Fatalf("expected authenticated user context 321/admin, got user=%d isAdmin=%v", mountPointService.lastRequest.UserID, mountPointService.lastRequest.IsAdmin)
	}

	if mountPointService.lastRequest.CloudToken != 9 {
		t.Fatalf("expected cloud token 9, got %d", mountPointService.lastRequest.CloudToken)
	}

	if mountPointService.lastRequest.Content != "订阅号：up-user" {
		t.Fatalf("expected trimmed content, got %q", mountPointService.lastRequest.Content)
	}
}

func TestBatchParseTextMapsServiceTokenNotFound(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cloudTokenService := &mockBatchParseCloudTokenService{}
	mountPointService := &mockBatchParseMountPointService{parseErr: gorm.ErrRecordNotFound}
	router := newBatchParseTextTestRouter(cloudTokenService, mountPointService)

	req := httptest.NewRequestWithContext(
		stdctx.Background(),
		http.MethodPost,
		"/batch_parse_text",
		strings.NewReader(`{"content":"123","cloudToken":9}`),
	)
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("expected not found, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	var response httpcontext.Response
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}

	if response.Code != busCodeStorageCloudTokenNotExist.GetCode() {
		t.Fatalf("expected business code %d, got %d", busCodeStorageCloudTokenNotExist.GetCode(), response.Code)
	}

	if len(cloudTokenService.queries) != 0 {
		t.Fatalf("expected handler token lookup to be skipped, got %v", cloudTokenService.queries)
	}
}

func TestBatchParseTextMapsServiceTokenQueryError(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cloudTokenService := &mockBatchParseCloudTokenService{}
	mountPointService := &mockBatchParseMountPointService{parseErr: errors.New("query failed")}
	router := newBatchParseTextTestRouter(cloudTokenService, mountPointService)

	req := httptest.NewRequestWithContext(
		stdctx.Background(),
		http.MethodPost,
		"/batch_parse_text",
		strings.NewReader(`{"content":"123","cloudToken":9}`),
	)
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected bad request, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	var response httpcontext.Response
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}

	if response.Code != busCodeStorageQueryCloudTokenError.GetCode() {
		t.Fatalf("expected business code %d, got %d", busCodeStorageQueryCloudTokenError.GetCode(), response.Code)
	}

	if len(cloudTokenService.queries) != 0 {
		t.Fatalf("expected handler token lookup to be skipped, got %v", cloudTokenService.queries)
	}
}

func newBatchParseTextTestRouter(
	cloudTokenService cloudtokenSvi.Service,
	mountPointService mountpointSvi.Service,
) *gin.Engine {
	return newBatchParseTextTestRouterWithIdentity(cloudTokenService, mountPointService, 100, false)
}

func newBatchParseTextTestRouterWithIdentity(
	cloudTokenService cloudtokenSvi.Service,
	mountPointService mountpointSvi.Service,
	userID int64,
	isAdmin bool,
) *gin.Engine {
	router := gin.New()
	router.Use(func(ctx *gin.Context) {
		ctx.Set(consts.CtxKeyUserId, userID)
		ctx.Set(consts.CtxKeyIsAdmin, isAdmin)
	})

	wrapper := httpcontext.NewHandlerFuncWrapper(zap.NewNop())
	router.POST("/batch_parse_text", wrapper.Wrap(NewHandler(
		nil,
		nil,
		nil,
		cloudTokenService,
		mountPointService,
		nil,
		nil,
		nil,
		nil,
		nil,
	).BatchParseFromText()))

	return router
}
