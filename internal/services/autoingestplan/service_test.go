package autoingestplan

import (
	stdctx "context"
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

type testDB struct {
	db *gorm.DB
}

func (t *testDB) GetDB(ctx context.Context) *gorm.DB {
	return t.db.WithContext(ctx)
}

func (t *testDB) GetDBWithoutContext() *gorm.DB {
	return t.db
}

func (t *testDB) GetLogger(name string, fields ...zap.Field) *zap.Logger {
	return zap.NewNop()
}

func (t *testDB) Close() {}

func (t *testDB) GetPort() int {
	return 9999
}

func (t *testDB) GetTaskEngine() taskengine.TaskEngine {
	return nil
}

func (t *testDB) GetHTTPEngine() *gin.Engine {
	return nil
}

var _ bootstrap.ServiceContext = (*testDB)(nil)

func setupTestDB(t *testing.T) *testDB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("Failed to setup test DB: %v", err)
	}

	err = db.AutoMigrate(&models.AutoIngestPlan{})
	if err != nil {
		t.Fatalf("Failed to migrate: %v", err)
	}

	return &testDB{db: db}
}

func createTestPlans(t *testing.T, db *gorm.DB, count int) []int64 {
	var ids []int64
	for i := 0; i < count; i++ {
		plan := &models.AutoIngestPlan{
			Name:       "Test Plan",
			Enabled:    true,
			SourceType: autoingest.SourceTypeSubscribe,
			Offset:     int64(i + 1),
			ParentPath: "/test",
		}
		result := db.Create(plan)
		if result.Error != nil {
			t.Fatalf("Failed to create plan: %v", result.Error)
		}
		ids = append(ids, plan.ID)
	}
	return ids
}

func TestServiceCRUD(t *testing.T) {
	tDB := setupTestDB(t)
	svc := NewService(tDB)

	ctx := context.NewContext(stdctx.Background())

	plan := &models.AutoIngestPlan{
		Name:       "Test Plan",
		Enabled:    true,
		SourceType: autoingest.SourceTypeSubscribe,
		Offset:     1,
		ParentPath: "/test",
	}

	id, err := svc.Create(ctx, plan)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if id <= 0 {
		t.Fatal("Expected positive ID")
	}

	retrieved, err := svc.Query(ctx, id)
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}
	if retrieved.Name != "Test Plan" {
		t.Errorf("Expected name 'Test Plan', got '%s'", retrieved.Name)
	}

	err = svc.Delete(ctx, id)
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	_, err = svc.Query(ctx, id)
	if err == nil {
		t.Error("Expected error after delete")
	}
}

func TestServiceEnableDisable(t *testing.T) {
	tDB := setupTestDB(t)
	svc := NewService(tDB)

	ctx := context.NewContext(stdctx.Background())

	plan := &models.AutoIngestPlan{
		Name:       "Test Plan",
		Enabled:    true,
		SourceType: autoingest.SourceTypeSubscribe,
		Offset:     1,
		ParentPath: "/test",
	}
	svc.Create(ctx, plan)

	err := svc.Disable(ctx, plan.ID)
	if err != nil {
		t.Fatalf("Disable failed: %v", err)
	}

	retrieved, _ := svc.Query(ctx, plan.ID)
	if retrieved.Enabled {
		t.Error("Expected plan to be disabled")
	}

	err = svc.Enable(ctx, plan.ID)
	if err != nil {
		t.Fatalf("Enable failed: %v", err)
	}

	retrieved, _ = svc.Query(ctx, plan.ID)
	if !retrieved.Enabled {
		t.Error("Expected plan to be enabled")
	}
}

func TestServiceUpdateOffset(t *testing.T) {
	tDB := setupTestDB(t)
	svc := NewService(tDB)

	ctx := context.NewContext(stdctx.Background())

	plan := &models.AutoIngestPlan{
		Name:       "Test Plan",
		Enabled:    true,
		SourceType: autoingest.SourceTypeSubscribe,
		Offset:     1,
		ParentPath: "/test",
	}
	svc.Create(ctx, plan)

	err := svc.UpdateOffset(ctx, plan.ID, 100)
	if err != nil {
		t.Fatalf("UpdateOffset failed: %v", err)
	}

	retrieved, _ := svc.Query(ctx, plan.ID)
	if retrieved.Offset != 100 {
		t.Errorf("Expected offset 100, got %d", retrieved.Offset)
	}
}

func TestServiceCounters(t *testing.T) {
	tDB := setupTestDB(t)
	svc := NewService(tDB)

	ctx := context.NewContext(stdctx.Background())

	plan := &models.AutoIngestPlan{
		Name:        "Test Plan",
		Enabled:     true,
		SourceType:  autoingest.SourceTypeSubscribe,
		Offset:      1,
		ParentPath:  "/test",
		AddCount:    0,
		FailedCount: 0,
	}
	svc.Create(ctx, plan)

	svc.IncrAddCount(ctx, plan.ID, 5)
	svc.IncrFailedCount(ctx, plan.ID, 2)

	retrieved, _ := svc.Query(ctx, plan.ID)
	if retrieved.AddCount != 5 {
		t.Errorf("Expected AddCount 5, got %d", retrieved.AddCount)
	}
	if retrieved.FailedCount != 2 {
		t.Errorf("Expected FailedCount 2, got %d", retrieved.FailedCount)
	}

	svc.ResetCounters(ctx, plan.ID)

	retrieved, _ = svc.Query(ctx, plan.ID)
	if retrieved.AddCount != 0 {
		t.Errorf("Expected AddCount 0 after reset, got %d", retrieved.AddCount)
	}
	if retrieved.FailedCount != 0 {
		t.Errorf("Expected FailedCount 0 after reset, got %d", retrieved.FailedCount)
	}
}

func TestServiceListByIDs(t *testing.T) {
	tDB := setupTestDB(t)
	svc := NewService(tDB)

	ctx := context.NewContext(stdctx.Background())

	ids := createTestPlans(t, tDB.db, 3)

	plans, err := svc.ListByIDs(ctx, ids)
	if err != nil {
		t.Fatalf("ListByIDs failed: %v", err)
	}
	if len(plans) != 3 {
		t.Errorf("Expected 3 plans, got %d", len(plans))
	}

	plans, err = svc.ListByIDs(ctx, []int64{999, 998})
	if err != nil {
		t.Fatalf("ListByIDs with invalid IDs failed: %v", err)
	}
	if len(plans) != 0 {
		t.Errorf("Expected 0 plans for invalid IDs, got %d", len(plans))
	}
}

func TestServiceList(t *testing.T) {
	tDB := setupTestDB(t)
	svc := NewService(tDB)

	ctx := context.NewContext(stdctx.Background())

	createTestPlans(t, tDB.db, 10)

	req := &ListRequest{
		CurrentPage: 1,
		PageSize:    5,
	}

	plans, err := svc.List(ctx, req)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(plans) != 5 {
		t.Errorf("Expected 5 plans, got %d", len(plans))
	}

	count, err := svc.Count(ctx, req)
	if err != nil {
		t.Fatalf("Count failed: %v", err)
	}
	if count != 10 {
		t.Errorf("Expected count 10, got %d", count)
	}
}

func TestServiceFindDue(t *testing.T) {
	tDB := setupTestDB(t)
	svc := NewService(tDB)

	ctx := context.NewContext(stdctx.Background())

	plan := &models.AutoIngestPlan{
		Name:               "Due Plan",
		Enabled:            true,
		SourceType:         autoingest.SourceTypeSubscribe,
		Offset:             1,
		ParentPath:         "/test",
		AutoIngestInterval: 1,
	}
	svc.Create(ctx, plan)

	now := time.Now()
	plans, err := svc.FindDue(ctx, now)
	if err != nil {
		t.Fatalf("FindDue failed: %v", err)
	}
	if len(plans) != 1 {
		t.Errorf("Expected 1 due plan with interval=1, got %d", len(plans))
	}
}
