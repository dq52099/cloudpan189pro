package dav

import (
	stdctx "context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/xxcheng123/cloudpan189-share/internal/consts"
	"github.com/xxcheng123/cloudpan189-share/internal/framework/httpcontext"
	"github.com/xxcheng123/cloudpan189-share/internal/shared"
	"go.uber.org/zap"
)

func TestAuthMiddlewareBypassesBasicAuthWhenAuthDisabled(t *testing.T) {
	oldEnableAuth := shared.EnableAuth
	defer func() {
		shared.EnableAuth = oldEnableAuth
	}()

	shared.EnableAuth = false

	gin.SetMode(gin.TestMode)

	router := gin.New()
	wrapper := httpcontext.NewHandlerFuncWrapper(zap.NewNop())
	middleware := newAuthMiddleware(nil)

	router.GET("/dav",
		wrapper.Wrap(middleware.Auth()),
		wrapper.Wrap(func(ctx *httpcontext.Context) {
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
		}),
	)

	req := httptest.NewRequestWithContext(stdctx.Background(), http.MethodGet, "/dav", nil)
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusOK, recorder.Code, recorder.Body.String())
	}
}
