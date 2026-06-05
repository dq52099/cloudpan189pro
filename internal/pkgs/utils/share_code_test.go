package utils

import "testing"

func TestParseCloud189ShareCode(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		input          string
		explicitAccess string
		wantShare      string
		wantAccess     string
	}{
		{
			name:      "chinese share label only",
			input:     "分享码：abcDEF",
			wantShare: "abcDEF",
		},
		{
			name:       "chinese share and access labels",
			input:      "分享码：abcDEF 提取码：wxyz",
			wantShare:  "abcDEF",
			wantAccess: "wxyz",
		},
		{
			name:       "chinese share and access labels separated by ideographic comma",
			input:      "分享码：abcDEF、提取码：wxyz",
			wantShare:  "abcDEF",
			wantAccess: "wxyz",
		},
		{
			name:       "cloud link with parenthesized access",
			input:      "https://cloud.189.cn/t/abcDEF（访问码：wxyz）",
			wantShare:  "abcDEF",
			wantAccess: "wxyz",
		},
		{
			name:       "www cloud link with query and access",
			input:      "https://www.cloud.189.cn/t/abcDEF?shareId=1&x=y 提取码：wxyz",
			wantShare:  "abcDEF",
			wantAccess: "wxyz",
		},
		{
			name:      "bare cloud link",
			input:     "cloud.189.cn/t/bare123?shareId=1",
			wantShare: "bare123",
		},
		{
			name:      "protocol relative cloud link",
			input:     "//cloud.189.cn/t/proto123?shareId=1",
			wantShare: "proto123",
		},
		{
			name:      "cloud link with port",
			input:     "https://cloud.189.cn:443/t/port123?shareId=1",
			wantShare: "port123",
		},
		{
			name:      "uppercase cloud link host",
			input:     "HTTPS://CLOUD.189.CN/t/AbC123?foo=bar",
			wantShare: "AbC123",
		},
		{
			name:       "cloud link followed by chinese punctuation and access",
			input:      "https://cloud.189.cn/t/abcDEF，提取码：wxyz",
			wantShare:  "abcDEF",
			wantAccess: "wxyz",
		},
		{
			name:       "cloud link followed by ideographic comma and access",
			input:      "https://cloud.189.cn/t/abcDEF、提取码：wxyz",
			wantShare:  "abcDEF",
			wantAccess: "wxyz",
		},
		{
			name:      "cloud link followed by sentence punctuation",
			input:     "https://cloud.189.cn/t/abcDEF。",
			wantShare: "abcDEF",
		},
		{
			name:      "cloud link followed by fullwidth exclamation",
			input:     "https://cloud.189.cn/t/abcDEF！",
			wantShare: "abcDEF",
		},
		{
			name:      "cloud link followed by fullwidth question",
			input:     "https://cloud.189.cn/t/abcDEF？",
			wantShare: "abcDEF",
		},
		{
			name:      "cloud link followed by closing chinese bracket",
			input:     "https://cloud.189.cn/t/abcDEF】",
			wantShare: "abcDEF",
		},
		{
			name:      "malformed dotted link share segment is not truncated",
			input:     "https://cloud.189.cn/t/abc.DEF.",
			wantShare: "abc.DEF",
		},
		{
			name:       "english camel labels",
			input:      "shareCode:abc accessCode:wxyz",
			wantShare:  "abc",
			wantAccess: "wxyz",
		},
		{
			name:       "english spaced labels",
			input:      "share code: abc access code: wxyz",
			wantShare:  "abc",
			wantAccess: "wxyz",
		},
		{
			name:      "english spaced share label only",
			input:     "share code: abc",
			wantShare: "abc",
		},
		{
			name:       "generic code after link is access code",
			input:      "https://cloud.189.cn/t/abcDEF code: wxyz",
			wantShare:  "abcDEF",
			wantAccess: "wxyz",
		},
		{
			name:       "space separated fallback",
			input:      "abcDEF wxyz",
			wantShare:  "abcDEF",
			wantAccess: "wxyz",
		},
		{
			name:           "explicit access fallback",
			input:          "分享码：abcDEF",
			explicitAccess: "zzzz",
			wantShare:      "abcDEF",
			wantAccess:     "zzzz",
		},
		{
			name:           "explicit access overrides embedded access",
			input:          "分享码：abcDEF 提取码：wxyz",
			explicitAccess: "zzzz",
			wantShare:      "abcDEF",
			wantAccess:     "zzzz",
		},
		{
			name:           "explicit access is trimmed",
			input:          "https://cloud.189.cn/t/abcDEF（访问码：wxyz）",
			explicitAccess: "  zzzz  ",
			wantShare:      "abcDEF",
			wantAccess:     "zzzz",
		},
		{
			name:           "explicit access trims fullwidth punctuation",
			input:          "https://cloud.189.cn/t/abcDEF",
			explicitAccess: "wxyz！",
			wantShare:      "abcDEF",
			wantAccess:     "wxyz",
		},
		{
			name:           "explicit chinese access label is parsed",
			input:          "https://cloud.189.cn/t/abcDEF",
			explicitAccess: "提取码：wxyz",
			wantShare:      "abcDEF",
			wantAccess:     "wxyz",
		},
		{
			name:           "explicit english access label overrides embedded access",
			input:          "https://cloud.189.cn/t/abcDEF（访问码：old1）",
			explicitAccess: "access code: wxyz",
			wantShare:      "abcDEF",
			wantAccess:     "wxyz",
		},
		{
			name:           "explicit full share text extracts access only",
			input:          "https://cloud.189.cn/t/abcDEF",
			explicitAccess: "分享码：ignored 提取码：wxyz",
			wantShare:      "abcDEF",
			wantAccess:     "wxyz",
		},
		{
			name:       "malformed link share segment is not truncated",
			input:      "https://cloud.189.cn/t/abc-DEF?token=secret",
			wantShare:  "abc-DEF",
			wantAccess: "",
		},
		{
			name:       "hyphenated access code is not truncated",
			input:      "分享码：abcDEF 提取码：bad-code",
			wantShare:  "abcDEF",
			wantAccess: "bad-code",
		},
		{
			name:       "url access code is not truncated",
			input:      "分享码：abcDEF 提取码：https://example.com/share?token=secret-token",
			wantShare:  "abcDEF",
			wantAccess: "https://example.com/share?token=secret-token",
		},
		{
			name:       "trailing sentence punctuation is ignored",
			input:      "分享码：abcDEF，提取码：wxyz。",
			wantShare:  "abcDEF",
			wantAccess: "wxyz",
		},
		{
			name:       "fullwidth access punctuation is ignored",
			input:      "分享码：abcDEF 提取码：wxyz！",
			wantShare:  "abcDEF",
			wantAccess: "wxyz",
		},
		{
			name:      "malformed ideographic comma link share segment is not truncated",
			input:     "https://cloud.189.cn/t/abc、DEF",
			wantShare: "abc、DEF",
		},
		{
			name:      "malformed ideographic comma labeled share code is not truncated",
			input:     "分享码：abc、DEF",
			wantShare: "abc、DEF",
		},
		{
			name:      "non cloud url does not fall back to first token",
			input:     "note https://example.com/share?token=secret-token",
			wantShare: "",
		},
		{
			name:      "bare non cloud url does not fall back to first token",
			input:     "note example.com/share?token=secret-token",
			wantShare: "",
		},
		{
			name:      "lookalike suffix host does not fall back to first token",
			input:     "note https://cloud.189.cn.evil.test/t/abcDEF",
			wantShare: "",
		},
		{
			name:      "bare lookalike suffix host does not fall back to first token",
			input:     "note cloud.189.cn.evil.test/t/abcDEF",
			wantShare: "",
		},
		{
			name:      "lookalike prefix host does not fall back to first token",
			input:     "note https://evil-cloud.189.cn/t/abcDEF",
			wantShare: "",
		},
		{
			name:      "bare lookalike prefix host does not fall back to first token",
			input:     "note evil-cloud.189.cn/t/abcDEF",
			wantShare: "",
		},
		{
			name:      "unsupported scheme does not parse as cloud link",
			input:     "ftp://cloud.189.cn/t/abcDEF",
			wantShare: "",
		},
		{
			name:      "cloud host inside another url path does not parse",
			input:     "https://example.com/path/cloud.189.cn/t/abcDEF",
			wantShare: "",
		},
		{
			name:      "cloud host inside url userinfo does not parse",
			input:     "https://evil.test@cloud.189.cn/t/abcDEF",
			wantShare: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			gotShare, gotAccess := ParseCloud189ShareCode(tt.input, tt.explicitAccess)
			if gotShare != tt.wantShare || gotAccess != tt.wantAccess {
				t.Fatalf("expected share/access %q/%q, got %q/%q", tt.wantShare, tt.wantAccess, gotShare, gotAccess)
			}
		})
	}
}

func TestIsCloud189ShareLink(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		value string
		want  bool
	}{
		{name: "plain link", value: "https://cloud.189.cn/t/abcDEF", want: true},
		{name: "bare link", value: "cloud.189.cn/t/bare123", want: true},
		{name: "www link", value: "https://www.cloud.189.cn/t/abcDEF", want: true},
		{name: "uppercase link", value: "HTTPS://CLOUD.189.CN/t/AbC123", want: true},
		{name: "protocol relative link", value: "//cloud.189.cn/t/proto123", want: true},
		{name: "link with port", value: "https://cloud.189.cn:443/t/port123", want: true},
		{name: "link in text", value: "分享 https://cloud.189.cn/t/abcDEF", want: true},
		{name: "reject suffix host", value: "https://cloud.189.cn.evil.test/t/abcDEF", want: false},
		{name: "reject prefix host", value: "https://evil-cloud.189.cn/t/abcDEF", want: false},
		{name: "reject embedded host", value: "note https://evilcloud.189.cn/t/abcDEF", want: false},
		{name: "reject unsupported scheme", value: "ftp://cloud.189.cn/t/abcDEF", want: false},
		{name: "reject host inside another url path", value: "https://example.com/path/cloud.189.cn/t/abcDEF", want: false},
		{name: "reject host inside url userinfo", value: "https://evil.test@cloud.189.cn/t/abcDEF", want: false},
		{name: "missing share code", value: "https://cloud.189.cn/t/?token=secret", want: false},
		{name: "plain code is not link", value: "abcDEF", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := IsCloud189ShareLink(tt.value); got != tt.want {
				t.Fatalf("expected %t, got %t", tt.want, got)
			}
		})
	}
}

func TestContainsURLLike(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		value string
		want  bool
	}{
		{name: "http url", value: "note https://example.com/share?token=secret-token", want: true},
		{name: "protocol relative url", value: "note //example.com/share", want: true},
		{name: "bare domain path url", value: "note example.com/share?token=secret-token", want: true},
		{name: "bare domain query url", value: "note example.com?token=secret-token", want: true},
		{name: "bare domain without path is not url like", value: "example.com", want: false},
		{name: "local path is not url like", value: "/Movies/123456", want: false},
		{name: "plain share code is not url like", value: "abcDEF", want: false},
		{name: "spaces only", value: "   ", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := ContainsURLLike(tt.value); got != tt.want {
				t.Fatalf("expected %t, got %t", tt.want, got)
			}
		})
	}
}

func TestIsCloud189ShareCode(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		value string
		want  bool
	}{
		{name: "lowercase alphanumeric", value: "abc123", want: true},
		{name: "mixed case alphanumeric", value: "AbC123", want: true},
		{name: "trimmed value", value: "  abcDEF  ", want: true},
		{name: "empty", value: "", want: false},
		{name: "spaces only", value: "   ", want: false},
		{name: "cloud url is not raw code", value: "https://cloud.189.cn/t/abc123", want: false},
		{name: "punctuation is rejected", value: "abc-123", want: false},
		{name: "non cloud url is rejected", value: "https://example.com/share?token=secret", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := IsCloud189ShareCode(tt.value); got != tt.want {
				t.Fatalf("expected %t, got %t", tt.want, got)
			}
		})
	}
}

func TestIsCloud189AccessCode(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		value string
		want  bool
	}{
		{name: "lowercase alphanumeric", value: "wxyz", want: true},
		{name: "mixed case alphanumeric", value: "Ab12", want: true},
		{name: "trimmed value", value: "  a1B2  ", want: true},
		{name: "empty", value: "", want: false},
		{name: "spaces only", value: "   ", want: false},
		{name: "punctuation is rejected", value: "ab-12", want: false},
		{name: "url is rejected", value: "https://example.com/share?token=secret", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := IsCloud189AccessCode(tt.value); got != tt.want {
				t.Fatalf("expected %t, got %t", tt.want, got)
			}
		})
	}
}
