package cloudtoken

import (
	stdctx "context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

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

func TestQueryReturnsNotFoundWhenCloudTokenQueryReturnsNil(t *testing.T) {
	recorder := performCloudTokenQueryRequest(t, &mockQueryCloudTokenService{}, "/cloud_token/99999")

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("expected not found, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	var response httpcontext.Response
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if response.Code != codeTokenNotFound.GetCode() {
		t.Fatalf("expected business code %d, got %d", codeTokenNotFound.GetCode(), response.Code)
	}
}

func TestQueryReturnsCloudToken(t *testing.T) {
	issuedAt := time.Now().Add(-30 * time.Minute)
	token := &models.CloudToken{
		ID:          11,
		Name:        "token",
		AccessToken: "secret-access-token",
		ExpiresIn:   7200,
		Addition: map[string]interface{}{
			"access_token":                         "secret-addition-token",
			"safe":                                 "value",
			models.CloudTokenAdditionTokenIssuedAt: issuedAt.UnixMilli(),
		},
	}
	token.SetIssuedAt(issuedAt)

	recorder := performCloudTokenQueryRequest(t, &mockQueryCloudTokenService{
		token: token,
	}, "/cloud_token/11")

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected ok, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	var response struct {
		Code int `json:"code"`
		Data struct {
			ID          int64                  `json:"id"`
			AccessToken *string                `json:"accessToken"`
			ExpiresIn   int64                  `json:"expiresIn"`
			Addition    map[string]interface{} `json:"addition"`
		} `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if response.Code != http.StatusOK {
		t.Fatalf("expected business code 200, got %d", response.Code)
	}

	if response.Data.ID != token.ID {
		t.Fatalf("expected token id %d, got %d", token.ID, response.Data.ID)
	}

	if response.Data.ExpiresIn <= 0 || response.Data.ExpiresIn >= token.ExpiresIn {
		t.Fatalf("expected remaining expiresIn smaller than raw ttl %d, got %d", token.ExpiresIn, response.Data.ExpiresIn)
	}

	if response.Data.AccessToken != nil {
		t.Fatal("expected query response to omit accessToken")
	}

	if _, ok := response.Data.Addition["access_token"]; ok {
		t.Fatal("expected query response addition to omit access_token")
	}

	if response.Data.Addition["safe"] != "value" {
		t.Fatalf("expected safe addition key to be preserved, got %v", response.Data.Addition["safe"])
	}
}
