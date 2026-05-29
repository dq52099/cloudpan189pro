package autoingest

import (
	stdctx "context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/xxcheng123/cloudpan189-share/internal/consts"
	appContext "github.com/xxcheng123/cloudpan189-share/internal/framework/context"
	"github.com/xxcheng123/cloudpan189-share/internal/framework/httpcontext"
	"github.com/xxcheng123/cloudpan189-share/internal/repository/models"
	autoingestlogSvi "github.com/xxcheng123/cloudpan189-share/internal/services/autoingestlog"
	autoingestplanSvi "github.com/xxcheng123/cloudpan189-share/internal/services/autoingestplan"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type mockDeleteErrorLogsService struct {
	autoingestlogSvi.Service
	allCalled    bool
	planIDCalled *int64
	planIDs      []int64
}

func (m *mockDeleteErrorLogsService) DeleteAllErrorLogs(ctx appContext.Context) (int64, error) {
	m.allCalled = true

	return 7, nil
}

func (m *mockDeleteErrorLogsService) DeleteErrorLogsByPlanId(ctx appContext.Context, planID int64) (int64, error) {
	m.planIDCalled = &planID

	return 3, nil
}

func (m *mockDeleteErrorLogsService) DeleteErrorLogsByPlanIds(ctx appContext.Context, planIDs []int64) (int64, error) {
	m.planIDs = append([]int64(nil), planIDs...)

	return int64(len(planIDs)), nil
}

type mockDeleteErrorLogsPlanService struct {
	autoingestplanSvi.Service
	plans    map[int64]*models.AutoIngestPlan
	listReqs []*autoingestplanSvi.ListRequest
}

func (m *mockDeleteErrorLogsPlanService) Query(ctx appContext.Context, id int64) (*models.AutoIngestPlan, error) {
	if plan, ok := m.plans[id]; ok {
		return plan, nil
	}

	return nil, gorm.ErrRecordNotFound
}

func (m *mockDeleteErrorLogsPlanService) List(ctx appContext.Context, req *autoingestplanSvi.ListRequest) ([]*models.AutoIngestPlan, error) {
	copiedReq := *req
	m.listReqs = append(m.listReqs, &copiedReq)

	if !req.IsAdmin && req.UserID <= 0 {
		return nil, errors.New("missing user")
	}

	plans := make([]*models.AutoIngestPlan, 0, len(m.plans))
	for _, plan := range m.plans {
		if req.IsAdmin || plan.UserID == req.UserID {
			plans = append(plans, plan)
		}
	}

	return plans, nil
}

func newDeleteErrorLogsRouter(
	planService autoingestplanSvi.Service,
	logService autoingestlogSvi.Service,
	isAdmin bool,
) *gin.Engine {
	router := gin.New()
	router.Use(func(ctx *gin.Context) {
		ctx.Set(consts.CtxKeyUserId, int64(100))
		ctx.Set(consts.CtxKeyIsAdmin, isAdmin)
	})

	wrapper := httpcontext.NewHandlerFuncWrapper(zap.NewNop())
	router.POST("/delete_error", wrapper.Wrap(NewHandler(nil, planService, logService, nil).DeleteErrorLogs()))

	return router
}

func TestDeleteErrorLogsAdminAllowsEmptyBodyAsDeleteAll(t *testing.T) {
	gin.SetMode(gin.TestMode)

	logService := &mockDeleteErrorLogsService{}
	planService := &mockDeleteErrorLogsPlanService{plans: map[int64]*models.AutoIngestPlan{}}
	router := newDeleteErrorLogsRouter(planService, logService, true)

	req := httptest.NewRequestWithContext(stdctx.Background(), http.MethodPost, "/delete_error", nil)
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected ok, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	if !logService.allCalled {
		t.Fatal("expected all error logs to be deleted")
	}

	if logService.planIDCalled != nil {
		t.Fatalf("did not expect plan-scoped delete, got %d", *logService.planIDCalled)
	}
}

func TestDeleteErrorLogsEmptyBodyDeletesCurrentUsersErrors(t *testing.T) {
	gin.SetMode(gin.TestMode)

	logService := &mockDeleteErrorLogsService{}
	planService := &mockDeleteErrorLogsPlanService{
		plans: map[int64]*models.AutoIngestPlan{
			11: {ID: 11, UserID: 100},
			22: {ID: 22, UserID: 200},
			33: {ID: 33, UserID: 100},
		},
	}
	router := newDeleteErrorLogsRouter(planService, logService, false)

	req := httptest.NewRequestWithContext(stdctx.Background(), http.MethodPost, "/delete_error", nil)
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected ok, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	if logService.allCalled {
		t.Fatal("did not expect all error logs to be deleted")
	}

	gotPlanIDs := append([]int64(nil), logService.planIDs...)
	sort.Slice(gotPlanIDs, func(i, j int) bool { return gotPlanIDs[i] < gotPlanIDs[j] })

	if want := []int64{11, 33}; !int64SlicesEqual(gotPlanIDs, want) {
		t.Fatalf("expected current user's plan IDs %v, got %v", want, gotPlanIDs)
	}

	if len(planService.listReqs) != 1 {
		t.Fatalf("expected one plan list request, got %d", len(planService.listReqs))
	}

	listReq := planService.listReqs[0]
	if listReq.UserID != 100 || listReq.IsAdmin || !listReq.NoPaginate {
		t.Fatalf("unexpected plan list request: %+v", listReq)
	}
}

func TestDeleteErrorLogsRejectsMalformedJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)

	logService := &mockDeleteErrorLogsService{}
	planService := &mockDeleteErrorLogsPlanService{plans: map[int64]*models.AutoIngestPlan{}}
	router := newDeleteErrorLogsRouter(planService, logService, false)

	req := httptest.NewRequestWithContext(stdctx.Background(), http.MethodPost, "/delete_error", strings.NewReader("{"))
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected bad request, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	if logService.allCalled || logService.planIDCalled != nil {
		t.Fatal("expected malformed request to skip deletion")
	}
}

func TestDeleteErrorLogsRejectsInvalidPlanIDWithoutDeleting(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name string
		body string
	}{
		{name: "zero", body: `{"planId":0}`},
		{name: "negative", body: `{"planId":-1}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logService := &mockDeleteErrorLogsService{}
			planService := &mockDeleteErrorLogsPlanService{plans: map[int64]*models.AutoIngestPlan{}}
			router := newDeleteErrorLogsRouter(planService, logService, false)

			req := httptest.NewRequestWithContext(stdctx.Background(), http.MethodPost, "/delete_error", strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/json")

			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, req)

			if recorder.Code != http.StatusBadRequest {
				t.Fatalf("expected bad request, got %d body=%s", recorder.Code, recorder.Body.String())
			}

			if logService.allCalled || logService.planIDCalled != nil {
				t.Fatal("expected invalid plan id to skip deletion")
			}
		})
	}
}

func TestDeleteErrorLogsWithPlanIDDeletesPlanErrors(t *testing.T) {
	gin.SetMode(gin.TestMode)

	logService := &mockDeleteErrorLogsService{}
	planService := &mockDeleteErrorLogsPlanService{
		plans: map[int64]*models.AutoIngestPlan{
			42: {ID: 42, UserID: 100},
		},
	}
	router := newDeleteErrorLogsRouter(planService, logService, false)

	req := httptest.NewRequestWithContext(stdctx.Background(), http.MethodPost, "/delete_error", strings.NewReader(`{"planId":42}`))
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected ok, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	if logService.planIDCalled == nil || *logService.planIDCalled != 42 {
		t.Fatalf("expected plan 42 delete, got %v", logService.planIDCalled)
	}

	if logService.allCalled {
		t.Fatal("did not expect all error logs to be deleted")
	}
}

func TestDeleteErrorLogsRejectsOtherUsersPlan(t *testing.T) {
	gin.SetMode(gin.TestMode)

	logService := &mockDeleteErrorLogsService{}
	planService := &mockDeleteErrorLogsPlanService{
		plans: map[int64]*models.AutoIngestPlan{
			42: {ID: 42, UserID: 200},
		},
	}
	router := newDeleteErrorLogsRouter(planService, logService, false)

	req := httptest.NewRequestWithContext(stdctx.Background(), http.MethodPost, "/delete_error", strings.NewReader(`{"planId":42}`))
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusForbidden {
		t.Fatalf("expected forbidden, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	if logService.allCalled || logService.planIDCalled != nil || len(logService.planIDs) != 0 {
		t.Fatal("expected forbidden request to skip deletion")
	}
}
