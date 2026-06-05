package autoingest

import (
	stdctx "context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/xxcheng123/cloudpan189-share/internal/consts"
	appContext "github.com/xxcheng123/cloudpan189-share/internal/framework/context"
	"github.com/xxcheng123/cloudpan189-share/internal/framework/httpcontext"
	"github.com/xxcheng123/cloudpan189-share/internal/pkgs/taskengine"
	"github.com/xxcheng123/cloudpan189-share/internal/pkgs/utils"
	"github.com/xxcheng123/cloudpan189-share/internal/repository/models"
	autoingestplanSvi "github.com/xxcheng123/cloudpan189-share/internal/services/autoingestplan"
	cloudbridgeSvi "github.com/xxcheng123/cloudpan189-share/internal/services/cloudbridge"
	"github.com/xxcheng123/cloudpan189-share/internal/types/autoingest"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

var errCreateSubscribePlanQueueFailed = errors.New("queue unavailable")

type mockCreateSubscribePlanService struct {
	autoingestplanSvi.Service
	plan *models.AutoIngestPlan
	id   int64
}

func (m *mockCreateSubscribePlanService) Create(ctx appContext.Context, plan *models.AutoIngestPlan) (int64, error) {
	m.plan = plan

	return m.id, nil
}

type mockCreateSubscribeCloudBridgeService struct {
	cloudbridgeSvi.Service
	userIDs []string
}

func (m *mockCreateSubscribeCloudBridgeService) GetSubscribeUserInfo(ctx appContext.Context, userID string) (*cloudbridgeSvi.SubscribeUserInfo, error) {
	m.userIDs = append(m.userIDs, userID)

	return &cloudbridgeSvi.SubscribeUserInfo{UserId: userID, Name: "订阅号"}, nil
}

type mockCreateSubscribeTaskEngine struct {
	taskengine.TaskEngine
	err error
}

func (m *mockCreateSubscribeTaskEngine) PushMessage(ctx stdctx.Context, taskTopic taskengine.Topic, payload []byte) error {
	return m.err
}

func TestCreateSubscribePlanReportsHistoryQueueFailure(t *testing.T) {
	gin.SetMode(gin.TestMode)

	planService := &mockCreateSubscribePlanService{id: 42}
	router := gin.New()
	router.Use(func(ctx *gin.Context) {
		ctx.Set(consts.CtxKeyUserId, int64(100))
	})

	wrapper := httpcontext.NewHandlerFuncWrapper(zap.NewNop())
	router.POST("/create", wrapper.Wrap(NewHandler(
		&mockCreateSubscribeTaskEngine{err: errCreateSubscribePlanQueueFailed},
		planService,
		nil,
		&mockCreateSubscribeCloudBridgeService{},
	).CreateSubscribePlan()))

	req := httptest.NewRequestWithContext(
		stdctx.Background(),
		http.MethodPost,
		"/create",
		strings.NewReader(`{"name":"计划","autoIngestInterval":30,"parentPath":"/Movies","upUserId":"up-user","oneClickAddHistory":true}`),
	)
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected ok, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	if planService.plan == nil {
		t.Fatal("expected plan to be created")
	}

	var response struct {
		Code int                         `json:"code"`
		Data createSubscribePlanResponse `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}

	if response.Code != http.StatusOK {
		t.Fatalf("expected business code 200, got %d", response.Code)
	}

	if response.Data.ID != 42 {
		t.Fatalf("expected created plan id 42, got %d", response.Data.ID)
	}

	if response.Data.HistoryQueued {
		t.Fatalf("expected history queued false, got %+v", response.Data)
	}

	if !strings.Contains(response.Data.HistoryError, errCreateSubscribePlanQueueFailed.Error()) {
		t.Fatalf("expected history error to contain queue failure, got %q", response.Data.HistoryError)
	}

	var addition models.AutoIngestPlanSubscribeAddition
	if err := planService.plan.Addition.Unmarshal(&addition); err != nil {
		t.Fatalf("unmarshal addition: %v", err)
	}

	if addition.OffsetResourceID != "" {
		t.Fatalf("expected history plan to start without same-second cursor, got %q", addition.OffsetResourceID)
	}
}

func TestCreateSubscribePlanRedactsHistoryQueueFailure(t *testing.T) {
	gin.SetMode(gin.TestMode)

	rawErr := `queue failed: https://proxy-user:proxy-pass@example.test/scan?access_token=query-secret&filename=private-name.mkv#token=fragment-secret accessCode=abcd Authorization: Bearer secret-token`
	planService := &mockCreateSubscribePlanService{id: 42}
	core, logs := observer.New(zap.WarnLevel)
	logger := zap.New(core)
	router := gin.New()
	router.Use(func(ctx *gin.Context) {
		ctx.Set(consts.CtxKeyUserId, int64(100))
	})

	wrapper := httpcontext.NewHandlerFuncWrapper(logger)
	router.POST("/create", wrapper.Wrap(NewHandler(
		&mockCreateSubscribeTaskEngine{err: errors.New(rawErr)},
		planService,
		nil,
		&mockCreateSubscribeCloudBridgeService{},
	).CreateSubscribePlan()))

	req := httptest.NewRequestWithContext(
		stdctx.Background(),
		http.MethodPost,
		"/create",
		strings.NewReader(`{"name":"计划","autoIngestInterval":30,"parentPath":"/Movies","upUserId":"up-user","oneClickAddHistory":true}`),
	)
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected ok, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	var response struct {
		Code int                         `json:"code"`
		Data createSubscribePlanResponse `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}

	logText := ""
	for _, entry := range logs.All() {
		logText += entry.Message + fmt.Sprint(entry.Context)
	}

	for _, text := range []string{response.Data.HistoryError, logText} {
		for _, leaked := range []string{"proxy-user", "proxy-pass", "query-secret", "private-name.mkv", "fragment-secret", "abcd", "secret-token"} {
			if strings.Contains(text, leaked) {
				t.Fatalf("expected %q to be redacted from %s", leaked, text)
			}
		}

		if !strings.Contains(text, utils.RedactedSecret) {
			t.Fatalf("expected redacted marker in %s", text)
		}
	}
}

func TestCreateSubscribePlanDefaultsConflictPolicyToRename(t *testing.T) {
	gin.SetMode(gin.TestMode)

	planService := &mockCreateSubscribePlanService{id: 42}
	router := gin.New()
	router.Use(func(ctx *gin.Context) {
		ctx.Set(consts.CtxKeyUserId, int64(100))
	})

	wrapper := httpcontext.NewHandlerFuncWrapper(zap.NewNop())
	router.POST("/create", wrapper.Wrap(NewHandler(
		&mockCreateSubscribeTaskEngine{},
		planService,
		nil,
		&mockCreateSubscribeCloudBridgeService{},
	).CreateSubscribePlan()))

	req := httptest.NewRequestWithContext(
		stdctx.Background(),
		http.MethodPost,
		"/create",
		strings.NewReader(`{"name":"计划","autoIngestInterval":30,"parentPath":"/Movies","upUserId":"up-user"}`),
	)
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected ok, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	if planService.plan == nil {
		t.Fatal("expected plan to be created")
	}

	if planService.plan.OnConflict != autoingest.OnConflictRename {
		t.Fatalf("expected default conflict policy rename, got %q", planService.plan.OnConflict)
	}

	var addition models.AutoIngestPlanSubscribeAddition
	if err := planService.plan.Addition.Unmarshal(&addition); err != nil {
		t.Fatalf("unmarshal addition: %v", err)
	}

	if addition.OffsetResourceID != models.AutoIngestSubscribeOffsetResourceIDMax {
		t.Fatalf("expected non-history plan to skip current-second resources, got %q", addition.OffsetResourceID)
	}
}

func TestCreateSubscribePlanNormalizesParentPath(t *testing.T) {
	gin.SetMode(gin.TestMode)

	planService := &mockCreateSubscribePlanService{id: 42}
	router := gin.New()
	router.Use(func(ctx *gin.Context) {
		ctx.Set(consts.CtxKeyUserId, int64(100))
	})

	wrapper := httpcontext.NewHandlerFuncWrapper(zap.NewNop())
	router.POST("/create", wrapper.Wrap(NewHandler(
		&mockCreateSubscribeTaskEngine{},
		planService,
		nil,
		&mockCreateSubscribeCloudBridgeService{},
	).CreateSubscribePlan()))

	req := httptest.NewRequestWithContext(
		stdctx.Background(),
		http.MethodPost,
		"/create",
		strings.NewReader(`{"name":"计划","autoIngestInterval":30,"parentPath":"/media//%E4%B8%AD%E6%96%87%20/a%3ab","upUserId":"up-user"}`),
	)
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected ok, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	if planService.plan == nil {
		t.Fatal("expected plan to be created")
	}

	if planService.plan.ParentPath != "/media/中文/a_b" {
		t.Fatalf("expected normalized parent path, got %q", planService.plan.ParentPath)
	}
}

func TestCreateSubscribePlanRejectsInvalidParentPathBeforeLookup(t *testing.T) {
	gin.SetMode(gin.TestMode)

	planService := &mockCreateSubscribePlanService{id: 42}
	cloudBridge := &mockCreateSubscribeCloudBridgeService{}
	router := gin.New()
	router.Use(func(ctx *gin.Context) {
		ctx.Set(consts.CtxKeyUserId, int64(100))
	})

	wrapper := httpcontext.NewHandlerFuncWrapper(zap.NewNop())
	router.POST("/create", wrapper.Wrap(NewHandler(
		&mockCreateSubscribeTaskEngine{},
		planService,
		nil,
		cloudBridge,
	).CreateSubscribePlan()))

	req := httptest.NewRequestWithContext(
		stdctx.Background(),
		http.MethodPost,
		"/create",
		strings.NewReader(`{"name":"计划","autoIngestInterval":30,"parentPath":"/bad/%2F/path","upUserId":"up-user"}`),
	)
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected bad request, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	if len(cloudBridge.userIDs) != 0 {
		t.Fatalf("expected invalid parent path not to call cloud bridge, got %v", cloudBridge.userIDs)
	}

	if planService.plan != nil {
		t.Fatal("expected invalid request not to create plan")
	}
}

func TestCreateSubscribePlanNormalizesSubscribeUserLink(t *testing.T) {
	gin.SetMode(gin.TestMode)

	planService := &mockCreateSubscribePlanService{id: 42}
	cloudBridge := &mockCreateSubscribeCloudBridgeService{}
	router := gin.New()
	router.Use(func(ctx *gin.Context) {
		ctx.Set(consts.CtxKeyUserId, int64(100))
	})

	wrapper := httpcontext.NewHandlerFuncWrapper(zap.NewNop())
	router.POST("/create", wrapper.Wrap(NewHandler(
		&mockCreateSubscribeTaskEngine{},
		planService,
		nil,
		cloudBridge,
	).CreateSubscribePlan()))

	req := httptest.NewRequestWithContext(
		stdctx.Background(),
		http.MethodPost,
		"/create",
		strings.NewReader(`{"name":"计划","autoIngestInterval":30,"parentPath":"/Movies","upUserId":"https://content.21cn.com/h5/subscrip/?uuid=encoded%2Duser%5F9。"}`),
	)
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected ok, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	if len(cloudBridge.userIDs) != 1 || cloudBridge.userIDs[0] != "encoded-user_9" {
		t.Fatalf("expected normalized subscribe user lookup, got %v", cloudBridge.userIDs)
	}

	if planService.plan == nil {
		t.Fatal("expected plan to be created")
	}

	var addition models.AutoIngestPlanSubscribeAddition
	if err := planService.plan.Addition.Unmarshal(&addition); err != nil {
		t.Fatalf("unmarshal addition: %v", err)
	}

	if addition.UpUserId != "encoded-user_9" {
		t.Fatalf("expected normalized up user id, got %q", addition.UpUserId)
	}
}

func TestCreateSubscribePlanRejectsLookalikeSubscribeUserLinkBeforeLookup(t *testing.T) {
	gin.SetMode(gin.TestMode)

	planService := &mockCreateSubscribePlanService{id: 42}
	cloudBridge := &mockCreateSubscribeCloudBridgeService{}
	router := gin.New()
	router.Use(func(ctx *gin.Context) {
		ctx.Set(consts.CtxKeyUserId, int64(100))
	})

	wrapper := httpcontext.NewHandlerFuncWrapper(zap.NewNop())
	router.POST("/create", wrapper.Wrap(NewHandler(
		&mockCreateSubscribeTaskEngine{},
		planService,
		nil,
		cloudBridge,
	).CreateSubscribePlan()))

	req := httptest.NewRequestWithContext(
		stdctx.Background(),
		http.MethodPost,
		"/create",
		strings.NewReader(`{"name":"计划","autoIngestInterval":30,"parentPath":"/Movies","upUserId":"note https://content.21cn.com.evil.test/h5/subscrip/?uuid=bad-user"}`),
	)
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected bad request, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	if len(cloudBridge.userIDs) != 0 {
		t.Fatalf("expected subscribe lookup not to be called, got %v", cloudBridge.userIDs)
	}

	if planService.plan != nil {
		t.Fatal("expected invalid request not to create plan")
	}
}

func TestCreateSubscribePlanRejectsRefreshIntervalOverMax(t *testing.T) {
	gin.SetMode(gin.TestMode)

	planService := &mockCreateSubscribePlanService{id: 42}
	router := gin.New()
	router.Use(func(ctx *gin.Context) {
		ctx.Set(consts.CtxKeyUserId, int64(100))
	})

	wrapper := httpcontext.NewHandlerFuncWrapper(zap.NewNop())
	router.POST("/create", wrapper.Wrap(NewHandler(
		&mockCreateSubscribeTaskEngine{},
		planService,
		nil,
		&mockCreateSubscribeCloudBridgeService{},
	).CreateSubscribePlan()))

	req := httptest.NewRequestWithContext(
		stdctx.Background(),
		http.MethodPost,
		"/create",
		strings.NewReader(`{"name":"计划","autoIngestInterval":30,"parentPath":"/Movies","upUserId":"up-user","refreshStrategy":{"enableAutoRefresh":true,"refreshInterval":1441}}`),
	)
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected bad request, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	if planService.plan != nil {
		t.Fatal("expected invalid request not to create plan")
	}
}
