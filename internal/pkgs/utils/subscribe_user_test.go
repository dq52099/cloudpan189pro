package utils

import "testing"

func TestParseCloud189SubscribeUserID(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "plain subscribe link",
			input: "https://content.21cn.com/h5/subscrip/?uuid=up-user",
			want:  "up-user",
		},
		{
			name:  "uppercase host and middle query",
			input: "HTTPS://CONTENT.21CN.COM/h5/subscrip/?foo=1&uuid=user_123&bar=2",
			want:  "user_123",
		},
		{
			name:  "encoded user id",
			input: "https://content.21cn.com/h5/subscrip/?uuid=encoded%2Duser%5F9。",
			want:  "encoded-user_9",
		},
		{
			name:  "bare subscribe link",
			input: "content.21cn.com/h5/subscrip/?uuid=bare-user",
			want:  "bare-user",
		},
		{
			name:  "protocol relative link",
			input: "//content.21cn.com/h5/subscrip/?uuid=protocol-relative-user",
			want:  "protocol-relative-user",
		},
		{
			name:  "link with port",
			input: "https://content.21cn.com:443/h5/subscrip/?uuid=port-user",
			want:  "port-user",
		},
		{
			name:  "numeric prefix does not become folder id",
			input: "123 https://content.21cn.com/h5/subscrip/?uuid=number-prefix-user",
			want:  "number-prefix-user",
		},
		{
			name:  "chinese subscribe label",
			input: "订阅号：up-user",
			want:  "up-user",
		},
		{
			name:  "chinese subscribe id label",
			input: "订阅号ID：up-user_123。",
			want:  "up-user_123",
		},
		{
			name:  "english subscribe label",
			input: "subscribeUser: encoded%2Duser%5F9",
			want:  "encoded-user_9",
		},
		{
			name:  "uuid label",
			input: "uuid=uuid-user",
			want:  "uuid-user",
		},
		{
			name:  "trailing punctuation",
			input: "https://content.21cn.com/h5/subscrip/?uuid=up-user！",
			want:  "up-user",
		},
		{
			name:  "missing uuid",
			input: "https://content.21cn.com/h5/subscrip/?foo=bar",
			want:  "",
		},
		{
			name:  "reject lookalike suffix host",
			input: "https://content.21cn.com.evil.test/h5/subscrip/?uuid=bad-user",
			want:  "",
		},
		{
			name:  "reject lookalike prefix host",
			input: "https://evil-content.21cn.com/h5/subscrip/?uuid=bad-user",
			want:  "",
		},
		{
			name:  "reject unsupported scheme",
			input: "ftp://content.21cn.com/h5/subscrip/?uuid=bad-user",
			want:  "",
		},
		{
			name:  "reject host inside another url path",
			input: "https://example.com/path/content.21cn.com/h5/subscrip/?uuid=bad-user",
			want:  "",
		},
		{
			name:  "reject host inside url userinfo",
			input: "https://evil.test@content.21cn.com/h5/subscrip/?uuid=bad-user",
			want:  "",
		},
		{
			name:  "reject non cloud url with uuid query",
			input: "https://example.com/path?uuid=bad-user",
			want:  "",
		},
		{
			name:  "empty input",
			input: "",
			want:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := ParseCloud189SubscribeUserID(tt.input); got != tt.want {
				t.Fatalf("expected %q, got %q", tt.want, got)
			}
		})
	}
}

func TestNormalizeCloud189SubscribeUserID(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "raw subscribe user id",
			input: " up-user_123 ",
			want:  "up-user_123",
		},
		{
			name:  "raw subscribe user id with trailing punctuation",
			input: " up-user_123。",
			want:  "up-user_123",
		},
		{
			name:  "chinese subscribe label",
			input: "订阅号：up-user",
			want:  "up-user",
		},
		{
			name:  "english subscribe label",
			input: "subscribeUser: encoded%2Duser%5F9。",
			want:  "encoded-user_9",
		},
		{
			name:  "valid subscribe link",
			input: "https://content.21cn.com/h5/subscrip/?uuid=encoded%2Duser%5F9。",
			want:  "encoded-user_9",
		},
		{
			name:  "valid bare subscribe link",
			input: "content.21cn.com/h5/subscrip/?uuid=bare-user",
			want:  "bare-user",
		},
		{
			name:  "valid subscribe link with numeric prefix",
			input: "123 https://content.21cn.com/h5/subscrip/?uuid=number-prefix-user",
			want:  "number-prefix-user",
		},
		{
			name:  "reject non cloud url",
			input: "note https://example.com/share?token=secret-token",
			want:  "",
		},
		{
			name:  "reject bare non cloud url",
			input: "note example.com/share?token=secret-token",
			want:  "",
		},
		{
			name:  "reject lookalike suffix host",
			input: "note https://content.21cn.com.evil.test/h5/subscrip/?uuid=bad-user",
			want:  "",
		},
		{
			name:  "reject bare lookalike suffix host",
			input: "note content.21cn.com.evil.test/h5/subscrip/?uuid=bad-user",
			want:  "",
		},
		{
			name:  "reject lookalike prefix host",
			input: "note https://evil-content.21cn.com/h5/subscrip/?uuid=bad-user",
			want:  "",
		},
		{
			name:  "reject bare lookalike prefix host",
			input: "note evil-content.21cn.com/h5/subscrip/?uuid=bad-user",
			want:  "",
		},
		{
			name:  "reject unsupported scheme",
			input: "ftp://content.21cn.com/h5/subscrip/?uuid=bad-user",
			want:  "",
		},
		{
			name:  "reject host inside another url path",
			input: "https://example.com/path/content.21cn.com/h5/subscrip/?uuid=bad-user",
			want:  "",
		},
		{
			name:  "reject host inside url userinfo",
			input: "https://evil.test@content.21cn.com/h5/subscrip/?uuid=bad-user",
			want:  "",
		},
		{
			name:  "reject non cloud url with uuid query",
			input: "https://example.com/path?uuid=bad-user",
			want:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := NormalizeCloud189SubscribeUserID(tt.input); got != tt.want {
				t.Fatalf("expected %q, got %q", tt.want, got)
			}
		})
	}
}
