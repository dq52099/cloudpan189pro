package scheduler

import (
	"context"
	"errors"
	"testing"

	appContext "github.com/xxcheng123/cloudpan189-share/internal/framework/context"
	"github.com/xxcheng123/cloudpan189-share/internal/repository/models"
	autoingestlogSvi "github.com/xxcheng123/cloudpan189-share/internal/services/autoingestlog"
	autoingestplanSvi "github.com/xxcheng123/cloudpan189-share/internal/services/autoingestplan"
	"github.com/xxcheng123/cloudpan189-share/internal/types/autoingest"
	"go.uber.org/zap"
)

type mockAutoIngestRefreshPlanService struct {
	autoingestplanSvi.Service
}

type mockAutoIngestRefreshLogService struct {
	autoingestlogSvi.Service
}

func TestAutoIngestRefreshSchedulerStartRejectsMissingTaskEngine(t *testing.T) {
	scheduler := NewAutoIngestRefreshScheduler(
		nil,
		&mockAutoIngestRefreshPlanService{},
		&mockAutoIngestRefreshLogService{},
	)
	ctx := appContext.NewContext(context.Background(), appContext.WithLogger(zap.NewNop()))

	if err := scheduler.Start(ctx); !errors.Is(err, ErrSchedulerTaskEngineMissing) {
		t.Fatalf("expected missing task engine error, got %v", err)
	}
}

func TestAutoIngestRefreshSchedulerStartRejectsMissingPlanService(t *testing.T) {
	scheduler := NewAutoIngestRefreshScheduler(
		&mockRebuildStrmTaskEngine{},
		nil,
		&mockAutoIngestRefreshLogService{},
	)
	ctx := appContext.NewContext(context.Background(), appContext.WithLogger(zap.NewNop()))

	if err := scheduler.Start(ctx); !errors.Is(err, ErrSchedulerAutoIngestPlanServiceMissing) {
		t.Fatalf("expected missing auto ingest plan service error, got %v", err)
	}
}

func TestAutoIngestRefreshSchedulerStartRejectsMissingLogService(t *testing.T) {
	scheduler := NewAutoIngestRefreshScheduler(
		&mockRebuildStrmTaskEngine{},
		&mockAutoIngestRefreshPlanService{},
		nil,
	)
	ctx := appContext.NewContext(context.Background(), appContext.WithLogger(zap.NewNop()))

	if err := scheduler.Start(ctx); !errors.Is(err, ErrSchedulerAutoIngestLogServiceMissing) {
		t.Fatalf("expected missing auto ingest log service error, got %v", err)
	}
}

func TestAutoIngestRefreshSchedulerDispatchDuePlansSkipsNilPlan(t *testing.T) {
	engine := &mockRebuildStrmTaskEngine{}
	scheduler := &AutoIngestRefreshScheduler{taskEngine: engine}
	ctx := appContext.NewContext(context.Background(), appContext.WithLogger(zap.NewNop()))

	scheduler.dispatchDuePlans(ctx, []*models.AutoIngestPlan{
		nil,
		{
			ID:         21,
			Name:       "subscribe plan",
			SourceType: autoingest.SourceTypeSubscribe,
		},
	})

	if engine.count != 1 {
		t.Fatalf("expected nil plan to be skipped and valid plan queued, got %d", engine.count)
	}
}
