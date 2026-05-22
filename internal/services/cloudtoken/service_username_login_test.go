package cloudtoken

import (
	stdctx "context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/tickstep/cloudpan189-api/cloudpan"
	"github.com/xxcheng123/cloudpan189-share/internal/framework/context"
	"github.com/xxcheng123/cloudpan189-share/internal/repository/models"
	"gorm.io/gorm"
)

func stubAppLogin(t *testing.T) {
	t.Helper()

	stubAppLoginFunc(t, func(username, password string) (*cloudpan.AppLoginToken, error) {
		return &cloudpan.AppLoginToken{
			SskAccessToken:          "new-access-token",
			SskAccessTokenExpiresIn: 7200,
		}, nil
	})
}

func stubAppLoginFunc(t *testing.T, fn func(username, password string) (*cloudpan.AppLoginToken, error)) {
	t.Helper()

	original := appLogin
	appLogin = fn

	t.Cleanup(func() {
		appLogin = original
	})
}

func TestUsernameLoginRejectsInvalidRequestBeforeAppLogin(t *testing.T) {
	tDB := setupCloudTokenTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())
	appLoginCalled := false

	stubAppLoginFunc(t, func(username, password string) (*cloudpan.AppLoginToken, error) {
		appLoginCalled = true

		return &cloudpan.AppLoginToken{}, nil
	})

	tests := []struct {
		name string
		req  *UsernameLoginRequest
	}{
		{name: "nil request", req: nil},
		{name: "missing username", req: &UsernameLoginRequest{Password: "pass"}},
		{name: "missing password", req: &UsernameLoginRequest{Username: "user"}},
	}

	for _, tt := range tests {
		_, err := svc.UsernameLogin(ctx, tt.req)
		if !errors.Is(err, errInvalidUsernameLoginCredentials) {
			t.Fatalf("%s: expected invalid credentials error, got %v", tt.name, err)
		}
	}

	if appLoginCalled {
		t.Fatal("expected invalid request to return before app login")
	}

	var count int64
	if err := tDB.db.Model(&models.CloudToken{}).Count(&count).Error; err != nil {
		t.Fatalf("count cloud tokens: %v", err)
	}

	if count != 0 {
		t.Fatalf("expected no cloud tokens to be created, got %d", count)
	}
}

func TestUsernameLoginRefreshesPasswordTokenWithNilAddition(t *testing.T) {
	tDB := setupCloudTokenTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())

	stubAppLogin(t)

	token := &models.CloudToken{
		Name:        "password-token",
		AccessToken: "old-access-token",
		ExpiresIn:   3600,
		Status:      1,
		LoginType:   models.LoginTypePassword,
		Username:    "old-user",
		Password:    "old-pass",
		UserID:      10,
	}
	if err := tDB.db.Create(token).Error; err != nil {
		t.Fatalf("create cloud token: %v", err)
	}

	resp, err := svc.UsernameLogin(ctx, &UsernameLoginRequest{
		ID:       token.ID,
		Username: "new-user",
		Password: "new-pass",
		UserID:   10,
	})
	if err != nil {
		t.Fatalf("username login refresh: %v", err)
	}

	if resp.ID != token.ID {
		t.Fatalf("expected response token id %d, got %d", token.ID, resp.ID)
	}

	var updated models.CloudToken
	if err := tDB.db.First(&updated, token.ID).Error; err != nil {
		t.Fatalf("query updated token: %v", err)
	}

	if updated.AccessToken != "new-access-token" {
		t.Fatalf("expected refreshed access token, got %q", updated.AccessToken)
	}

	if updated.ExpiresIn != 7200 {
		t.Fatalf("expected refreshed expires in 7200, got %d", updated.ExpiresIn)
	}

	if updated.Username != "new-user" || updated.Password != "new-pass" {
		t.Fatalf("expected refreshed credentials, got username=%q password=%q", updated.Username, updated.Password)
	}

	if updated.Addition == nil {
		t.Fatal("expected nil addition to be initialized")
	}

	if got := fmt.Sprint(updated.Addition[models.CloudTokenAdditionAutoLoginTimes]); got != "0" {
		t.Fatalf("expected auto login times reset to 0, got %s", got)
	}

	result, _ := updated.Addition[models.CloudTokenAdditionAutoLoginResultKey].(string)
	if !strings.Contains(result, "token 刷新成功") {
		t.Fatalf("expected refresh success result, got %q", result)
	}
}

func TestUsernameLoginRestrictsNonAdminRefreshBeforeAppLogin(t *testing.T) {
	tDB := setupCloudTokenTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())
	appLoginCalled := false

	stubAppLoginFunc(t, func(username, password string) (*cloudpan.AppLoginToken, error) {
		appLoginCalled = true

		return &cloudpan.AppLoginToken{}, nil
	})

	token := &models.CloudToken{
		Name:        "other-password-token",
		AccessToken: "old-access-token",
		ExpiresIn:   3600,
		Status:      1,
		LoginType:   models.LoginTypePassword,
		Username:    "old-user",
		Password:    "old-pass",
		UserID:      20,
	}
	if err := tDB.db.Create(token).Error; err != nil {
		t.Fatalf("create cloud token: %v", err)
	}

	_, err := svc.UsernameLogin(ctx, &UsernameLoginRequest{
		ID:       token.ID,
		Username: "new-user",
		Password: "new-pass",
		UserID:   10,
	})
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("expected record not found, got %v", err)
	}

	if appLoginCalled {
		t.Fatal("expected unauthorized refresh to return before app login")
	}

	var unchanged models.CloudToken
	if err := tDB.db.First(&unchanged, token.ID).Error; err != nil {
		t.Fatalf("query unchanged token: %v", err)
	}

	if unchanged.AccessToken != "old-access-token" {
		t.Fatalf("expected token access token unchanged, got %q", unchanged.AccessToken)
	}
}

func TestUsernameLoginAllowsAdminRefresh(t *testing.T) {
	tDB := setupCloudTokenTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())

	stubAppLogin(t)

	token := &models.CloudToken{
		Name:        "admin-password-token",
		AccessToken: "old-access-token",
		ExpiresIn:   3600,
		Status:      1,
		LoginType:   models.LoginTypePassword,
		Username:    "old-user",
		Password:    "old-pass",
		UserID:      20,
	}
	if err := tDB.db.Create(token).Error; err != nil {
		t.Fatalf("create cloud token: %v", err)
	}

	resp, err := svc.UsernameLogin(ctx, &UsernameLoginRequest{
		ID:       token.ID,
		Username: "new-user",
		Password: "new-pass",
		IsAdmin:  true,
	})
	if err != nil {
		t.Fatalf("admin username login refresh: %v", err)
	}

	if resp.ID != token.ID {
		t.Fatalf("expected response token id %d, got %d", token.ID, resp.ID)
	}

	var updated models.CloudToken
	if err := tDB.db.First(&updated, token.ID).Error; err != nil {
		t.Fatalf("query updated token: %v", err)
	}

	if updated.AccessToken != "new-access-token" {
		t.Fatalf("expected refreshed access token, got %q", updated.AccessToken)
	}
}

func TestUsernameLoginRejectsMissingUserIDForNewTokenBeforeAppLogin(t *testing.T) {
	tDB := setupCloudTokenTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())
	appLoginCalled := false

	stubAppLoginFunc(t, func(username, password string) (*cloudpan.AppLoginToken, error) {
		appLoginCalled = true

		return &cloudpan.AppLoginToken{}, nil
	})

	_, err := svc.UsernameLogin(ctx, &UsernameLoginRequest{
		Username: "new-user",
		Password: "new-pass",
		Name:     "new-token",
	})
	if !errors.Is(err, errInvalidCloudTokenUserID) {
		t.Fatalf("expected invalid user id, got %v", err)
	}

	if appLoginCalled {
		t.Fatal("expected missing user id to return before app login")
	}
}
