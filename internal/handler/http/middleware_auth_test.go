package http

import (
	stdctx "context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	appContext "github.com/xxcheng123/cloudpan189-share/internal/framework/context"
	"github.com/xxcheng123/cloudpan189-share/internal/framework/httpcontext"
	"github.com/xxcheng123/cloudpan189-share/internal/pkgs/utils"
	"github.com/xxcheng123/cloudpan189-share/internal/repository/models"
	userSvi "github.com/xxcheng123/cloudpan189-share/internal/services/user"
	"go.uber.org/zap"
)

type authMiddlewareUserServiceStub struct {
	userSvi.Service
	user *models.User
}

func (s *authMiddlewareUserServiceStub) ParseAccessToken(string) (int64, string, int, error) {
	return 7, "normal-user", 1, nil
}

func (s *authMiddlewareUserServiceStub) Query(appContext.Context, int64) (*models.User, error) {
	return s.user, nil
}

func TestAuthMiddlewareReturnsForbiddenForNonAdmin(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	wrapper := httpcontext.NewHandlerFuncWrapper(zap.NewNop())
	middleware := newAuthMiddleware(&authMiddlewareUserServiceStub{
		user: &models.User{
			ID:       7,
			Username: "normal-user",
			Version:  1,
			Status:   1,
			IsAdmin:  false,
		},
	})

	router.GET("/admin",
		wrapper.Wrap(middleware.Auth(true)),
		wrapper.Wrap(func(ctx *httpcontext.Context) {
			ctx.Success()
		}),
	)

	req := httptest.NewRequestWithContext(stdctx.Background(), http.MethodGet, "/admin", nil)
	req.Header.Set("Authorization", "Bearer token")

	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusForbidden, recorder.Code, recorder.Body.String())
	}

	var response httpcontext.Response
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if response.Code != http.StatusForbidden {
		t.Fatalf("expected business code %d, got %d", http.StatusForbidden, response.Code)
	}

	if response.Msg != errMessageRequireAdmin {
		t.Fatalf("expected message %q, got %q", errMessageRequireAdmin, response.Msg)
	}
}

func (s *authMiddlewareUserServiceStub) Add(appContext.Context, *userSvi.AddRequest, ...userSvi.AddOptionFunc) (*userSvi.AddResponse, error) {
	return nil, errors.New("not implemented")
}

func (s *authMiddlewareUserServiceStub) Del(appContext.Context, *userSvi.DelRequest) error {
	return errors.New("not implemented")
}

func (s *authMiddlewareUserServiceStub) QueryByUsername(appContext.Context, string) (*models.User, error) {
	return nil, errors.New("not implemented")
}

func (s *authMiddlewareUserServiceStub) Update(appContext.Context, int64, ...utils.Field) error {
	return errors.New("not implemented")
}

func (s *authMiddlewareUserServiceStub) List(appContext.Context, *userSvi.ListRequest) ([]*models.User, error) {
	return nil, errors.New("not implemented")
}

func (s *authMiddlewareUserServiceStub) Count(appContext.Context, *userSvi.ListRequest) (int64, error) {
	return 0, errors.New("not implemented")
}

func (s *authMiddlewareUserServiceStub) ModifyPass(appContext.Context, int64, string) error {
	return errors.New("not implemented")
}

func (s *authMiddlewareUserServiceStub) BindGroup(appContext.Context, *userSvi.BindGroupRequest) error {
	return errors.New("not implemented")
}

func (s *authMiddlewareUserServiceStub) GenerateAccessToken(int64, string, int) (string, error) {
	return "", errors.New("not implemented")
}

func (s *authMiddlewareUserServiceStub) GenerateRefreshToken(int64, string, int) (string, error) {
	return "", errors.New("not implemented")
}

func (s *authMiddlewareUserServiceStub) ParseRefreshToken(string) (int64, string, int, error) {
	return 0, "", 0, errors.New("not implemented")
}

func (s *authMiddlewareUserServiceStub) GetExpire() int64 {
	return 0
}
