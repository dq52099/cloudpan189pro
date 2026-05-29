package autoingestplan

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
	"github.com/xxcheng123/cloudpan189-share/internal/pkgs/utils"
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

	err = db.AutoMigrate(&models.AutoIngestPlan{}, &models.AutoIngestLog{})
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

	err = svc.Delete(ctx, &DeleteRequest{ID: id, IsAdmin: true})
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	_, err = svc.Query(ctx, id)
	if err == nil {
		t.Error("Expected error after delete")
	}
}

func TestCreatePersistsDisabledState(t *testing.T) {
	tDB := setupTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())

	plan := &models.AutoIngestPlan{
		Name:               "Disabled Plan",
		Enabled:            false,
		AutoIngestInterval: 30,
		SourceType:         autoingest.SourceTypeSubscribe,
		Offset:             1,
		ParentPath:         "/test",
		OnConflict:         autoingest.OnConflictRename,
		ConcurrentCount:    4,
		MaxRetryCount:      3,
		UserID:             10,
	}

	id, err := svc.Create(ctx, plan)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	if plan.Enabled {
		t.Fatal("expected returned plan to stay disabled")
	}

	var created models.AutoIngestPlan
	if err := tDB.db.First(&created, id).Error; err != nil {
		t.Fatalf("query created plan: %v", err)
	}

	if created.Enabled {
		t.Fatal("expected created plan to stay disabled")
	}
}

func TestQueryRejectsInvalidID(t *testing.T) {
	tDB := setupTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())

	for _, id := range []int64{0, -1} {
		if _, err := svc.Query(ctx, id); !errors.Is(err, errInvalidAutoIngestPlanID) {
			t.Fatalf("expected invalid plan id for %d, got %v", id, err)
		}
	}
}

func TestServiceDeleteReturnsNotFoundWhenAdminPlanMissing(t *testing.T) {
	tDB := setupTestDB(t)
	svc := NewService(tDB)

	ctx := context.NewContext(stdctx.Background())

	err := svc.Delete(ctx, &DeleteRequest{ID: 99999, IsAdmin: true})
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("expected record not found, got %v", err)
	}
}

func TestServiceDeleteReturnsNotFoundWhenOwnerPlanMissing(t *testing.T) {
	tDB := setupTestDB(t)
	svc := NewService(tDB)

	ctx := context.NewContext(stdctx.Background())

	err := svc.Delete(ctx, &DeleteRequest{ID: 99999, UserID: 10})
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("expected record not found, got %v", err)
	}
}

func TestServiceDeleteRemovesPlanLogs(t *testing.T) {
	tDB := setupTestDB(t)
	svc := NewService(tDB)

	ctx := context.NewContext(stdctx.Background())

	plan := &models.AutoIngestPlan{
		Name:       "Delete Logs",
		Enabled:    true,
		SourceType: autoingest.SourceTypeSubscribe,
		ParentPath: "/test",
		UserID:     10,
	}

	id, err := svc.Create(ctx, plan)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	log := &models.AutoIngestLog{
		PlanId:  id,
		Level:   autoingest.LogLevelInfo,
		Content: "created",
	}
	if err := tDB.db.Create(log).Error; err != nil {
		t.Fatalf("create plan log: %v", err)
	}

	if err := svc.Delete(ctx, &DeleteRequest{ID: id, UserID: 10}); err != nil {
		t.Fatalf("delete plan: %v", err)
	}

	var logCount int64
	if err := tDB.db.Model(&models.AutoIngestLog{}).Where("plan_id = ?", id).Count(&logCount).Error; err != nil {
		t.Fatalf("count plan logs: %v", err)
	}

	if logCount != 0 {
		t.Fatalf("expected plan logs deleted, got count %d", logCount)
	}
}

func TestServiceDeleteKeepsLogsWhenOwnerMismatch(t *testing.T) {
	tDB := setupTestDB(t)
	svc := NewService(tDB)

	ctx := context.NewContext(stdctx.Background())

	plan := &models.AutoIngestPlan{
		Name:       "Keep Logs",
		Enabled:    true,
		SourceType: autoingest.SourceTypeSubscribe,
		ParentPath: "/test",
		UserID:     20,
	}

	id, err := svc.Create(ctx, plan)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	log := &models.AutoIngestLog{
		PlanId:  id,
		Level:   autoingest.LogLevelError,
		Content: "error",
	}
	if err := tDB.db.Create(log).Error; err != nil {
		t.Fatalf("create plan log: %v", err)
	}

	err = svc.Delete(ctx, &DeleteRequest{ID: id, UserID: 10})
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("expected record not found, got %v", err)
	}

	var logCount int64
	if err := tDB.db.Model(&models.AutoIngestLog{}).Where("plan_id = ?", id).Count(&logCount).Error; err != nil {
		t.Fatalf("count plan logs: %v", err)
	}

	if logCount != 1 {
		t.Fatalf("expected plan log to remain, got count %d", logCount)
	}
}

func TestServiceDeleteRejectsInvalidID(t *testing.T) {
	tDB := setupTestDB(t)
	svc := NewService(tDB)

	ctx := context.NewContext(stdctx.Background())

	tests := []struct {
		name string
		req  *DeleteRequest
	}{
		{name: "nil request", req: nil},
		{name: "zero id", req: &DeleteRequest{ID: 0, IsAdmin: true}},
		{name: "negative id", req: &DeleteRequest{ID: -1, UserID: 10}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := svc.Delete(ctx, tt.req)
			if !errors.Is(err, errInvalidAutoIngestPlanID) {
				t.Fatalf("expected invalid auto ingest plan id, got %v", err)
			}
		})
	}
}

func TestServiceDeleteRejectsMissingUserIDForNonAdmin(t *testing.T) {
	tDB := setupTestDB(t)
	svc := NewService(tDB)

	ctx := context.NewContext(stdctx.Background())

	plan := &models.AutoIngestPlan{
		Name:       "Missing User",
		Enabled:    true,
		SourceType: autoingest.SourceTypeSubscribe,
		Offset:     1,
		ParentPath: "/test",
		UserID:     10,
	}

	id, err := svc.Create(ctx, plan)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	err = svc.Delete(ctx, &DeleteRequest{ID: id})
	if !errors.Is(err, errInvalidAutoIngestPlanUserID) {
		t.Fatalf("expected invalid auto ingest plan user id, got %v", err)
	}

	if _, err := svc.Query(ctx, id); err != nil {
		t.Fatalf("expected plan to remain, got %v", err)
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
	if _, err := svc.Create(ctx, plan); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

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
	if _, err := svc.Create(ctx, plan); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	err := svc.UpdateOffset(ctx, plan.ID, 100)
	if err != nil {
		t.Fatalf("UpdateOffset failed: %v", err)
	}

	retrieved, _ := svc.Query(ctx, plan.ID)
	if retrieved.Offset != 100 {
		t.Errorf("Expected offset 100, got %d", retrieved.Offset)
	}
}

func TestServiceUpdateNoOpExistingPlan(t *testing.T) {
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
	if _, err := svc.Create(ctx, plan); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	if err := svc.Update(ctx, plan.ID, utils.Field{Key: "name", Value: plan.Name}); err != nil {
		t.Fatalf("Update no-op failed: %v", err)
	}
}

func TestServiceUpdateReturnsNotFoundWhenPlanMissing(t *testing.T) {
	tDB := setupTestDB(t)
	svc := NewService(tDB)

	ctx := context.NewContext(stdctx.Background())

	err := svc.Update(ctx, 99999, utils.Field{Key: "name", Value: "missing"})
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("expected record not found, got %v", err)
	}
}

func TestServiceUpdateRejectsInvalidID(t *testing.T) {
	tDB := setupTestDB(t)
	svc := NewService(tDB)

	ctx := context.NewContext(stdctx.Background())

	err := svc.Update(ctx, 0, utils.Field{Key: "name", Value: "invalid"})
	if !errors.Is(err, errInvalidAutoIngestPlanID) {
		t.Fatalf("expected invalid auto ingest plan id, got %v", err)
	}
}

func TestServiceUpdateRejectsEmptyFields(t *testing.T) {
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
	if _, err := svc.Create(ctx, plan); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	err := svc.Update(ctx, plan.ID)
	if !errors.Is(err, errEmptyAutoIngestPlanUpdateFields) {
		t.Fatalf("expected empty auto ingest plan update fields, got %v", err)
	}

	retrieved, err := svc.Query(ctx, plan.ID)
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}

	if retrieved.Name != "Test Plan" {
		t.Fatalf("expected plan unchanged, got name %q", retrieved.Name)
	}
}

func TestServiceUpdateOffsetReturnsNotFoundWhenPlanMissing(t *testing.T) {
	tDB := setupTestDB(t)
	svc := NewService(tDB)

	ctx := context.NewContext(stdctx.Background())

	err := svc.UpdateOffset(ctx, 99999, 100)
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("expected record not found, got %v", err)
	}
}

func TestServiceUpdateOffsetNoOpExistingPlan(t *testing.T) {
	tDB := setupTestDB(t)
	svc := NewService(tDB)

	ctx := context.NewContext(stdctx.Background())

	plan := &models.AutoIngestPlan{
		Name:       "Test Plan",
		Enabled:    true,
		SourceType: autoingest.SourceTypeSubscribe,
		Offset:     100,
		ParentPath: "/test",
	}
	if _, err := svc.Create(ctx, plan); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	if err := svc.UpdateOffset(ctx, plan.ID, plan.Offset); err != nil {
		t.Fatalf("UpdateOffset no-op failed: %v", err)
	}
}

func TestServiceUpdateOffsetRejectsInvalidID(t *testing.T) {
	tDB := setupTestDB(t)
	svc := NewService(tDB)

	ctx := context.NewContext(stdctx.Background())

	err := svc.UpdateOffset(ctx, 0, 100)
	if !errors.Is(err, errInvalidAutoIngestPlanID) {
		t.Fatalf("expected invalid auto ingest plan id, got %v", err)
	}
}

func TestServiceUpdateOffsetRejectsNegativeOffset(t *testing.T) {
	tDB := setupTestDB(t)
	svc := NewService(tDB)

	ctx := context.NewContext(stdctx.Background())

	plan := &models.AutoIngestPlan{
		Name:       "Test Plan",
		Enabled:    true,
		SourceType: autoingest.SourceTypeSubscribe,
		Offset:     100,
		ParentPath: "/test",
	}
	if _, err := svc.Create(ctx, plan); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	err := svc.UpdateOffset(ctx, plan.ID, -1)
	if !errors.Is(err, errInvalidAutoIngestPlanOffset) {
		t.Fatalf("expected invalid auto ingest plan offset, got %v", err)
	}

	retrieved, err := svc.Query(ctx, plan.ID)
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}

	if retrieved.Offset != 100 {
		t.Fatalf("expected offset to remain 100, got %d", retrieved.Offset)
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
	if _, err := svc.Create(ctx, plan); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	if err := svc.IncrAddCount(ctx, plan.ID, 5); err != nil {
		t.Fatalf("IncrAddCount failed: %v", err)
	}

	if err := svc.IncrFailedCount(ctx, plan.ID, 2); err != nil {
		t.Fatalf("IncrFailedCount failed: %v", err)
	}

	retrieved, _ := svc.Query(ctx, plan.ID)
	if retrieved.AddCount != 5 {
		t.Errorf("Expected AddCount 5, got %d", retrieved.AddCount)
	}

	if retrieved.FailedCount != 2 {
		t.Errorf("Expected FailedCount 2, got %d", retrieved.FailedCount)
	}

	if err := svc.ResetCounters(ctx, plan.ID); err != nil {
		t.Fatalf("ResetCounters failed: %v", err)
	}

	retrieved, _ = svc.Query(ctx, plan.ID)
	if retrieved.AddCount != 0 {
		t.Errorf("Expected AddCount 0 after reset, got %d", retrieved.AddCount)
	}

	if retrieved.FailedCount != 0 {
		t.Errorf("Expected FailedCount 0 after reset, got %d", retrieved.FailedCount)
	}
}

func TestServiceCountersNoOpExistingPlan(t *testing.T) {
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
	if _, err := svc.Create(ctx, plan); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	if err := svc.IncrAddCount(ctx, plan.ID, 0); err != nil {
		t.Fatalf("IncrAddCount no-op failed: %v", err)
	}

	if err := svc.IncrFailedCount(ctx, plan.ID, 0); err != nil {
		t.Fatalf("IncrFailedCount no-op failed: %v", err)
	}

	if err := svc.ResetCounters(ctx, plan.ID); err != nil {
		t.Fatalf("ResetCounters no-op failed: %v", err)
	}

	retrieved, err := svc.Query(ctx, plan.ID)
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}

	if retrieved.AddCount != 0 {
		t.Fatalf("expected AddCount to remain 0, got %d", retrieved.AddCount)
	}

	if retrieved.FailedCount != 0 {
		t.Fatalf("expected FailedCount to remain 0, got %d", retrieved.FailedCount)
	}
}

func TestServiceCountersRejectNegativeDelta(t *testing.T) {
	tDB := setupTestDB(t)
	svc := NewService(tDB)

	ctx := context.NewContext(stdctx.Background())

	plan := &models.AutoIngestPlan{
		Name:        "Test Plan",
		Enabled:     true,
		SourceType:  autoingest.SourceTypeSubscribe,
		Offset:      1,
		ParentPath:  "/test",
		AddCount:    5,
		FailedCount: 2,
	}
	if _, err := svc.Create(ctx, plan); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	tests := []struct {
		name string
		run  func() error
	}{
		{name: "incr add", run: func() error { return svc.IncrAddCount(ctx, plan.ID, -1) }},
		{name: "incr failed", run: func() error { return svc.IncrFailedCount(ctx, plan.ID, -1) }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.run()
			if !errors.Is(err, errInvalidAutoIngestPlanCounterDelta) {
				t.Fatalf("expected invalid auto ingest plan counter delta, got %v", err)
			}
		})
	}

	retrieved, err := svc.Query(ctx, plan.ID)
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}

	if retrieved.AddCount != 5 {
		t.Fatalf("expected AddCount to remain 5, got %d", retrieved.AddCount)
	}

	if retrieved.FailedCount != 2 {
		t.Fatalf("expected FailedCount to remain 2, got %d", retrieved.FailedCount)
	}
}

func TestServiceCountersReturnNotFoundWhenPlanMissing(t *testing.T) {
	tDB := setupTestDB(t)
	svc := NewService(tDB)

	ctx := context.NewContext(stdctx.Background())

	tests := []struct {
		name string
		run  func() error
	}{
		{name: "incr add", run: func() error { return svc.IncrAddCount(ctx, 99999, 0) }},
		{name: "incr failed", run: func() error { return svc.IncrFailedCount(ctx, 99999, 0) }},
		{name: "reset", run: func() error { return svc.ResetCounters(ctx, 99999) }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.run()
			if !errors.Is(err, gorm.ErrRecordNotFound) {
				t.Fatalf("expected record not found, got %v", err)
			}
		})
	}
}

func TestServiceCountersRejectInvalidID(t *testing.T) {
	tDB := setupTestDB(t)
	svc := NewService(tDB)

	ctx := context.NewContext(stdctx.Background())

	tests := []struct {
		name string
		run  func() error
	}{
		{name: "incr add", run: func() error { return svc.IncrAddCount(ctx, 0, 1) }},
		{name: "incr failed", run: func() error { return svc.IncrFailedCount(ctx, 0, 1) }},
		{name: "reset", run: func() error { return svc.ResetCounters(ctx, 0) }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.run()
			if !errors.Is(err, errInvalidAutoIngestPlanID) {
				t.Fatalf("expected invalid auto ingest plan id, got %v", err)
			}
		})
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

func TestServiceListByIDsRejectsInvalidID(t *testing.T) {
	tDB := setupTestDB(t)
	svc := NewService(tDB)

	ctx := context.NewContext(stdctx.Background())

	_, err := svc.ListByIDs(ctx, []int64{1, 0})
	if !errors.Is(err, errInvalidAutoIngestPlanID) {
		t.Fatalf("expected invalid auto ingest plan id, got %v", err)
	}
}

func TestServiceListByIDsDeduplicatesIDs(t *testing.T) {
	tDB := setupTestDB(t)
	svc := NewService(tDB)

	ctx := context.NewContext(stdctx.Background())

	ids := createTestPlans(t, tDB.db, 2)

	plans, err := svc.ListByIDs(ctx, []int64{ids[0], ids[0], ids[1]})
	if err != nil {
		t.Fatalf("ListByIDs failed: %v", err)
	}

	if len(plans) != 2 {
		t.Fatalf("expected 2 plans, got %d", len(plans))
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
		IsAdmin:     true,
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

func TestServiceListRejectsMissingUserID(t *testing.T) {
	tDB := setupTestDB(t)
	svc := NewService(tDB)

	ctx := context.NewContext(stdctx.Background())

	createTestPlans(t, tDB.db, 12)

	tests := []struct {
		name string
		req  *ListRequest
	}{
		{name: "nil request", req: nil},
		{name: "empty request", req: &ListRequest{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := svc.List(ctx, tt.req)
			if !errors.Is(err, errInvalidAutoIngestPlanUserID) {
				t.Fatalf("expected invalid auto ingest plan user id, got %v", err)
			}
		})
	}

	_, err := svc.Count(ctx, nil)
	if !errors.Is(err, errInvalidAutoIngestPlanUserID) {
		t.Fatalf("expected invalid auto ingest plan user id for count, got %v", err)
	}
}

func TestServiceListDefaultsPaginationWhenPageValuesMissing(t *testing.T) {
	tDB := setupTestDB(t)
	svc := NewService(tDB)

	ctx := context.NewContext(stdctx.Background())

	createTestPlans(t, tDB.db, 12)

	req := &ListRequest{IsAdmin: true}

	plans, err := svc.List(ctx, req)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}

	if len(plans) != defaultAutoIngestPlanPageSize {
		t.Fatalf("expected default page size %d, got %d", defaultAutoIngestPlanPageSize, len(plans))
	}

	if req.CurrentPage != defaultAutoIngestPlanCurrentPage {
		t.Fatalf("expected current page normalized to %d, got %d", defaultAutoIngestPlanCurrentPage, req.CurrentPage)
	}

	if req.PageSize != defaultAutoIngestPlanPageSize {
		t.Fatalf("expected page size normalized to %d, got %d", defaultAutoIngestPlanPageSize, req.PageSize)
	}
}

func TestServiceListCapsPageSize(t *testing.T) {
	tDB := setupTestDB(t)
	svc := NewService(tDB)

	ctx := context.NewContext(stdctx.Background())

	createTestPlans(t, tDB.db, 3)

	req := &ListRequest{
		CurrentPage: 1,
		PageSize:    maxAutoIngestPlanPageSize + 100,
		IsAdmin:     true,
	}

	if _, err := svc.List(ctx, req); err != nil {
		t.Fatalf("List failed: %v", err)
	}

	if req.PageSize != maxAutoIngestPlanPageSize {
		t.Fatalf("expected page size capped to %d, got %d", maxAutoIngestPlanPageSize, req.PageSize)
	}
}

func TestServiceListNoPaginateKeepsAllRows(t *testing.T) {
	tDB := setupTestDB(t)
	svc := NewService(tDB)

	ctx := context.NewContext(stdctx.Background())

	createTestPlans(t, tDB.db, 12)

	plans, err := svc.List(ctx, &ListRequest{NoPaginate: true, IsAdmin: true})
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}

	if len(plans) != 12 {
		t.Fatalf("expected all rows with no paginate, got %d", len(plans))
	}
}

func TestServiceListRestrictsNonAdminPlans(t *testing.T) {
	tDB := setupTestDB(t)
	svc := NewService(tDB)

	ctx := context.NewContext(stdctx.Background())

	ownPlan := &models.AutoIngestPlan{
		Name:       "Own Plan",
		Enabled:    true,
		SourceType: autoingest.SourceTypeSubscribe,
		Offset:     1,
		ParentPath: "/own",
		UserID:     10,
	}
	if _, err := svc.Create(ctx, ownPlan); err != nil {
		t.Fatalf("Create own plan failed: %v", err)
	}

	otherPlan := &models.AutoIngestPlan{
		Name:       "Other Plan",
		Enabled:    true,
		SourceType: autoingest.SourceTypeSubscribe,
		Offset:     1,
		ParentPath: "/other",
		UserID:     20,
	}
	if _, err := svc.Create(ctx, otherPlan); err != nil {
		t.Fatalf("Create other plan failed: %v", err)
	}

	plans, err := svc.List(ctx, &ListRequest{UserID: 10, NoPaginate: true})
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}

	if len(plans) != 1 || plans[0].ID != ownPlan.ID {
		t.Fatalf("expected only own plan %d, got %+v", ownPlan.ID, plans)
	}

	count, err := svc.Count(ctx, &ListRequest{UserID: 10})
	if err != nil {
		t.Fatalf("Count failed: %v", err)
	}

	if count != 1 {
		t.Fatalf("expected own plan count 1, got %d", count)
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
	if _, err := svc.Create(ctx, plan); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	now := time.Now()

	plans, err := svc.FindDue(ctx, now)
	if err != nil {
		t.Fatalf("FindDue failed: %v", err)
	}

	if len(plans) != 1 {
		t.Errorf("Expected 1 due plan with interval=1, got %d", len(plans))
	}
}
