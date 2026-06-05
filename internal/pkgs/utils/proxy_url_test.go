package utils

import (
	"errors"
	"testing"
)

func TestNormalizeHTTPProxyURL(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		want    string
		wantErr bool
	}{
		{name: "empty", in: "   ", want: ""},
		{name: "http", in: " http://127.0.0.1:7890 ", want: "http://127.0.0.1:7890"},
		{name: "https with credentials", in: "https://proxy-user:proxy-pass@example.test:8443", want: "https://proxy-user:proxy-pass@example.test:8443"},
		{name: "unsupported scheme", in: "socks5://127.0.0.1:1080", wantErr: true},
		{name: "missing host", in: "http:///proxy", wantErr: true},
		{name: "invalid escape", in: "http://proxy-user:proxy-pass@%zz", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NormalizeHTTPProxyURL(tt.in)
			if tt.wantErr {
				if !errors.Is(err, ErrInvalidHTTPProxyURL) {
					t.Fatalf("expected invalid proxy URL error, got %v", err)
				}

				return
			}

			if err != nil {
				t.Fatalf("expected success, got %v", err)
			}

			if got != tt.want {
				t.Fatalf("expected %q, got %q", tt.want, got)
			}
		})
	}
}
