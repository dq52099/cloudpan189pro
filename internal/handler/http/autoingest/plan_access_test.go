package autoingest

import (
	stdctx "context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/xxcheng123/cloudpan189-share/internal/consts"
	appContext "github.com/xxcheng123/cloudpan189-share/internal/framework/context"
	"github.com/xxcheng123/cloudpan189-share/internal/framework/httpcontext"
	"github.com/xxcheng123/cloudpan189-share/internal/pkgs/taskengine"
	"github.com/xxcheng123/cloudpan189-share/internal/pkgs/utils"
	"github.com/xxcheng123/cloudpan189-share/internal/repository/models"
	autoingestlogSvi "github.com/xxcheng123/cloudpan189-share/internal/services/autoingestlog"
	autoingestplanSvi "github.com/xxcheng123/cloudpan189-share/internal/services/autoingestplan"
	"github.com/xxcheng123/cloudpan189-share/internal/types/autoingest"
	"github.com/xxcheng123/cloudpan189-share/internal/types/topic"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type mockPlanAccessService struct {
	autoingestplanSvi.Service
	mu              sync.Mutex
	plans           map[int64]*models.AutoIngestPlan
	enabledIDs      []int64
	disabledIDs     []int64
	enableRequests  []*autoingestplanSvi.UpdateRequest
	disableRequests []*autoingestplanSvi.UpdateRequest
	updatedIDs      []int64
	updatedFields   [][]utils.Field
	updateRequests  []*autoingestplanSvi.UpdateRequest
	offsetIDs       []int64
	resetCounterIDs []int64
	deleteRequests  []*autoingestplanSvi.DeleteRequest
	queryErr        error
	deleteErr       error
	updateErr       error
}

func (m *mockPlanAccessService) Query(ctx appContext.Context, id int64) (*models.AutoIngestPlan, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.queryErr != nil {
		return nil, m.queryErr
	}

	return m.plans[id], nil
}

func (m *mockPlanAccessService) Enable(ctx appContext.Context, id int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.enabledIDs = append(m.enabledIDs, id)

	return nil
}

func (m *mockPlanAccessService) EnableByOwner(ctx appContext.Context, req *autoingestplanSvi.UpdateRequest) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if req != nil {
		copied := *req
		m.enableRequests = append(m.enableRequests, &copied)
		m.enabledIDs = append(m.enabledIDs, req.ID)
	}

	return nil
}

func (m *mockPlanAccessService) Disable(ctx appContext.Context, id int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.disabledIDs = append(m.disabledIDs, id)

	return nil
}

func (m *mockPlanAccessService) DisableByOwner(ctx appContext.Context, req *autoingestplanSvi.UpdateRequest) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if req != nil {
		copied := *req
		m.disableRequests = append(m.disableRequests, &copied)
		m.disabledIDs = append(m.disabledIDs, req.ID)
	}

	return nil
}

func (m *mockPlanAccessService) Update(ctx appContext.Context, id int64, fields ...utils.Field) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.updateErr != nil {
		return m.updateErr
	}

	m.updatedIDs = append(m.updatedIDs, id)
	m.updatedFields = append(m.updatedFields, append([]utils.Field(nil), fields...))

	return nil
}

func (m *mockPlanAccessService) UpdateByOwner(ctx appContext.Context, req *autoingestplanSvi.UpdateRequest, fields ...utils.Field) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if req != nil {
		copied := *req
		m.updateRequests = append(m.updateRequests, &copied)
	}

	if m.updateErr != nil {
		return m.updateErr
	}

	if req != nil {
		m.updatedIDs = append(m.updatedIDs, req.ID)
	}

	m.updatedFields = append(m.updatedFields, append([]utils.Field(nil), fields...))

	return nil
}

func (m *mockPlanAccessService) Delete(ctx appContext.Context, req *autoingestplanSvi.DeleteRequest) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if req != nil {
		copied := *req
		m.deleteRequests = append(m.deleteRequests, &copied)
	}

	return m.deleteErr
}

func (m *mockPlanAccessService) UpdateOffset(ctx appContext.Context, id int64, offset int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.offsetIDs = append(m.offsetIDs, id)

	return nil
}

func (m *mockPlanAccessService) ResetCounters(ctx appContext.Context, id int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.resetCounterIDs = append(m.resetCounterIDs, id)

	return nil
}

func fieldsToMap(fields []utils.Field) map[string]any {
	result := make(map[string]any, len(fields))
	for _, field := range fields {
		result[field.Key] = field.Value
	}

	return result
}

type mockPlanAccessTaskEngine struct {
	taskengine.TaskEngine
	mu       sync.Mutex
	payloads [][]byte
	pushErr  error
}

func (m *mockPlanAccessTaskEngine) PushMessage(ctx stdctx.Context, taskTopic taskengine.Topic, payload []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.payloads = append(m.payloads, payload)

	return m.pushErr
}

type mockPlanAccessLogService struct {
	autoingestlogSvi.Service
}

func (m *mockPlanAccessLogService) Create(ctx appContext.Context, planID int64, level autoingest.LogLevel, content string) (int64, error) {
	return 1, nil
}

func newPlanAccessRouter(handler httpcontext.HandlerFunc) *gin.Engine {
	router := gin.New()
	router.Use(func(ctx *gin.Context) {
		ctx.Set(consts.CtxKeyUserId, int64(100))
		ctx.Set(consts.CtxKeyIsAdmin, false)
		ctx.Next()
	})

	wrapper := httpcontext.NewHandlerFuncWrapper(zap.NewNop())
	router.POST("/action", wrapper.Wrap(handler))

	return router
}

func assertPlanNotFoundResponse(t *testing.T, recorder *httptest.ResponseRecorder) {
	t.Helper()

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("expected not found, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	var response httpcontext.Response
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if response.Code != codePlanNotFound.GetCode() {
		t.Fatalf("expected business code %d, got %d", codePlanNotFound.GetCode(), response.Code)
	}

	if response.Msg != codePlanNotFound.GetMessage() {
		t.Fatalf("expected message %q, got %q", codePlanNotFound.GetMessage(), response.Msg)
	}
}

func TestDeletePlanReturnsNotFoundWhenPlanMissing(t *testing.T) {
	gin.SetMode(gin.TestMode)

	planService := &mockPlanAccessService{queryErr: gorm.ErrRecordNotFound}

	router := newPlanAccessRouter(NewHandler(nil, planService, nil, nil).DeletePlan())
	req := httptest.NewRequestWithContext(stdctx.Background(), http.MethodPost, "/action", strings.NewReader(`{"id":11}`))
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	assertPlanNotFoundResponse(t, recorder)

	if len(planService.deleteRequests) != 0 {
		t.Fatalf("expected delete not to be called, got %v", planService.deleteRequests)
	}
}

func TestDeletePlanRejectsOtherUsersPlan(t *testing.T) {
	gin.SetMode(gin.TestMode)

	planService := &mockPlanAccessService{
		plans: map[int64]*models.AutoIngestPlan{
			11: {ID: 11, UserID: 200, SourceType: autoingest.SourceTypeSubscribe},
		},
	}

	router := newPlanAccessRouter(NewHandler(nil, planService, nil, nil).DeletePlan())
	req := httptest.NewRequestWithContext(stdctx.Background(), http.MethodPost, "/action", strings.NewReader(`{"id":11}`))
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusForbidden {
		t.Fatalf("expected forbidden, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	if len(planService.deleteRequests) != 0 {
		t.Fatalf("expected delete not to be called, got %v", planService.deleteRequests)
	}
}

func TestDeletePlanKeepsUserContext(t *testing.T) {
	gin.SetMode(gin.TestMode)

	planService := &mockPlanAccessService{
		plans: map[int64]*models.AutoIngestPlan{
			11: {ID: 11, UserID: 100, SourceType: autoingest.SourceTypeSubscribe},
		},
	}

	router := newPlanAccessRouter(NewHandler(nil, planService, nil, nil).DeletePlan())
	req := httptest.NewRequestWithContext(stdctx.Background(), http.MethodPost, "/action", strings.NewReader(`{"id":11}`))
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected ok, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	if len(planService.deleteRequests) != 1 {
		t.Fatalf("expected one delete request, got %d", len(planService.deleteRequests))
	}

	deleteReq := planService.deleteRequests[0]
	if deleteReq.ID != 11 || deleteReq.UserID != 100 || deleteReq.IsAdmin {
		t.Fatalf("unexpected delete request: %+v", deleteReq)
	}
}

func TestUpdatePlanReturnsNotFoundWhenQueryMissing(t *testing.T) {
	gin.SetMode(gin.TestMode)

	planService := &mockPlanAccessService{queryErr: gorm.ErrRecordNotFound}

	router := newPlanAccessRouter(NewHandler(nil, planService, nil, nil).UpdatePlan())
	req := httptest.NewRequestWithContext(stdctx.Background(), http.MethodPost, "/action", strings.NewReader(`{"id":11,"name":"new-name"}`))
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	assertPlanNotFoundResponse(t, recorder)

	if len(planService.updatedIDs) != 0 {
		t.Fatalf("expected update not to be called, got %v", planService.updatedIDs)
	}
}

func TestUpdatePlanReturnsNotFoundWhenUpdateSeesMissingPlan(t *testing.T) {
	gin.SetMode(gin.TestMode)

	planService := &mockPlanAccessService{
		plans: map[int64]*models.AutoIngestPlan{
			11: {ID: 11, UserID: 100, SourceType: autoingest.SourceTypeSubscribe},
		},
		updateErr: gorm.ErrRecordNotFound,
	}

	router := newPlanAccessRouter(NewHandler(nil, planService, nil, nil).UpdatePlan())
	req := httptest.NewRequestWithContext(stdctx.Background(), http.MethodPost, "/action", strings.NewReader(`{"id":11,"name":"new-name"}`))
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	assertPlanNotFoundResponse(t, recorder)
}

func TestUpdatePlanPassesOwnerToServiceUpdate(t *testing.T) {
	gin.SetMode(gin.TestMode)

	planService := &mockPlanAccessService{
		plans: map[int64]*models.AutoIngestPlan{
			11: {ID: 11, UserID: 100, SourceType: autoingest.SourceTypeSubscribe},
		},
	}

	router := newPlanAccessRouter(NewHandler(nil, planService, nil, nil).UpdatePlan())
	req := httptest.NewRequestWithContext(stdctx.Background(), http.MethodPost, "/action", strings.NewReader(`{"id":11,"name":"new-name"}`))
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected ok, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	if len(planService.updateRequests) != 1 {
		t.Fatalf("expected one update request, got %d", len(planService.updateRequests))
	}

	updateReq := planService.updateRequests[0]
	if updateReq.ID != 11 || updateReq.UserID != 100 || updateReq.IsAdmin {
		t.Fatalf("unexpected update request: %+v", updateReq)
	}
}

func TestPlanActionsReturnNotFoundWhenQueryMissing(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name    string
		body    string
		handler func(Handler) httpcontext.HandlerFunc
	}{
		{
			name: "enable",
			body: `{"id":11}`,
			handler: func(h Handler) httpcontext.HandlerFunc {
				return h.EnablePlan()
			},
		},
		{
			name: "disable",
			body: `{"id":11}`,
			handler: func(h Handler) httpcontext.HandlerFunc {
				return h.DisablePlan()
			},
		},
		{
			name: "refresh",
			body: `{"planId":11}`,
			handler: func(h Handler) httpcontext.HandlerFunc {
				return h.Refresh()
			},
		},
		{
			name: "retry failed",
			body: `{"planId":11}`,
			handler: func(h Handler) httpcontext.HandlerFunc {
				return h.RetryFailed()
			},
		},
		{
			name: "retry plan",
			body: `{"id":11}`,
			handler: func(h Handler) httpcontext.HandlerFunc {
				return h.RetryPlan()
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			taskEngine := &mockPlanAccessTaskEngine{}
			planService := &mockPlanAccessService{queryErr: gorm.ErrRecordNotFound}

			router := newPlanAccessRouter(tt.handler(NewHandler(taskEngine, planService, nil, nil)))
			req := httptest.NewRequestWithContext(stdctx.Background(), http.MethodPost, "/action", strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/json")

			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, req)

			assertPlanNotFoundResponse(t, recorder)

			if len(taskEngine.payloads) != 0 {
				t.Fatalf("expected no queued tasks, got %d", len(taskEngine.payloads))
			}
		})
	}
}

func TestEnablePlanRejectsOtherUsersPlan(t *testing.T) {
	gin.SetMode(gin.TestMode)

	planService := &mockPlanAccessService{
		plans: map[int64]*models.AutoIngestPlan{
			11: {ID: 11, UserID: 200, SourceType: autoingest.SourceTypeSubscribe},
		},
	}

	router := newPlanAccessRouter(NewHandler(nil, planService, nil, nil).EnablePlan())
	req := httptest.NewRequestWithContext(stdctx.Background(), http.MethodPost, "/action", strings.NewReader(`{"id":11}`))
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusForbidden {
		t.Fatalf("expected forbidden, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	if len(planService.enabledIDs) != 0 {
		t.Fatalf("expected enable not to be called, got %v", planService.enabledIDs)
	}
}

func TestEnablePlanPassesOwnerToService(t *testing.T) {
	gin.SetMode(gin.TestMode)

	planService := &mockPlanAccessService{
		plans: map[int64]*models.AutoIngestPlan{
			11: {ID: 11, UserID: 100, SourceType: autoingest.SourceTypeSubscribe},
		},
	}

	router := newPlanAccessRouter(NewHandler(nil, planService, nil, nil).EnablePlan())
	req := httptest.NewRequestWithContext(stdctx.Background(), http.MethodPost, "/action", strings.NewReader(`{"id":11}`))
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected ok, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	if len(planService.enableRequests) != 1 {
		t.Fatalf("expected one enable request, got %d", len(planService.enableRequests))
	}

	enableReq := planService.enableRequests[0]
	if enableReq.ID != 11 || enableReq.UserID != 100 || enableReq.IsAdmin {
		t.Fatalf("unexpected enable request: %+v", enableReq)
	}
}

func TestDisablePlanPassesOwnerToService(t *testing.T) {
	gin.SetMode(gin.TestMode)

	planService := &mockPlanAccessService{
		plans: map[int64]*models.AutoIngestPlan{
			11: {ID: 11, UserID: 100, SourceType: autoingest.SourceTypeSubscribe},
		},
	}

	router := newPlanAccessRouter(NewHandler(nil, planService, nil, nil).DisablePlan())
	req := httptest.NewRequestWithContext(stdctx.Background(), http.MethodPost, "/action", strings.NewReader(`{"id":11}`))
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected ok, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	if len(planService.disableRequests) != 1 {
		t.Fatalf("expected one disable request, got %d", len(planService.disableRequests))
	}

	disableReq := planService.disableRequests[0]
	if disableReq.ID != 11 || disableReq.UserID != 100 || disableReq.IsAdmin {
		t.Fatalf("unexpected disable request: %+v", disableReq)
	}
}

func TestUpdatePlanRejectsOtherUsersPlan(t *testing.T) {
	gin.SetMode(gin.TestMode)

	planService := &mockPlanAccessService{
		plans: map[int64]*models.AutoIngestPlan{
			11: {ID: 11, UserID: 200, SourceType: autoingest.SourceTypeSubscribe},
		},
	}

	router := newPlanAccessRouter(NewHandler(nil, planService, nil, nil).UpdatePlan())
	req := httptest.NewRequestWithContext(stdctx.Background(), http.MethodPost, "/action", strings.NewReader(`{"id":11,"name":"new-name"}`))
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusForbidden {
		t.Fatalf("expected forbidden, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	if len(planService.updatedIDs) != 0 {
		t.Fatalf("expected update not to be called, got %v", planService.updatedIDs)
	}
}

func TestUpdatePlanRejectsRefreshIntervalOverMax(t *testing.T) {
	gin.SetMode(gin.TestMode)

	planService := &mockPlanAccessService{
		plans: map[int64]*models.AutoIngestPlan{
			11: {ID: 11, UserID: 100, SourceType: autoingest.SourceTypeSubscribe},
		},
	}

	router := newPlanAccessRouter(NewHandler(nil, planService, nil, nil).UpdatePlan())
	req := httptest.NewRequestWithContext(stdctx.Background(), http.MethodPost, "/action", strings.NewReader(`{"id":11,"refreshStrategy":{"refreshInterval":1441}}`))
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected bad request, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	if len(planService.updatedIDs) != 0 {
		t.Fatalf("expected update not to be called, got %v", planService.updatedIDs)
	}
}

func TestRefreshPlanRejectsOtherUsersPlanBeforeQueueing(t *testing.T) {
	gin.SetMode(gin.TestMode)

	taskEngine := &mockPlanAccessTaskEngine{}
	planService := &mockPlanAccessService{
		plans: map[int64]*models.AutoIngestPlan{
			11: {ID: 11, UserID: 200, SourceType: autoingest.SourceTypeSubscribe},
		},
	}

	router := newPlanAccessRouter(NewHandler(taskEngine, planService, nil, nil).Refresh())
	req := httptest.NewRequestWithContext(stdctx.Background(), http.MethodPost, "/action", strings.NewReader(`{"planId":11}`))
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusForbidden {
		t.Fatalf("expected forbidden, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	if len(taskEngine.payloads) != 0 {
		t.Fatalf("expected no queued tasks, got %d", len(taskEngine.payloads))
	}
}

func TestRefreshPlanQueuesOwnerSnapshot(t *testing.T) {
	gin.SetMode(gin.TestMode)

	taskEngine := &mockPlanAccessTaskEngine{}
	planService := &mockPlanAccessService{
		plans: map[int64]*models.AutoIngestPlan{
			11: {ID: 11, UserID: 100, SourceType: autoingest.SourceTypeSubscribe},
		},
	}

	router := newPlanAccessRouter(NewHandler(taskEngine, planService, nil, nil).Refresh())
	req := httptest.NewRequestWithContext(stdctx.Background(), http.MethodPost, "/action", strings.NewReader(`{"planId":11}`))
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected ok, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	if len(taskEngine.payloads) != 1 {
		t.Fatalf("expected one queued task, got %d", len(taskEngine.payloads))
	}

	var taskReq topic.AutoIngestRefreshSubscribeRequest
	if err := json.Unmarshal(taskEngine.payloads[0], &taskReq); err != nil {
		t.Fatal(err)
	}

	if taskReq.PlanId != 11 || taskReq.IsRetry || taskReq.ExpectedUserID != 100 || taskReq.TriggeredByAdmin {
		t.Fatalf("unexpected refresh task payload: %+v", taskReq)
	}
}

func TestRefreshAndRetryFailedRejectNegativePlanIDBeforeQuerying(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name    string
		handler func(Handler) httpcontext.HandlerFunc
	}{
		{
			name: "refresh",
			handler: func(h Handler) httpcontext.HandlerFunc {
				return h.Refresh()
			},
		},
		{
			name: "retry failed",
			handler: func(h Handler) httpcontext.HandlerFunc {
				return h.RetryFailed()
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			taskEngine := &mockPlanAccessTaskEngine{}
			planService := &mockPlanAccessService{plans: map[int64]*models.AutoIngestPlan{}}

			router := newPlanAccessRouter(tt.handler(NewHandler(taskEngine, planService, nil, nil)))
			req := httptest.NewRequestWithContext(stdctx.Background(), http.MethodPost, "/action", strings.NewReader(`{"planId":-1}`))
			req.Header.Set("Content-Type", "application/json")

			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, req)

			if recorder.Code != http.StatusBadRequest {
				t.Fatalf("expected bad request, got %d body=%s", recorder.Code, recorder.Body.String())
			}

			if len(taskEngine.payloads) != 0 {
				t.Fatalf("expected no queued tasks, got %d", len(taskEngine.payloads))
			}
		})
	}
}

func TestRetryPlanDoesNotResetStateBeforeQueueing(t *testing.T) {
	gin.SetMode(gin.TestMode)

	taskEngine := &mockPlanAccessTaskEngine{}
	planService := &mockPlanAccessService{
		plans: map[int64]*models.AutoIngestPlan{
			11: {
				ID:          11,
				UserID:      100,
				SourceType:  autoingest.SourceTypeSubscribe,
				Offset:      88,
				AddCount:    7,
				FailedCount: 3,
			},
		},
	}

	router := newPlanAccessRouter(NewHandler(taskEngine, planService, nil, nil).RetryPlan())
	req := httptest.NewRequestWithContext(stdctx.Background(), http.MethodPost, "/action", strings.NewReader(`{"id":11}`))
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected ok, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	if len(planService.offsetIDs) != 0 {
		t.Fatalf("expected retry plan not to use separate offset update, got %v", planService.offsetIDs)
	}

	if len(planService.resetCounterIDs) != 0 {
		t.Fatalf("expected retry plan not to use separate counter reset, got %v", planService.resetCounterIDs)
	}

	if len(planService.updatedIDs) != 0 || len(planService.updateRequests) != 0 || len(planService.updatedFields) != 0 {
		t.Fatalf("expected retry plan not to update state before queueing, ids=%v reqs=%d fields=%d",
			planService.updatedIDs, len(planService.updateRequests), len(planService.updatedFields))
	}

	if len(taskEngine.payloads) != 1 {
		t.Fatalf("expected one queued retry task, got %d", len(taskEngine.payloads))
	}

	var taskReq topic.AutoIngestRefreshSubscribeRequest
	if err := json.Unmarshal(taskEngine.payloads[0], &taskReq); err != nil {
		t.Fatal(err)
	}

	if taskReq.PlanId != 11 || !taskReq.IsRetry || taskReq.ExpectedUserID != 100 || taskReq.TriggeredByAdmin {
		t.Fatalf("unexpected retry task payload: %+v", taskReq)
	}

	if taskReq.RetryReset == nil || taskReq.RetryReset.Offset != 1 || !taskReq.RetryReset.ResetCounters {
		t.Fatalf("unexpected retry reset payload: %+v", taskReq.RetryReset)
	}
}

func TestRetryFailedDoesNotResetStateBeforeQueueing(t *testing.T) {
	gin.SetMode(gin.TestMode)

	taskEngine := &mockPlanAccessTaskEngine{}
	planService := &mockPlanAccessService{
		plans: map[int64]*models.AutoIngestPlan{
			11: {
				ID:          11,
				UserID:      100,
				SourceType:  autoingest.SourceTypeSubscribe,
				Offset:      66,
				AddCount:    4,
				FailedCount: 1,
			},
		},
	}

	router := newPlanAccessRouter(NewHandler(taskEngine, planService, &mockPlanAccessLogService{}, nil).RetryFailed())
	req := httptest.NewRequestWithContext(stdctx.Background(), http.MethodPost, "/action", strings.NewReader(`{"planId":11}`))
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected ok, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	if len(planService.offsetIDs) != 0 {
		t.Fatalf("expected retry failed not to use separate offset update, got %v", planService.offsetIDs)
	}

	if len(planService.resetCounterIDs) != 0 {
		t.Fatalf("expected retry failed not to reset counters, got %v", planService.resetCounterIDs)
	}

	if len(planService.updatedIDs) != 0 || len(planService.updateRequests) != 0 || len(planService.updatedFields) != 0 {
		t.Fatalf("expected retry failed not to update state before queueing, ids=%v reqs=%d fields=%d",
			planService.updatedIDs, len(planService.updateRequests), len(planService.updatedFields))
	}

	if len(taskEngine.payloads) != 1 {
		t.Fatalf("expected one queued retry task, got %d", len(taskEngine.payloads))
	}

	var taskReq topic.AutoIngestRefreshSubscribeRequest
	if err := json.Unmarshal(taskEngine.payloads[0], &taskReq); err != nil {
		t.Fatal(err)
	}

	if taskReq.PlanId != 11 || taskReq.IsRetry || taskReq.ExpectedUserID != 100 || taskReq.TriggeredByAdmin {
		t.Fatalf("unexpected retry failed task payload: %+v", taskReq)
	}

	if taskReq.RetryReset == nil || taskReq.RetryReset.Offset != 0 || taskReq.RetryReset.ResetCounters {
		t.Fatalf("unexpected retry failed reset payload: %+v", taskReq.RetryReset)
	}
}
