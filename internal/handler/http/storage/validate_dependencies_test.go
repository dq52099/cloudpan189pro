package storage

import (
	stdctx "context"
	"testing"

	appContext "github.com/xxcheng123/cloudpan189-share/internal/framework/context"
	"github.com/xxcheng123/cloudpan189-share/internal/framework/httpcontext"
	"github.com/xxcheng123/cloudpan189-share/internal/repository/models"
)

func TestExecuteOsTypeValidationReturnsTypedNilDependencyErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		run  func(appContext.Context) httpcontext.BusinessError
		want httpcontext.BusinessError
	}{
		{
			name: "subscribe cloud bridge",
			run: func(ctx appContext.Context) httpcontext.BusinessError {
				var cloudBridgeService *mockBatchAddCloudBridge

				h := &handler{cloudBridgeService: cloudBridgeService}

				_, busErr := h.executeOsTypeSubscribe(ctx, &addRequest{
					OsType:        models.OsTypeSubscribe,
					SubscribeUser: "up-user",
				})

				return busErr
			},
			want: busCodeStorageQuerySubscribeUserError,
		},
		{
			name: "subscribe share cloud bridge",
			run: func(ctx appContext.Context) httpcontext.BusinessError {
				var cloudBridgeService *mockBatchAddCloudBridge

				h := &handler{cloudBridgeService: cloudBridgeService}

				_, _, busErr := h.executeOsTypeSubscribeShare(ctx, &addRequest{
					OsType:          models.OsTypeSubscribeShareFolder,
					SubscribeUser:   "up-user",
					ShareCode:       "abcDEF",
					ShareAccessCode: "wxyz",
				})

				return busErr
			},
			want: busCodeStorageQuerySubscribeShareError,
		},
		{
			name: "share cloud bridge",
			run: func(ctx appContext.Context) httpcontext.BusinessError {
				var cloudBridgeService *mockBatchAddCloudBridge

				h := &handler{cloudBridgeService: cloudBridgeService}

				_, _, busErr := h.executeOsTypeShare(ctx, &addRequest{
					OsType:    models.OsTypeShareFolder,
					ShareCode: "abcDEF",
				})

				return busErr
			},
			want: busCodeStorageQuerySubscribeShareError,
		},
		{
			name: "personal cloud token",
			run: func(ctx appContext.Context) httpcontext.BusinessError {
				var cloudTokenService *mockAddCloudTokenService

				h := &handler{
					cloudBridgeService: &mockBatchAddCloudBridge{},
					cloudTokenService:  cloudTokenService,
				}

				return h.executeOsTypePersonal(ctx, &addRequest{
					OsType:     models.OsTypePersonFolder,
					FileId:     "file-1",
					CloudToken: 9,
				}, 100, false)
			},
			want: busCodeStorageQueryCloudTokenError,
		},
		{
			name: "personal cloud bridge",
			run: func(ctx appContext.Context) httpcontext.BusinessError {
				var cloudBridgeService *mockBatchAddCloudBridge

				h := &handler{
					cloudBridgeService: cloudBridgeService,
					cloudTokenService:  &mockAddCloudTokenService{},
				}

				return h.executeOsTypePersonal(ctx, &addRequest{
					OsType:     models.OsTypePersonFolder,
					FileId:     "file-1",
					CloudToken: 9,
				}, 100, false)
			},
			want: busCodeStoragePersonFileQueryError,
		},
		{
			name: "family cloud token",
			run: func(ctx appContext.Context) httpcontext.BusinessError {
				var cloudTokenService *mockAddCloudTokenService

				h := &handler{
					cloudBridgeService: &mockBatchAddCloudBridge{},
					cloudTokenService:  cloudTokenService,
				}

				_, busErr := h.executeOsTypeFamily(ctx, &addRequest{
					OsType:     models.OsTypeFamilyFolder,
					FileId:     "file-1",
					FamilyId:   "family-1",
					CloudToken: 9,
				}, 100, false)

				return busErr
			},
			want: busCodeStorageQueryCloudTokenError,
		},
		{
			name: "family cloud bridge",
			run: func(ctx appContext.Context) httpcontext.BusinessError {
				var cloudBridgeService *mockBatchAddCloudBridge

				h := &handler{
					cloudBridgeService: cloudBridgeService,
					cloudTokenService:  &mockAddCloudTokenService{},
				}

				_, busErr := h.executeOsTypeFamily(ctx, &addRequest{
					OsType:     models.OsTypeFamilyFolder,
					FileId:     "file-1",
					FamilyId:   "family-1",
					CloudToken: 9,
				}, 100, false)

				return busErr
			},
			want: busCodeStorageFamilyFileQueryError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctx := appContext.NewContext(stdctx.Background())
			assertStorageBusinessErrorCode(t, tt.run(ctx), tt.want)
		})
	}
}

func assertStorageBusinessErrorCode(t *testing.T, got, want httpcontext.BusinessError) {
	t.Helper()

	if got == nil {
		t.Fatalf("expected business code %d, got nil", want.GetCode())
	}

	if got.GetCode() != want.GetCode() {
		t.Fatalf("expected business code %d, got %d", want.GetCode(), got.GetCode())
	}
}
