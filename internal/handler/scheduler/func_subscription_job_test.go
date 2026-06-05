package scheduler

import (
	stdContext "context"
	"errors"
	"testing"
	"time"

	"github.com/xxcheng123/cloudpan189-share/internal/framework/context"
	"github.com/xxcheng123/cloudpan189-share/internal/services/subscription"
	"go.uber.org/zap"
)

type mockSubscriptionJobService struct {
	subscription.Service
	runCount int
	err      error
}

func (m *mockSubscriptionJobService) RunSubscriptionJob() error {
	m.runCount++

	return m.err
}

func TestSubscriptionSchedulerStartRejectsMissingService(t *testing.T) {
	scheduler := NewSubscriptionScheduler(nil)
	ctx := context.NewContext(stdContext.Background(), context.WithLogger(zap.NewNop()))

	if err := scheduler.Start(ctx); !errors.Is(err, ErrSchedulerSubscriptionServiceMissing) {
		t.Fatalf("expected missing subscription service error, got %v", err)
	}
}

func TestSubscriptionSchedulerStartRejectsTypedNilService(t *testing.T) {
	var subscriptionSvc *mockSubscriptionJobService

	scheduler := NewSubscriptionScheduler(subscriptionSvc)
	ctx := context.NewContext(stdContext.Background(), context.WithLogger(zap.NewNop()))

	if err := scheduler.Start(ctx); !errors.Is(err, ErrSchedulerSubscriptionServiceMissing) {
		t.Fatalf("expected missing subscription service error, got %v", err)
	}
}

func TestSubscriptionSchedulerUpdateConfigChangesCron(t *testing.T) {
	scheduler := NewSubscriptionScheduler(&mockSubscriptionJobService{})
	scheduler.UpdateConfig(subscription.SubscriptionConfig{CronExpression: "0 4 * * *"})

	after := time.Date(2026, 6, 3, 3, 0, 0, 0, time.UTC)
	nextRunAt := scheduler.computeNextRun(after)
	expected := time.Date(2026, 6, 3, 4, 0, 0, 0, time.UTC)

	if !nextRunAt.Equal(expected) {
		t.Fatalf("expected next run at %s, got %s", expected, nextRunAt)
	}
}

func TestSubscriptionSchedulerUpdateConfigBlankCronUsesDefault(t *testing.T) {
	scheduler := NewSubscriptionScheduler(&mockSubscriptionJobService{})
	scheduler.UpdateConfig(subscription.SubscriptionConfig{CronExpression: " "})

	after := time.Date(2026, 6, 3, 1, 0, 0, 0, time.UTC)
	nextRunAt := scheduler.computeNextRun(after)
	expected := time.Date(2026, 6, 3, 2, 0, 0, 0, time.UTC)

	if !nextRunAt.Equal(expected) {
		t.Fatalf("expected next run at %s, got %s", expected, nextRunAt)
	}
}

func TestSubscriptionSchedulerUpdateConfigRecomputesRunningNextRun(t *testing.T) {
	scheduler := NewSubscriptionScheduler(&mockSubscriptionJobService{})
	scheduler.ctx = context.NewContext(stdContext.Background(), context.WithLogger(zap.NewNop()))
	scheduler.running = true
	scheduler.nextRunAt = time.Now().Add(24 * time.Hour)

	before := time.Now()

	scheduler.UpdateConfig(subscription.SubscriptionConfig{CronExpression: "*/5 * * * *"})

	scheduler.mu.Lock()
	defer scheduler.mu.Unlock()

	if scheduler.cronExpr != "*/5 * * * *" {
		t.Fatalf("expected cron expression updated, got %q", scheduler.cronExpr)
	}

	if scheduler.nextRunAt.Before(before) || scheduler.nextRunAt.After(before.Add(6*time.Minute)) {
		t.Fatalf("expected next run to be recomputed within 5 minutes, got %s", scheduler.nextRunAt)
	}
}

func TestSubscriptionSchedulerDefaultsToDisabled(t *testing.T) {
	subscriptionSvc := &mockSubscriptionJobService{}
	scheduler := NewSubscriptionScheduler(subscriptionSvc)
	scheduler.ctx = context.NewContext(stdContext.Background(), context.WithLogger(zap.NewNop()))
	scheduler.running = true
	scheduler.nextRunAt = time.Now().Add(-time.Minute)
	scheduler.tick()

	if subscriptionSvc.runCount != 0 {
		t.Fatalf("expected default scheduler not to run job, got %d runs", subscriptionSvc.runCount)
	}
}

func TestSubscriptionSchedulerSkipsJobWhenDisabled(t *testing.T) {
	subscriptionSvc := &mockSubscriptionJobService{}
	scheduler := NewSubscriptionScheduler(subscriptionSvc)
	scheduler.ctx = context.NewContext(stdContext.Background(), context.WithLogger(zap.NewNop()))
	scheduler.running = true
	scheduler.nextRunAt = time.Now().Add(-time.Minute)

	scheduler.UpdateConfig(subscription.SubscriptionConfig{
		Enabled:        false,
		CronExpression: "*/5 * * * *",
	})
	scheduler.nextRunAt = time.Now().Add(-time.Minute)
	scheduler.tick()

	if subscriptionSvc.runCount != 0 {
		t.Fatalf("expected disabled scheduler not to run job, got %d runs", subscriptionSvc.runCount)
	}
}

func TestSubscriptionSchedulerRunsJobWhenEnabled(t *testing.T) {
	subscriptionSvc := &mockSubscriptionJobService{}
	scheduler := NewSubscriptionScheduler(subscriptionSvc)
	scheduler.ctx = context.NewContext(stdContext.Background(), context.WithLogger(zap.NewNop()))
	scheduler.running = true
	scheduler.nextRunAt = time.Now().Add(-time.Minute)

	scheduler.UpdateConfig(subscription.SubscriptionConfig{
		Enabled:        true,
		CronExpression: "*/5 * * * *",
	})
	scheduler.nextRunAt = time.Now().Add(-time.Minute)
	scheduler.tick()

	if subscriptionSvc.runCount != 1 {
		t.Fatalf("expected enabled scheduler to run job once, got %d runs", subscriptionSvc.runCount)
	}
}
