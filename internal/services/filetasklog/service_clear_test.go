package filetasklog

import (
	stdctx "context"
	"testing"
	"time"

	"github.com/xxcheng123/cloudpan189-share/internal/framework/context"
	"github.com/xxcheng123/cloudpan189-share/internal/repository/models"
)

func TestResolveCutoffRejectsEmptyAndNonPositiveDuration(t *testing.T) {
	tests := []string{"", "0s", "0h", "-1h"}

	for _, tt := range tests {
		t.Run(tt, func(t *testing.T) {
			if _, err := resolveCutoff(tt); err == nil {
				t.Fatal("expected invalid duration error")
			}
		})
	}
}

func TestResolveCutoffAcceptsPositiveStandardDuration(t *testing.T) {
	before := time.Now()

	cutoff, err := resolveCutoff("24h")
	if err != nil {
		t.Fatalf("resolve cutoff: %v", err)
	}

	after := time.Now()
	minCutoff := before.Add(-24 * time.Hour)
	maxCutoff := after.Add(-24 * time.Hour)

	if cutoff.Before(minCutoff) || cutoff.After(maxCutoff) {
		t.Fatalf("expected cutoff between %s and %s, got %s", minCutoff, maxCutoff, cutoff)
	}
}

func TestResolveCutoffAcceptsKnownDayShorthand(t *testing.T) {
	before := time.Now()

	cutoff, err := resolveCutoff("7d")
	if err != nil {
		t.Fatalf("resolve cutoff: %v", err)
	}

	after := time.Now()
	minCutoff := before.AddDate(0, 0, -7)
	maxCutoff := after.AddDate(0, 0, -7)

	if cutoff.Before(minCutoff) || cutoff.After(maxCutoff) {
		t.Fatalf("expected cutoff between %s and %s, got %s", minCutoff, maxCutoff, cutoff)
	}
}

func TestClearReturnsDeletedCount(t *testing.T) {
	tDB := setupFileTaskLogTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())

	for i := 0; i < 3; i++ {
		if _, err := svc.Create(ctx, "scan", "clear count"); err != nil {
			t.Fatalf("create task log: %v", err)
		}
	}

	count, err := svc.Clear(ctx)
	if err != nil {
		t.Fatalf("clear task logs: %v", err)
	}

	if count != 3 {
		t.Fatalf("expected deleted count 3, got %d", count)
	}
}

func TestClearByDurationReturnsDeletedCount(t *testing.T) {
	tDB := setupFileTaskLogTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())
	now := time.Now()

	logs := []*models.FileTaskLog{
		{
			Title:     "old",
			Type:      "scan",
			BeginAt:   now.AddDate(0, 0, -10),
			Status:    models.StatusCompleted,
			CreatedAt: now.AddDate(0, 0, -10),
			UpdatedAt: now.AddDate(0, 0, -10),
		},
		{
			Title:     "recent",
			Type:      "scan",
			BeginAt:   now.AddDate(0, 0, -1),
			Status:    models.StatusCompleted,
			CreatedAt: now.AddDate(0, 0, -1),
			UpdatedAt: now.AddDate(0, 0, -1),
		},
	}

	if err := tDB.db.Create(&logs).Error; err != nil {
		t.Fatalf("create task logs: %v", err)
	}

	count, err := svc.ClearByDuration(ctx, "7d")
	if err != nil {
		t.Fatalf("clear by duration: %v", err)
	}

	if count != 1 {
		t.Fatalf("expected deleted count 1, got %d", count)
	}

	var remaining int64
	if err := tDB.db.Model(&models.FileTaskLog{}).Count(&remaining).Error; err != nil {
		t.Fatalf("count remaining logs: %v", err)
	}

	if remaining != 1 {
		t.Fatalf("expected one remaining log, got %d", remaining)
	}
}
