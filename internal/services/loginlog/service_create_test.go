package loginlog

import (
	stdctx "context"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/xxcheng123/cloudpan189-share/internal/framework/context"
	"github.com/xxcheng123/cloudpan189-share/internal/pkgs/utils"
	"github.com/xxcheng123/cloudpan189-share/internal/repository/models"
	loginlogType "github.com/xxcheng123/cloudpan189-share/internal/types/loginlog"
)

func TestCreateSanitizesReasonAndUserAgentBeforePersisting(t *testing.T) {
	tDB := setupLoginLogTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())

	rawReason := "刷新失败: GET https://proxy-user:proxy-pass@auth.example.test/callback?token=secret-token&filename=private-name.mkv#access_token=fragment-secret accessCode=abcd Authorization: Bearer bearer-secret"
	rawUserAgent := "Mozilla/5.0 token=ua-secret https://ua-user:ua-pass@client.example.test/path?clientSecret=client-secret"

	id, err := svc.Create(ctx, &models.LoginLog{
		Method:    loginlogType.MethodWeb,
		Event:     loginlogType.EventRefreshToken,
		Status:    loginlogType.StatusFailed,
		Reason:    rawReason,
		UserAgent: rawUserAgent,
	})
	if err != nil {
		t.Fatalf("create login log: %v", err)
	}

	var got models.LoginLog
	if err := tDB.db.First(&got, id).Error; err != nil {
		t.Fatalf("query login log: %v", err)
	}

	assertLoginLogTextRedacted(t, got.Reason, "proxy-user", "proxy-pass", "secret-token", "private-name.mkv", "fragment-secret", "abcd", "bearer-secret")
	assertLoginLogTextRedacted(t, got.UserAgent, "ua-secret", "ua-user", "ua-pass", "client-secret")
}

func TestCreateTruncatesReasonAndUserAgentBeforePersisting(t *testing.T) {
	tDB := setupLoginLogTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())

	id, err := svc.Create(ctx, &models.LoginLog{
		Method:    loginlogType.MethodWeb,
		Event:     loginlogType.EventLogin,
		Status:    loginlogType.StatusFailed,
		Reason:    strings.Repeat("失败", maxLoginLogReasonRunes+10),
		UserAgent: strings.Repeat("客", maxLoginLogUserAgentRunes+10),
	})
	if err != nil {
		t.Fatalf("create login log: %v", err)
	}

	var got models.LoginLog
	if err := tDB.db.First(&got, id).Error; err != nil {
		t.Fatalf("query login log: %v", err)
	}

	if utf8.RuneCountInString(got.Reason) != maxLoginLogReasonRunes {
		t.Fatalf("expected reason to be truncated to %d runes, got %d", maxLoginLogReasonRunes, utf8.RuneCountInString(got.Reason))
	}

	if utf8.RuneCountInString(got.UserAgent) != maxLoginLogUserAgentRunes {
		t.Fatalf("expected user agent to be truncated to %d runes, got %d", maxLoginLogUserAgentRunes, utf8.RuneCountInString(got.UserAgent))
	}
}

func assertLoginLogTextRedacted(t *testing.T, text string, leakedValues ...string) {
	t.Helper()

	for _, leaked := range leakedValues {
		if strings.Contains(text, leaked) {
			t.Fatalf("expected %q to be redacted from %q", leaked, text)
		}
	}

	if !strings.Contains(text, utils.RedactedSecret) {
		t.Fatalf("expected redacted marker in %q", text)
	}
}
