package cloudtoken

import (
	stdctx "context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/xxcheng123/cloudpan189-share/internal/consts"
	appContext "github.com/xxcheng123/cloudpan189-share/internal/framework/context"
	"github.com/xxcheng123/cloudpan189-share/internal/framework/httpcontext"
	"github.com/xxcheng123/cloudpan189-share/internal/repository/models"
	cloudtokenSvi "github.com/xxcheng123/cloudpan189-share/internal/services/cloudtoken"
	"go.uber.org/zap"
)

type mockListCloudTokenService struct {
	cloudtokenSvi.Service
	tokens []*models.CloudToken
	count  int64
}

func (m *mockListCloudTokenService) List(ctx appContext.Context, req *cloudtokenSvi.ListRequest) ([]*models.CloudToken, error) {
	return m.tokens, nil
}

func (m *mockListCloudTokenService) Count(ctx appContext.Context, req *cloudtokenSvi.ListRequest) (int64, error) {
	return m.count, nil
}

func TestListReturnsRemainingExpiresIn(t *testing.T) {
	gin.SetMode(gin.TestMode)

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
		UserID: 10,
	}
	token.SetIssuedAt(issuedAt)

	router := gin.New()
	router.Use(func(ctx *gin.Context) {
		ctx.Set(consts.CtxKeyUserId, int64(10))
		ctx.Set(consts.CtxKeyIsAdmin, false)
	})

	wrapper := httpcontext.NewHandlerFuncWrapper(zap.NewNop())
	router.GET("/cloud_token/list", wrapper.Wrap(NewHandler(&mockListCloudTokenService{
		tokens: []*models.CloudToken{token},
		count:  1,
	}, nil, nil).List()))

	req := httptest.NewRequestWithContext(stdctx.Background(), http.MethodGet, "/cloud_token/list", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected ok, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	var response struct {
		Code int `json:"code"`
		Data struct {
			Total int64 `json:"total"`
			Data  []struct {
				ID          int64                  `json:"id"`
				AccessToken *string                `json:"accessToken"`
				ExpiresIn   int64                  `json:"expiresIn"`
				Addition    map[string]interface{} `json:"addition"`
			} `json:"data"`
		} `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if response.Code != http.StatusOK {
		t.Fatalf("expected business code 200, got %d", response.Code)
	}

	if response.Data.Total != 1 || len(response.Data.Data) != 1 {
		t.Fatalf("expected one token, got total=%d len=%d", response.Data.Total, len(response.Data.Data))
	}

	got := response.Data.Data[0].ExpiresIn
	if got <= 0 || got >= token.ExpiresIn {
		t.Fatalf("expected remaining expiresIn smaller than raw ttl %d, got %d", token.ExpiresIn, got)
	}

	if response.Data.Data[0].AccessToken != nil {
		t.Fatal("expected list response to omit accessToken")
	}

	if _, ok := response.Data.Data[0].Addition["access_token"]; ok {
		t.Fatal("expected list response addition to omit access_token")
	}

	if response.Data.Data[0].Addition["safe"] != "value" {
		t.Fatalf("expected safe addition key to be preserved, got %v", response.Data.Data[0].Addition["safe"])
	}
}

func TestListSkipsNilCloudTokenRows(t *testing.T) {
	gin.SetMode(gin.TestMode)

	token := &models.CloudToken{
		ID:        12,
		Name:      "valid-token",
		ExpiresIn: 3600,
		UserID:    10,
		CreatedAt: time.Now(),
	}

	router := gin.New()
	router.Use(func(ctx *gin.Context) {
		ctx.Set(consts.CtxKeyUserId, int64(10))
		ctx.Set(consts.CtxKeyIsAdmin, false)
	})

	wrapper := httpcontext.NewHandlerFuncWrapper(zap.NewNop())
	router.GET("/cloud_token/list", wrapper.Wrap(NewHandler(&mockListCloudTokenService{
		tokens: []*models.CloudToken{nil, token},
		count:  2,
	}, nil, nil).List()))

	req := httptest.NewRequestWithContext(stdctx.Background(), http.MethodGet, "/cloud_token/list?noPaginate=true", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected ok, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	var response struct {
		Code int `json:"code"`
		Data struct {
			Total int64 `json:"total"`
			Data  []struct {
				ID   int64  `json:"id"`
				Name string `json:"name"`
			} `json:"data"`
		} `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if response.Data.Total != 1 {
		t.Fatalf("expected total to count non-nil rows, got %d", response.Data.Total)
	}

	if len(response.Data.Data) != 1 {
		t.Fatalf("expected one non-nil token, got %d", len(response.Data.Data))
	}

	if response.Data.Data[0].ID != token.ID || response.Data.Data[0].Name != token.Name {
		t.Fatalf("expected valid token response, got %#v", response.Data.Data[0])
	}
}
