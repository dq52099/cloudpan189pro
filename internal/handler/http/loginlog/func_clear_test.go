package loginlog

import (
	stdctx "context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	appContext "github.com/xxcheng123/cloudpan189-share/internal/framework/context"
	"github.com/xxcheng123/cloudpan189-share/internal/framework/httpcontext"
	"github.com/xxcheng123/cloudpan189-share/internal/repository/models"
	loginlogSvi "github.com/xxcheng123/cloudpan189-share/internal/services/loginlog"
	"go.uber.org/zap"
)

type mockLoginLogService struct {
	clearAllCalled bool
	clearBefore    time.Time
	logs           []*models.LoginLog
	count          int64
}

func (m *mockLoginLogService) Create(ctx appContext.Context, log *models.LoginLog) (int64, error) {
	return 0, nil
}

func (m *mockLoginLogService) RecordLogin(ctx appContext.Context, in *loginlogSvi.RecordLoginInput) (int64, error) {
	return 0, nil
}

func (m *mockLoginLogService) RecordRefreshToken(ctx appContext.Context, in *loginlogSvi.RecordRefreshInput) (int64, error) {
	return 0, nil
}

func (m *mockLoginLogService) List(ctx appContext.Context, req *loginlogSvi.ListRequest) ([]*models.LoginLog, error) {
	return m.logs, nil
}

func (m *mockLoginLogService) Count(ctx appContext.Context, req *loginlogSvi.ListRequest) (int64, error) {
	return m.count, nil
}

func (m *mockLoginLogService) ClearAll(ctx appContext.Context) (int64, error) {
	m.clearAllCalled = true

	return 12, nil
}

func (m *mockLoginLogService) ClearBefore(ctx appContext.Context, before time.Time) (int64, error) {
	m.clearBefore = before

	return 3, nil
}

var _ loginlogSvi.Service = (*mockLoginLogService)(nil)

func newLoginLogTestRouter(svc loginlogSvi.Service) *gin.Engine {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	wrapper := httpcontext.NewHandlerFuncWrapper(zap.NewNop())
	handler := NewHandler(svc)

	router.GET("/list", wrapper.Wrap(handler.List()))
	router.POST("/clear", wrapper.Wrap(handler.Clear()))

	return router
}

func performClearRequest(t *testing.T, svc loginlogSvi.Service, body string) *httptest.ResponseRecorder {
	t.Helper()

	router := newLoginLogTestRouter(svc)

	req := httptest.NewRequestWithContext(stdctx.Background(), http.MethodPost, "/clear", strings.NewReader(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	return recorder
}

func assertLoginLogHTTPError(t *testing.T, recorder *httptest.ResponseRecorder, expectedStatus int, expectedErr httpcontext.BusinessError) {
	t.Helper()

	if recorder.Code != expectedStatus {
		t.Fatalf("expected HTTP %d, got %d body=%s", expectedStatus, recorder.Code, recorder.Body.String())
	}

	var response httpcontext.Response
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if response.Code != expectedErr.GetCode() {
		t.Fatalf("expected business code %d, got %d", expectedErr.GetCode(), response.Code)
	}
}

func TestResolveCutoffRejectsNonPositiveDuration(t *testing.T) {
	tests := []string{"0s", "0h", "-1h"}

	for _, tt := range tests {
		t.Run(tt, func(t *testing.T) {
			if _, err := resolveCutoff(tt); err == nil {
				t.Fatal("expected invalid duration error")
			}
		})
	}
}

func TestListReturnsErrorWhenLoginLogServiceMissing(t *testing.T) {
	req := httptest.NewRequestWithContext(stdctx.Background(), http.MethodGet, "/list?currentPage=1&pageSize=10", nil)
	recorder := httptest.NewRecorder()
	newLoginLogTestRouter(nil).ServeHTTP(recorder, req)

	assertLoginLogHTTPError(t, recorder, http.StatusBadRequest, codeListFailed)
}

func TestListReturnsErrorWhenLoginLogServiceTypedNil(t *testing.T) {
	var svc *mockLoginLogService

	req := httptest.NewRequestWithContext(stdctx.Background(), http.MethodGet, "/list?currentPage=1&pageSize=10", nil)
	recorder := httptest.NewRecorder()
	newLoginLogTestRouter(svc).ServeHTTP(recorder, req)

	assertLoginLogHTTPError(t, recorder, http.StatusBadRequest, codeListFailed)
}

func TestClearReturnsErrorWhenLoginLogServiceMissing(t *testing.T) {
	recorder := performClearRequest(t, nil, "")

	assertLoginLogHTTPError(t, recorder, http.StatusBadRequest, codeClearFailed)
}

func TestClearReturnsErrorWhenLoginLogServiceTypedNil(t *testing.T) {
	var svc *mockLoginLogService

	recorder := performClearRequest(t, svc, "")

	assertLoginLogHTTPError(t, recorder, http.StatusBadRequest, codeClearFailed)
}

func TestClearRejectsMalformedJSON(t *testing.T) {
	svc := &mockLoginLogService{}

	recorder := performClearRequest(t, svc, "{")

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected bad request, got %d", recorder.Code)
	}

	if svc.clearAllCalled {
		t.Fatal("expected malformed JSON not to clear all login logs")
	}

	if !svc.clearBefore.IsZero() {
		t.Fatalf("expected malformed JSON not to clear by time, got %s", svc.clearBefore)
	}
}

func TestClearAllowsEmptyBodyAsClearAll(t *testing.T) {
	svc := &mockLoginLogService{}

	recorder := performClearRequest(t, svc, "")

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected success, got %d", recorder.Code)
	}

	if !svc.clearAllCalled {
		t.Fatal("expected empty body to clear all login logs")
	}
}

func TestListSkipsNilLoginLogRows(t *testing.T) {
	gin.SetMode(gin.TestMode)

	svc := &mockLoginLogService{
		logs: []*models.LoginLog{
			nil,
			{ID: 12, Username: "admin"},
		},
	}

	router := gin.New()
	wrapper := httpcontext.NewHandlerFuncWrapper(zap.NewNop())
	router.GET("/list", wrapper.Wrap(NewHandler(svc).List()))

	req := httptest.NewRequestWithContext(stdctx.Background(), http.MethodGet, "/list?noPaginate=true", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected ok, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	var response struct {
		Data struct {
			Total int64 `json:"total"`
			Data  []struct {
				ID       int64  `json:"id"`
				Username string `json:"username"`
			} `json:"data"`
		} `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if response.Data.Total != 1 {
		t.Fatalf("expected one non-nil row in total, got %d", response.Data.Total)
	}

	if len(response.Data.Data) != 1 {
		t.Fatalf("expected one non-nil row, got %d", len(response.Data.Data))
	}

	if response.Data.Data[0].ID != 12 || response.Data.Data[0].Username != "admin" {
		t.Fatalf("unexpected login log row: %#v", response.Data.Data[0])
	}
}

func TestResolveCutoffAcceptsPositiveDuration(t *testing.T) {
	before := time.Now()

	cutoff, err := resolveCutoff("24h")
	if err != nil {
		t.Fatalf("resolve cutoff: %v", err)
	}

	after := time.Now()
	minCutoff := before.Add(-24 * time.Hour)
	maxCutoff := after.Add(-24 * time.Hour)

	if cutoff.Before(minCutoff) || cutoff.After(maxCutoff) {
		t.Fatalf("expected cutoff between %s and %s, got %s", minCutoff, maxCutoff, cutoff)
	}
}

func TestResolveCutoffAcceptsKnownShorthand(t *testing.T) {
	before := time.Now()

	cutoff, err := resolveCutoff("30d")
	if err != nil {
		t.Fatalf("resolve cutoff: %v", err)
	}

	after := time.Now()
	minCutoff := before.AddDate(0, 0, -30)
	maxCutoff := after.AddDate(0, 0, -30)

	if cutoff.Before(minCutoff) || cutoff.After(maxCutoff) {
		t.Fatalf("expected cutoff between %s and %s, got %s", minCutoff, maxCutoff, cutoff)
	}
}
