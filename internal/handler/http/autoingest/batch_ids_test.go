package autoingest

import (
	stdctx "context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sort"
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
	"github.com/xxcheng123/cloudpan189-share/internal/types/topic"
	"go.uber.org/zap"
)

type mockBatchAutoIngestPlanService struct {
	autoingestplanSvi.Service
	mu            sync.Mutex
	enabledIDs    []int64
	disabledIDs   []int64
	enableReqs    []*autoingestplanSvi.UpdateRequest
	disableReqs   []*autoingestplanSvi.UpdateRequest
	updateReqs    []*autoingestplanSvi.UpdateRequest
	updatedIDs    []int64
	updatedFields [][]utils.Field
	deletedReqs   []*autoingestplanSvi.DeleteRequest
	listByIDsReqs [][]int64
	offsetIDs     []int64
	resetIDs      []int64
	plans         map[int64]*models.AutoIngestPlan
	enableErrs    map[int64]error
	disableErrs   map[int64]error
	deleteErrs    map[int64]error
}

func (m *mockBatchAutoIngestPlanService) Enable(ctx appContext.Context, id int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.enabledIDs = append(m.enabledIDs, id)
	if err, ok := m.enableErrs[id]; ok {
		return err
	}

	return nil
}

func (m *mockBatchAutoIngestPlanService) EnableByOwner(ctx appContext.Context, req *autoingestplanSvi.UpdateRequest) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.enableReqs = append(m.enableReqs, &autoingestplanSvi.UpdateRequest{
		ID:      req.ID,
		UserID:  req.UserID,
		IsAdmin: req.IsAdmin,
	})

	m.enabledIDs = append(m.enabledIDs, req.ID)
	if err, ok := m.enableErrs[req.ID]; ok {
		return err
	}

	return nil
}

func (m *mockBatchAutoIngestPlanService) Disable(ctx appContext.Context, id int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.disabledIDs = append(m.disabledIDs, id)
	if err, ok := m.disableErrs[id]; ok {
		return err
	}

	return nil
}

func (m *mockBatchAutoIngestPlanService) DisableByOwner(ctx appContext.Context, req *autoingestplanSvi.UpdateRequest) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.disableReqs = append(m.disableReqs, &autoingestplanSvi.UpdateRequest{
		ID:      req.ID,
		UserID:  req.UserID,
		IsAdmin: req.IsAdmin,
	})

	m.disabledIDs = append(m.disabledIDs, req.ID)
	if err, ok := m.disableErrs[req.ID]; ok {
		return err
	}

	return nil
}

func (m *mockBatchAutoIngestPlanService) Update(ctx appContext.Context, id int64, fields ...utils.Field) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.updatedIDs = append(m.updatedIDs, id)
	m.updatedFields = append(m.updatedFields, append([]utils.Field(nil), fields...))

	return nil
}

func (m *mockBatchAutoIngestPlanService) UpdateByOwner(ctx appContext.Context, req *autoingestplanSvi.UpdateRequest, fields ...utils.Field) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if req != nil {
		m.updateReqs = append(m.updateReqs, &autoingestplanSvi.UpdateRequest{
			ID:      req.ID,
			UserID:  req.UserID,
			IsAdmin: req.IsAdmin,
		})
		m.updatedIDs = append(m.updatedIDs, req.ID)
	}

	m.updatedFields = append(m.updatedFields, append([]utils.Field(nil), fields...))

	return nil
}

func (m *mockBatchAutoIngestPlanService) Delete(ctx appContext.Context, req *autoingestplanSvi.DeleteRequest) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.deletedReqs = append(m.deletedReqs, &autoingestplanSvi.DeleteRequest{
		ID:      req.ID,
		UserID:  req.UserID,
		IsAdmin: req.IsAdmin,
	})
	if err, ok := m.deleteErrs[req.ID]; ok {
		return err
	}

	if m.plans != nil {
		plan, ok := m.plans[req.ID]
		if !ok {
			return errors.New("plan missing")
		}

		if !req.IsAdmin && (req.UserID <= 0 || plan.UserID != req.UserID) {
			return errors.New("plan forbidden")
		}
	}

	return nil
}

func (m *mockBatchAutoIngestPlanService) ListByIDs(ctx appContext.Context, ids []int64) ([]*models.AutoIngestPlan, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.listByIDsReqs = append(m.listByIDsReqs, append([]int64(nil), ids...))

	plans := make([]*models.AutoIngestPlan, 0, len(ids))
	for _, id := range ids {
		if plan, ok := m.plans[id]; ok {
			plans = append(plans, plan)
		}
	}

	return plans, nil
}

func (m *mockBatchAutoIngestPlanService) UpdateOffset(ctx appContext.Context, id int64, offset int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.offsetIDs = append(m.offsetIDs, id)

	return nil
}

func (m *mockBatchAutoIngestPlanService) ResetCounters(ctx appContext.Context, id int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.resetIDs = append(m.resetIDs, id)

	return nil
}

type mockBatchAutoIngestTaskEngine struct {
	taskengine.TaskEngine
	mu       sync.Mutex
	payloads [][]byte
	pushErr  error
}

func (m *mockBatchAutoIngestTaskEngine) PushMessage(ctx stdctx.Context, taskTopic taskengine.Topic, payload []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.payloads = append(m.payloads, payload)

	return m.pushErr
}

func newBatchTestRouter(
	planService *mockBatchAutoIngestPlanService,
	taskEngine *mockBatchAutoIngestTaskEngine,
	isAdmin bool,
	register func(*gin.Engine, *httpcontext.HandlerFuncWrapper, Handler),
) *gin.Engine {
	router := gin.New()
	router.Use(func(ctx *gin.Context) {
		ctx.Set(consts.CtxKeyUserId, int64(100))
		ctx.Set(consts.CtxKeyIsAdmin, isAdmin)
	})

	wrapper := httpcontext.NewHandlerFuncWrapper(zap.NewNop())
	register(router, wrapper, NewHandler(taskEngine, planService, nil, nil))

	return router
}

func postBatchRequest(router *gin.Engine, path string, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequestWithContext(stdctx.Background(), http.MethodPost, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	return recorder
}

func parseBatchResponse(t *testing.T, recorder *httptest.ResponseRecorder) map[string]int {
	t.Helper()

	var response struct {
		Data map[string]int `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}

	return response.Data
}

func TestBatchOperationsRejectEmptyIDs(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name     string
		path     string
		register func(*gin.Engine, *httpcontext.HandlerFuncWrapper, Handler)
		assert   func(*testing.T, *mockBatchAutoIngestPlanService, *mockBatchAutoIngestTaskEngine)
	}{
		{
			name: "enable",
			path: "/batch_enable",
			register: func(router *gin.Engine, wrapper *httpcontext.HandlerFuncWrapper, handler Handler) {
				router.POST("/batch_enable", wrapper.Wrap(handler.BatchEnable()))
			},
			assert: func(t *testing.T, planService *mockBatchAutoIngestPlanService, taskEngine *mockBatchAutoIngestTaskEngine) {
				t.Helper()

				if len(planService.enabledIDs) != 0 || len(planService.listByIDsReqs) != 0 {
					t.Fatalf("expected no enable side effects, got enabled=%v list=%v", planService.enabledIDs, planService.listByIDsReqs)
				}
			},
		},
		{
			name: "disable",
			path: "/batch_disable",
			register: func(router *gin.Engine, wrapper *httpcontext.HandlerFuncWrapper, handler Handler) {
				router.POST("/batch_disable", wrapper.Wrap(handler.BatchDisable()))
			},
			assert: func(t *testing.T, planService *mockBatchAutoIngestPlanService, taskEngine *mockBatchAutoIngestTaskEngine) {
				t.Helper()

				if len(planService.disabledIDs) != 0 || len(planService.listByIDsReqs) != 0 {
					t.Fatalf("expected no disable side effects, got disabled=%v list=%v", planService.disabledIDs, planService.listByIDsReqs)
				}
			},
		},
		{
			name: "delete",
			path: "/batch_delete",
			register: func(router *gin.Engine, wrapper *httpcontext.HandlerFuncWrapper, handler Handler) {
				router.POST("/batch_delete", wrapper.Wrap(handler.BatchDelete()))
			},
			assert: func(t *testing.T, planService *mockBatchAutoIngestPlanService, taskEngine *mockBatchAutoIngestTaskEngine) {
				t.Helper()

				if len(planService.deletedReqs) != 0 {
					t.Fatalf("expected no delete side effects, got %v", planService.deletedReqs)
				}
			},
		},
		{
			name: "refresh",
			path: "/batch_refresh",
			register: func(router *gin.Engine, wrapper *httpcontext.HandlerFuncWrapper, handler Handler) {
				router.POST("/batch_refresh", wrapper.Wrap(handler.BatchRefresh()))
			},
			assert: func(t *testing.T, planService *mockBatchAutoIngestPlanService, taskEngine *mockBatchAutoIngestTaskEngine) {
				t.Helper()

				if len(planService.listByIDsReqs) != 0 || len(taskEngine.payloads) != 0 {
					t.Fatalf("expected no refresh side effects, got list=%v payloads=%d", planService.listByIDsReqs, len(taskEngine.payloads))
				}
			},
		},
		{
			name: "retry",
			path: "/batch_retry",
			register: func(router *gin.Engine, wrapper *httpcontext.HandlerFuncWrapper, handler Handler) {
				router.POST("/batch_retry", wrapper.Wrap(handler.BatchRetry()))
			},
			assert: func(t *testing.T, planService *mockBatchAutoIngestPlanService, taskEngine *mockBatchAutoIngestTaskEngine) {
				t.Helper()

				if len(planService.listByIDsReqs) != 0 || len(planService.offsetIDs) != 0 ||
					len(planService.resetIDs) != 0 || len(planService.updatedIDs) != 0 || len(taskEngine.payloads) != 0 {
					t.Fatalf("expected no retry side effects, got list=%v offset=%v reset=%v update=%v payloads=%d",
						planService.listByIDsReqs, planService.offsetIDs, planService.resetIDs, planService.updatedIDs, len(taskEngine.payloads))
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			planService := &mockBatchAutoIngestPlanService{plans: map[int64]*models.AutoIngestPlan{}}
			taskEngine := &mockBatchAutoIngestTaskEngine{}
			router := newBatchTestRouter(planService, taskEngine, false, tt.register)

			recorder := postBatchRequest(router, tt.path, `{"ids":[]}`)
			if recorder.Code != http.StatusBadRequest {
				t.Fatalf("expected bad request, got %d body=%s", recorder.Code, recorder.Body.String())
			}

			tt.assert(t, planService, taskEngine)
		})
	}
}

func TestBatchOperationsReturnNotFoundWhenNoAccessiblePlans(t *testing.T) {
	gin.SetMode(gin.TestMode)

	scenarios := []struct {
		name  string
		plans map[int64]*models.AutoIngestPlan
	}{
		{
			name:  "missing",
			plans: map[int64]*models.AutoIngestPlan{},
		},
		{
			name: "forbidden",
			plans: map[int64]*models.AutoIngestPlan{
				11: {ID: 11, UserID: 200, SourceType: autoingest.SourceTypeSubscribe},
				22: {ID: 22, UserID: 200, SourceType: autoingest.SourceTypeSubscribe},
			},
		},
	}

	tests := []struct {
		name     string
		path     string
		register func(*gin.Engine, *httpcontext.HandlerFuncWrapper, Handler)
		assert   func(*testing.T, *mockBatchAutoIngestPlanService, *mockBatchAutoIngestTaskEngine)
	}{
		{
			name: "enable",
			path: "/batch_enable",
			register: func(router *gin.Engine, wrapper *httpcontext.HandlerFuncWrapper, handler Handler) {
				router.POST("/batch_enable", wrapper.Wrap(handler.BatchEnable()))
			},
			assert: func(t *testing.T, planService *mockBatchAutoIngestPlanService, taskEngine *mockBatchAutoIngestTaskEngine) {
				t.Helper()

				if len(planService.enabledIDs) != 0 {
					t.Fatalf("expected no enable side effects, got %v", planService.enabledIDs)
				}
			},
		},
		{
			name: "disable",
			path: "/batch_disable",
			register: func(router *gin.Engine, wrapper *httpcontext.HandlerFuncWrapper, handler Handler) {
				router.POST("/batch_disable", wrapper.Wrap(handler.BatchDisable()))
			},
			assert: func(t *testing.T, planService *mockBatchAutoIngestPlanService, taskEngine *mockBatchAutoIngestTaskEngine) {
				t.Helper()

				if len(planService.disabledIDs) != 0 {
					t.Fatalf("expected no disable side effects, got %v", planService.disabledIDs)
				}
			},
		},
		{
			name: "delete",
			path: "/batch_delete",
			register: func(router *gin.Engine, wrapper *httpcontext.HandlerFuncWrapper, handler Handler) {
				router.POST("/batch_delete", wrapper.Wrap(handler.BatchDelete()))
			},
			assert: func(t *testing.T, planService *mockBatchAutoIngestPlanService, taskEngine *mockBatchAutoIngestTaskEngine) {
				t.Helper()

				if len(planService.deletedReqs) != 0 {
					t.Fatalf("expected no delete side effects, got %v", planService.deletedReqs)
				}
			},
		},
		{
			name: "refresh",
			path: "/batch_refresh",
			register: func(router *gin.Engine, wrapper *httpcontext.HandlerFuncWrapper, handler Handler) {
				router.POST("/batch_refresh", wrapper.Wrap(handler.BatchRefresh()))
			},
			assert: func(t *testing.T, planService *mockBatchAutoIngestPlanService, taskEngine *mockBatchAutoIngestTaskEngine) {
				t.Helper()

				if len(taskEngine.payloads) != 0 {
					t.Fatalf("expected no refresh side effects, got payloads=%d", len(taskEngine.payloads))
				}
			},
		},
		{
			name: "retry",
			path: "/batch_retry",
			register: func(router *gin.Engine, wrapper *httpcontext.HandlerFuncWrapper, handler Handler) {
				router.POST("/batch_retry", wrapper.Wrap(handler.BatchRetry()))
			},
			assert: func(t *testing.T, planService *mockBatchAutoIngestPlanService, taskEngine *mockBatchAutoIngestTaskEngine) {
				t.Helper()

				if len(planService.offsetIDs) != 0 || len(planService.resetIDs) != 0 ||
					len(planService.updatedIDs) != 0 || len(taskEngine.payloads) != 0 {
					t.Fatalf("expected no retry side effects, got offset=%v reset=%v update=%v payloads=%d",
						planService.offsetIDs, planService.resetIDs, planService.updatedIDs, len(taskEngine.payloads))
				}
			},
		},
	}

	for _, scenario := range scenarios {
		for _, tt := range tests {
			t.Run(scenario.name+"/"+tt.name, func(t *testing.T) {
				planService := &mockBatchAutoIngestPlanService{plans: scenario.plans}
				taskEngine := &mockBatchAutoIngestTaskEngine{}
				router := newBatchTestRouter(planService, taskEngine, false, tt.register)

				recorder := postBatchRequest(router, tt.path, `{"ids":[11,11,22]}`)
				assertPlanNotFoundResponse(t, recorder)

				if len(planService.listByIDsReqs) != 1 {
					t.Fatalf("expected one list call, got %d", len(planService.listByIDsReqs))
				}

				if got, want := planService.listByIDsReqs[0], []int64{11, 22}; !int64SlicesEqual(got, want) {
					t.Fatalf("expected deduplicated list IDs %v, got %v", want, got)
				}

				tt.assert(t, planService, taskEngine)
			})
		}
	}
}

func TestBatchEnableDeduplicatesIDs(t *testing.T) {
	gin.SetMode(gin.TestMode)

	planService := &mockBatchAutoIngestPlanService{
		plans: map[int64]*models.AutoIngestPlan{
			11: {ID: 11, UserID: 100},
			22: {ID: 22, UserID: 100},
		},
	}
	router := gin.New()
	router.Use(func(ctx *gin.Context) {
		ctx.Set(consts.CtxKeyUserId, int64(100))
		ctx.Set(consts.CtxKeyIsAdmin, false)
	})

	wrapper := httpcontext.NewHandlerFuncWrapper(zap.NewNop())
	router.POST("/batch_enable", wrapper.Wrap(NewHandler(nil, planService, nil, nil).BatchEnable()))

	req := httptest.NewRequestWithContext(stdctx.Background(), http.MethodPost, "/batch_enable", strings.NewReader(`{"ids":[11,11,22]}`))
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected ok, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	got := append([]int64(nil), planService.enabledIDs...)
	sort.Slice(got, func(i, j int) bool { return got[i] < got[j] })

	if want := []int64{11, 22}; !int64SlicesEqual(got, want) {
		t.Fatalf("expected enabled IDs %v, got %v", want, got)
	}

	if len(planService.enableReqs) != 2 {
		t.Fatalf("expected two enable requests, got %d", len(planService.enableReqs))
	}

	for _, req := range planService.enableReqs {
		if req.UserID != 100 || req.IsAdmin {
			t.Fatalf("expected enable request to keep user context, got %+v", req)
		}
	}

	var response struct {
		Data map[string]int `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}

	if response.Data["success"] != 2 || response.Data["failed"] != 0 {
		t.Fatalf("unexpected response: %+v", response.Data)
	}
}

func TestBatchEnableAllowsDuplicateIDsBeyondRawLimit(t *testing.T) {
	gin.SetMode(gin.TestMode)

	planService := &mockBatchAutoIngestPlanService{
		plans: map[int64]*models.AutoIngestPlan{
			11: {ID: 11, UserID: 100},
		},
	}
	router := gin.New()
	router.Use(func(ctx *gin.Context) {
		ctx.Set(consts.CtxKeyUserId, int64(100))
		ctx.Set(consts.CtxKeyIsAdmin, false)
	})

	wrapper := httpcontext.NewHandlerFuncWrapper(zap.NewNop())
	router.POST("/batch_enable", wrapper.Wrap(NewHandler(nil, planService, nil, nil).BatchEnable()))

	ids := make([]string, maxBatchIDs+1)
	for i := range ids {
		ids[i] = "11"
	}

	req := httptest.NewRequestWithContext(
		stdctx.Background(),
		http.MethodPost,
		"/batch_enable",
		strings.NewReader(fmt.Sprintf(`{"ids":[%s]}`, strings.Join(ids, ","))),
	)
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected ok, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	if got, want := planService.enabledIDs, []int64{11}; !int64SlicesEqual(got, want) {
		t.Fatalf("expected one enabled ID %v, got %v", want, got)
	}
}

func TestBatchEnableSkipsOtherUsersPlans(t *testing.T) {
	gin.SetMode(gin.TestMode)

	planService := &mockBatchAutoIngestPlanService{
		plans: map[int64]*models.AutoIngestPlan{
			11: {ID: 11, UserID: 100},
			22: {ID: 22, UserID: 200},
		},
	}
	router := gin.New()
	router.Use(func(ctx *gin.Context) {
		ctx.Set(consts.CtxKeyUserId, int64(100))
		ctx.Set(consts.CtxKeyIsAdmin, false)
	})

	wrapper := httpcontext.NewHandlerFuncWrapper(zap.NewNop())
	router.POST("/batch_enable", wrapper.Wrap(NewHandler(nil, planService, nil, nil).BatchEnable()))

	req := httptest.NewRequestWithContext(stdctx.Background(), http.MethodPost, "/batch_enable", strings.NewReader(`{"ids":[11,22,33]}`))
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected ok, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	if got, want := planService.enabledIDs, []int64{11}; !int64SlicesEqual(got, want) {
		t.Fatalf("expected enabled IDs %v, got %v", want, got)
	}

	var response struct {
		Data map[string]int `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}

	if response.Data["success"] != 1 || response.Data["failed"] != 2 {
		t.Fatalf("unexpected response: %+v", response.Data)
	}
}

func TestBatchDisableDeduplicatesIDsAndCountsFailures(t *testing.T) {
	gin.SetMode(gin.TestMode)

	planService := &mockBatchAutoIngestPlanService{
		plans: map[int64]*models.AutoIngestPlan{
			11: {ID: 11, UserID: 100},
			22: {ID: 22, UserID: 100},
			33: {ID: 33, UserID: 200},
		},
		disableErrs: map[int64]error{
			22: errors.New("disable failed"),
		},
	}
	router := gin.New()
	router.Use(func(ctx *gin.Context) {
		ctx.Set(consts.CtxKeyUserId, int64(100))
		ctx.Set(consts.CtxKeyIsAdmin, false)
	})

	wrapper := httpcontext.NewHandlerFuncWrapper(zap.NewNop())
	router.POST("/batch_disable", wrapper.Wrap(NewHandler(nil, planService, nil, nil).BatchDisable()))

	req := httptest.NewRequestWithContext(stdctx.Background(), http.MethodPost, "/batch_disable", strings.NewReader(`{"ids":[11,11,22,33,44]}`))
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected ok, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	if got, want := planService.listByIDsReqs[0], []int64{11, 22, 33, 44}; !int64SlicesEqual(got, want) {
		t.Fatalf("expected deduplicated list IDs %v, got %v", want, got)
	}

	got := append([]int64(nil), planService.disabledIDs...)
	sort.Slice(got, func(i, j int) bool { return got[i] < got[j] })

	if want := []int64{11, 22}; !int64SlicesEqual(got, want) {
		t.Fatalf("expected disabled IDs %v, got %v", want, got)
	}

	if len(planService.disableReqs) != 2 {
		t.Fatalf("expected two disable requests, got %d", len(planService.disableReqs))
	}

	for _, req := range planService.disableReqs {
		if req.UserID != 100 || req.IsAdmin {
			t.Fatalf("expected disable request to keep user context, got %+v", req)
		}
	}

	response := parseBatchResponse(t, recorder)
	if response["success"] != 1 || response["failed"] != 3 {
		t.Fatalf("unexpected response: %+v", response)
	}
}

func TestBatchDeleteDeduplicatesIDsAndKeepsUserContext(t *testing.T) {
	gin.SetMode(gin.TestMode)

	planService := &mockBatchAutoIngestPlanService{
		plans: map[int64]*models.AutoIngestPlan{
			11: {ID: 11, UserID: 100},
			22: {ID: 22, UserID: 100},
		},
	}
	router := gin.New()
	router.Use(func(ctx *gin.Context) {
		ctx.Set(consts.CtxKeyUserId, int64(100))
		ctx.Set(consts.CtxKeyIsAdmin, false)
	})

	wrapper := httpcontext.NewHandlerFuncWrapper(zap.NewNop())
	router.POST("/batch_delete", wrapper.Wrap(NewHandler(nil, planService, nil, nil).BatchDelete()))

	req := httptest.NewRequestWithContext(stdctx.Background(), http.MethodPost, "/batch_delete", strings.NewReader(`{"ids":[11,11,22]}`))
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected ok, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	if got, want := planService.listByIDsReqs[0], []int64{11, 22}; !int64SlicesEqual(got, want) {
		t.Fatalf("expected deduplicated list IDs %v, got %v", want, got)
	}

	gotIDs := make([]int64, 0, len(planService.deletedReqs))
	for _, req := range planService.deletedReqs {
		gotIDs = append(gotIDs, req.ID)
		if req.UserID != 100 || req.IsAdmin {
			t.Fatalf("unexpected delete request: %+v", req)
		}
	}

	sort.Slice(gotIDs, func(i, j int) bool { return gotIDs[i] < gotIDs[j] })

	if want := []int64{11, 22}; !int64SlicesEqual(gotIDs, want) {
		t.Fatalf("expected deleted IDs %v, got %v", want, gotIDs)
	}
}

func TestBatchDeleteCountsMissingForbiddenAndServiceErrors(t *testing.T) {
	gin.SetMode(gin.TestMode)

	planService := &mockBatchAutoIngestPlanService{
		plans: map[int64]*models.AutoIngestPlan{
			11: {ID: 11, UserID: 100},
			22: {ID: 22, UserID: 200},
			33: {ID: 33, UserID: 100},
		},
		deleteErrs: map[int64]error{
			33: errors.New("delete failed"),
		},
	}
	router := gin.New()
	router.Use(func(ctx *gin.Context) {
		ctx.Set(consts.CtxKeyUserId, int64(100))
		ctx.Set(consts.CtxKeyIsAdmin, false)
	})

	wrapper := httpcontext.NewHandlerFuncWrapper(zap.NewNop())
	router.POST("/batch_delete", wrapper.Wrap(NewHandler(nil, planService, nil, nil).BatchDelete()))

	req := httptest.NewRequestWithContext(stdctx.Background(), http.MethodPost, "/batch_delete", strings.NewReader(`{"ids":[11,11,22,33,44]}`))
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected ok, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	gotIDs := make([]int64, 0, len(planService.deletedReqs))
	for _, req := range planService.deletedReqs {
		gotIDs = append(gotIDs, req.ID)
		if req.UserID != 100 || req.IsAdmin {
			t.Fatalf("unexpected delete request: %+v", req)
		}
	}

	sort.Slice(gotIDs, func(i, j int) bool { return gotIDs[i] < gotIDs[j] })

	if want := []int64{11, 33}; !int64SlicesEqual(gotIDs, want) {
		t.Fatalf("expected delete attempts %v, got %v", want, gotIDs)
	}

	response := parseBatchResponse(t, recorder)
	if response["success"] != 1 || response["failed"] != 3 {
		t.Fatalf("unexpected response: %+v", response)
	}
}

func TestBatchRefreshDeduplicatesIDsBeforeListAndQueueing(t *testing.T) {
	gin.SetMode(gin.TestMode)

	taskEngine := &mockBatchAutoIngestTaskEngine{}
	planService := &mockBatchAutoIngestPlanService{
		plans: map[int64]*models.AutoIngestPlan{
			11: {ID: 11, UserID: 100, SourceType: autoingest.SourceTypeSubscribe},
			22: {ID: 22, UserID: 100, SourceType: autoingest.SourceTypeSubscribe},
		},
	}

	router := gin.New()
	router.Use(func(ctx *gin.Context) {
		ctx.Set(consts.CtxKeyUserId, int64(100))
		ctx.Set(consts.CtxKeyIsAdmin, false)
	})

	wrapper := httpcontext.NewHandlerFuncWrapper(zap.NewNop())
	router.POST("/batch_refresh", wrapper.Wrap(NewHandler(taskEngine, planService, nil, nil).BatchRefresh()))

	req := httptest.NewRequestWithContext(stdctx.Background(), http.MethodPost, "/batch_refresh", strings.NewReader(`{"ids":[11,11,22,33]}`))
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected ok, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	if len(planService.listByIDsReqs) != 1 {
		t.Fatalf("expected one list call, got %d", len(planService.listByIDsReqs))
	}

	if got, want := planService.listByIDsReqs[0], []int64{11, 22, 33}; !int64SlicesEqual(got, want) {
		t.Fatalf("expected deduplicated list IDs %v, got %v", want, got)
	}

	if len(taskEngine.payloads) != 2 {
		t.Fatalf("expected 2 queued refresh tasks, got %d", len(taskEngine.payloads))
	}

	queuedIDs := make([]int64, 0, len(taskEngine.payloads))
	for _, payload := range taskEngine.payloads {
		var taskReq topic.AutoIngestRefreshSubscribeRequest
		if err := json.Unmarshal(payload, &taskReq); err != nil {
			t.Fatal(err)
		}

		if taskReq.IsRetry {
			t.Fatalf("unexpected retry task: %+v", taskReq)
		}

		if taskReq.ExpectedUserID != 100 || taskReq.TriggeredByAdmin {
			t.Fatalf("unexpected refresh task owner snapshot: %+v", taskReq)
		}

		queuedIDs = append(queuedIDs, taskReq.PlanId)
	}

	sort.Slice(queuedIDs, func(i, j int) bool { return queuedIDs[i] < queuedIDs[j] })

	if want := []int64{11, 22}; !int64SlicesEqual(queuedIDs, want) {
		t.Fatalf("expected queued plan IDs %v, got %v", want, queuedIDs)
	}

	var response struct {
		Data map[string]int `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}

	if response.Data["success"] != 2 || response.Data["failed"] != 1 {
		t.Fatalf("unexpected response: %+v", response.Data)
	}
}

func TestBatchRefreshSkipsOtherUsersPlans(t *testing.T) {
	gin.SetMode(gin.TestMode)

	taskEngine := &mockBatchAutoIngestTaskEngine{}
	planService := &mockBatchAutoIngestPlanService{
		plans: map[int64]*models.AutoIngestPlan{
			11: {ID: 11, UserID: 100, SourceType: autoingest.SourceTypeSubscribe},
			22: {ID: 22, UserID: 200, SourceType: autoingest.SourceTypeSubscribe},
		},
	}

	router := gin.New()
	router.Use(func(ctx *gin.Context) {
		ctx.Set(consts.CtxKeyUserId, int64(100))
		ctx.Set(consts.CtxKeyIsAdmin, false)
	})

	wrapper := httpcontext.NewHandlerFuncWrapper(zap.NewNop())
	router.POST("/batch_refresh", wrapper.Wrap(NewHandler(taskEngine, planService, nil, nil).BatchRefresh()))

	req := httptest.NewRequestWithContext(stdctx.Background(), http.MethodPost, "/batch_refresh", strings.NewReader(`{"ids":[11,22]}`))
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected ok, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	if len(taskEngine.payloads) != 1 {
		t.Fatalf("expected 1 queued refresh task, got %d", len(taskEngine.payloads))
	}

	var taskReq topic.AutoIngestRefreshSubscribeRequest
	if err := json.Unmarshal(taskEngine.payloads[0], &taskReq); err != nil {
		t.Fatal(err)
	}

	if taskReq.PlanId != 11 {
		t.Fatalf("expected plan 11 queued, got %+v", taskReq)
	}

	if taskReq.ExpectedUserID != 100 || taskReq.TriggeredByAdmin {
		t.Fatalf("unexpected refresh task owner snapshot: %+v", taskReq)
	}

	var response struct {
		Data map[string]int `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}

	if response.Data["success"] != 1 || response.Data["failed"] != 1 {
		t.Fatalf("unexpected response: %+v", response.Data)
	}
}

func TestBatchRefreshRejectsInvalidIDBeforeList(t *testing.T) {
	gin.SetMode(gin.TestMode)

	taskEngine := &mockBatchAutoIngestTaskEngine{}
	planService := &mockBatchAutoIngestPlanService{
		plans: map[int64]*models.AutoIngestPlan{},
	}

	router := gin.New()
	wrapper := httpcontext.NewHandlerFuncWrapper(zap.NewNop())
	router.POST("/batch_refresh", wrapper.Wrap(NewHandler(taskEngine, planService, nil, nil).BatchRefresh()))

	req := httptest.NewRequestWithContext(stdctx.Background(), http.MethodPost, "/batch_refresh", strings.NewReader(`{"ids":[0,11]}`))
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected bad request, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	if len(planService.listByIDsReqs) != 0 {
		t.Fatalf("expected invalid request to stop before ListByIDs, got %v", planService.listByIDsReqs)
	}

	if len(taskEngine.payloads) != 0 {
		t.Fatalf("expected no queued tasks for invalid request, got %d", len(taskEngine.payloads))
	}
}

func TestBatchRetryCountsMissingPlansAsFailed(t *testing.T) {
	gin.SetMode(gin.TestMode)

	taskEngine := &mockBatchAutoIngestTaskEngine{}
	planService := &mockBatchAutoIngestPlanService{
		plans: map[int64]*models.AutoIngestPlan{
			11: {ID: 11, UserID: 100, SourceType: autoingest.SourceTypeSubscribe},
			22: {ID: 22, UserID: 100, SourceType: autoingest.SourceTypeSubscribe},
		},
	}

	router := gin.New()
	router.Use(func(ctx *gin.Context) {
		ctx.Set(consts.CtxKeyUserId, int64(100))
		ctx.Set(consts.CtxKeyIsAdmin, false)
	})

	wrapper := httpcontext.NewHandlerFuncWrapper(zap.NewNop())
	router.POST("/batch_retry", wrapper.Wrap(NewHandler(taskEngine, planService, nil, nil).BatchRetry()))

	req := httptest.NewRequestWithContext(stdctx.Background(), http.MethodPost, "/batch_retry", strings.NewReader(`{"ids":[11,11,22,33]}`))
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected ok, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	if got, want := planService.listByIDsReqs[0], []int64{11, 22, 33}; !int64SlicesEqual(got, want) {
		t.Fatalf("expected deduplicated list IDs %v, got %v", want, got)
	}

	if len(planService.offsetIDs) != 0 {
		t.Fatalf("expected batch retry not to use separate offset updates, got %v", planService.offsetIDs)
	}

	if len(planService.resetIDs) != 0 {
		t.Fatalf("expected batch retry not to use separate counter resets, got %v", planService.resetIDs)
	}

	if len(planService.updatedIDs) != 0 || len(planService.updateReqs) != 0 || len(planService.updatedFields) != 0 {
		t.Fatalf("expected batch retry not to update state before queueing, ids=%v reqs=%d fields=%d",
			planService.updatedIDs, len(planService.updateReqs), len(planService.updatedFields))
	}

	if len(taskEngine.payloads) != 2 {
		t.Fatalf("expected 2 queued retry tasks, got %d", len(taskEngine.payloads))
	}

	queuedIDs := make([]int64, 0, len(taskEngine.payloads))
	for _, payload := range taskEngine.payloads {
		var taskReq topic.AutoIngestRefreshSubscribeRequest
		if err := json.Unmarshal(payload, &taskReq); err != nil {
			t.Fatal(err)
		}

		if !taskReq.IsRetry || taskReq.ExpectedUserID != 100 || taskReq.TriggeredByAdmin {
			t.Fatalf("unexpected retry task owner snapshot: %+v", taskReq)
		}

		if taskReq.RetryReset == nil || taskReq.RetryReset.Offset != 1 || !taskReq.RetryReset.ResetCounters {
			t.Fatalf("unexpected retry reset payload: %+v", taskReq.RetryReset)
		}

		queuedIDs = append(queuedIDs, taskReq.PlanId)
	}

	sort.Slice(queuedIDs, func(i, j int) bool { return queuedIDs[i] < queuedIDs[j] })

	if want := []int64{11, 22}; !int64SlicesEqual(queuedIDs, want) {
		t.Fatalf("expected retry queued IDs %v, got %v", want, queuedIDs)
	}

	var response struct {
		Data map[string]int `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}

	if response.Data["success"] != 2 || response.Data["failed"] != 1 {
		t.Fatalf("unexpected response: %+v", response.Data)
	}
}

func TestBatchRetryDoesNotResetStateWhenQueueingFails(t *testing.T) {
	gin.SetMode(gin.TestMode)

	taskEngine := &mockBatchAutoIngestTaskEngine{pushErr: errors.New("queue down")}
	planService := &mockBatchAutoIngestPlanService{
		plans: map[int64]*models.AutoIngestPlan{
			11: {
				ID:          11,
				UserID:      100,
				SourceType:  autoingest.SourceTypeSubscribe,
				Offset:      44,
				AddCount:    5,
				FailedCount: 2,
			},
		},
	}

	router := gin.New()
	router.Use(func(ctx *gin.Context) {
		ctx.Set(consts.CtxKeyUserId, int64(100))
		ctx.Set(consts.CtxKeyIsAdmin, false)
	})

	wrapper := httpcontext.NewHandlerFuncWrapper(zap.NewNop())
	router.POST("/batch_retry", wrapper.Wrap(NewHandler(taskEngine, planService, nil, nil).BatchRetry()))

	req := httptest.NewRequestWithContext(stdctx.Background(), http.MethodPost, "/batch_retry", strings.NewReader(`{"ids":[11]}`))
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected ok, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	if len(planService.offsetIDs) != 0 {
		t.Fatalf("expected batch retry not to use separate offset update, got %v", planService.offsetIDs)
	}

	if len(planService.resetIDs) != 0 {
		t.Fatalf("expected batch retry not to use separate counter reset, got %v", planService.resetIDs)
	}

	if len(planService.updatedIDs) != 0 || len(planService.updateReqs) != 0 || len(planService.updatedFields) != 0 {
		t.Fatalf("expected batch retry not to update state before queueing, ids=%v reqs=%d fields=%d",
			planService.updatedIDs, len(planService.updateReqs), len(planService.updatedFields))
	}

	var response struct {
		Data map[string]int `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}

	if response.Data["success"] != 0 || response.Data["failed"] != 1 {
		t.Fatalf("unexpected response: %+v", response.Data)
	}
}

func TestBatchRetrySkipsOtherUsersPlans(t *testing.T) {
	gin.SetMode(gin.TestMode)

	taskEngine := &mockBatchAutoIngestTaskEngine{}
	planService := &mockBatchAutoIngestPlanService{
		plans: map[int64]*models.AutoIngestPlan{
			11: {ID: 11, UserID: 100, SourceType: autoingest.SourceTypeSubscribe},
			22: {ID: 22, UserID: 200, SourceType: autoingest.SourceTypeSubscribe},
		},
	}

	router := gin.New()
	router.Use(func(ctx *gin.Context) {
		ctx.Set(consts.CtxKeyUserId, int64(100))
		ctx.Set(consts.CtxKeyIsAdmin, false)
	})

	wrapper := httpcontext.NewHandlerFuncWrapper(zap.NewNop())
	router.POST("/batch_retry", wrapper.Wrap(NewHandler(taskEngine, planService, nil, nil).BatchRetry()))

	req := httptest.NewRequestWithContext(stdctx.Background(), http.MethodPost, "/batch_retry", strings.NewReader(`{"ids":[11,22]}`))
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected ok, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	if len(planService.offsetIDs) != 0 {
		t.Fatalf("expected batch retry not to use separate offset update, got %v", planService.offsetIDs)
	}

	if len(planService.resetIDs) != 0 {
		t.Fatalf("expected batch retry not to use separate counter reset, got %v", planService.resetIDs)
	}

	if len(planService.updatedIDs) != 0 || len(planService.updateReqs) != 0 || len(planService.updatedFields) != 0 {
		t.Fatalf("expected batch retry not to update state before queueing, ids=%v reqs=%d fields=%d",
			planService.updatedIDs, len(planService.updateReqs), len(planService.updatedFields))
	}

	if len(taskEngine.payloads) != 1 {
		t.Fatalf("expected 1 queued retry task, got %d", len(taskEngine.payloads))
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

	var response struct {
		Data map[string]int `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}

	if response.Data["success"] != 1 || response.Data["failed"] != 1 {
		t.Fatalf("unexpected response: %+v", response.Data)
	}
}

func int64SlicesEqual(a []int64, b []int64) bool {
	if len(a) != len(b) {
		return false
	}

	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}

	return true
}
