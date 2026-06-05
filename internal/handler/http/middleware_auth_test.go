package http

import (
	stdctx "context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/xxcheng123/cloudpan189-share/internal/consts"
	appContext "github.com/xxcheng123/cloudpan189-share/internal/framework/context"
	"github.com/xxcheng123/cloudpan189-share/internal/framework/httpcontext"
	"github.com/xxcheng123/cloudpan189-share/internal/pkgs/utils"
	"github.com/xxcheng123/cloudpan189-share/internal/repository/models"
	userSvi "github.com/xxcheng123/cloudpan189-share/internal/services/user"
	"github.com/xxcheng123/cloudpan189-share/internal/shared"
)

type authMiddlewareUserServiceStub struct {
	userSvi.Service
	user                  *models.User
	returnNilUser         bool
	queryErr              error
	parseAccessTokenCalls int
	parsedAccessToken     string
	parseAccessTokenErr   error
	queryCalls            int
}

func (s *authMiddlewareUserServiceStub) ParseAccessToken(token string) (int64, string, int, error) {
	s.parseAccessTokenCalls++

	s.parsedAccessToken = token
	if s.parseAccessTokenErr != nil {
		return 0, "", 0, s.parseAccessTokenErr
	}

	return 7, "normal-user", 1, nil
}

func (s *authMiddlewareUserServiceStub) Query(appContext.Context, int64) (*models.User, error) {
	s.queryCalls++
	if s.queryErr != nil {
		return nil, s.queryErr
	}

	if s.returnNilUser {
		return nil, nil
	}

	if s.user == nil {
		return &models.User{
			ID:       7,
			Username: "normal-user",
			Version:  1,
			Status:   1,
			IsAdmin:  true,
		}, nil
	}

	return s.user, nil
}

func TestAuthMiddlewareReturnsUnauthorizedWhenUserQueryReturnsNil(t *testing.T) {
	setAuthEnabledForTest(t, true)

	stub := &authMiddlewareUserServiceStub{returnNilUser: true}
	router := newAuthMiddlewareTestRouter(stub, false, func(ctx *httpcontext.Context) {
		ctx.Success()
	})

	req := httptest.NewRequestWithContext(stdctx.Background(), http.MethodGet, "/auth", nil)
	req.Header.Set("Authorization", "Bearer token")

	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusUnauthorized, recorder.Code, recorder.Body.String())
	}

	var response httpcontext.Response
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if response.Msg != errMessageUserInfoQueryErr {
		t.Fatalf("expected message %q, got %q", errMessageUserInfoQueryErr, response.Msg)
	}

	if stub.parseAccessTokenCalls != 1 || stub.queryCalls != 1 {
		t.Fatalf("expected parse/query once, got parse=%d query=%d", stub.parseAccessTokenCalls, stub.queryCalls)
	}
}

func setAuthEnabledForTest(t *testing.T, enabled bool) {
	t.Helper()

	oldEnableAuth := shared.EnableAuth
	shared.EnableAuth = enabled

	t.Cleanup(func() {
		shared.EnableAuth = oldEnableAuth
	})
}

func newAuthMiddlewareTestRouter(
	stub *authMiddlewareUserServiceStub,
	requireAdmin bool,
	next httpcontext.HandlerFunc,
) *gin.Engine {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	wrapper := httpcontext.NewHandlerFuncWrapper(zap.NewNop())
	middleware := newAuthMiddleware(stub)

	router.GET("/auth",
		wrapper.Wrap(middleware.Auth(requireAdmin)),
		wrapper.Wrap(next),
	)

	return router
}

func TestAuthMiddlewareReturnsForbiddenForNonAdmin(t *testing.T) {
	setAuthEnabledForTest(t, true)

	router := newAuthMiddlewareTestRouter(&authMiddlewareUserServiceStub{
		user: &models.User{
			ID:       7,
			Username: "normal-user",
			Version:  1,
			Status:   1,
			IsAdmin:  false,
		},
	}, true, func(ctx *httpcontext.Context) {
		ctx.Success()
	})

	req := httptest.NewRequestWithContext(stdctx.Background(), http.MethodGet, "/auth", nil)
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

func TestAuthMiddlewareReturnsUnauthorizedWhenTokenParseFails(t *testing.T) {
	setAuthEnabledForTest(t, true)

	stub := &authMiddlewareUserServiceStub{
		parseAccessTokenErr: errors.New("bad token"),
	}
	router := newAuthMiddlewareTestRouter(stub, false, func(ctx *httpcontext.Context) {
		ctx.Success()
	})

	req := httptest.NewRequestWithContext(stdctx.Background(), http.MethodGet, "/auth", nil)
	req.Header.Set("Authorization", "Bearer token")

	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusUnauthorized, recorder.Code, recorder.Body.String())
	}

	var response httpcontext.Response
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if response.Msg != errMessageTokenParseErr {
		t.Fatalf("expected message %q, got %q", errMessageTokenParseErr, response.Msg)
	}

	if stub.parseAccessTokenCalls != 1 {
		t.Fatalf("expected ParseAccessToken to be called once, got %d calls", stub.parseAccessTokenCalls)
	}

	if stub.queryCalls != 0 {
		t.Fatalf("expected Query not to be called, got %d calls", stub.queryCalls)
	}
}

func TestAuthMiddlewareRejectsMalformedAuthorizationHeader(t *testing.T) {
	setAuthEnabledForTest(t, true)

	tests := []struct {
		name      string
		header    string
		setHeader bool
		wantMsg   string
	}{
		{
			name:    "missing header",
			wantMsg: errMessageMissTokenHeader,
		},
		{
			name:      "blank header",
			header:    "   ",
			setHeader: true,
			wantMsg:   errMessageTokenHeaderFormatError,
		},
		{
			name:      "missing token",
			header:    "Bearer",
			setHeader: true,
			wantMsg:   errMessageTokenHeaderFormatError,
		},
		{
			name:      "blank bearer token",
			header:    "Bearer    ",
			setHeader: true,
			wantMsg:   errMessageTokenHeaderFormatError,
		},
		{
			name:      "unsupported scheme",
			header:    "Basic token",
			setHeader: true,
			wantMsg:   errMessageTokenHeaderFormatError,
		},
		{
			name:      "extra token parts",
			header:    "Bearer token extra",
			setHeader: true,
			wantMsg:   errMessageTokenHeaderFormatError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stub := &authMiddlewareUserServiceStub{}
			router := newAuthMiddlewareTestRouter(stub, false, func(ctx *httpcontext.Context) {
				ctx.Success()
			})

			req := httptest.NewRequestWithContext(stdctx.Background(), http.MethodGet, "/auth", nil)
			if tt.setHeader {
				req.Header.Set("Authorization", tt.header)
			}

			recorder := httptest.NewRecorder()

			router.ServeHTTP(recorder, req)

			if recorder.Code != http.StatusUnauthorized {
				t.Fatalf("expected status %d, got %d body=%s", http.StatusUnauthorized, recorder.Code, recorder.Body.String())
			}

			var response httpcontext.Response
			if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
				t.Fatalf("decode response: %v", err)
			}

			if response.Msg != tt.wantMsg {
				t.Fatalf("expected message %q, got %q", tt.wantMsg, response.Msg)
			}

			if stub.parseAccessTokenCalls != 0 {
				t.Fatalf("expected ParseAccessToken not to be called, got %d calls", stub.parseAccessTokenCalls)
			}

			if stub.queryCalls != 0 {
				t.Fatalf("expected Query not to be called, got %d calls", stub.queryCalls)
			}
		})
	}
}

func TestAuthMiddlewareAcceptsCaseInsensitiveBearerWithExtraWhitespace(t *testing.T) {
	setAuthEnabledForTest(t, true)

	stub := &authMiddlewareUserServiceStub{}
	router := newAuthMiddlewareTestRouter(stub, false, func(ctx *httpcontext.Context) {
		if ctx.GetInt64(consts.CtxKeyUserId) != 7 {
			t.Fatalf("expected user id 7, got %d", ctx.GetInt64(consts.CtxKeyUserId))
		}

		if ctx.GetString(consts.CtxKeyUsername) != "normal-user" {
			t.Fatalf("expected username normal-user, got %q", ctx.GetString(consts.CtxKeyUsername))
		}

		ctx.Success()
	})

	req := httptest.NewRequestWithContext(stdctx.Background(), http.MethodGet, "/auth", nil)
	req.Header.Set("Authorization", "bearer    token-value")

	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusOK, recorder.Code, recorder.Body.String())
	}

	if stub.parseAccessTokenCalls != 1 {
		t.Fatalf("expected ParseAccessToken to be called once, got %d calls", stub.parseAccessTokenCalls)
	}

	if stub.parsedAccessToken != "token-value" {
		t.Fatalf("expected parsed token %q, got %q", "token-value", stub.parsedAccessToken)
	}

	if stub.queryCalls != 1 {
		t.Fatalf("expected Query to be called once, got %d calls", stub.queryCalls)
	}
}

func TestAuthMiddlewareBypassesTokenWhenAuthDisabled(t *testing.T) {
	setAuthEnabledForTest(t, false)

	stub := &authMiddlewareUserServiceStub{}
	router := newAuthMiddlewareTestRouter(stub, true, func(ctx *httpcontext.Context) {
		if ctx.GetInt64(consts.CtxKeyUserId) != 0 {
			t.Fatalf("expected anonymous user id 0, got %d", ctx.GetInt64(consts.CtxKeyUserId))
		}

		if ctx.GetString(consts.CtxKeyUsername) != "anonymous" {
			t.Fatalf("expected anonymous username, got %q", ctx.GetString(consts.CtxKeyUsername))
		}

		if !ctx.GetBool(consts.CtxKeyIsAdmin) {
			t.Fatal("expected disabled auth request to be treated as admin")
		}

		ctx.Success()
	})

	req := httptest.NewRequestWithContext(stdctx.Background(), http.MethodGet, "/auth", nil)
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusOK, recorder.Code, recorder.Body.String())
	}

	if stub.parseAccessTokenCalls != 0 {
		t.Fatalf("expected ParseAccessToken not to be called, got %d calls", stub.parseAccessTokenCalls)
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
