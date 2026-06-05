package scheduler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	appContext "github.com/xxcheng123/cloudpan189-share/internal/framework/context"
	"github.com/xxcheng123/cloudpan189-share/internal/pkgs/utils"
	"github.com/xxcheng123/cloudpan189-share/internal/repository/models"
	cloudtokenSvi "github.com/xxcheng123/cloudpan189-share/internal/services/cloudtoken"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
	"gorm.io/datatypes"
)

type mockRefreshCloudTokenService struct {
	usernameLoginReq             *cloudtokenSvi.UsernameLoginRequest
	updatedAddition              map[string]interface{}
	passwordLoginTokens          []*models.CloudToken
	afterListPasswordLoginTokens func()
	listPasswordLoginEntered     chan struct{}
	releaseListPasswordLogin     chan struct{}
	listPasswordLoginCalls       atomic.Int64
	panicOnListPasswordLogin     bool
}

func (m *mockRefreshCloudTokenService) InitQrcode(appContext.Context) (*cloudtokenSvi.InitQrcodeResponse, error) {
	return nil, nil
}

func (m *mockRefreshCloudTokenService) CheckQrcode(appContext.Context, *cloudtokenSvi.CheckQrcodeRequest) error {
	return nil
}

func (m *mockRefreshCloudTokenService) ModifyName(appContext.Context, *cloudtokenSvi.ModifyNameRequest) error {
	return nil
}

func (m *mockRefreshCloudTokenService) Delete(appContext.Context, *cloudtokenSvi.DeleteRequest) error {
	return nil
}

func (m *mockRefreshCloudTokenService) List(appContext.Context, *cloudtokenSvi.ListRequest) ([]*models.CloudToken, error) {
	return nil, nil
}

func (m *mockRefreshCloudTokenService) Count(appContext.Context, *cloudtokenSvi.ListRequest) (int64, error) {
	return 0, nil
}

func (m *mockRefreshCloudTokenService) UsernameLogin(
	_ appContext.Context,
	req *cloudtokenSvi.UsernameLoginRequest,
) (*cloudtokenSvi.UsernameLoginResponse, error) {
	m.usernameLoginReq = req

	return &cloudtokenSvi.UsernameLoginResponse{ID: req.ID}, nil
}

func (m *mockRefreshCloudTokenService) Query(appContext.Context, int64) (*models.CloudToken, error) {
	return nil, nil
}

func (m *mockRefreshCloudTokenService) QueryAccessible(
	appContext.Context,
	int64,
	int64,
	bool,
) (*models.CloudToken, error) {
	return nil, nil
}

func (m *mockRefreshCloudTokenService) ListPasswordLoginTokens(appContext.Context) ([]*models.CloudToken, error) {
	m.listPasswordLoginCalls.Add(1)

	if m.panicOnListPasswordLogin {
		panic(`GET "https://proxy-user:proxy-pass@example.test/api/token?accessToken=secret-access": password=secret-password`)
	}

	if m.listPasswordLoginEntered != nil {
		select {
		case m.listPasswordLoginEntered <- struct{}{}:
		default:
		}
	}

	if m.releaseListPasswordLogin != nil {
		<-m.releaseListPasswordLogin
	}

	if m.afterListPasswordLoginTokens != nil {
		m.afterListPasswordLoginTokens()
	}

	return m.passwordLoginTokens, nil
}

func (m *mockRefreshCloudTokenService) UpdateAddition(
	_ appContext.Context,
	_ int64,
	addition map[string]interface{},
) error {
	m.updatedAddition = addition

	return nil
}

func TestRefreshCloudTokenSchedulerUsesIssuedAtForExpiryWindow(t *testing.T) {
	issuedAt := time.Now().Add(-5 * 24 * time.Hour)
	token := &models.CloudToken{
		ExpiresIn: 7 * 24 * 60 * 60,
	}
	token.SetIssuedAt(issuedAt)

	scheduler := &RefreshCloudTokenScheduler{}
	if !scheduler.willExpireInThreeDays(token) {
		t.Fatal("expected token issued five days ago with seven day ttl to be refreshed")
	}
}

func TestRefreshCloudTokenSchedulerTreatsMissingExpiryAsExpired(t *testing.T) {
	scheduler := &RefreshCloudTokenScheduler{}
	if !scheduler.isExpired(&models.CloudToken{}) {
		t.Fatal("expected token without expiry to be expired")
	}
}

func TestRefreshCloudTokenSchedulerMasksUsernameInRefreshLogs(t *testing.T) {
	core, logs := observer.New(zap.InfoLevel)
	logger := zap.New(core)
	baseCtx, cancel := context.WithCancel(context.Background())
	cloudTokenService := &mockRefreshCloudTokenService{
		passwordLoginTokens: []*models.CloudToken{
			{
				ID:        31,
				Name:      "token",
				Username:  "alice@example.com",
				Password:  "secret",
				ExpiresIn: 0,
			},
		},
		afterListPasswordLoginTokens: cancel,
	}
	scheduler := &RefreshCloudTokenScheduler{
		ctx:               appContext.NewContext(baseCtx, appContext.WithLogger(logger)),
		cloudTokenService: cloudTokenService,
	}

	if scheduler.doJob() {
		t.Fatal("expected scheduler job to stop after context cancellation")
	}

	entries := logs.FilterMessage("检测到已过期的令牌，尝试刷新").All()
	if len(entries) != 1 {
		t.Fatalf("expected one expired token log, got %d", len(entries))
	}

	logText := entries[0].Message + fmt.Sprint(entries[0].Context)
	if strings.Contains(logText, "alice@example.com") {
		t.Fatalf("expected raw username to be masked from log %s", logText)
	}

	if !strings.Contains(logText, utils.MaskSecret("alice@example.com")) {
		t.Fatalf("expected masked username in log %s", logText)
	}
}

func TestRefreshCloudTokenSchedulerStartRejectsMissingService(t *testing.T) {
	scheduler := NewRefreshCloudTokenScheduler(nil)
	ctx := appContext.NewContext(context.Background(), appContext.WithLogger(zap.NewNop()))

	if err := scheduler.Start(ctx); !errors.Is(err, ErrSchedulerCloudTokenServiceMissing) {
		t.Fatalf("expected missing cloud token service error, got %v", err)
	}
}

func TestRefreshCloudTokenSchedulerStopWaitsForRunningJob(t *testing.T) {
	entered := make(chan struct{}, 1)
	release := make(chan struct{})
	releaseClosed := false

	closeRelease := func() {
		if !releaseClosed {
			close(release)

			releaseClosed = true
		}
	}
	defer closeRelease()

	cloudTokenService := &mockRefreshCloudTokenService{
		listPasswordLoginEntered: entered,
		releaseListPasswordLogin: release,
	}

	scheduler, ok := NewRefreshCloudTokenScheduler(cloudTokenService).(*RefreshCloudTokenScheduler)
	if !ok {
		t.Fatal("expected refresh cloud token scheduler")
	}

	scheduler.refreshInterval = time.Hour

	ctx := appContext.NewContext(context.Background(), appContext.WithLogger(zap.NewNop()))
	if err := scheduler.Start(ctx); err != nil {
		t.Fatalf("start scheduler: %v", err)
	}

	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for scheduler job to start")
	}

	stopDone := make(chan struct{})

	go func() {
		scheduler.Stop()
		close(stopDone)
	}()

	select {
	case <-stopDone:
		closeRelease()
		t.Fatal("expected Stop to wait for running job")
	case <-time.After(50 * time.Millisecond):
	}

	closeRelease()

	select {
	case <-stopDone:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for scheduler to stop")
	}

	if err := scheduler.Start(ctx); err != nil {
		t.Fatalf("restart scheduler after stop: %v", err)
	}

	scheduler.Stop()
}

func TestRefreshCloudTokenSchedulerDoJobWaitsAfterImmediateFirstRun(t *testing.T) {
	baseCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cloudTokenService := &mockRefreshCloudTokenService{}
	scheduler := &RefreshCloudTokenScheduler{
		ctx:               appContext.NewContext(baseCtx, appContext.WithLogger(zap.NewNop())),
		cloudTokenService: cloudTokenService,
		refreshInterval:   time.Hour,
	}

	if !scheduler.doJob() {
		t.Fatal("expected first scheduler job to complete and continue")
	}

	if got := cloudTokenService.listPasswordLoginCalls.Load(); got != 1 {
		t.Fatalf("expected first job to query tokens once, got %d", got)
	}

	done := make(chan bool, 1)
	go func() {
		done <- scheduler.doJob()
	}()

	select {
	case keepRunning := <-done:
		t.Fatalf("expected second job to wait for refresh interval, returned %v", keepRunning)
	case <-time.After(50 * time.Millisecond):
	}

	cancel()

	select {
	case keepRunning := <-done:
		if keepRunning {
			t.Fatal("expected second scheduler job to stop after context cancellation")
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for delayed scheduler job to stop")
	}

	if got := cloudTokenService.listPasswordLoginCalls.Load(); got != 1 {
		t.Fatalf("expected delayed job not to query tokens after cancellation, got %d calls", got)
	}
}

func TestRefreshCloudTokenSchedulerDoJobContinuesAfterRecoveredPanic(t *testing.T) {
	core, logs := observer.New(zap.ErrorLevel)
	cloudTokenService := &mockRefreshCloudTokenService{
		panicOnListPasswordLogin: true,
	}
	scheduler := &RefreshCloudTokenScheduler{
		ctx:               appContext.NewContext(context.Background(), appContext.WithLogger(zap.New(core))),
		cloudTokenService: cloudTokenService,
	}

	if !scheduler.doJob() {
		t.Fatal("expected scheduler job to continue after recovered panic")
	}

	entries := logs.FilterMessage("云盘令牌刷新执行器发生异常").All()
	if len(entries) != 1 {
		t.Fatalf("expected one recovered panic log, got %d", len(entries))
	}

	panicText, ok := entries[0].ContextMap()["panic"].(string)
	if !ok {
		t.Fatalf("expected string panic field, got %#v", entries[0].ContextMap())
	}

	for _, leaked := range []string{"proxy-user", "proxy-pass", "secret-access", "secret-password"} {
		if strings.Contains(panicText, leaked) {
			t.Fatalf("expected %q to be redacted from %q", leaked, panicText)
		}
	}
}

func TestRefreshCloudTokenSchedulerSkipsNilToken(t *testing.T) {
	baseCtx, cancel := context.WithCancel(context.Background())
	cloudTokenService := &mockRefreshCloudTokenService{
		passwordLoginTokens: []*models.CloudToken{
			nil,
			{
				ID:        31,
				Name:      "token",
				Username:  "alice@example.com",
				Password:  "secret",
				ExpiresIn: 0,
			},
		},
		afterListPasswordLoginTokens: cancel,
	}
	scheduler := &RefreshCloudTokenScheduler{
		ctx:               appContext.NewContext(baseCtx, appContext.WithLogger(zap.NewNop())),
		cloudTokenService: cloudTokenService,
	}

	if scheduler.doJob() {
		t.Fatal("expected scheduler job to stop after context cancellation")
	}

	if cloudTokenService.usernameLoginReq == nil {
		t.Fatal("expected valid token to be refreshed after nil token is skipped")
	}

	if cloudTokenService.usernameLoginReq.ID != 31 {
		t.Fatalf("expected valid token 31 to be refreshed, got %d", cloudTokenService.usernameLoginReq.ID)
	}
}

func TestRefreshCloudTokenSchedulerHasTooManyFailuresParsesStoredTypes(t *testing.T) {
	scheduler := &RefreshCloudTokenScheduler{}

	tests := []struct {
		name        string
		value       interface{}
		wantTimes   int
		wantTooMany bool
	}{
		{
			name:        "int",
			value:       3,
			wantTimes:   3,
			wantTooMany: true,
		},
		{
			name:        "int64",
			value:       int64(3),
			wantTimes:   3,
			wantTooMany: true,
		},
		{
			name:        "float64",
			value:       float64(3),
			wantTimes:   3,
			wantTooMany: true,
		},
		{
			name:        "json number",
			value:       json.Number("3"),
			wantTimes:   3,
			wantTooMany: true,
		},
		{
			name:        "string",
			value:       "3",
			wantTimes:   3,
			wantTooMany: true,
		},
		{
			name:        "trimmed string",
			value:       " 3 ",
			wantTimes:   3,
			wantTooMany: true,
		},
		{
			name:        "below limit",
			value:       "2",
			wantTimes:   2,
			wantTooMany: false,
		},
		{
			name:        "negative",
			value:       int64(-1),
			wantTimes:   0,
			wantTooMany: false,
		},
		{
			name:        "invalid",
			value:       "failed",
			wantTimes:   0,
			wantTooMany: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			token := &models.CloudToken{
				Addition: datatypes.JSONMap{
					models.CloudTokenAdditionAutoLoginTimes: tt.value,
				},
			}

			if got := cloudTokenAutoLoginTimes(token); got != tt.wantTimes {
				t.Fatalf("expected auto login times %d, got %d", tt.wantTimes, got)
			}

			if got := scheduler.hasTooManyFailures(token); got != tt.wantTooMany {
				t.Fatalf("expected hasTooManyFailures %v, got %v", tt.wantTooMany, got)
			}
		})
	}
}

func TestRefreshCloudTokenSchedulerRecordFailureIncrementsStoredTypes(t *testing.T) {
	tests := []struct {
		name string
		from interface{}
		want int
	}{
		{
			name: "int",
			from: 2,
			want: 3,
		},
		{
			name: "int64",
			from: int64(2),
			want: 3,
		},
		{
			name: "float64",
			from: float64(2),
			want: 3,
		},
		{
			name: "json number",
			from: json.Number("2"),
			want: 3,
		},
		{
			name: "string",
			from: "2",
			want: 3,
		},
		{
			name: "negative",
			from: -1,
			want: 1,
		},
		{
			name: "invalid",
			from: "failed",
			want: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cloudTokenService := &mockRefreshCloudTokenService{}
			scheduler := &RefreshCloudTokenScheduler{
				ctx:               appContext.NewContext(context.Background(), appContext.WithLogger(zap.NewNop())),
				cloudTokenService: cloudTokenService,
			}
			token := &models.CloudToken{
				ID: 21,
				Addition: datatypes.JSONMap{
					models.CloudTokenAdditionAutoLoginTimes: tt.from,
				},
			}

			scheduler.recordFailure(token)

			if got := token.Addition[models.CloudTokenAdditionAutoLoginTimes]; got != tt.want {
				t.Fatalf("expected token addition times %d, got %#v", tt.want, got)
			}

			if got := cloudTokenService.updatedAddition[models.CloudTokenAdditionAutoLoginTimes]; got != tt.want {
				t.Fatalf("expected updated addition times %d, got %#v", tt.want, got)
			}

			if got, ok := cloudTokenService.updatedAddition[models.CloudTokenAdditionAutoLoginResultKey].(string); !ok || got == "" {
				t.Fatalf("expected updated addition result string, got %#v", cloudTokenService.updatedAddition)
			}
		})
	}
}

func TestRefreshCloudTokenSchedulerRecordFailureInitializesNilAddition(t *testing.T) {
	cloudTokenService := &mockRefreshCloudTokenService{}
	scheduler := &RefreshCloudTokenScheduler{
		ctx:               appContext.NewContext(context.Background(), appContext.WithLogger(zap.NewNop())),
		cloudTokenService: cloudTokenService,
	}
	token := &models.CloudToken{ID: 22}

	scheduler.recordFailure(token)

	if token.Addition == nil {
		t.Fatal("expected token addition to be initialized")
	}

	if got := token.Addition[models.CloudTokenAdditionAutoLoginTimes]; got != 1 {
		t.Fatalf("expected token addition times 1, got %#v", got)
	}

	if got := cloudTokenService.updatedAddition[models.CloudTokenAdditionAutoLoginTimes]; got != 1 {
		t.Fatalf("expected updated addition times 1, got %#v", got)
	}
}

func TestRefreshTokenKeepsIssuedAtWhenResettingFailures(t *testing.T) {
	cloudTokenService := &mockRefreshCloudTokenService{}
	scheduler := &RefreshCloudTokenScheduler{
		ctx:               appContext.NewContext(context.Background(), appContext.WithLogger(zap.NewNop())),
		cloudTokenService: cloudTokenService,
	}
	token := &models.CloudToken{
		ID:       12,
		Name:     "token",
		Username: "alice",
		Password: "secret",
	}

	if err := scheduler.refreshToken(scheduler.ctx, token); err != nil {
		t.Fatalf("refresh token: %v", err)
	}

	scheduler.resetFailureCount(token)

	if cloudTokenService.usernameLoginReq == nil {
		t.Fatal("expected username login to be called")
	}

	issuedAt, ok := cloudTokenService.updatedAddition[models.CloudTokenAdditionTokenIssuedAt].(int64)
	if !ok || issuedAt <= 0 {
		t.Fatalf("expected reset addition to keep token issued time, got %#v", cloudTokenService.updatedAddition)
	}
}
