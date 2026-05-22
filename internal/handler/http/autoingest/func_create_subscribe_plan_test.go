package autoingest

import (
	stdctx "context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/xxcheng123/cloudpan189-share/internal/consts"
	appContext "github.com/xxcheng123/cloudpan189-share/internal/framework/context"
	"github.com/xxcheng123/cloudpan189-share/internal/framework/httpcontext"
	"github.com/xxcheng123/cloudpan189-share/internal/pkgs/taskengine"
	"github.com/xxcheng123/cloudpan189-share/internal/repository/models"
	autoingestplanSvi "github.com/xxcheng123/cloudpan189-share/internal/services/autoingestplan"
	cloudbridgeSvi "github.com/xxcheng123/cloudpan189-share/internal/services/cloudbridge"
	"go.uber.org/zap"
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
}

func (m *mockCreateSubscribeCloudBridgeService) GetSubscribeUserInfo(ctx appContext.Context, userID string) (*cloudbridgeSvi.SubscribeUserInfo, error) {
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
