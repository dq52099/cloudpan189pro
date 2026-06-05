package httpcontext

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/xxcheng123/cloudpan189-share/internal/pkgs/utils"
)

func TestSanitizeLoggedHeadersRedactsSensitiveHeadersAndValues(t *testing.T) {
	headers := http.Header{
		"Authorization": []string{"Bearer secret-access"},
		"Cookie":        []string{"sid=secret-session"},
		"X-Trace":       []string{"accessToken=secret-token"},
		"Referer":       []string{"https://proxy-user:proxy-pass@example.test/search?keyword=private-movie#token=secret-fragment"},
		"Accept":        []string{"application/json"},
	}

	got := sanitizeLoggedHeaders(headers)

	if got["Authorization"][0] != utils.RedactedSecret {
		t.Fatalf("expected Authorization header to be redacted, got %#v", got["Authorization"])
	}

	if got["Cookie"][0] != utils.RedactedSecret {
		t.Fatalf("expected Cookie header to be redacted, got %#v", got["Cookie"])
	}

	if strings.Contains(got["X-Trace"][0], "secret-token") {
		t.Fatalf("expected embedded sensitive value to be redacted, got %#v", got["X-Trace"])
	}

	for _, leaked := range []string{"proxy-user", "proxy-pass", "private-movie", "secret-fragment"} {
		if strings.Contains(got["Referer"][0], leaked) {
			t.Fatalf("expected %q to be redacted from %#v", leaked, got["Referer"])
		}
	}

	if got["Accept"][0] != "application/json" {
		t.Fatalf("expected Accept header to remain, got %#v", got["Accept"])
	}
}

func TestSanitizeLoggedURLRedactsOpaqueQueryValues(t *testing.T) {
	got := sanitizeLoggedURL("/api/search?keyword=private-movie&filename=secret.mkv#access_token=secret-fragment")

	for _, leaked := range []string{"private-movie", "secret.mkv", "secret-fragment"} {
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

func TestSanitizeLoggedTextRedactsEmbeddedURLsAndSensitiveText(t *testing.T) {
	got := sanitizeLoggedText(`{"callback":"https://proxy-user:proxy-pass@example.test/callback?keyword=private-movie&filename=secret.mkv#access_token=secret-fragment","message":"share access code: abcd","status":"ok"}`)

	for _, leaked := range []string{"proxy-user", "proxy-pass", "private-movie", "secret.mkv", "secret-fragment", "abcd"} {
		if strings.Contains(got, leaked) {
			t.Fatalf("expected %q to be redacted from %q", leaked, got)
		}
	}

	if !strings.Contains(got, "example.test/callback") {
		t.Fatalf("expected URL host and path to remain in %q", got)
	}

	if !strings.Contains(got, `"status":"ok"`) {
		t.Fatalf("expected non-sensitive body fields to remain in %q", got)
	}

	if !strings.Contains(got, utils.RedactedSecret) {
		t.Fatalf("expected redacted marker in %q", got)
	}
}

func TestSanitizeLoggedErrorsRedactsEmbeddedURLsAndSensitiveText(t *testing.T) {
	got := sanitizeLoggedErrors([]error{
		errors.New(`GET "/api/search?keyword=private-movie&filename=secret.mkv": accessCode=abcd`),
		errors.New(`proxy "https://proxy-user:proxy-pass@example.test/api/search?keyword=private-tv#access_token=secret-fragment" failed`),
	})

	if len(got) != 2 {
		t.Fatalf("expected two sanitized errors, got %#v", got)
	}

	combined := strings.Join(got, "\n")
	for _, leaked := range []string{"private-movie", "secret.mkv", "abcd", "proxy-user", "proxy-pass", "private-tv", "secret-fragment"} {
		if strings.Contains(combined, leaked) {
			t.Fatalf("expected %q to be redacted from %q", leaked, combined)
		}
	}

	if !strings.Contains(combined, "/api/search") {
		t.Fatalf("expected path to remain in %q", combined)
	}

	if !strings.Contains(combined, utils.RedactedSecret) {
		t.Fatalf("expected redacted marker in %q", combined)
	}
}

func TestSanitizeLoggedPanicValueRedactsSensitiveText(t *testing.T) {
	got := sanitizeLoggedPanicValue(`GET "https://proxy-user:proxy-pass@example.test/api/search?keyword=private-movie&filename=secret.mkv#access_token=secret-fragment": accessCode=abcd`)

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

type countingReadCloser struct {
	reader    *strings.Reader
	readBytes int
}

func (c *countingReadCloser) Read(p []byte) (int, error) {
	n, err := c.reader.Read(p)
	c.readBytes += n

	return n, err
}

func (c *countingReadCloser) Close() error {
	return nil
}

func TestReadRequestBodyForLogLimitsReadAndReplaysFullBody(t *testing.T) {
	bodyText := `{"message":"` + strings.Repeat("x", maxLoggedJSONRequestBodySize+2048) + `"}`
	body := &countingReadCloser{reader: strings.NewReader(bodyText)}
	req := httptest.NewRequest(http.MethodPost, "/log", nil)
	req.Body = body

	got, err := readRequestBodyForLog(req, maxLoggedJSONRequestBodySize)
	if err != nil {
		t.Fatalf("read request body for log: %v", err)
	}

	if body.readBytes > maxLoggedJSONRequestBodySize+1 {
		t.Fatalf("expected log reader to consume at most %d bytes, got %d", maxLoggedJSONRequestBodySize+1, body.readBytes)
	}

	if !strings.HasSuffix(got, loggedBodyTruncatedSuffix) {
		t.Fatalf("expected truncated log body suffix, got %q", got[len(got)-len(loggedBodyTruncatedSuffix):])
	}

	replayed, err := io.ReadAll(req.Body)
	if err != nil {
		t.Fatalf("read replayed request body: %v", err)
	}

	if string(replayed) != bodyText {
		t.Fatalf("expected full body to be replayed, got length=%d want=%d", len(replayed), len(bodyText))
	}
}

func TestResponseWriterLimitsLoggedBodyWithoutTruncatingResponse(t *testing.T) {
	gin.SetMode(gin.TestMode)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	writer := &responseWriter{
		ResponseWriter: ctx.Writer,
		b:              &bytes.Buffer{},
		maxBodySize:    maxLoggedResponseBodySize,
	}
	responseText := strings.Repeat("x", maxLoggedResponseBodySize+2048)

	n, err := writer.Write([]byte(responseText))
	if err != nil {
		t.Fatalf("write response: %v", err)
	}

	if n != len(responseText) {
		t.Fatalf("expected full response write length %d, got %d", len(responseText), n)
	}

	if recorder.Body.String() != responseText {
		t.Fatalf("expected full response body to be written, got length=%d want=%d", recorder.Body.Len(), len(responseText))
	}

	if writer.b.Len() != maxLoggedResponseBodySize {
		t.Fatalf("expected logged body length %d, got %d", maxLoggedResponseBodySize, writer.b.Len())
	}

	if !strings.HasSuffix(writer.loggedBody(), loggedBodyTruncatedSuffix) {
		t.Fatalf("expected logged body to be marked truncated")
	}
}

func TestResponseWriterLimitsLoggedStringWithoutTruncatingResponse(t *testing.T) {
	gin.SetMode(gin.TestMode)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	writer := &responseWriter{
		ResponseWriter: ctx.Writer,
		b:              &bytes.Buffer{},
		maxBodySize:    maxLoggedResponseBodySize,
	}
	responseText := strings.Repeat("x", maxLoggedResponseBodySize+2048)

	n, err := writer.WriteString(responseText)
	if err != nil {
		t.Fatalf("write response string: %v", err)
	}

	if n != len(responseText) {
		t.Fatalf("expected full response write length %d, got %d", len(responseText), n)
	}

	if recorder.Body.String() != responseText {
		t.Fatalf("expected full response body to be written, got length=%d want=%d", recorder.Body.Len(), len(responseText))
	}

	if writer.b.Len() != maxLoggedResponseBodySize {
		t.Fatalf("expected logged body length %d, got %d", maxLoggedResponseBodySize, writer.b.Len())
	}

	if !strings.HasSuffix(writer.loggedBody(), loggedBodyTruncatedSuffix) {
		t.Fatalf("expected logged body to be marked truncated")
	}
}
