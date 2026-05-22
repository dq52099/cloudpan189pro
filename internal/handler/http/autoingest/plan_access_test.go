package autoingest

import (
	stdctx "context"
	"errors"
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
	autoingestplanSvi "github.com/xxcheng123/cloudpan189-share/internal/services/autoingestplan"
	"github.com/xxcheng123/cloudpan189-share/internal/types/autoingest"
	"go.uber.org/zap"
)

type mockPlanAccessService struct {
	autoingestplanSvi.Service
	mu              sync.Mutex
	plans           map[int64]*models.AutoIngestPlan
	enabledIDs      []int64
	disabledIDs     []int64
	updatedIDs      []int64
	updatedFields   [][]utils.Field
	offsetIDs       []int64
	resetCounterIDs []int64
}

func (m *mockPlanAccessService) Query(ctx appContext.Context, id int64) (*models.AutoIngestPlan, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	return m.plans[id], nil
}

func (m *mockPlanAccessService) Enable(ctx appContext.Context, id int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.enabledIDs = append(m.enabledIDs, id)

	return nil
}

func (m *mockPlanAccessService) Disable(ctx appContext.Context, id int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.disabledIDs = append(m.disabledIDs, id)

	return nil
}

func (m *mockPlanAccessService) Update(ctx appContext.Context, id int64, fields ...utils.Field) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.updatedIDs = append(m.updatedIDs, id)
	m.updatedFields = append(m.updatedFields, append([]utils.Field(nil), fields...))

	return nil
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

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected unauthorized, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	if len(planService.enabledIDs) != 0 {
		t.Fatalf("expected enable not to be called, got %v", planService.enabledIDs)
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

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected unauthorized, got %d body=%s", recorder.Code, recorder.Body.String())
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

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected unauthorized, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	if len(taskEngine.payloads) != 0 {
		t.Fatalf("expected no queued tasks, got %d", len(taskEngine.payloads))
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

func TestRetryPlanRestoresStateWhenQueueingFails(t *testing.T) {
	gin.SetMode(gin.TestMode)

	taskEngine := &mockPlanAccessTaskEngine{pushErr: errors.New("queue down")}
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

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected bad request, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	if got, want := planService.offsetIDs, []int64{11}; !int64SlicesEqual(got, want) {
		t.Fatalf("expected retry offset reset %v, got %v", want, got)
	}

	if got, want := planService.resetCounterIDs, []int64{11}; !int64SlicesEqual(got, want) {
		t.Fatalf("expected retry counter reset %v, got %v", want, got)
	}

	if got, want := planService.updatedIDs, []int64{11}; !int64SlicesEqual(got, want) {
		t.Fatalf("expected rollback update %v, got %v", want, got)
	}

	if len(planService.updatedFields) != 1 {
		t.Fatalf("expected one rollback update, got %d", len(planService.updatedFields))
	}

	fields := map[string]any{}
	for _, field := range planService.updatedFields[0] {
		fields[field.Key] = field.Value
	}

	if fields["offset"] != int64(88) || fields["add_count"] != int64(7) || fields["failed_count"] != int64(3) {
		t.Fatalf("unexpected rollback fields: %+v", fields)
	}
}

func TestRetryFailedRestoresOffsetWhenQueueingFails(t *testing.T) {
	gin.SetMode(gin.TestMode)

	taskEngine := &mockPlanAccessTaskEngine{pushErr: errors.New("queue down")}
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

	router := newPlanAccessRouter(NewHandler(taskEngine, planService, nil, nil).RetryFailed())
	req := httptest.NewRequestWithContext(stdctx.Background(), http.MethodPost, "/action", strings.NewReader(`{"planId":11}`))
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected bad request, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	if got, want := planService.offsetIDs, []int64{11}; !int64SlicesEqual(got, want) {
		t.Fatalf("expected retry offset reset %v, got %v", want, got)
	}

	if len(planService.resetCounterIDs) != 0 {
		t.Fatalf("expected retry failed not to reset counters, got %v", planService.resetCounterIDs)
	}

	if got, want := planService.updatedIDs, []int64{11}; !int64SlicesEqual(got, want) {
		t.Fatalf("expected rollback update %v, got %v", want, got)
	}

	fields := map[string]any{}
	for _, field := range planService.updatedFields[0] {
		fields[field.Key] = field.Value
	}

	if fields["offset"] != int64(66) || fields["add_count"] != int64(4) || fields["failed_count"] != int64(1) {
		t.Fatalf("unexpected rollback fields: %+v", fields)
	}
}
