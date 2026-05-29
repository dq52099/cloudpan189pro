package filetasklog

import (
	stdctx "context"
	"errors"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/xxcheng123/cloudpan189-share/internal/bootstrap"
	"github.com/xxcheng123/cloudpan189-share/internal/framework/context"
	"github.com/xxcheng123/cloudpan189-share/internal/pkgs/taskengine"
	"github.com/xxcheng123/cloudpan189-share/internal/repository/models"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type fileTaskLogTestDB struct {
	db *gorm.DB
}

func (t *fileTaskLogTestDB) GetDB(ctx context.Context) *gorm.DB {
	return t.db.WithContext(ctx)
}

func (t *fileTaskLogTestDB) GetDBWithoutContext() *gorm.DB {
	return t.db
}

func (t *fileTaskLogTestDB) GetLogger(name string, fields ...zap.Field) *zap.Logger {
	return zap.NewNop()
}

func (t *fileTaskLogTestDB) Close() {}

func (t *fileTaskLogTestDB) GetPort() int {
	return 9999
}

func (t *fileTaskLogTestDB) GetTaskEngine() taskengine.TaskEngine {
	return nil
}

func (t *fileTaskLogTestDB) GetHTTPEngine() *gin.Engine {
	return nil
}

var _ bootstrap.ServiceContext = (*fileTaskLogTestDB)(nil)

func setupFileTaskLogTestDB(t *testing.T) *fileTaskLogTestDB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}

	if err := db.AutoMigrate(&models.FileTaskLog{}); err != nil {
		t.Fatalf("migrate test db: %v", err)
	}

	return &fileTaskLogTestDB{db: db}
}

func TestToggleStatusReturnsNotFoundWhenTaskLogMissing(t *testing.T) {
	tDB := setupFileTaskLogTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())

	err := svc.Running(ctx, LogID(99999))
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("expected record not found, got %v", err)
	}
}

func TestTaskLogUpdatesRejectInvalidKey(t *testing.T) {
	tDB := setupFileTaskLogTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())

	var nilTracker *Tracker

	tests := []struct {
		name string
		run  func() error
	}{
		{name: "running nil key", run: func() error { return svc.Running(ctx, nil) }},
		{name: "running typed nil key", run: func() error { return svc.Running(ctx, nilTracker) }},
		{name: "running zero id", run: func() error { return svc.Running(ctx, LogID(0)) }},
		{name: "flush count zero id", run: func() error { return svc.FlushCount(ctx, LogID(0), WithCompletedOneCounter()) }},
		{name: "with error zero id", run: func() error { return svc.WithError(ctx, LogID(0), errors.New("scan failed")) }},
		{name: "clear error zero id", run: func() error { return svc.ClearError(ctx, LogID(0)) }},
		{name: "failed with reason zero id", run: func() error { return svc.FailedWithReason(ctx, LogID(0), "failed") }},
		{name: "completed with progress zero id", run: func() error { return svc.CompletedWithProgress(ctx, LogID(0), 1, 2) }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.run()
			if !errors.Is(err, errInvalidFileTaskLogID) {
				t.Fatalf("expected invalid file task log id, got %v", err)
			}
		})
	}
}

func TestFlushCountWithoutCountersAllowsNilKey(t *testing.T) {
	tDB := setupFileTaskLogTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())

	if err := svc.FlushCount(ctx, nil); err != nil {
		t.Fatalf("expected no-op flush count without counters, got %v", err)
	}
}

func TestFlushCountReturnsNotFoundWhenTaskLogMissing(t *testing.T) {
	tDB := setupFileTaskLogTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())

	err := svc.FlushCount(ctx, LogID(99999), WithCompletedOneCounter())
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("expected record not found, got %v", err)
	}
}

func TestWithErrorReturnsNotFoundWhenTaskLogMissing(t *testing.T) {
	tDB := setupFileTaskLogTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())

	err := svc.WithError(ctx, LogID(99999), errors.New("scan failed"))
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("expected record not found, got %v", err)
	}
}

func TestClearErrorReturnsNotFoundWhenTaskLogMissing(t *testing.T) {
	tDB := setupFileTaskLogTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())

	err := svc.ClearError(ctx, LogID(99999))
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("expected record not found, got %v", err)
	}
}

func TestTaskLogUpdatesExistingRows(t *testing.T) {
	tDB := setupFileTaskLogTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())

	tracker, err := svc.Create(ctx, "scan", "scan files", WithFile(1001))
	if err != nil {
		t.Fatalf("create task log: %v", err)
	}

	if err := svc.Running(ctx, tracker); err != nil {
		t.Fatalf("running task log: %v", err)
	}

	if err := svc.FlushCount(ctx, tracker, WithCompletedOneCounter(), WithTotalCounter(3), WithFailedCounter(2)); err != nil {
		t.Fatalf("flush count: %v", err)
	}

	if err := svc.WithError(ctx, tracker, errors.New("temporary failure")); err != nil {
		t.Fatalf("with error: %v", err)
	}

	if err := svc.ClearError(ctx, tracker); err != nil {
		t.Fatalf("clear error: %v", err)
	}

	var log models.FileTaskLog
	if err := tDB.db.First(&log, tracker.GetID()).Error; err != nil {
		t.Fatalf("query task log: %v", err)
	}

	if log.Status != models.StatusRunning {
		t.Fatalf("expected status running, got %q", log.Status)
	}

	if log.Completed != 1 || log.Total != 3 || log.Failed != 2 {
		t.Fatalf("expected completed=1 total=3 failed=2, got completed=%d total=%d failed=%d", log.Completed, log.Total, log.Failed)
	}

	if log.ErrorMsg != "" {
		t.Fatalf("expected error message cleared, got %q", log.ErrorMsg)
	}
}

func TestRunningNoOpUpdateReturnsNil(t *testing.T) {
	tDB := setupFileTaskLogTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())

	tracker, err := svc.Create(ctx, "scan", "scan files")
	if err != nil {
		t.Fatalf("create task log: %v", err)
	}

	if err := svc.Running(ctx, tracker); err != nil {
		t.Fatalf("first running update: %v", err)
	}

	if err := svc.Running(ctx, tracker); err != nil {
		t.Fatalf("second running no-op update: %v", err)
	}
}

func TestTerminalTaskLogStatusDoesNotGetOverwritten(t *testing.T) {
	tDB := setupFileTaskLogTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())

	failedTracker, err := svc.Create(ctx, "scan", "failed scan")
	if err != nil {
		t.Fatalf("create failed task log: %v", err)
	}

	if err := svc.Running(ctx, failedTracker); err != nil {
		t.Fatalf("running failed task log: %v", err)
	}

	if err := svc.Failed(ctx, failedTracker); err != nil {
		t.Fatalf("fail task log: %v", err)
	}

	if err := svc.Completed(ctx, failedTracker); err != nil {
		t.Fatalf("late completed should be ignored without error: %v", err)
	}

	completedTracker, err := svc.Create(ctx, "scan", "completed scan")
	if err != nil {
		t.Fatalf("create completed task log: %v", err)
	}

	if err := svc.Running(ctx, completedTracker); err != nil {
		t.Fatalf("running completed task log: %v", err)
	}

	if err := svc.Completed(ctx, completedTracker); err != nil {
		t.Fatalf("complete task log: %v", err)
	}

	if err := svc.Failed(ctx, completedTracker); err != nil {
		t.Fatalf("late failed should be ignored without error: %v", err)
	}

	var failedLog models.FileTaskLog
	if err := tDB.db.First(&failedLog, failedTracker.GetID()).Error; err != nil {
		t.Fatalf("query failed task log: %v", err)
	}

	if failedLog.Status != models.StatusFailed {
		t.Fatalf("expected failed status to stay failed, got %q", failedLog.Status)
	}

	var completedLog models.FileTaskLog
	if err := tDB.db.First(&completedLog, completedTracker.GetID()).Error; err != nil {
		t.Fatalf("query completed task log: %v", err)
	}

	if completedLog.Status != models.StatusCompleted {
		t.Fatalf("expected completed status to stay completed, got %q", completedLog.Status)
	}
}

func TestFlushCountNoOpUpdateReturnsNil(t *testing.T) {
	tDB := setupFileTaskLogTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())

	tracker, err := svc.Create(ctx, "scan", "scan files")
	if err != nil {
		t.Fatalf("create task log: %v", err)
	}

	if err := svc.FlushCount(ctx, tracker, WithCompletedCounter(0), WithTotalCounter(0), WithFailedCounter(0)); err != nil {
		t.Fatalf("flush count no-op update: %v", err)
	}
}

func TestClearErrorNoOpUpdateReturnsNil(t *testing.T) {
	tDB := setupFileTaskLogTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())

	tracker, err := svc.Create(ctx, "scan", "scan files")
	if err != nil {
		t.Fatalf("create task log: %v", err)
	}

	if err := svc.ClearError(ctx, tracker); err != nil {
		t.Fatalf("clear empty error no-op update: %v", err)
	}
}

func TestCompleteIfProgressDoneReturnsNotFoundWhenTaskLogMissing(t *testing.T) {
	tDB := setupFileTaskLogTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())

	err := svc.CompleteIfProgressDone(ctx, LogID(99999))
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("expected record not found, got %v", err)
	}
}

func TestCompleteIfProgressDoneNoopsWhenProgressIncomplete(t *testing.T) {
	tDB := setupFileTaskLogTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())

	tracker, err := svc.Create(ctx, "batch", "batch delete")
	if err != nil {
		t.Fatalf("create task log: %v", err)
	}

	if err := svc.Running(ctx, tracker); err != nil {
		t.Fatalf("running task log: %v", err)
	}

	if err := svc.FlushCount(ctx, tracker, WithCompletedCounter(1), WithTotalCounter(2)); err != nil {
		t.Fatalf("flush count: %v", err)
	}

	if err := svc.CompleteIfProgressDone(ctx, tracker); err != nil {
		t.Fatalf("expected incomplete progress to be a no-op, got %v", err)
	}

	var log models.FileTaskLog
	if err := tDB.db.First(&log, tracker.GetID()).Error; err != nil {
		t.Fatalf("query task log: %v", err)
	}

	if log.Status != models.StatusRunning {
		t.Fatalf("expected status running, got %q", log.Status)
	}
}

func TestCompleteIfProgressDoneReturnsErrorWhenStatusDoesNotMatch(t *testing.T) {
	tDB := setupFileTaskLogTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())

	tracker, err := svc.Create(ctx, "batch", "batch delete")
	if err != nil {
		t.Fatalf("create task log: %v", err)
	}

	if err := svc.FlushCount(ctx, tracker, WithCompletedCounter(2), WithTotalCounter(2)); err != nil {
		t.Fatalf("flush count: %v", err)
	}

	err = svc.CompleteIfProgressDone(ctx, tracker)
	if !errors.Is(err, errFileTaskLogProgressNotDone) {
		t.Fatalf("expected progress not done error, got %v", err)
	}

	var log models.FileTaskLog
	if err := tDB.db.First(&log, tracker.GetID()).Error; err != nil {
		t.Fatalf("query task log: %v", err)
	}

	if log.Status != models.StatusPending {
		t.Fatalf("expected status pending, got %q", log.Status)
	}
}

func TestCompleteIfProgressDoneNoopsWhenFailedProgressIncomplete(t *testing.T) {
	tDB := setupFileTaskLogTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())

	tracker, err := svc.Create(ctx, "batch", "batch delete")
	if err != nil {
		t.Fatalf("create task log: %v", err)
	}

	if err := svc.Running(ctx, tracker); err != nil {
		t.Fatalf("running task log: %v", err)
	}

	if err := svc.FlushCount(ctx, tracker, WithFailedCounter(1), WithTotalCounter(2)); err != nil {
		t.Fatalf("flush count: %v", err)
	}

	if err := svc.CompleteIfProgressDone(ctx, tracker); err != nil {
		t.Fatalf("expected incomplete failed progress to be a no-op, got %v", err)
	}

	var log models.FileTaskLog
	if err := tDB.db.First(&log, tracker.GetID()).Error; err != nil {
		t.Fatalf("query task log: %v", err)
	}

	if log.Status != models.StatusRunning {
		t.Fatalf("expected status running, got %q", log.Status)
	}
}

func TestCompleteIfProgressDoneMarksCompleted(t *testing.T) {
	tDB := setupFileTaskLogTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())

	tracker, err := svc.Create(ctx, "batch", "batch delete")
	if err != nil {
		t.Fatalf("create task log: %v", err)
	}

	if err := svc.Running(ctx, tracker); err != nil {
		t.Fatalf("running task log: %v", err)
	}

	if err := svc.FlushCount(ctx, tracker, WithCompletedCounter(2), WithTotalCounter(2)); err != nil {
		t.Fatalf("flush count: %v", err)
	}

	if err := svc.CompleteIfProgressDone(ctx, tracker); err != nil {
		t.Fatalf("complete if progress done: %v", err)
	}

	var log models.FileTaskLog
	if err := tDB.db.First(&log, tracker.GetID()).Error; err != nil {
		t.Fatalf("query task log: %v", err)
	}

	if log.Status != models.StatusCompleted {
		t.Fatalf("expected status completed, got %q", log.Status)
	}
}

func TestCompleteIfProgressDoneMarksFailedWhenAnyItemFailed(t *testing.T) {
	tDB := setupFileTaskLogTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())

	tracker, err := svc.Create(ctx, "batch", "batch delete")
	if err != nil {
		t.Fatalf("create task log: %v", err)
	}

	if err := svc.Running(ctx, tracker); err != nil {
		t.Fatalf("running task log: %v", err)
	}

	if err := svc.FlushCount(ctx, tracker, WithCompletedCounter(1), WithFailedCounter(1), WithTotalCounter(2)); err != nil {
		t.Fatalf("flush count: %v", err)
	}

	if err := svc.CompleteIfProgressDone(ctx, tracker); err != nil {
		t.Fatalf("complete if progress done: %v", err)
	}

	var log models.FileTaskLog
	if err := tDB.db.First(&log, tracker.GetID()).Error; err != nil {
		t.Fatalf("query task log: %v", err)
	}

	if log.Status != models.StatusFailed {
		t.Fatalf("expected status failed, got %q", log.Status)
	}

	if log.Result != "批量任务部分失败" {
		t.Fatalf("expected partial failure result, got %q", log.Result)
	}
}
