package autoingest

import (
	stdctx "context"
	"encoding/json"
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
	deletedReqs   []*autoingestplanSvi.DeleteRequest
	listByIDsReqs [][]int64
	offsetIDs     []int64
	resetIDs      []int64
	plans         map[int64]*models.AutoIngestPlan
}

func (m *mockBatchAutoIngestPlanService) Enable(ctx appContext.Context, id int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.enabledIDs = append(m.enabledIDs, id)

	return nil
}

func (m *mockBatchAutoIngestPlanService) Disable(ctx appContext.Context, id int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.disabledIDs = append(m.disabledIDs, id)

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
}

func (m *mockBatchAutoIngestTaskEngine) PushMessage(ctx stdctx.Context, taskTopic taskengine.Topic, payload []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.payloads = append(m.payloads, payload)

	return nil
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

func TestBatchDeleteDeduplicatesIDsAndKeepsUserContext(t *testing.T) {
	gin.SetMode(gin.TestMode)

	planService := &mockBatchAutoIngestPlanService{}
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

	gotOffsetIDs := append([]int64(nil), planService.offsetIDs...)
	sort.Slice(gotOffsetIDs, func(i, j int) bool { return gotOffsetIDs[i] < gotOffsetIDs[j] })

	if want := []int64{11, 22}; !int64SlicesEqual(gotOffsetIDs, want) {
		t.Fatalf("expected offset updates %v, got %v", want, gotOffsetIDs)
	}

	if len(taskEngine.payloads) != 2 {
		t.Fatalf("expected 2 queued retry tasks, got %d", len(taskEngine.payloads))
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

	if got, want := planService.offsetIDs, []int64{11}; !int64SlicesEqual(got, want) {
		t.Fatalf("expected offset updates %v, got %v", want, got)
	}

	if got, want := planService.resetIDs, []int64{11}; !int64SlicesEqual(got, want) {
		t.Fatalf("expected reset counter IDs %v, got %v", want, got)
	}

	if len(taskEngine.payloads) != 1 {
		t.Fatalf("expected 1 queued retry task, got %d", len(taskEngine.payloads))
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
