package loginlog

import (
	stdctx "context"
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
	return nil, nil
}

func (m *mockLoginLogService) Count(ctx appContext.Context, req *loginlogSvi.ListRequest) (int64, error) {
	return 0, nil
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

func performClearRequest(t *testing.T, svc *mockLoginLogService, body string) *httptest.ResponseRecorder {
	t.Helper()

	gin.SetMode(gin.TestMode)

	router := gin.New()
	wrapper := httpcontext.NewHandlerFuncWrapper(zap.NewNop())
	router.POST("/clear", wrapper.Wrap(NewHandler(svc).Clear()))

	req := httptest.NewRequestWithContext(stdctx.Background(), http.MethodPost, "/clear", strings.NewReader(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	return recorder
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
