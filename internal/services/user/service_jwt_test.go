package user

import (
	"testing"

	"github.com/xxcheng123/cloudpan189-share/internal/bootstrap"
	"github.com/xxcheng123/cloudpan189-share/internal/shared"
)

func TestJWTTokenRoundTripPreservesUsername(t *testing.T) {
	oldSaltKey := shared.GetSaltKey()
	oldBaseURL := shared.GetBaseURL()
	oldEnableAuth := shared.IsAuthEnabled()
	oldAddition := shared.GetSettingAddition()

	t.Cleanup(func() {
		shared.SetSetting(oldSaltKey, oldBaseURL, oldEnableAuth, oldAddition)
	})

	shared.SetSetting("jwt-round-trip-test-salt", oldBaseURL, oldEnableAuth, oldAddition)

	service := NewService(bootstrap.NewMockServiceContext())

	accessToken, err := service.GenerateAccessToken(42, "alice", 3)
	if err != nil {
		t.Fatalf("generate access token: %v", err)
	}

	uid, username, version, err := service.ParseAccessToken(accessToken)
	if err != nil {
		t.Fatalf("parse access token: %v", err)
	}

	if uid != 42 || username != "alice" || version != 3 {
		t.Fatalf("expected access claims uid=42 username=alice version=3, got uid=%d username=%q version=%d", uid, username, version)
	}

	refreshToken, err := service.GenerateRefreshToken(42, "alice", 3)
	if err != nil {
		t.Fatalf("generate refresh token: %v", err)
	}

	uid, username, version, err = service.ParseRefreshToken(refreshToken)
	if err != nil {
		t.Fatalf("parse refresh token: %v", err)
	}

	if uid != 42 || username != "alice" || version != 3 {
		t.Fatalf("expected refresh claims uid=42 username=alice version=3, got uid=%d username=%q version=%d", uid, username, version)
	}
}
