package scheduler

import (
	"strings"
	"testing"

	"github.com/xxcheng123/cloudpan189-share/internal/pkgs/utils"
)

func TestSanitizeSchedulerPanicValueRedactsSensitiveText(t *testing.T) {
	got := sanitizeSchedulerPanicValue(`GET "https://proxy-user:proxy-pass@example.test/api/search?keyword=private-movie&filename=secret.mkv#access_token=secret-fragment": accessCode=abcd`)

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

func TestSanitizeSchedulerPanicValueOmitsOversizeText(t *testing.T) {
	got := sanitizeSchedulerPanicValue(`panic with accessToken=secret-access ` + strings.Repeat("x", maxSchedulerPanicTextSize+1))

	for _, leaked := range []string{"secret-access", strings.Repeat("x", 32)} {
		if strings.Contains(got, leaked) {
			t.Fatalf("expected oversized panic value %q to be omitted, got %q", leaked, got)
		}
	}

	if !strings.Contains(got, oversizeSchedulerPanicText) {
		t.Fatalf("expected oversized panic marker, got %q", got)
	}

	if !strings.Contains(got, "[truncated]") {
		t.Fatalf("expected oversized panic to be marked truncated, got %q", got)
	}
}
