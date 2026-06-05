package autoingest

import (
	stdctx "context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sort"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/xxcheng123/cloudpan189-share/internal/consts"
	appContext "github.com/xxcheng123/cloudpan189-share/internal/framework/context"
	"github.com/xxcheng123/cloudpan189-share/internal/framework/httpcontext"
	"github.com/xxcheng123/cloudpan189-share/internal/repository/models"
	autoingestlogSvi "github.com/xxcheng123/cloudpan189-share/internal/services/autoingestlog"
	autoingestplanSvi "github.com/xxcheng123/cloudpan189-share/internal/services/autoingestplan"
	"go.uber.org/zap"
)

type mockLogListPlanService struct {
	autoingestplanSvi.Service
	plans []*models.AutoIngestPlan
	count int64
	reqs  []*autoingestplanSvi.ListRequest
}

func (m *mockLogListPlanService) List(ctx appContext.Context, req *autoingestplanSvi.ListRequest) ([]*models.AutoIngestPlan, error) {
	copiedReq := *req
	m.reqs = append(m.reqs, &copiedReq)

	return m.plans, nil
}

func (m *mockLogListPlanService) Count(ctx appContext.Context, req *autoingestplanSvi.ListRequest) (int64, error) {
	return m.count, nil
}

type mockLogListLogService struct {
	autoingestlogSvi.Service
	listReqs  []*autoingestlogSvi.ListRequest
	countReqs []*autoingestlogSvi.ListRequest
	logs      []*models.AutoIngestLog
}

func (m *mockLogListLogService) List(ctx appContext.Context, req *autoingestlogSvi.ListRequest) ([]*models.AutoIngestLog, error) {
	copiedReq := *req
	copiedReq.PlanIdList = append([]int64(nil), req.PlanIdList...)
	m.listReqs = append(m.listReqs, &copiedReq)

	return m.logs, nil
}

func (m *mockLogListLogService) Count(ctx appContext.Context, req *autoingestlogSvi.ListRequest) (int64, error) {
	copiedReq := *req
	copiedReq.PlanIdList = append([]int64(nil), req.PlanIdList...)
	m.countReqs = append(m.countReqs, &copiedReq)

	return int64(len(m.logs)), nil
}

func newLogListRouter(planService autoingestplanSvi.Service, logService autoingestlogSvi.Service) *gin.Engine {
	router := gin.New()
	router.Use(func(ctx *gin.Context) {
		ctx.Set(consts.CtxKeyUserId, int64(10))
		ctx.Set(consts.CtxKeyIsAdmin, false)
	})

	wrapper := httpcontext.NewHandlerFuncWrapper(zap.NewNop())
	router.GET("/logs", wrapper.Wrap(NewHandler(nil, planService, logService, nil).LogList()))

	return router
}

func newPlanListRouter(planService autoingestplanSvi.Service) *gin.Engine {
	router := gin.New()
	router.Use(func(ctx *gin.Context) {
		ctx.Set(consts.CtxKeyUserId, int64(10))
		ctx.Set(consts.CtxKeyIsAdmin, false)
	})

	wrapper := httpcontext.NewHandlerFuncWrapper(zap.NewNop())
	router.GET("/plans", wrapper.Wrap(NewHandler(nil, planService, nil, nil).PlanList()))

	return router
}

func TestPlanListSkipsNilPlanRows(t *testing.T) {
	gin.SetMode(gin.TestMode)

	planService := &mockLogListPlanService{
		plans: []*models.AutoIngestPlan{
			nil,
			{ID: 11, UserID: 10, Name: "owned"},
		},
		count: 2,
	}
	router := newPlanListRouter(planService)

	req := httptest.NewRequestWithContext(stdctx.Background(), http.MethodGet, "/plans?pageSize=10&currentPage=1", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected ok, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	var response struct {
		Data planListResponse `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}

	if response.Data.Total != 2 {
		t.Fatalf("expected service count to be preserved, got %d", response.Data.Total)
	}

	if len(response.Data.Data) != 1 {
		t.Fatalf("expected one non-nil plan, got %d", len(response.Data.Data))
	}

	if response.Data.Data[0].ID != 11 || response.Data.Data[0].Name != "owned" {
		t.Fatalf("unexpected plan row: %#v", response.Data.Data[0])
	}
}

func TestLogListRestrictsNonAdminLogsToVisiblePlans(t *testing.T) {
	gin.SetMode(gin.TestMode)

	planService := &mockLogListPlanService{
		plans: []*models.AutoIngestPlan{
			{ID: 11, UserID: 10, Name: "owned-a"},
			{ID: 22, UserID: 10, Name: "owned-b"},
		},
	}
	logService := &mockLogListLogService{
		logs: []*models.AutoIngestLog{{ID: 1, PlanId: 11, Content: "visible"}},
	}
	router := newLogListRouter(planService, logService)

	req := httptest.NewRequestWithContext(stdctx.Background(), http.MethodGet, "/logs", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected ok, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	if len(logService.listReqs) != 1 {
		t.Fatalf("expected one list call, got %d", len(logService.listReqs))
	}

	got := append([]int64(nil), logService.listReqs[0].PlanIdList...)
	sort.Slice(got, func(i, j int) bool { return got[i] < got[j] })

	if want := []int64{11, 22}; !int64SlicesEqual(got, want) {
		t.Fatalf("expected log list filtered by plans %v, got %v", want, got)
	}

	if len(logService.countReqs) != 1 {
		t.Fatalf("expected one count call, got %d", len(logService.countReqs))
	}
}

func TestLogListReturnsEmptyForInvisiblePlan(t *testing.T) {
	gin.SetMode(gin.TestMode)

	planService := &mockLogListPlanService{
		plans: []*models.AutoIngestPlan{{ID: 11, UserID: 10, Name: "owned"}},
	}
	logService := &mockLogListLogService{}
	router := newLogListRouter(planService, logService)

	req := httptest.NewRequestWithContext(stdctx.Background(), http.MethodGet, "/logs?planId=99", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected ok, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	if len(logService.listReqs) != 0 || len(logService.countReqs) != 0 {
		t.Fatalf("expected invisible plan not to query logs, got list=%d count=%d", len(logService.listReqs), len(logService.countReqs))
	}

	var response struct {
		Data logListResponse `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}

	if response.Data.Total != 0 || len(response.Data.Data) != 0 {
		t.Fatalf("expected empty response, got %+v", response.Data)
	}
}

func TestLogListSkipsNilPlansAndLogs(t *testing.T) {
	gin.SetMode(gin.TestMode)

	planService := &mockLogListPlanService{
		plans: []*models.AutoIngestPlan{
			nil,
			{ID: 11, UserID: 10, Name: "owned"},
		},
	}
	logService := &mockLogListLogService{
		logs: []*models.AutoIngestLog{
			nil,
			{ID: 1, PlanId: 11, Content: "visible"},
			{ID: 2, PlanId: 99, Content: "orphan"},
		},
	}
	router := newLogListRouter(planService, logService)

	req := httptest.NewRequestWithContext(stdctx.Background(), http.MethodGet, "/logs", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected ok, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	if len(logService.listReqs) != 1 {
		t.Fatalf("expected one log list query, got %d", len(logService.listReqs))
	}

	if want := []int64{11}; !int64SlicesEqual(logService.listReqs[0].PlanIdList, want) {
		t.Fatalf("expected visible plan IDs %v, got %v", want, logService.listReqs[0].PlanIdList)
	}

	var response struct {
		Data logListResponse `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}

	if len(response.Data.Data) != 2 {
		t.Fatalf("expected nil log row to be skipped, got %+v", response.Data.Data)
	}

	if response.Data.Data[0].PlanName != "owned" {
		t.Fatalf("expected visible plan name, got %q", response.Data.Data[0].PlanName)
	}

	if response.Data.Data[1].PlanName != "计划不存在" {
		t.Fatalf("expected missing plan name fallback, got %q", response.Data.Data[1].PlanName)
	}
}
