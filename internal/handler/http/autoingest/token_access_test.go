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
	"github.com/xxcheng123/cloudpan189-share/internal/repository/models"
	cloudtokenSvi "github.com/xxcheng123/cloudpan189-share/internal/services/cloudtoken"
	"github.com/xxcheng123/cloudpan189-share/internal/types/autoingest"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type mockAutoIngestCloudTokenService struct {
	cloudtokenSvi.Service
	queries []mockAutoIngestCloudTokenQuery
	err     error
}

type mockAutoIngestCloudTokenQuery struct {
	id      int64
	userID  int64
	isAdmin bool
}

func (m *mockAutoIngestCloudTokenService) QueryAccessible(
	ctx appContext.Context,
	id int64,
	userID int64,
	isAdmin bool,
) (*models.CloudToken, error) {
	m.queries = append(m.queries, mockAutoIngestCloudTokenQuery{
		id:      id,
		userID:  userID,
		isAdmin: isAdmin,
	})

	if m.err != nil {
		return nil, m.err
	}

	return &models.CloudToken{ID: id, UserID: userID}, nil
}

func assertCloudTokenNotFoundResponse(t *testing.T, recorder *httptest.ResponseRecorder) {
	t.Helper()

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("expected not found, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	var response httpcontext.Response
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if response.Code != codeCloudTokenNotFound.GetCode() {
		t.Fatalf("expected business code %d, got %d", codeCloudTokenNotFound.GetCode(), response.Code)
	}

	if response.Msg != codeCloudTokenNotFound.GetMessage() {
		t.Fatalf("expected message %q, got %q", codeCloudTokenNotFound.GetMessage(), response.Msg)
	}
}

func TestCreateSubscribePlanReturnsNotFoundWhenCloudTokenMissing(t *testing.T) {
	gin.SetMode(gin.TestMode)

	planService := &mockCreateSubscribePlanService{id: 42}
	cloudTokenService := &mockAutoIngestCloudTokenService{err: gorm.ErrRecordNotFound}
	router := gin.New()
	router.Use(func(ctx *gin.Context) {
		ctx.Set(consts.CtxKeyUserId, int64(100))
		ctx.Set(consts.CtxKeyIsAdmin, false)
	})

	wrapper := httpcontext.NewHandlerFuncWrapper(zap.NewNop())
	router.POST("/create", wrapper.Wrap(NewHandler(
		&mockCreateSubscribeTaskEngine{},
		planService,
		nil,
		&mockCreateSubscribeCloudBridgeService{},
		cloudTokenService,
	).CreateSubscribePlan()))

	req := httptest.NewRequestWithContext(
		stdctx.Background(),
		http.MethodPost,
		"/create",
		strings.NewReader(`{"name":"计划","autoIngestInterval":30,"parentPath":"/Movies","upUserId":"up-user","cloudToken":77}`),
	)
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	assertCloudTokenNotFoundResponse(t, recorder)

	if planService.plan != nil {
		t.Fatal("expected inaccessible token not to create plan")
	}

	if got, want := cloudTokenService.queries, []mockAutoIngestCloudTokenQuery{{id: 77, userID: 100, isAdmin: false}}; !cloudTokenQueriesEqual(got, want) {
		t.Fatalf("expected cloud token access query %+v, got %+v", want, got)
	}
}

func TestUpdatePlanReturnsNotFoundWhenCloudTokenMissing(t *testing.T) {
	gin.SetMode(gin.TestMode)

	planService := &mockPlanAccessService{
		plans: map[int64]*models.AutoIngestPlan{
			11: {ID: 11, UserID: 100, SourceType: autoingest.SourceTypeSubscribe},
		},
	}
	cloudTokenService := &mockAutoIngestCloudTokenService{err: gorm.ErrRecordNotFound}
	router := newPlanAccessRouter(NewHandler(nil, planService, nil, nil, cloudTokenService).UpdatePlan())

	req := httptest.NewRequestWithContext(stdctx.Background(), http.MethodPost, "/action", strings.NewReader(`{"id":11,"tokenId":77}`))
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	assertCloudTokenNotFoundResponse(t, recorder)

	if len(planService.updatedIDs) != 0 {
		t.Fatalf("expected update not to be called, got %v", planService.updatedIDs)
	}

	if got, want := cloudTokenService.queries, []mockAutoIngestCloudTokenQuery{{id: 77, userID: 100, isAdmin: false}}; !cloudTokenQueriesEqual(got, want) {
		t.Fatalf("expected cloud token access query %+v, got %+v", want, got)
	}
}

func TestUpdatePlanAllowsUnbindingCloudTokenWithoutQuery(t *testing.T) {
	gin.SetMode(gin.TestMode)

	planService := &mockPlanAccessService{
		plans: map[int64]*models.AutoIngestPlan{
			11: {ID: 11, UserID: 100, SourceType: autoingest.SourceTypeSubscribe, TokenId: 77},
		},
	}
	cloudTokenService := &mockAutoIngestCloudTokenService{err: errors.New("must not query token")}
	router := newPlanAccessRouter(NewHandler(nil, planService, nil, nil, cloudTokenService).UpdatePlan())

	req := httptest.NewRequestWithContext(stdctx.Background(), http.MethodPost, "/action", strings.NewReader(`{"id":11,"tokenId":0}`))
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected ok, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	if len(cloudTokenService.queries) != 0 {
		t.Fatalf("expected token unbind not to query cloud token, got %+v", cloudTokenService.queries)
	}

	if got, want := planService.updatedIDs, []int64{11}; !int64SlicesEqual(got, want) {
		t.Fatalf("expected update ids %v, got %v", want, got)
	}

	fields := fieldsToMap(planService.updatedFields[0])
	if fields["token_id"] != int64(0) {
		t.Fatalf("expected token_id field to be 0, got %+v", fields)
	}
}

func cloudTokenQueriesEqual(a, b []mockAutoIngestCloudTokenQuery) bool {
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
