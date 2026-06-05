package autoingest

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/xxcheng123/cloudpan189-share/internal/consts"
	"github.com/xxcheng123/cloudpan189-share/internal/framework/httpcontext"
	"github.com/xxcheng123/cloudpan189-share/internal/repository/models"
	"github.com/xxcheng123/cloudpan189-share/internal/types/autoingest"
)

func TestPlanListReturnsBusinessErrorWhenPlanServiceMissing(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := newPlanListRouter(nil)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/plans", nil)
	router.ServeHTTP(recorder, request)

	assertAutoIngestBusinessError(t, recorder, codePlanListFailed)
}

func TestLogListReturnsBusinessErrorWhenLogServiceMissingAfterVisiblePlans(t *testing.T) {
	gin.SetMode(gin.TestMode)

	planService := &mockLogListPlanService{
		plans: []*models.AutoIngestPlan{{ID: 11, UserID: 10, Name: "owned"}},
	}
	router := newLogListRouter(planService, nil)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/logs", nil)
	router.ServeHTTP(recorder, request)

	assertAutoIngestBusinessError(t, recorder, codeLogListFailed)
}

func TestLogListDoesNotRequireLogServiceWhenNoPlanVisible(t *testing.T) {
	gin.SetMode(gin.TestMode)

	planService := &mockLogListPlanService{plans: []*models.AutoIngestPlan{}}
	router := newLogListRouter(planService, nil)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/logs", nil)
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestClearLogsReturnsBusinessErrorWhenLogServiceMissing(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := newClearLogsRouter(nil)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/clear", nil)
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, request)

	assertAutoIngestBusinessError(t, recorder, codeLogDeleteFailed)
}

func TestCreateSubscribePlanReturnsBusinessErrorWhenCloudBridgeMissing(t *testing.T) {
	gin.SetMode(gin.TestMode)

	planService := &mockCreateSubscribePlanService{id: 42}
	router := newCreateSubscribePlanDependencyRouter(NewHandler(
		&mockCreateSubscribeTaskEngine{},
		planService,
		nil,
		nil,
	).CreateSubscribePlan())

	recorder := postCreateSubscribePlan(router, `{"name":"计划","autoIngestInterval":30,"parentPath":"/Movies","upUserId":"up-user"}`)

	assertAutoIngestBusinessError(t, recorder, codeUpUserIdInvalid)

	if planService.plan != nil {
		t.Fatal("expected missing cloud bridge service not to create plan")
	}
}

func TestCreateSubscribePlanReturnsBusinessErrorWhenPlanServiceMissing(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := newCreateSubscribePlanDependencyRouter(NewHandler(
		&mockCreateSubscribeTaskEngine{},
		nil,
		nil,
		&mockCreateSubscribeCloudBridgeService{},
	).CreateSubscribePlan())

	recorder := postCreateSubscribePlan(router, `{"name":"计划","autoIngestInterval":30,"parentPath":"/Movies","upUserId":"up-user"}`)

	assertAutoIngestBusinessError(t, recorder, codeCreatePlanFailed)
}

func TestCreateSubscribePlanKeepsSuccessWhenHistoryTaskEngineMissing(t *testing.T) {
	gin.SetMode(gin.TestMode)

	planService := &mockCreateSubscribePlanService{id: 42}
	router := newCreateSubscribePlanDependencyRouter(NewHandler(
		nil,
		planService,
		nil,
		&mockCreateSubscribeCloudBridgeService{},
	).CreateSubscribePlan())

	recorder := postCreateSubscribePlan(router, `{"name":"计划","autoIngestInterval":30,"parentPath":"/Movies","upUserId":"up-user","oneClickAddHistory":true}`)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	var response struct {
		Code int                         `json:"code"`
		Data createSubscribePlanResponse `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if response.Code != http.StatusOK {
		t.Fatalf("expected business code 200, got %d", response.Code)
	}

	if response.Data.ID != 42 || response.Data.HistoryQueued || response.Data.HistoryError != "任务引擎未初始化" {
		t.Fatalf("unexpected response: %+v", response.Data)
	}

	if planService.plan == nil {
		t.Fatal("expected plan to be created")
	}
}

func TestRefreshReturnsBusinessErrorWhenTaskEngineMissing(t *testing.T) {
	gin.SetMode(gin.TestMode)

	planService := &mockPlanAccessService{
		plans: map[int64]*models.AutoIngestPlan{
			11: {ID: 11, UserID: 100, SourceType: autoingest.SourceTypeSubscribe},
		},
	}
	router := newPlanAccessRouter(NewHandler(nil, planService, nil, nil).Refresh())

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/action", strings.NewReader(`{"planId":11}`))
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, request)

	assertAutoIngestBusinessError(t, recorder, codePlanRefreshFailed)
}

func TestRetryFailedSkipsMissingLogServiceWithoutFailingRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)

	planService := &mockPlanAccessService{
		plans: map[int64]*models.AutoIngestPlan{
			11: {ID: 11, UserID: 100, SourceType: autoingest.SourceTypeSubscribe},
		},
	}
	router := newPlanAccessRouter(NewHandler(&mockPlanAccessTaskEngine{}, planService, nil, nil).RetryFailed())

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/action", strings.NewReader(`{"planId":11}`))
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestBatchRefreshReturnsBusinessErrorWhenTaskEngineMissing(t *testing.T) {
	gin.SetMode(gin.TestMode)

	planService := &mockBatchAutoIngestPlanService{
		plans: map[int64]*models.AutoIngestPlan{
			11: {ID: 11, UserID: 100, SourceType: autoingest.SourceTypeSubscribe},
		},
	}
	router := newBatchTestRouter(planService, nil, false, func(router *gin.Engine, wrapper *httpcontext.HandlerFuncWrapper, handler Handler) {
		router.POST("/batch_refresh", wrapper.Wrap(handler.BatchRefresh()))
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/batch_refresh", strings.NewReader(`{"ids":[11]}`))
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, request)

	assertAutoIngestBusinessError(t, recorder, codePlanRefreshFailed)
}

func TestDeleteErrorLogsReturnsBusinessErrorWhenPlanServiceMissingForSpecificPlan(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := newDeleteErrorLogsRouter(nil, &mockDeleteErrorLogsService{}, false)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/delete_error", strings.NewReader(`{"planId":11}`))
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, request)

	assertAutoIngestBusinessError(t, recorder, codePlanQueryFailed)
}

func newCreateSubscribePlanDependencyRouter(handler httpcontext.HandlerFunc) *gin.Engine {
	router := gin.New()
	router.Use(func(ctx *gin.Context) {
		ctx.Set(consts.CtxKeyUserId, int64(100))
		ctx.Set(consts.CtxKeyIsAdmin, false)
	})

	wrapper := httpcontext.NewHandlerFuncWrapper(nil)
	router.POST("/create", wrapper.Wrap(handler))

	return router
}

func postCreateSubscribePlan(router *gin.Engine, body string) *httptest.ResponseRecorder {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/create", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, request)

	return recorder
}

func assertAutoIngestBusinessError(t *testing.T, recorder *httptest.ResponseRecorder, want httpcontext.BusinessError) {
	t.Helper()

	if recorder.Code != want.GetHTTPCode() {
		t.Fatalf("expected http status %d, got %d body=%s", want.GetHTTPCode(), recorder.Code, recorder.Body.String())
	}

	var response httpcontext.Response
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if response.Code != want.GetCode() {
		t.Fatalf("expected business code %d, got %d", want.GetCode(), response.Code)
	}

	if response.Msg != want.GetMessage() {
		t.Fatalf("expected message %q, got %q", want.GetMessage(), response.Msg)
	}
}
