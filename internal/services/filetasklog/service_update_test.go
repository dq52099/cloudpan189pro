package filetasklog

import (
	stdctx "context"
	"errors"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/xxcheng123/cloudpan189-share/internal/bootstrap"
	"github.com/xxcheng123/cloudpan189-share/internal/framework/context"
	"github.com/xxcheng123/cloudpan189-share/internal/pkgs/taskengine"
	"github.com/xxcheng123/cloudpan189-share/internal/pkgs/utils"
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

type fileTaskLogCounterStub struct {
	name  string
	count int
}

func (c fileTaskLogCounterStub) Name() string {
	return c.name
}

func (c fileTaskLogCounterStub) Count() int {
	return c.count
}

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

func TestFlushCountRejectsNegativeCounters(t *testing.T) {
	tests := []struct {
		name    string
		counter Counter
	}{
		{name: "completed", counter: WithCompletedCounter(-1)},
		{name: "failed", counter: WithFailedCounter(-1)},
		{name: "total", counter: WithTotalCounter(-1)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tDB := setupFileTaskLogTestDB(t)
			svc := NewService(tDB)
			ctx := context.NewContext(stdctx.Background())

			tracker, err := svc.Create(ctx, "scan", "scan files")
			if err != nil {
				t.Fatalf("create task log: %v", err)
			}

			if err := svc.FlushCount(ctx, tracker, WithCompletedCounter(1), WithFailedCounter(1), WithTotalCounter(2)); err != nil {
				t.Fatalf("seed counters: %v", err)
			}

			err = svc.FlushCount(ctx, tracker, tt.counter)
			if !errors.Is(err, errInvalidFileTaskLogCounter) {
				t.Fatalf("expected invalid counter error, got %v", err)
			}

			var log models.FileTaskLog
			if err := tDB.db.First(&log, tracker.GetID()).Error; err != nil {
				t.Fatalf("query task log: %v", err)
			}

			if log.Completed != 1 || log.Failed != 1 || log.Total != 2 {
				t.Fatalf("expected counters unchanged, got completed=%d failed=%d total=%d", log.Completed, log.Failed, log.Total)
			}
		})
	}
}

func TestFlushCountRejectsUnknownCounters(t *testing.T) {
	tests := []struct {
		name    string
		counter Counter
	}{
		{name: "custom skipped", counter: fileTaskLogCounterStub{name: "skipped", count: 1}},
		{name: "processed constructor", counter: WithProcessedCounter(1)},
		{name: "nil counter", counter: nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tDB := setupFileTaskLogTestDB(t)
			svc := NewService(tDB)
			ctx := context.NewContext(stdctx.Background())

			tracker, err := svc.Create(ctx, "scan", "scan files")
			if err != nil {
				t.Fatalf("create task log: %v", err)
			}

			err = svc.FlushCount(ctx, tracker, tt.counter)
			if !errors.Is(err, errInvalidFileTaskLogCounter) {
				t.Fatalf("expected invalid counter error, got %v", err)
			}

			var log models.FileTaskLog
			if err := tDB.db.First(&log, tracker.GetID()).Error; err != nil {
				t.Fatalf("query task log: %v", err)
			}

			if log.Completed != 0 || log.Failed != 0 || log.Total != 0 {
				t.Fatalf("expected counters unchanged, got completed=%d failed=%d total=%d", log.Completed, log.Failed, log.Total)
			}
		})
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

func TestWithErrorRedactsSensitiveTextBeforePersisting(t *testing.T) {
	tDB := setupFileTaskLogTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())

	tracker, err := svc.Create(ctx, "scan", "scan files")
	if err != nil {
		t.Fatalf("create task log: %v", err)
	}

	rawErr := errors.New(`GET "https://proxy-user:proxy-pass@example.test/file?access_token=query-secret&filename=private-name.mkv#token=fragment-secret": accessCode=abcd Authorization: Bearer secret-token`)
	if err := svc.WithError(ctx, tracker, rawErr); err != nil {
		t.Fatalf("with error: %v", err)
	}

	var log models.FileTaskLog
	if err := tDB.db.First(&log, tracker.GetID()).Error; err != nil {
		t.Fatalf("query task log: %v", err)
	}

	assertSensitiveTaskLogTextRedacted(t, log.ErrorMsg, "proxy-user", "proxy-pass", "query-secret", "private-name.mkv", "fragment-secret", "abcd", "secret-token")
}

func TestCreateRedactsSensitiveDescBeforePersisting(t *testing.T) {
	tDB := setupFileTaskLogTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())

	rawDesc := `GET "https://proxy-user:proxy-pass@example.test/file?access_token=query-secret&filename=private-name.mkv#token=fragment-secret": accessCode=abcd Authorization: Bearer secret-token`

	tracker, err := svc.Create(ctx, "scan", "scan files", WithDesc(rawDesc))
	if err != nil {
		t.Fatalf("create task log: %v", err)
	}

	var log models.FileTaskLog
	if err := tDB.db.First(&log, tracker.GetID()).Error; err != nil {
		t.Fatalf("query task log: %v", err)
	}

	assertSensitiveTaskLogTextRedacted(t, log.Desc, "proxy-user", "proxy-pass", "query-secret", "private-name.mkv", "fragment-secret", "abcd", "secret-token")
}

func TestStatusFieldsRedactSensitiveTextBeforePersisting(t *testing.T) {
	tDB := setupFileTaskLogTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())

	tracker, err := svc.Create(ctx, "scan", "scan files")
	if err != nil {
		t.Fatalf("create task log: %v", err)
	}

	rawText := `GET "https://proxy-user:proxy-pass@example.test/file?access_token=query-secret&filename=private-name.mkv#token=fragment-secret": accessCode=abcd Authorization: Bearer secret-token`

	if err := svc.Running(ctx, tracker); err != nil {
		t.Fatalf("running task log: %v", err)
	}

	if err := svc.Failed(ctx, tracker, utils.WithField("result", rawText), utils.WithField("desc", rawText)); err != nil {
		t.Fatalf("fail task log: %v", err)
	}

	var log models.FileTaskLog
	if err := tDB.db.First(&log, tracker.GetID()).Error; err != nil {
		t.Fatalf("query task log: %v", err)
	}

	for _, text := range []string{log.Result, log.Desc} {
		assertSensitiveTaskLogTextRedacted(t, text, "proxy-user", "proxy-pass", "query-secret", "private-name.mkv", "fragment-secret", "abcd", "secret-token")
	}
}

func TestFailedWithReasonRedactsSensitiveTextBeforePersisting(t *testing.T) {
	tDB := setupFileTaskLogTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())

	tracker, err := svc.Create(ctx, "scan", "scan files")
	if err != nil {
		t.Fatalf("create task log: %v", err)
	}

	if err := svc.Running(ctx, tracker); err != nil {
		t.Fatalf("running task log: %v", err)
	}

	reason := `accessCode=abcd Authorization: Bearer secret-token`
	if err := svc.FailedWithReason(ctx, tracker, reason); err != nil {
		t.Fatalf("fail task log with reason: %v", err)
	}

	var log models.FileTaskLog
	if err := tDB.db.First(&log, tracker.GetID()).Error; err != nil {
		t.Fatalf("query task log: %v", err)
	}

	assertSensitiveTaskLogTextRedacted(t, log.Desc, "abcd", "secret-token")
}

func assertSensitiveTaskLogTextRedacted(t *testing.T, text string, leakedValues ...string) {
	t.Helper()

	for _, leaked := range leakedValues {
		if strings.Contains(text, leaked) {
			t.Fatalf("expected %q to be redacted from %q", leaked, text)
		}
	}

	if !strings.Contains(text, utils.RedactedSecret) {
		t.Fatalf("expected redacted marker in %q", text)
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

	if err := svc.Completed(ctx, failedTracker); !errors.Is(err, ErrFileTaskLogTerminalState) {
		t.Fatalf("expected late completed to report terminal state, got %v", err)
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

	if err := svc.Failed(ctx, completedTracker); !errors.Is(err, ErrFileTaskLogTerminalState) {
		t.Fatalf("expected late failed to report terminal state, got %v", err)
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
