package cloudtoken

import (
	stdctx "context"
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
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type mockUsernameLoginCloudTokenService struct {
	cloudtokenSvi.Service
	token               *models.CloudToken
	queryErr            error
	queryCalled         bool
	usernameLoginErr    error
	usernameLoginCalled bool
}

func (m *mockUsernameLoginCloudTokenService) QueryAccessible(
	ctx appContext.Context,
	id, userID int64,
	isAdmin bool,
) (*models.CloudToken, error) {
	m.queryCalled = true

	return m.token, m.queryErr
}

func (m *mockUsernameLoginCloudTokenService) UsernameLogin(
	ctx appContext.Context,
	req *cloudtokenSvi.UsernameLoginRequest,
) (*cloudtokenSvi.UsernameLoginResponse, error) {
	m.usernameLoginCalled = true
	if m.usernameLoginErr != nil {
		return nil, m.usernameLoginErr
	}

	return &cloudtokenSvi.UsernameLoginResponse{ID: req.ID}, nil
}

func newUsernameLoginRouter(cloudTokenService cloudtokenSvi.Service) *gin.Engine {
	router := gin.New()
	router.Use(func(ctx *gin.Context) {
		ctx.Set(consts.CtxKeyUserId, int64(88))
		ctx.Set(consts.CtxKeyIsAdmin, false)
	})

	wrapper := httpcontext.NewHandlerFuncWrapper(zap.NewNop())
	router.POST("/username_login", wrapper.Wrap(NewHandler(cloudTokenService, nil, nil).UsernameLogin()))

	return router
}

func performUsernameLoginRequest(t *testing.T, svc cloudtokenSvi.Service, body string) *httptest.ResponseRecorder {
	t.Helper()

	gin.SetMode(gin.TestMode)

	req := httptest.NewRequestWithContext(
		stdctx.Background(),
		http.MethodPost,
		"/username_login",
		strings.NewReader(body),
	)
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	newUsernameLoginRouter(svc).ServeHTTP(recorder, req)

	return recorder
}

func TestUsernameLoginReturnsNotFoundWhenStoredCredentialsTokenMissing(t *testing.T) {
	cloudTokenService := &mockUsernameLoginCloudTokenService{
		queryErr: errors.Join(errors.New("missing token"), gorm.ErrRecordNotFound),
	}

	recorder := performUsernameLoginRequest(t, cloudTokenService, `{"id":123}`)

	assertCloudTokenNotFoundResponse(t, recorder)

	if cloudTokenService.usernameLoginCalled {
		t.Fatal("expected username login service not to be called after missing token pre-query")
	}
}

func TestUsernameLoginRejectsInvalidIDBeforeServiceCall(t *testing.T) {
	cloudTokenService := &mockUsernameLoginCloudTokenService{}

	recorder := performUsernameLoginRequest(
		t,
		cloudTokenService,
		`{"id":-1,"username":"user","password":"pass"}`,
	)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected bad request, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	if cloudTokenService.queryCalled {
		t.Fatal("expected invalid id not to query existing token")
	}

	if cloudTokenService.usernameLoginCalled {
		t.Fatal("expected invalid id not to call username login service")
	}
}

func TestUsernameLoginReturnsNotFoundWhenUpdateTokenMissing(t *testing.T) {
	cloudTokenService := &mockUsernameLoginCloudTokenService{
		usernameLoginErr: errors.Join(errors.New("missing token"), gorm.ErrRecordNotFound),
	}

	recorder := performUsernameLoginRequest(
		t,
		cloudTokenService,
		`{"id":123,"username":"user","password":"pass"}`,
	)

	assertCloudTokenNotFoundResponse(t, recorder)
}
