package models

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"gorm.io/datatypes"
)

func TestCloudTokenAuthExpiresAtMillisUsesIssuedAtForTTL(t *testing.T) {
	issuedAt := time.Date(2026, 5, 1, 12, 0, 0, 123*int(time.Millisecond), time.UTC)
	now := issuedAt.Add(30 * time.Minute)
	token := &CloudToken{
		ExpiresIn: 7200,
	}
	token.SetIssuedAt(issuedAt)

	got := token.AuthExpiresAtMillis(now)
	want := issuedAt.Add(2 * time.Hour).UnixMilli()

	if got != want {
		t.Fatalf("expected auth expires at %d, got %d", want, got)
	}
}

func TestCloudTokenRemainingSecondsUsesAbsoluteMillis(t *testing.T) {
	now := time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)
	token := &CloudToken{
		ExpiresIn: now.Add(90 * time.Second).UnixMilli(),
	}

	if got := token.RemainingSeconds(now); got != 90 {
		t.Fatalf("expected 90 remaining seconds, got %d", got)
	}
}

func TestCloudTokenExpiresAtFallsBackToCreatedAt(t *testing.T) {
	createdAt := time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)
	token := &CloudToken{
		ExpiresIn: 3600,
		CreatedAt: createdAt,
		UpdatedAt: createdAt.Add(30 * time.Minute),
	}

	expiresAt, ok := token.ExpiresAt(createdAt.Add(time.Hour))
	if !ok {
		t.Fatal("expected expires at to be available")
	}

	if want := createdAt.Add(time.Hour); !expiresAt.Equal(want) {
		t.Fatalf("expected expires at %s, got %s", want, expiresAt)
	}
}

func TestCloudTokenJSONOmitsSecrets(t *testing.T) {
	token := &CloudToken{
		ID:          1,
		Name:        "token",
		AccessToken: "secret-access-token",
		Password:    "secret-password",
		ExpiresIn:   3600,
		Addition: map[string]interface{}{
			"access_token":                   "secret-addition-token",
			"accessToken":                    "secret-camel-token",
			"password":                       "secret-addition-password",
			"safe":                           "value",
			CloudTokenAdditionTokenIssuedAt:  float64(123),
			CloudTokenAdditionAutoLoginTimes: float64(1),
		},
	}

	data, err := json.Marshal(token)
	if err != nil {
		t.Fatalf("marshal cloud token: %v", err)
	}

	var payload map[string]interface{}
	if err := json.Unmarshal(data, &payload); err != nil {
		t.Fatalf("unmarshal cloud token: %v", err)
	}

	if _, ok := payload["accessToken"]; ok {
		t.Fatal("expected cloud token JSON to omit accessToken")
	}

	if _, ok := payload["password"]; ok {
		t.Fatal("expected cloud token JSON to omit password")
	}

	addition, ok := payload["addition"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected addition object, got %T", payload["addition"])
	}

	for _, key := range []string{"access_token", "accessToken", "password"} {
		if _, ok := addition[key]; ok {
			t.Fatalf("expected cloud token JSON addition to omit %s", key)
		}
	}

	if addition["safe"] != "value" {
		t.Fatalf("expected safe addition key to be preserved, got %v", addition["safe"])
	}

	if addition[CloudTokenAdditionTokenIssuedAt] != float64(123) {
		t.Fatalf("expected token issued-at addition key to be preserved, got %v", addition[CloudTokenAdditionTokenIssuedAt])
	}
}

func TestCloudTokenJSONSanitizesNestedAdditionSecrets(t *testing.T) {
	nested := map[string]interface{}{
		"Password": "nested-password",
		"message":  "password: nested-password",
		"safe":     "nested-value",
	}
	token := &CloudToken{
		ID:        1,
		Name:      "token",
		ExpiresIn: 3600,
		Addition: map[string]interface{}{
			"AccessToken": "top-secret-token",
			"safeNested":  nested,
			"typedNested": datatypes.JSONMap{
				"SskAccessToken": "typed-secret-token",
				"safe":           "typed-value",
			},
			"items": []interface{}{
				map[string]interface{}{
					"refresh-token": "nested-refresh-token",
					"label":         "keep",
				},
				"access_token=plain-secret-token",
			},
		},
	}

	data, err := json.Marshal(token)
	if err != nil {
		t.Fatalf("marshal cloud token: %v", err)
	}

	var payload map[string]interface{}
	if err := json.Unmarshal(data, &payload); err != nil {
		t.Fatalf("unmarshal cloud token: %v", err)
	}

	addition, ok := payload["addition"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected addition object, got %T", payload["addition"])
	}

	if _, ok := addition["AccessToken"]; ok {
		t.Fatal("expected normalized sensitive top-level key to be omitted")
	}

	nestedPayload, ok := addition["safeNested"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected safeNested object, got %T", addition["safeNested"])
	}

	if _, ok := nestedPayload["Password"]; ok {
		t.Fatal("expected normalized sensitive nested key to be omitted")
	}

	if nestedPayload["safe"] != "nested-value" {
		t.Fatalf("expected nested safe key to be preserved, got %v", nestedPayload["safe"])
	}

	if nestedPayload["message"] != "password: [REDACTED]" {
		t.Fatalf("expected nested sensitive text to be redacted, got %v", nestedPayload["message"])
	}

	typedNestedPayload, ok := addition["typedNested"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected typedNested object, got %T", addition["typedNested"])
	}

	if _, ok := typedNestedPayload["SskAccessToken"]; ok {
		t.Fatal("expected typed nested sensitive key to be omitted")
	}

	if typedNestedPayload["safe"] != "typed-value" {
		t.Fatalf("expected typed nested safe key to be preserved, got %v", typedNestedPayload["safe"])
	}

	items, ok := addition["items"].([]interface{})
	if !ok || len(items) != 2 {
		t.Fatalf("expected two sanitized items, got %#v", addition["items"])
	}

	item, ok := items[0].(map[string]interface{})
	if !ok {
		t.Fatalf("expected first item object, got %T", items[0])
	}

	if _, ok := item["refresh-token"]; ok {
		t.Fatal("expected normalized sensitive array item key to be omitted")
	}

	if item["label"] != "keep" {
		t.Fatalf("expected safe array item key to be preserved, got %v", item["label"])
	}

	if items[1] != "access_token=[REDACTED]" {
		t.Fatalf("expected plain text token to be redacted, got %v", items[1])
	}

	if _, ok := token.Addition["AccessToken"]; !ok {
		t.Fatal("expected original addition to remain unchanged")
	}

	if _, ok := nested["Password"]; !ok {
		t.Fatal("expected original nested addition to remain unchanged")
	}
}

func TestCloudTokenJSONSanitizesAdditionURLValues(t *testing.T) {
	rawURL := "https://proxy-user:proxy-pass@example.test/file?sign=secret-sign&filename=private-name.mkv#access_token=secret-fragment"
	token := &CloudToken{
		ID:        1,
		Name:      "token",
		ExpiresIn: 3600,
		Addition: map[string]interface{}{
			"downloadUrl": rawURL,
			"message":     "download failed: " + rawURL + " accessCode=abcd",
			"items": []interface{}{
				map[string]interface{}{
					"url": rawURL,
				},
			},
		},
	}

	data, err := json.Marshal(token)
	if err != nil {
		t.Fatalf("marshal cloud token: %v", err)
	}

	var payload map[string]interface{}
	if err := json.Unmarshal(data, &payload); err != nil {
		t.Fatalf("unmarshal cloud token: %v", err)
	}

	addition, ok := payload["addition"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected addition object, got %T", payload["addition"])
	}

	encodedAddition, err := json.Marshal(addition)
	if err != nil {
		t.Fatalf("marshal addition: %v", err)
	}

	additionText := string(encodedAddition)
	for _, leaked := range []string{"proxy-user", "proxy-pass", "secret-sign", "private-name.mkv", "secret-fragment", "abcd"} {
		if strings.Contains(additionText, leaked) {
			t.Fatalf("expected %q to be redacted from %s", leaked, additionText)
		}
	}

	if !strings.Contains(additionText, "example.test/file") {
		t.Fatalf("expected URL host and path to remain, got %s", additionText)
	}

	if !strings.Contains(additionText, "[REDACTED]") {
		t.Fatalf("expected redacted marker in %s", additionText)
	}

	if token.Addition["downloadUrl"] != rawURL {
		t.Fatalf("expected original addition to remain unchanged, got %v", token.Addition["downloadUrl"])
	}
}
