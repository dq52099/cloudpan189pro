package utils

import (
	"encoding/json"
	"net/url"
	"strings"
	"testing"
)

func TestMaskSecret(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "empty", in: "", want: ""},
		{name: "short", in: "ab", want: "**"},
		{name: "access code", in: "wxyz", want: "****"},
		{name: "long", in: "abcdefghi", want: "ab*****hi"},
		{name: "trim", in: "  abcdef  ", want: "ab**ef"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := MaskSecret(tt.in); got != tt.want {
				t.Fatalf("expected %q, got %q", tt.want, got)
			}
		})
	}
}

func TestMaskShareCodeForLog(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "plain share code", in: "abcDEF", want: "ab****"},
		{name: "cloud share link", in: "https://cloud.189.cn/t/abcDEF?x=1", want: "ab****"},
		{name: "share and access code text", in: "分享码：abcDEF 提取码：wxyz", want: "ab****"},
		{name: "short code", in: "ab", want: "**"},
		{name: "empty", in: "", want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := MaskShareCodeForLog(tt.in); got != tt.want {
				t.Fatalf("expected %q, got %q", tt.want, got)
			}
		})
	}
}

func TestRedactShareAccessCodeText(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "chinese access code", in: "abc（访问码：wxyz）", want: "abc（访问码：****）"},
		{name: "extract code", in: "分享码：abc 提取码：wxyz", want: "分享码：abc 提取码：****"},
		{name: "english access code", in: "shareCode:abc accessCode:wxyz", want: "shareCode:abc accessCode:****"},
		{name: "english spaced access code", in: "share code: abc access code: wxyz", want: "share code: abc access code: ****"},
		{name: "english share access code", in: "share access code: wxyz", want: "share access code: ****"},
		{name: "keep share code", in: "shareCode:abc", want: "shareCode:abc"},
		{name: "keep spaced share code", in: "share code: abc", want: "share code: abc"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := RedactShareAccessCodeText(tt.in); got != tt.want {
				t.Fatalf("expected %q, got %q", tt.want, got)
			}
		})
	}
}

func TestRedactSensitiveTextRedactsJSONRecursively(t *testing.T) {
	input := `{"username":"admin","password":"secret-password","botToken":"telegram-secret","botTokenEncrypted":"telegram-encrypted-secret","tmdbAPIKey":"tmdb-secret","SskAccessToken":"ssk-secret","SskAccessTokenExpiresIn":7200,"nested":{"refresh_token":"secret-refresh","clientSecret":"client-secret","message":"提取码：wxyz"},"items":[{"accessCode":"abcd","name":"folder"}]}`

	got := RedactSensitiveText(input)
	for _, leaked := range []string{"secret-password", "telegram-secret", "telegram-encrypted-secret", "tmdb-secret", "secret-refresh", "client-secret", "ssk-secret", "abcd", "wxyz"} {
		if strings.Contains(got, leaked) {
			t.Fatalf("expected %q to be redacted from %s", leaked, got)
		}
	}

	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(got), &payload); err != nil {
		t.Fatalf("expected redacted JSON to stay valid: %v", err)
	}

	if payload["username"] != "admin" {
		t.Fatalf("expected non-sensitive field to stay unchanged, got %#v", payload["username"])
	}

	if payload["password"] != RedactedSecret {
		t.Fatalf("expected password to be redacted, got %#v", payload["password"])
	}

	if payload["SskAccessToken"] != RedactedSecret {
		t.Fatalf("expected SskAccessToken to be redacted, got %#v", payload["SskAccessToken"])
	}

	if payload["botToken"] != RedactedSecret {
		t.Fatalf("expected botToken to be redacted, got %#v", payload["botToken"])
	}

	if payload["botTokenEncrypted"] != RedactedSecret {
		t.Fatalf("expected botTokenEncrypted to be redacted, got %#v", payload["botTokenEncrypted"])
	}

	if payload["tmdbAPIKey"] != RedactedSecret {
		t.Fatalf("expected tmdbAPIKey to be redacted, got %#v", payload["tmdbAPIKey"])
	}

	if payload["SskAccessTokenExpiresIn"] != float64(7200) {
		t.Fatalf("expected SskAccessTokenExpiresIn to stay unchanged, got %#v", payload["SskAccessTokenExpiresIn"])
	}

	nested := payload["nested"].(map[string]interface{})
	if nested["refresh_token"] != RedactedSecret {
		t.Fatalf("expected refresh_token to be redacted, got %#v", nested["refresh_token"])
	}

	if nested["clientSecret"] != RedactedSecret {
		t.Fatalf("expected clientSecret to be redacted, got %#v", nested["clientSecret"])
	}

	if nested["message"] != "提取码：****" {
		t.Fatalf("expected embedded access code to be masked, got %#v", nested["message"])
	}

	items := payload["items"].([]interface{})

	item := items[0].(map[string]interface{})
	if item["accessCode"] != RedactedSecret {
		t.Fatalf("expected accessCode to be redacted, got %#v", item["accessCode"])
	}

	if item["name"] != "folder" {
		t.Fatalf("expected non-sensitive nested field to stay unchanged, got %#v", item["name"])
	}
}

func TestRedactSensitiveTextRedactsURLsInJSONStrings(t *testing.T) {
	input := `{"callback":"https://proxy-user:proxy-pass@example.test/cb?keyword=private-movie&filename=secret.mkv#token=fragment-secret","nested":{"note":"download from /api/search?keyword=private-keyword&name=movie"},"items":["https://download.example.test/file?sign=secret-sign#session=secret-session"]}`

	got := RedactSensitiveText(input)

	decoded, err := url.QueryUnescape(got)
	if err != nil {
		t.Fatalf("decode redacted JSON: %v", err)
	}

	for _, leaked := range []string{
		"proxy-user",
		"proxy-pass",
		"private-movie",
		"secret.mkv",
		"fragment-secret",
		"private-keyword",
		"secret-sign",
		"secret-session",
	} {
		if strings.Contains(decoded, leaked) {
			t.Fatalf("expected %q to be redacted from %s", leaked, decoded)
		}
	}

	if !strings.Contains(decoded, "example.test/cb") || !strings.Contains(decoded, "/api/search") || !strings.Contains(decoded, "download.example.test/file") {
		t.Fatalf("expected URL host/path to remain, got %s", decoded)
	}

	if !strings.Contains(decoded, RedactedSecret) {
		t.Fatalf("expected redacted marker in %s", decoded)
	}

	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(got), &payload); err != nil {
		t.Fatalf("expected redacted JSON to stay valid: %v", err)
	}
}

func TestRedactSensitiveTextRedactsPlainTextPairs(t *testing.T) {
	input := "/api/user/refresh_token?refreshToken=secret-refresh&SskAccessToken=ssk-secret&botToken=telegram-secret&botTokenEncrypted=telegram-encrypted-secret&bot_token_encrypted=telegram-snake-secret&csrfToken=csrf-secret&sessionKey=session-secret&tokenId=42&tokenType=Bearer&name=alice Authorization: Bearer secret-access"

	got := RedactSensitiveText(input)
	for _, leaked := range []string{"secret-refresh", "ssk-secret", "telegram-secret", "telegram-encrypted-secret", "telegram-snake-secret", "csrf-secret", "session-secret", "secret-access"} {
		if strings.Contains(got, leaked) {
			t.Fatalf("expected %q to be redacted from %s", leaked, got)
		}
	}

	if !strings.Contains(got, "refreshToken="+RedactedSecret) {
		t.Fatalf("expected refreshToken query value to be redacted, got %s", got)
	}

	if !strings.Contains(got, "SskAccessToken="+RedactedSecret) {
		t.Fatalf("expected SskAccessToken query value to be redacted, got %s", got)
	}

	if !strings.Contains(got, "botToken="+RedactedSecret) {
		t.Fatalf("expected botToken query value to be redacted, got %s", got)
	}

	if !strings.Contains(got, "botTokenEncrypted="+RedactedSecret) {
		t.Fatalf("expected botTokenEncrypted query value to be redacted, got %s", got)
	}

	if !strings.Contains(got, "bot_token_encrypted="+RedactedSecret) {
		t.Fatalf("expected bot_token_encrypted query value to be redacted, got %s", got)
	}

	if !strings.Contains(got, "csrfToken="+RedactedSecret) {
		t.Fatalf("expected csrfToken query value to be redacted, got %s", got)
	}

	if !strings.Contains(got, "sessionKey="+RedactedSecret) {
		t.Fatalf("expected sessionKey query value to be redacted, got %s", got)
	}

	if !strings.Contains(got, "Authorization: "+RedactedSecret) {
		t.Fatalf("expected Authorization value to be redacted, got %s", got)
	}

	if !strings.Contains(got, "name=alice") {
		t.Fatalf("expected non-sensitive query value to remain, got %s", got)
	}

	if !strings.Contains(got, "tokenId=42") || !strings.Contains(got, "tokenType=Bearer") {
		t.Fatalf("expected token metadata query values to remain, got %s", got)
	}
}

func TestRedactSensitiveTextRedactsSpacedPlainTextKeys(t *testing.T) {
	input := "access token: access-secret, refresh token=refresh-secret; share access code: abcd; share code: efgh"

	got := RedactSensitiveText(input)
	for _, leaked := range []string{"access-secret", "refresh-secret", "abcd"} {
		if strings.Contains(got, leaked) {
			t.Fatalf("expected %q to be redacted from %s", leaked, got)
		}
	}

	if !strings.Contains(got, "access token: "+RedactedSecret) {
		t.Fatalf("expected spaced access token to be redacted, got %s", got)
	}

	if !strings.Contains(got, "refresh token="+RedactedSecret) {
		t.Fatalf("expected spaced refresh token to be redacted, got %s", got)
	}

	if !strings.Contains(got, "share access code: "+RedactedSecret) {
		t.Fatalf("expected spaced share access code to be redacted, got %s", got)
	}

	if !strings.Contains(got, "share code: efgh") {
		t.Fatalf("expected non-sensitive share code to remain, got %s", got)
	}
}

func TestRedactSensitiveTextRedactsTelegramBotTokenURL(t *testing.T) {
	tests := []struct {
		name  string
		input string
		token string
		want  string
	}{
		{
			name:  "official api token path",
			input: "https://api.telegram.org/bot123456:ABC-def/sendMessage?chat_id=1",
			token: "123456:ABC-def",
			want:  "https://api.telegram.org/bot[REDACTED]/sendMessage?chat_id=1",
		},
		{
			name:  "custom api token path",
			input: "http://127.0.0.1:8081/botold-token/getUpdates",
			token: "old-token",
			want:  "http://127.0.0.1:8081/bot[REDACTED]/getUpdates",
		},
		{
			name:  "embedded json string",
			input: `{"url":"https://api.telegram.org/bot123456:ABC-def/sendMessage"}`,
			token: "123456:ABC-def",
			want:  `{"url":"https://api.telegram.org/bot[REDACTED]/sendMessage"}`,
		},
		{
			name:  "plain bot route is unchanged",
			input: "https://example.test/bot/status?ok=1",
			want:  "https://example.test/bot/status?ok=1",
		},
		{
			name:  "bot word without method is unchanged",
			input: "https://example.test/botany",
			want:  "https://example.test/botany",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := RedactSensitiveText(tt.input)
			if got != tt.want {
				t.Fatalf("expected %q, got %q", tt.want, got)
			}

			if tt.token != "" && strings.Contains(got, tt.token) {
				t.Fatalf("expected token %q to be redacted from %s", tt.token, got)
			}
		})
	}
}

func TestRedactSensitiveTextRedactsURLUserInfo(t *testing.T) {
	tests := []struct {
		name string
		in   string
	}{
		{name: "plain url credentials", in: "proxy=https://proxy-user:proxy-pass@example.test/path?name=alice"},
		{name: "invalid url credentials", in: "proxy=http://proxy-user:proxy-pass@%zz"},
		{name: "json url credentials", in: `{"proxy":"https://proxy-user:proxy-pass@example.test/path","name":"alice"}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := RedactSensitiveText(tt.in)

			for _, leaked := range []string{"proxy-user", "proxy-pass"} {
				if strings.Contains(got, leaked) {
					t.Fatalf("expected %q to be redacted from %s", leaked, got)
				}
			}

			if !strings.Contains(got, RedactedSecret) {
				t.Fatalf("expected redacted marker in %s", got)
			}

			if strings.Contains(tt.in, "example.test") && !strings.Contains(got, "example.test/path") {
				t.Fatalf("expected URL host and path to remain, got %s", got)
			}

			if strings.Contains(tt.in, "alice") && !strings.Contains(got, "alice") {
				t.Fatalf("expected non-sensitive text to remain, got %s", got)
			}
		})
	}
}

func TestRedactURLForLogRedactsQueryAndFragment(t *testing.T) {
	input := "https://download.example.test/path/file.mkv?sign=secret-sign&filename=private-name.mkv#session=secret-fragment"

	got := RedactURLForLog(input)

	decoded, err := url.QueryUnescape(got)
	if err != nil {
		t.Fatalf("decode redacted URL: %v", err)
	}

	for _, leaked := range []string{"secret-sign", "private-name.mkv", "secret-fragment"} {
		if strings.Contains(decoded, leaked) {
			t.Fatalf("expected %q to be redacted from %s", leaked, decoded)
		}
	}

	if !strings.Contains(decoded, RedactedSecret) {
		t.Fatalf("expected redacted marker in %s", decoded)
	}

	if !strings.Contains(decoded, "https://download.example.test/path/file.mkv") {
		t.Fatalf("expected URL origin and path to remain, got %s", decoded)
	}
}

func TestRedactURLForLogRedactsUserInfo(t *testing.T) {
	tests := []struct {
		name string
		in   string
	}{
		{name: "http proxy credentials", in: "http://proxy-user:proxy-pass@proxy.example.test:8080"},
		{name: "socks proxy credentials with query", in: "socks5://proxy-user:proxy-pass@127.0.0.1:1080?token=secret-token"},
		{name: "invalid url fallback", in: "http://proxy-user:proxy-pass@%zz"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := RedactURLForLog(tt.in)

			decoded := got
			if unescaped, err := url.QueryUnescape(got); err == nil {
				decoded = unescaped
			}

			for _, leaked := range []string{"proxy-user", "proxy-pass", "secret-token"} {
				if strings.Contains(decoded, leaked) {
					t.Fatalf("expected %q to be redacted from %s", leaked, decoded)
				}
			}

			if !strings.Contains(decoded, RedactedSecret) {
				t.Fatalf("expected redacted marker in %s", decoded)
			}
		})
	}
}

func TestRedactURLsInTextForLogRedactsEmbeddedURLs(t *testing.T) {
	input := `failed via https://proxy-user:proxy-pass@example.test/file?sign=secret-sign&filename=private-name.mkv#access_token=secret-fragment and /api/search?keyword=private-keyword`

	got := RedactURLsInTextForLog(input)
	for _, leaked := range []string{"proxy-user", "proxy-pass", "secret-sign", "private-name.mkv", "secret-fragment", "private-keyword"} {
		if strings.Contains(got, leaked) {
			t.Fatalf("expected %q to be redacted from %s", leaked, got)
		}
	}

	if !strings.Contains(got, "example.test/file") || !strings.Contains(got, "/api/search") {
		t.Fatalf("expected URL host/path to remain, got %s", got)
	}

	if !strings.Contains(got, RedactedSecret) {
		t.Fatalf("expected redacted marker in %s", got)
	}
}

func TestIsSensitiveLogKey(t *testing.T) {
	for _, key := range []string{"Authorization", "Set-Cookie", "access_token", "refreshToken", "SskAccessToken", "secret-key", "sessionKey", "super-password", "shareAccessCode", "botToken", "botTokenEncrypted", "tmdbAPIKey", "openaiAPIKey", "clientSecret"} {
		if !IsSensitiveLogKey(key) {
			t.Fatalf("expected %s to be sensitive", key)
		}
	}

	for _, key := range []string{"tokenId", "tokenType", "SskAccessTokenExpiresIn", "username", "contentType"} {
		if IsSensitiveLogKey(key) {
			t.Fatalf("expected %s to be non-sensitive", key)
		}
	}
}
