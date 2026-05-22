package autoingestlog

import (
	stdctx "context"
	"errors"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/xxcheng123/cloudpan189-share/internal/bootstrap"
	"github.com/xxcheng123/cloudpan189-share/internal/framework/context"
	"github.com/xxcheng123/cloudpan189-share/internal/pkgs/taskengine"
	"github.com/xxcheng123/cloudpan189-share/internal/repository/models"
	"github.com/xxcheng123/cloudpan189-share/internal/types/autoingest"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type autoIngestLogTestDB struct {
	db *gorm.DB
}

func (t *autoIngestLogTestDB) GetDB(ctx context.Context) *gorm.DB {
	return t.db.WithContext(ctx)
}

func (t *autoIngestLogTestDB) GetDBWithoutContext() *gorm.DB {
	return t.db
}

func (t *autoIngestLogTestDB) GetLogger(name string, fields ...zap.Field) *zap.Logger {
	return zap.NewNop()
}

func (t *autoIngestLogTestDB) Close() {}

func (t *autoIngestLogTestDB) GetPort() int {
	return 9999
}

func (t *autoIngestLogTestDB) GetTaskEngine() taskengine.TaskEngine {
	return nil
}

func (t *autoIngestLogTestDB) GetHTTPEngine() *gin.Engine {
	return nil
}

var _ bootstrap.ServiceContext = (*autoIngestLogTestDB)(nil)

func setupAutoIngestLogTestDB(t *testing.T) *autoIngestLogTestDB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}

	if err := db.AutoMigrate(&models.AutoIngestLog{}); err != nil {
		t.Fatalf("migrate test db: %v", err)
	}

	return &autoIngestLogTestDB{db: db}
}

func createAutoIngestLog(t *testing.T, db *gorm.DB, planID int64, level autoingest.LogLevel) *models.AutoIngestLog {
	t.Helper()

	log := &models.AutoIngestLog{
		PlanId:  planID,
		Level:   level,
		Content: "test log",
	}
	if err := db.Create(log).Error; err != nil {
		t.Fatalf("create auto ingest log: %v", err)
	}

	return log
}

func createAutoIngestLogWithCreatedAt(
	t *testing.T,
	db *gorm.DB,
	planID int64,
	level autoingest.LogLevel,
	createdAt time.Time,
) *models.AutoIngestLog {
	t.Helper()

	log := &models.AutoIngestLog{
		PlanId:    planID,
		Level:     level,
		Content:   "test log",
		CreatedAt: createdAt,
		UpdatedAt: createdAt,
	}
	if err := db.Create(log).Error; err != nil {
		t.Fatalf("create auto ingest log: %v", err)
	}

	return log
}

func countAutoIngestLogs(t *testing.T, db *gorm.DB, query string, args ...any) int64 {
	t.Helper()

	var count int64
	if err := db.Model(&models.AutoIngestLog{}).Where(query, args...).Count(&count).Error; err != nil {
		t.Fatalf("count auto ingest logs: %v", err)
	}

	return count
}

func TestDeleteByPlanIdsRejectsInvalidPlanIDWithoutDeletingLogs(t *testing.T) {
	tDB := setupAutoIngestLogTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())

	log := createAutoIngestLog(t, tDB.db, 10, autoingest.LogLevelInfo)

	deleted, err := svc.DeleteByPlanIds(ctx, []int64{log.PlanId, 0})
	if !errors.Is(err, errInvalidAutoIngestLogPlanID) {
		t.Fatalf("expected invalid auto ingest log plan id, got %v", err)
	}

	if deleted != 0 {
		t.Fatalf("expected zero deleted rows, got %d", deleted)
	}

	if count := countAutoIngestLogs(t, tDB.db, "id = ?", log.ID); count != 1 {
		t.Fatalf("expected log to remain, got count %d", count)
	}
}

func TestDeleteByIdsRejectsInvalidIDWithoutDeletingLogs(t *testing.T) {
	tDB := setupAutoIngestLogTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())

	log := createAutoIngestLog(t, tDB.db, 10, autoingest.LogLevelInfo)

	deleted, err := svc.DeleteByIds(ctx, []int64{log.ID, -1})
	if !errors.Is(err, errInvalidAutoIngestLogID) {
		t.Fatalf("expected invalid auto ingest log id, got %v", err)
	}

	if deleted != 0 {
		t.Fatalf("expected zero deleted rows, got %d", deleted)
	}

	if count := countAutoIngestLogs(t, tDB.db, "id = ?", log.ID); count != 1 {
		t.Fatalf("expected log to remain, got count %d", count)
	}
}

func TestDeleteByIdsReturnsNotFoundWhenSomeIDsMissing(t *testing.T) {
	tDB := setupAutoIngestLogTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())

	log := createAutoIngestLog(t, tDB.db, 10, autoingest.LogLevelInfo)

	deleted, err := svc.DeleteByIds(ctx, []int64{log.ID, 99999})
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("expected record not found, got %v", err)
	}

	if deleted != 0 {
		t.Fatalf("expected zero deleted rows on not found, got %d", deleted)
	}

	if count := countAutoIngestLogs(t, tDB.db, "id = ?", log.ID); count != 1 {
		t.Fatalf("expected existing log to remain, got count %d", count)
	}
}

func TestDeleteErrorLogsByPlanIdRejectsInvalidPlanIDWithoutDeletingLogs(t *testing.T) {
	tDB := setupAutoIngestLogTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())

	log := createAutoIngestLog(t, tDB.db, 0, autoingest.LogLevelError)

	deleted, err := svc.DeleteErrorLogsByPlanId(ctx, 0)
	if !errors.Is(err, errInvalidAutoIngestLogPlanID) {
		t.Fatalf("expected invalid auto ingest log plan id, got %v", err)
	}

	if deleted != 0 {
		t.Fatalf("expected zero deleted rows, got %d", deleted)
	}

	if count := countAutoIngestLogs(t, tDB.db, "id = ?", log.ID); count != 1 {
		t.Fatalf("expected log to remain, got count %d", count)
	}
}

func TestDeleteByPlanIdsDeduplicatesIDs(t *testing.T) {
	tDB := setupAutoIngestLogTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())

	first := createAutoIngestLog(t, tDB.db, 10, autoingest.LogLevelInfo)
	second := createAutoIngestLog(t, tDB.db, 10, autoingest.LogLevelError)
	other := createAutoIngestLog(t, tDB.db, 20, autoingest.LogLevelInfo)

	deleted, err := svc.DeleteByPlanIds(ctx, []int64{10, 10})
	if err != nil {
		t.Fatalf("delete by plan ids: %v", err)
	}

	if deleted != 2 {
		t.Fatalf("expected two deleted rows, got %d", deleted)
	}

	if count := countAutoIngestLogs(t, tDB.db, "id IN ?", []int64{first.ID, second.ID}); count != 0 {
		t.Fatalf("expected plan logs deleted, got count %d", count)
	}

	if count := countAutoIngestLogs(t, tDB.db, "id = ?", other.ID); count != 1 {
		t.Fatalf("expected other log to remain, got count %d", count)
	}
}

func TestResolveAutoIngestLogCutoffRejectsInvalidDuration(t *testing.T) {
	tests := []string{"", "0s", "0h", "-1h", "bad"}

	for _, tt := range tests {
		t.Run(tt, func(t *testing.T) {
			if _, err := resolveAutoIngestLogCutoff(tt); err == nil {
				t.Fatal("expected invalid duration error")
			}
		})
	}
}

func TestClearByDurationReturnsDeletedCount(t *testing.T) {
	tDB := setupAutoIngestLogTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())
	now := time.Now()

	oldLog := createAutoIngestLogWithCreatedAt(t, tDB.db, 10, autoingest.LogLevelInfo, now.AddDate(0, 0, -10))
	recentLog := createAutoIngestLogWithCreatedAt(t, tDB.db, 10, autoingest.LogLevelError, now.AddDate(0, 0, -1))

	deleted, err := svc.ClearByDuration(ctx, "7d")
	if err != nil {
		t.Fatalf("clear by duration: %v", err)
	}

	if deleted != 1 {
		t.Fatalf("expected one deleted row, got %d", deleted)
	}

	if count := countAutoIngestLogs(t, tDB.db, "id = ?", oldLog.ID); count != 0 {
		t.Fatalf("expected old log deleted, got count %d", count)
	}

	if count := countAutoIngestLogs(t, tDB.db, "id = ?", recentLog.ID); count != 1 {
		t.Fatalf("expected recent log to remain, got count %d", count)
	}
}
