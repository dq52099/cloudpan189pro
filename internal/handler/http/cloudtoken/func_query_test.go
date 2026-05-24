package cloudtoken

import (
	stdctx "context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	appContext "github.com/xxcheng123/cloudpan189-share/internal/framework/context"
	"github.com/xxcheng123/cloudpan189-share/internal/framework/httpcontext"
	"github.com/xxcheng123/cloudpan189-share/internal/repository/models"
	cloudtokenSvi "github.com/xxcheng123/cloudpan189-share/internal/services/cloudtoken"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type mockQueryCloudTokenService struct {
	cloudtokenSvi.Service
	token *models.CloudToken
	err   error
}

func (m *mockQueryCloudTokenService) Query(ctx appContext.Context, id int64) (*models.CloudToken, error) {
	return m.token, m.err
}

func (m *mockQueryCloudTokenService) QueryAccessible(ctx appContext.Context, id, userID int64, isAdmin bool) (*models.CloudToken, error) {
	return m.token, m.err
}

func performCloudTokenQueryRequest(t *testing.T, svc cloudtokenSvi.Service, path string) *httptest.ResponseRecorder {
	t.Helper()

	gin.SetMode(gin.TestMode)

	router := gin.New()
	wrapper := httpcontext.NewHandlerFuncWrapper(zap.NewNop())
	router.GET("/cloud_token/:id", wrapper.Wrap(NewHandler(svc, nil, nil).Query()))

	req := httptest.NewRequestWithContext(stdctx.Background(), http.MethodGet, path, nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	return recorder
}

func TestQueryReturnsNotFoundWhenCloudTokenMissing(t *testing.T) {
	recorder := performCloudTokenQueryRequest(t, &mockQueryCloudTokenService{err: gorm.ErrRecordNotFound}, "/cloud_token/99999")

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("expected not found, got %d body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestQueryReturnsCloudToken(t *testing.T) {
	recorder := performCloudTokenQueryRequest(t, &mockQueryCloudTokenService{
		token: &models.CloudToken{ID: 11, Name: "token"},
	}, "/cloud_token/11")

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected ok, got %d body=%s", recorder.Code, recorder.Body.String())
	}
}
