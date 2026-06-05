package context

import (
	stdcontext "context"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/xxcheng123/cloudpan189-share/internal/pkgs/utils"
)

func TestSanitizeHeadersRedactsSensitiveHeadersAndValues(t *testing.T) {
	ctx := NewContext(stdcontext.Background())
	headers := http.Header{
		"Accept":        []string{"application/json"},
		"Authorization": []string{"Bearer secret-access"},
		"Referer":       []string{"https://proxy-user:proxy-pass@example.test/api/search?keyword=private-movie&filename=secret.mkv#access_token=secret-fragment"},
		"Set-Cookie":    []string{"sid=secret-session"},
		"X-Custom":      []string{"refreshToken=secret-refresh"},
	}

	got := ctx.sanitizeHeaders(headers, DefaultHTTPLogConfig().SensitiveHeaders)

	if got["Authorization"][0] != utils.RedactedSecret {
		t.Fatalf("expected Authorization header to be redacted, got %#v", got["Authorization"])
	}

	if got["Set-Cookie"][0] != utils.RedactedSecret {
		t.Fatalf("expected Set-Cookie header to be redacted, got %#v", got["Set-Cookie"])
	}

	if got["Accept"][0] != "application/json" {
		t.Fatalf("expected Accept header to remain, got %#v", got["Accept"])
	}

	if strings.Contains(got["X-Custom"][0], "secret-refresh") {
		t.Fatalf("expected embedded sensitive value to be redacted, got %#v", got["X-Custom"])
	}

	for _, leaked := range []string{"proxy-user", "proxy-pass", "private-movie", "secret.mkv", "secret-fragment"} {
		if strings.Contains(got["Referer"][0], leaked) {
			t.Fatalf("expected %q to be redacted from %#v", leaked, got["Referer"])
		}
	}

	if !strings.Contains(got["Referer"][0], "/api/search") {
		t.Fatalf("expected referer path to remain, got %#v", got["Referer"])
	}

	if !strings.Contains(got["Referer"][0], utils.RedactedSecret) {
		t.Fatalf("expected redacted marker in referer, got %#v", got["Referer"])
	}
}

func TestFormatRequestBodyRedactsSensitiveJSON(t *testing.T) {
	ctx := NewContext(stdcontext.Background())

	got := ctx.formatRequestBody(`{"username":"admin","password":"secret-password","accessToken":"secret-access"}`, 0)
	for _, leaked := range []string{"secret-password", "secret-access"} {
		if strings.Contains(got, leaked) {
			t.Fatalf("expected %q to be redacted from request body %s", leaked, got)
		}
	}

	if !strings.Contains(got, `"username":"admin"`) {
		t.Fatalf("expected non-sensitive request body field to remain, got %s", got)
	}
}

func TestFormatRequestBodyRedactsURLValuesInJSON(t *testing.T) {
	ctx := NewContext(stdcontext.Background())

	got := ctx.formatRequestBody(`{"callback":"https://proxy-user:proxy-pass@example.test/cb?access_token=query-secret&filename=private-name.mkv#refreshToken=fragment-secret","note":"keep"}`, 0)
	for _, leaked := range []string{"proxy-user", "proxy-pass", "query-secret", "private-name.mkv", "fragment-secret"} {
		if strings.Contains(got, leaked) {
			t.Fatalf("expected %q to be redacted from request body %s", leaked, got)
		}
	}

	if !strings.Contains(got, `"note":"keep"`) {
		t.Fatalf("expected non-sensitive request body field to remain, got %s", got)
	}

	if !strings.Contains(got, utils.RedactedSecret) {
		t.Fatalf("expected redacted marker in request body %s", got)
	}
}

func TestFormatResponseBodyRedactsSensitiveJSON(t *testing.T) {
	ctx := NewContext(stdcontext.Background())

	got := ctx.formatResponseBody([]byte(`{"accessToken":"secret-access","refreshToken":"secret-refresh","tokenType":"Bearer"}`), 0)
	for _, leaked := range []string{"secret-access", "secret-refresh"} {
		if strings.Contains(got, leaked) {
			t.Fatalf("expected %q to be redacted from response body %s", leaked, got)
		}
	}

	if !strings.Contains(got, `"tokenType":"Bearer"`) {
		t.Fatalf("expected non-sensitive response body field to remain, got %s", got)
	}
}

func TestFormatResponseBodyRedactsURLValuesInJSON(t *testing.T) {
	ctx := NewContext(stdcontext.Background())

	got := ctx.formatResponseBody([]byte(`{"downloadUrl":"https://proxy-user:proxy-pass@example.test/file?sign=query-secret&filename=private-name.mkv#token=fragment-secret","status":"ok"}`), 0)
	for _, leaked := range []string{"proxy-user", "proxy-pass", "query-secret", "private-name.mkv", "fragment-secret"} {
		if strings.Contains(got, leaked) {
			t.Fatalf("expected %q to be redacted from response body %s", leaked, got)
		}
	}

	if !strings.Contains(got, `"status":"ok"`) {
		t.Fatalf("expected non-sensitive response body field to remain, got %s", got)
	}

	if !strings.Contains(got, utils.RedactedSecret) {
		t.Fatalf("expected redacted marker in response body %s", got)
	}
}

func TestFormatRequestBodyRedactsBeforeTruncating(t *testing.T) {
	ctx := NewContext(stdcontext.Background())
	body := `{"password":"secret-password","padding":"` + strings.Repeat("x", 200) + `"}`

	got := ctx.formatRequestBody(body, 30)
	if strings.Contains(got, "secret-password") {
		t.Fatalf("expected secret to be redacted before truncation, got %s", got)
	}

	if !strings.Contains(got, "[truncated]") {
		t.Fatalf("expected long redacted body to be truncated, got %s", got)
	}
}

func TestFormatRequestBodyOmitsOversizeRawBody(t *testing.T) {
	ctx := NewContext(stdcontext.Background())
	body := `{"password":"secret-password","padding":"` + strings.Repeat("x", 200) + `"}`

	got := ctx.formatRequestBody(body, 64)
	for _, leaked := range []string{"secret-password", strings.Repeat("x", 32)} {
		if strings.Contains(got, leaked) {
			t.Fatalf("expected oversized request body value %q to be omitted, got %s", leaked, got)
		}
	}

	if !strings.Contains(got, oversizeHTTPLogBodyMessage) {
		t.Fatalf("expected oversized request body marker, got %s", got)
	}

	if !strings.Contains(got, "[truncated]") {
		t.Fatalf("expected oversized request body to be marked truncated, got %s", got)
	}
}

func TestFormatResponseBodyOmitsOversizeRawBody(t *testing.T) {
	ctx := NewContext(stdcontext.Background())
	body := []byte(`{"accessToken":"secret-access","padding":"` + strings.Repeat("x", 200) + `"}`)

	got := ctx.formatResponseBody(body, 64)
	for _, leaked := range []string{"secret-access", strings.Repeat("x", 32)} {
		if strings.Contains(got, leaked) {
			t.Fatalf("expected oversized response body value %q to be omitted, got %s", leaked, got)
		}
	}

	if !strings.Contains(got, oversizeHTTPLogBodyMessage) {
		t.Fatalf("expected oversized response body marker, got %s", got)
	}

	if !strings.Contains(got, "[truncated]") {
		t.Fatalf("expected oversized response body to be marked truncated, got %s", got)
	}
}

func TestRedactHTTPLogURLRedactsOpaqueQueryValues(t *testing.T) {
	got := redactHTTPLogURL("https://proxy-user:proxy-pass@example.test/api/search?keyword=private-movie&filename=secret.mkv#access_token=secret-fragment")

	for _, leaked := range []string{"proxy-user", "proxy-pass", "private-movie", "secret.mkv", "secret-fragment"} {
		if strings.Contains(got, leaked) {
			t.Fatalf("expected %q to be redacted from %q", leaked, got)
		}
	}

	if !strings.Contains(got, "/api/search") {
		t.Fatalf("expected path to remain in %q", got)
	}

	if !strings.Contains(got, utils.RedactedSecret) {
		t.Fatalf("expected redacted marker in %q", got)
	}
}

func TestRedactHTTPLogErrorRedactsEmbeddedRequestURL(t *testing.T) {
	rawURL := "https://proxy-user:proxy-pass@example.test/api/search?keyword=private-movie&filename=secret.mkv#access_token=secret-fragment"
	got := redactHTTPLogError(rawURL, errors.New("Get \""+rawURL+"\": accessCode=abcd"))

	for _, leaked := range []string{"proxy-user", "proxy-pass", "private-movie", "secret.mkv", "secret-fragment", "abcd"} {
		if strings.Contains(got, leaked) {
			t.Fatalf("expected %q to be redacted from %q", leaked, got)
		}
	}

	if !strings.Contains(got, "/api/search") {
		t.Fatalf("expected path to remain in %q", got)
	}

	if !strings.Contains(got, utils.RedactedSecret) {
		t.Fatalf("expected redacted marker in %q", got)
	}
}

func TestRedactHTTPLogErrorRedactsAdditionalURLs(t *testing.T) {
	rawURL := "https://api.example.test/request"
	got := redactHTTPLogError(rawURL, errors.New(`upstream failed via https://proxy-user:proxy-pass@example.test/file?access_token=query-secret&filename=private-name.mkv#token=fragment-secret accessCode=abcd`))

	for _, leaked := range []string{"proxy-user", "proxy-pass", "query-secret", "private-name.mkv", "fragment-secret", "abcd"} {
		if strings.Contains(got, leaked) {
			t.Fatalf("expected %q to be redacted from %q", leaked, got)
		}
	}

	if !strings.Contains(got, "/file") {
		t.Fatalf("expected path to remain in %q", got)
	}

	if !strings.Contains(got, utils.RedactedSecret) {
		t.Fatalf("expected redacted marker in %q", got)
	}
}
