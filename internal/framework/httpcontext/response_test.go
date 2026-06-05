package httpcontext

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/xxcheng123/cloudpan189-share/internal/pkgs/utils"
	"go.uber.org/zap"
)

func TestFailRedactsSensitiveErrorMessageInResponse(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	wrapper := NewHandlerFuncWrapper(zap.NewNop())
	router.GET("/fail", wrapper.Wrap(func(ctx *Context) {
		message := "upstream failed https://proxy-user:proxy-pass@example.test/api/search?accessToken=secret-access&kw=private-movie#refreshToken=fragment-secret accessCode=abcd Authorization: Bearer bearer-secret"
		ctx.Fail(NewBusinessGenerator(7000).Next(message))
	}))

	request := httptest.NewRequest(http.MethodGet, "/fail", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected bad request, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	body := recorder.Body.String()
	for _, leaked := range []string{
		"proxy-user",
		"proxy-pass",
		"secret-access",
		"private-movie",
		"fragment-secret",
		"abcd",
		"bearer-secret",
	} {
		if strings.Contains(body, leaked) {
			t.Fatalf("expected %q to be redacted from %s", leaked, body)
		}
	}

	if !strings.Contains(body, utils.RedactedSecret) {
		t.Fatalf("expected redacted marker in response %s", body)
	}
}
