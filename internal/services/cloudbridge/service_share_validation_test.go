package cloudbridge

import (
	stdctx "context"
	"strings"
	"testing"

	"github.com/xxcheng123/cloudpan189-share/internal/framework/context"
)

func TestShareInfoEntrypointsRejectInvalidShareCodeBeforeRequest(t *testing.T) {
	svc := NewService(nil)
	ctx := context.NewContext(stdctx.Background())

	tests := []struct {
		name string
		run  func() error
	}{
		{
			name: "get share info",
			run: func() error {
				_, err := svc.GetShareInfo(ctx, "https://cloud.189.cn/t/?token=secret-token", "")

				return err
			},
		},
		{
			name: "check share",
			run: func() error {
				_, err := svc.CheckShare(ctx, "https://cloud.189.cn/t/!!!?token=secret-token", "")

				return err
			},
		},
		{
			name: "check subscribe share",
			run: func() error {
				_, _, _, _, _, err := svc.CheckSubscribeShare(ctx, "up-user", "", "")

				return err
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.run()
			if err == nil {
				t.Fatal("expected invalid share code error")
			}

			if !strings.Contains(err.Error(), "分享码格式无效") {
				t.Fatalf("expected invalid share code error, got %v", err)
			}

			if strings.Contains(err.Error(), "secret-token") {
				t.Fatalf("expected error not to echo sensitive input, got %v", err)
			}
		})
	}
}

func TestShareInfoEntrypointsRejectInvalidAccessCodeBeforeRequest(t *testing.T) {
	svc := NewService(nil)
	ctx := context.NewContext(stdctx.Background())

	tests := []struct {
		name string
		run  func() error
	}{
		{
			name: "get share info",
			run: func() error {
				_, err := svc.GetShareInfo(ctx, "abcDEF", "https://example.com/share?token=secret-token")

				return err
			},
		},
		{
			name: "check share",
			run: func() error {
				_, err := svc.CheckShare(ctx, "abcDEF", "bad-code")

				return err
			},
		},
		{
			name: "check subscribe share",
			run: func() error {
				_, _, _, _, _, err := svc.CheckSubscribeShare(ctx, "up-user", "abcDEF", "access code: bad-code")

				return err
			},
		},
		{
			name: "get share files",
			run: func() error {
				_, err := svc.GetShareFiles(ctx, 12345, "file-id", 1, "https://example.com/share?token=secret-token", true)

				return err
			},
		},
		{
			name: "get subscribe share files",
			run: func() error {
				_, err := svc.GetSubscribeShareFiles(ctx, "up-user", 12345, "file-id", true, 5, "bad-code")

				return err
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.run()
			if err == nil {
				t.Fatal("expected invalid access code error")
			}

			if !strings.Contains(err.Error(), "访问码格式无效") {
				t.Fatalf("expected invalid access code error, got %v", err)
			}

			if strings.Contains(err.Error(), "secret-token") || strings.Contains(err.Error(), "bad-code") {
				t.Fatalf("expected error not to echo sensitive input, got %v", err)
			}
		})
	}
}

func TestCloud189ShareValidationNormalizesCodes(t *testing.T) {
	shareCode, err := validateCloud189ShareCode("  abcDEF  ")
	if err != nil {
		t.Fatalf("validate share code: %v", err)
	}

	if shareCode != "abcDEF" {
		t.Fatalf("expected trimmed share code, got %q", shareCode)
	}

	accessCode, err := validateCloud189AccessCode("  wxyz  ")
	if err != nil {
		t.Fatalf("validate access code: %v", err)
	}

	if accessCode != "wxyz" {
		t.Fatalf("expected trimmed access code, got %q", accessCode)
	}

	accessCode, err = validateCloud189AccessCode("   ")
	if err != nil {
		t.Fatalf("expected blank access code to be allowed, got %v", err)
	}

	if accessCode != "" {
		t.Fatalf("expected blank access code to normalize empty, got %q", accessCode)
	}
}

func TestCloud189SubscribeUserValidationNormalizesInput(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "raw id",
			input: " up-user_123 ",
			want:  "up-user_123",
		},
		{
			name:  "valid subscribe link",
			input: "content.21cn.com/h5/subscrip/?uuid=encoded%2Duser%5F9。",
			want:  "encoded-user_9",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := validateCloud189SubscribeUserID(tt.input)
			if err != nil {
				t.Fatalf("validate subscribe user id: %v", err)
			}

			if got != tt.want {
				t.Fatalf("expected %q, got %q", tt.want, got)
			}
		})
	}
}

func TestSubscribeUserEntrypointsRejectInvalidUserIDBeforeRequest(t *testing.T) {
	svc := NewService(nil)
	ctx := context.NewContext(stdctx.Background())

	tests := []struct {
		name string
		run  func() error
	}{
		{
			name: "check subscribe user",
			run: func() error {
				_, err := svc.CheckSubscribeUser(ctx, "https://content.21cn.com.evil.test/h5/subscrip/?uuid=secret-user")

				return err
			},
		},
		{
			name: "check subscribe share",
			run: func() error {
				_, _, _, _, _, err := svc.CheckSubscribeShare(ctx, "https://evil-content.21cn.com/h5/subscrip/?uuid=secret-user", "abcDEF", "")

				return err
			},
		},
		{
			name: "get subscribe user info",
			run: func() error {
				_, err := svc.GetSubscribeUserInfo(ctx, "ftp://content.21cn.com/h5/subscrip/?uuid=secret-user")

				return err
			},
		},
		{
			name: "get subscribe share resource",
			run: func() error {
				_, _, err := svc.GetSubscribeUserShareResource(ctx, "https://example.com/path/content.21cn.com/h5/subscrip/?uuid=secret-user")

				return err
			},
		},
		{
			name: "get all subscribe share resource",
			run: func() error {
				_, _, err := svc.GetSubscribeUserShareResourceAll(ctx, "content.21cn.com.evil.test/h5/subscrip/?uuid=secret-user")

				return err
			},
		},
		{
			name: "get subscribe user files",
			run: func() error {
				_, err := svc.GetSubscribeUserFiles(ctx, "example.com/share?uuid=secret-user")

				return err
			},
		},
		{
			name: "get subscribe share files",
			run: func() error {
				_, err := svc.GetSubscribeShareFiles(ctx, "https://evil.test@content.21cn.com/h5/subscrip/?uuid=secret-user", 123, "file-id", true, 5, "")

				return err
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.run()
			if err == nil {
				t.Fatal("expected invalid subscribe user id error")
			}

			if !strings.Contains(err.Error(), "订阅用户ID格式无效") {
				t.Fatalf("expected invalid subscribe user id error, got %v", err)
			}

			if strings.Contains(err.Error(), "secret-user") {
				t.Fatalf("expected error not to echo sensitive input, got %v", err)
			}
		})
	}
}
