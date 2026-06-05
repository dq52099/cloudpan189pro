package verify

import (
	stdContext "context"
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/xxcheng123/cloudpan189-share/internal/framework/context"
	"github.com/xxcheng123/cloudpan189-share/internal/pkgs/utils"
	"github.com/xxcheng123/cloudpan189-share/internal/shared"
)

func TestSignV1GeneratedTokenVerifies(t *testing.T) {
	svc := &service{}
	ctx := context.NewContext(stdContext.Background())

	values, err := svc.SignV1(ctx, 1001)
	if err != nil {
		t.Fatalf("sign v1: %v", err)
	}

	err = svc.VerifyV1(
		ctx,
		1001,
		values.Get("sign"),
		values.Get("uuid"),
		values.Get("timestamp"),
		values.Get("signer"),
	)
	if err != nil {
		t.Fatalf("verify generated token: %v", err)
	}
}

func TestSignV1NoExpireTokenVerifies(t *testing.T) {
	svc := &service{}
	ctx := context.NewContext(stdContext.Background())

	values, err := svc.SignV1(ctx, 1001, WithV1NoExpire())
	if err != nil {
		t.Fatalf("sign v1 no expire: %v", err)
	}

	if values.Get("timestamp") != "-1" {
		t.Fatalf("expected no-expire timestamp -1, got %q", values.Get("timestamp"))
	}

	err = svc.VerifyV1(
		ctx,
		1001,
		values.Get("sign"),
		values.Get("uuid"),
		values.Get("timestamp"),
		values.Get("signer"),
	)
	if err != nil {
		t.Fatalf("verify no-expire token: %v", err)
	}
}

func TestVerifyV1RejectsTamperedSignature(t *testing.T) {
	svc := &service{}
	ctx := context.NewContext(stdContext.Background())

	values, err := svc.SignV1(ctx, 1001)
	if err != nil {
		t.Fatalf("sign v1: %v", err)
	}

	err = svc.VerifyV1(
		ctx,
		1001,
		"bad"+values.Get("sign"),
		values.Get("uuid"),
		values.Get("timestamp"),
		values.Get("signer"),
	)
	if err == nil {
		t.Fatal("expected tampered signature to be rejected")
	}
}

func TestVerifyV1RejectsExpiredToken(t *testing.T) {
	svc := &service{}
	ctx := context.NewContext(stdContext.Background())
	fileID := int64(1001)
	uuid := "f3d1f04f-75d0-43f5-90b6-8a21b158c0ad"
	timestamp := strconv.FormatInt(time.Now().Add(-time.Minute).Unix(), 10)
	sign := buildVerifyV1TestSign(fileID, timestamp, uuid)

	err := svc.VerifyV1(ctx, fileID, sign, uuid, timestamp, signerV1Name)
	if err == nil {
		t.Fatal("expected expired token to be rejected")
	}
}

func TestVerifyV1RejectsInvalidUUID(t *testing.T) {
	svc := &service{}
	ctx := context.NewContext(stdContext.Background())
	fileID := int64(1001)
	uuid := "not-a-uuid"
	timestamp := strconv.FormatInt(time.Now().Add(time.Minute).Unix(), 10)
	sign := buildVerifyV1TestSign(fileID, timestamp, uuid)

	err := svc.VerifyV1(ctx, fileID, sign, uuid, timestamp, signerV1Name)
	if err == nil {
		t.Fatal("expected invalid UUID to be rejected")
	}
}

func TestVerifyV1RejectsInvalidSigner(t *testing.T) {
	svc := &service{}
	ctx := context.NewContext(stdContext.Background())
	fileID := int64(1001)
	uuid := "aa68f7d2-ee7e-4200-8a2e-328cc763eac7"
	timestamp := strconv.FormatInt(time.Now().Add(time.Minute).Unix(), 10)
	sign := buildVerifyV1TestSign(fileID, timestamp, uuid)

	err := svc.VerifyV1(ctx, fileID, sign, uuid, timestamp, "v2")
	if err == nil {
		t.Fatal("expected invalid signer to be rejected")
	}
}

func buildVerifyV1TestSign(fileID int64, timestamp string, uuid string) string {
	return utils.MD5(fmt.Sprintf(v1EncFormat, fileID, shared.GetSaltKey(), signerV1Name, timestamp, uuid))
}
