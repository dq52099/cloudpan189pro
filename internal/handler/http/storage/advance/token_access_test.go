package advance

import (
	stdctx "context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
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

type mockAdvanceCloudTokenService struct {
	cloudtokenSvi.Service
	token   *models.CloudToken
	err     error
	queries []int64
}

func (m *mockAdvanceCloudTokenService) QueryAccessible(ctx appContext.Context, id, userID int64, isAdmin bool) (*models.CloudToken, error) {
	m.queries = append(m.queries, id)

	if m.err != nil {
		return nil, m.err
	}

	return m.token, nil
}

func performAdvanceFamilyListRequest(t *testing.T, cloudTokenService cloudtokenSvi.Service) *httptest.ResponseRecorder {
	t.Helper()

	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(func(ctx *gin.Context) {
		ctx.Set(consts.CtxKeyUserId, int64(100))
		ctx.Set(consts.CtxKeyIsAdmin, false)
	})

	wrapper := httpcontext.NewHandlerFuncWrapper(zap.NewNop())
	router.GET("/family/list", wrapper.Wrap(NewHandler(nil, cloudTokenService).FamilyList()))

	req := httptest.NewRequestWithContext(stdctx.Background(), http.MethodGet, "/family/list?cloudToken=123", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	return recorder
}

func assertAdvanceHTTPError(t *testing.T, recorder *httptest.ResponseRecorder, expectedStatus int, expectedErr httpcontext.BusinessError) {
	t.Helper()

	if recorder.Code != expectedStatus {
		t.Fatalf("expected HTTP %d, got %d body=%s", expectedStatus, recorder.Code, recorder.Body.String())
	}

	var response httpcontext.Response
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if response.Code != expectedErr.GetCode() {
		t.Fatalf("expected business code %d, got %d", expectedErr.GetCode(), response.Code)
	}
}

func TestFamilyListReturnsNotFoundWhenCloudTokenMissing(t *testing.T) {
	recorder := performAdvanceFamilyListRequest(t, &mockAdvanceCloudTokenService{err: gorm.ErrRecordNotFound})

	assertAdvanceHTTPError(t, recorder, http.StatusNotFound, codeStorageAdvanceCloudTokenNotExist)
}

func TestFamilyListReturnsNotFoundWhenCloudTokenQueryReturnsNil(t *testing.T) {
	recorder := performAdvanceFamilyListRequest(t, &mockAdvanceCloudTokenService{})

	assertAdvanceHTTPError(t, recorder, http.StatusNotFound, codeStorageAdvanceCloudTokenNotExist)
}

func TestFamilyListReturnsQueryErrorWhenCloudTokenLookupFails(t *testing.T) {
	recorder := performAdvanceFamilyListRequest(t, &mockAdvanceCloudTokenService{err: errors.New("database unavailable")})

	assertAdvanceHTTPError(t, recorder, http.StatusBadRequest, codeStorageAdvanceQueryCloudTokenError)
}
