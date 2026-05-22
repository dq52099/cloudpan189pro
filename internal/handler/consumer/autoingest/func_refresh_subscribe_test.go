package autoingest

import (
	stdctx "context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	mysqlDriver "github.com/go-sql-driver/mysql"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/xxcheng123/cloudpan189-share/internal/framework/context"
	"github.com/xxcheng123/cloudpan189-share/internal/framework/taskcontext"
	"github.com/xxcheng123/cloudpan189-share/internal/pkgs/taskengine"
	"github.com/xxcheng123/cloudpan189-share/internal/pkgs/utils"
	"github.com/xxcheng123/cloudpan189-share/internal/repository/models"
	"github.com/xxcheng123/cloudpan189-share/internal/services/autoingestlog"
	"github.com/xxcheng123/cloudpan189-share/internal/services/autoingestplan"
	"github.com/xxcheng123/cloudpan189-share/internal/services/cloudbridge"
	"github.com/xxcheng123/cloudpan189-share/internal/services/storagefacade"
	"github.com/xxcheng123/cloudpan189-share/internal/services/virtualfile"
	"github.com/xxcheng123/cloudpan189-share/internal/types/autoingest"
	"github.com/xxcheng123/cloudpan189-share/internal/types/topic"
	"go.uber.org/zap"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type duplicateSQLiteCodeError struct {
	code int
}

func (e duplicateSQLiteCodeError) Error() string {
	return "sqlite constraint error"
}

func (e duplicateSQLiteCodeError) Code() int {
	return e.code
}

type mockRefreshSubscribeTaskEngine struct {
	taskengine.TaskEngine

	mu        sync.Mutex
	err       error
	pushCount int
}

func (m *mockRefreshSubscribeTaskEngine) PushMessage(ctx stdctx.Context, taskTopic taskengine.Topic, payload []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.pushCount++

	return m.err
}

func (m *mockRefreshSubscribeTaskEngine) Count() int {
	m.mu.Lock()
	defer m.mu.Unlock()

	return m.pushCount
}

func TestIsDuplicateEntryErrorRecognizesDriverErrors(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "mysql duplicate",
			err:  &mysqlDriver.MySQLError{Number: 1062, Message: "Duplicate entry"},
			want: true,
		},
		{
			name: "wrapped postgres unique violation",
			err:  fmt.Errorf("create storage: %w", &pgconn.PgError{Code: "23505", Message: "duplicate key value violates unique constraint"}),
			want: true,
		},
		{
			name: "sqlite unique constraint code",
			err:  duplicateSQLiteCodeError{code: 2067},
			want: true,
		},
		{
			name: "sqlite primary key constraint code",
			err:  fmt.Errorf("wrapped: %w", duplicateSQLiteCodeError{code: 1555}),
			want: true,
		},
		{
			name: "sqlite text fallback",
			err:  errors.New("UNIQUE constraint failed: virtual_files.parent_id, virtual_files.name"),
			want: true,
		},
		{
			name: "generic error",
			err:  errors.New("connection refused"),
			want: false,
		},
		{
			name: "nil",
			err:  nil,
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isDuplicateEntryError(tt.err); got != tt.want {
				t.Fatalf("expected %v, got %v", tt.want, got)
			}
		})
	}
}

type mockRefreshSubscribeCloudBridgeService struct {
	cloudbridge.Service

	items     []*cloudbridge.ShareResourceInfo
	pageCalls []int
}

func (m *mockRefreshSubscribeCloudBridgeService) GetSubscribeUserShareResource(
	ctx context.Context,
	userID string,
	opts ...cloudbridge.SubscribeUserShareResourceOptionFunc,
) ([]*cloudbridge.ShareResourceInfo, int64, error) {
	option := &cloudbridge.SubscribeUserShareResourceOption{
		PageNum:  1,
		PageSize: 30,
	}
	for _, opt := range opts {
		opt(option)
	}

	m.pageCalls = append(m.pageCalls, option.PageNum)
	if option.PageNum > 1 {
		return nil, 0, nil
	}

	return m.items, int64(len(m.items)), nil
}

type mockRefreshSubscribePlanService struct {
	autoingestplan.Service

	plan          *models.AutoIngestPlan
	updatedOffset int64
	addDelta      int64
	failedDelta   int64
}

func (m *mockRefreshSubscribePlanService) Query(ctx context.Context, id int64) (*models.AutoIngestPlan, error) {
	return m.plan, nil
}

func (m *mockRefreshSubscribePlanService) UpdateOffset(ctx context.Context, id int64, offset int64) error {
	m.updatedOffset = offset

	return nil
}

func (m *mockRefreshSubscribePlanService) IncrAddCount(ctx context.Context, id int64, delta int64) error {
	m.addDelta += delta

	return nil
}

func (m *mockRefreshSubscribePlanService) IncrFailedCount(ctx context.Context, id int64, delta int64) error {
	m.failedDelta += delta

	return nil
}

type mockRefreshSubscribeLogService struct {
	autoingestlog.Service

	levels []autoingest.LogLevel
}

func (m *mockRefreshSubscribeLogService) Create(ctx context.Context, planID int64, level autoingest.LogLevel, content string) (int64, error) {
	m.levels = append(m.levels, level)

	return int64(len(m.levels)), nil
}

type mockRefreshSubscribeStorageFacadeService struct {
	storagefacade.Service

	mu          sync.Mutex
	err         error
	createCalls int
}

func (m *mockRefreshSubscribeStorageFacadeService) CreateStorage(ctx context.Context, req *storagefacade.CreateStorageRequest) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.createCalls++
	if m.err != nil {
		return 0, m.err
	}

	return 9000 + int64(m.createCalls), nil
}

func (m *mockRefreshSubscribeStorageFacadeService) Count() int {
	m.mu.Lock()
	defer m.mu.Unlock()

	return m.createCalls
}

type mockRefreshSubscribeVirtualFileService struct {
	virtualfile.Service

	queryCalls  int
	existing    *models.VirtualFile
	err         error
	queryFiles  []*models.VirtualFile
	queryErrors []error
}

func (m *mockRefreshSubscribeVirtualFileService) QueryByPath(ctx context.Context, filePath string) (*models.VirtualFile, error) {
	callIndex := m.queryCalls
	m.queryCalls++

	if callIndex < len(m.queryErrors) && m.queryErrors[callIndex] != nil {
		return nil, m.queryErrors[callIndex]
	}

	if callIndex < len(m.queryFiles) {
		if m.queryFiles[callIndex] != nil {
			return m.queryFiles[callIndex], nil
		}

		return nil, gorm.ErrRecordNotFound
	}

	if m.err != nil {
		return nil, m.err
	}

	if m.existing != nil {
		return m.existing, nil
	}

	return nil, gorm.ErrRecordNotFound
}

func (m *mockRefreshSubscribeVirtualFileService) BatchQueryParentFiles(ctx context.Context, id int64) ([]*models.VirtualFile, error) {
	return nil, nil
}

func (m *mockRefreshSubscribeVirtualFileService) GetMaxId(ctx context.Context) (int64, error) {
	return 0, nil
}

func (m *mockRefreshSubscribeVirtualFileService) CalFullPath(ctx context.Context, id int64) (string, error) {
	return "", nil
}

func (m *mockRefreshSubscribeVirtualFileService) CalFilePath(ctx context.Context, id int64) (string, error) {
	return "", nil
}

func (m *mockRefreshSubscribeVirtualFileService) List(ctx context.Context, req *virtualfile.ListRequest) ([]*models.VirtualFile, error) {
	return nil, nil
}

func (m *mockRefreshSubscribeVirtualFileService) Count(ctx context.Context, req *virtualfile.ListRequest) (int64, error) {
	return 0, nil
}

func (m *mockRefreshSubscribeVirtualFileService) Query(ctx context.Context, fid int64) (*models.VirtualFile, error) {
	return nil, nil
}

func (m *mockRefreshSubscribeVirtualFileService) QueryTop(ctx context.Context, fid int64) (*models.VirtualFile, error) {
	return nil, nil
}

func (m *mockRefreshSubscribeVirtualFileService) FindOrCreateAncestors(ctx context.Context, filePath string) (int64, error) {
	return 0, nil
}

func (m *mockRefreshSubscribeVirtualFileService) Create(ctx context.Context, parentId int64, file *models.VirtualFile) (int64, error) {
	return 0, nil
}

func (m *mockRefreshSubscribeVirtualFileService) CreateTop(ctx context.Context, parentId int64, file *models.VirtualFile) (int64, error) {
	return 0, nil
}

func (m *mockRefreshSubscribeVirtualFileService) BatchCreate(
	ctx context.Context,
	parentId int64,
	files []*models.VirtualFile,
	hooks ...virtualfile.BatchCreateHook,
) (int64, error) {
	return 0, nil
}

func (m *mockRefreshSubscribeVirtualFileService) Delete(ctx context.Context, id int64, hooks ...virtualfile.DeleteHook) error {
	return nil
}

func (m *mockRefreshSubscribeVirtualFileService) BatchDelete(
	ctx context.Context,
	ids []int64,
	hooks ...virtualfile.BatchDeleteHook,
) ([]int64, error) {
	return nil, nil
}

func (m *mockRefreshSubscribeVirtualFileService) Update(
	ctx context.Context,
	id int64,
	opts []utils.Field,
	hooks ...virtualfile.UpdateHook,
) error {
	return nil
}

func (m *mockRefreshSubscribeVirtualFileService) ModifyAddition(ctx context.Context, id int64, key string, value any) error {
	return nil
}

func (m *mockRefreshSubscribeVirtualFileService) BatchUpdate(ctx context.Context, filesToUpdate map[int64][]utils.Field) error {
	return nil
}

func (m *mockRefreshSubscribeVirtualFileService) BatchUpdatePlus(ctx context.Context, values []utils.Field, exps []clause.Expression) error {
	return nil
}

func (m *mockRefreshSubscribeVirtualFileService) GroupCountByTopId(
	ctx context.Context,
	req *virtualfile.GroupCountByTopIdRequest,
) ([]*virtualfile.GroupCountByTopId, error) {
	return nil, nil
}

func (m *mockRefreshSubscribeVirtualFileService) ClearUnusedAncestorFolder(ctx context.Context, subId int64) error {
	return nil
}

func (m *mockRefreshSubscribeVirtualFileService) ClearAll(ctx context.Context) error {
	return nil
}

func newRefreshSubscribePlan(offset int64, conflict autoingest.OnConflict) *models.AutoIngestPlan {
	return &models.AutoIngestPlan{
		ID:              12,
		SourceType:      autoingest.SourceTypeSubscribe,
		Offset:          offset,
		ParentPath:      "/subscriptions",
		OnConflict:      conflict,
		ConcurrentCount: 1,
		MaxRetryCount:   1,
		Addition: (&models.AutoIngestPlanSubscribeAddition{
			UpUserId: "up-user",
		}).JSONMap(),
		TokenId: 99,
		UserID:  7,
	}
}

func runRefreshSubscribeHandler(
	t *testing.T,
	planService *mockRefreshSubscribePlanService,
	cloudService *mockRefreshSubscribeCloudBridgeService,
	logService *mockRefreshSubscribeLogService,
	storageService *mockRefreshSubscribeStorageFacadeService,
	virtualService *mockRefreshSubscribeVirtualFileService,
) error {
	t.Helper()

	return runRefreshSubscribeHandlerWithTaskEngine(t, &mockRefreshSubscribeTaskEngine{}, planService, cloudService, logService, storageService, virtualService)
}

func runRefreshSubscribeHandlerWithTaskEngine(
	t *testing.T,
	taskEngine *mockRefreshSubscribeTaskEngine,
	planService *mockRefreshSubscribePlanService,
	cloudService *mockRefreshSubscribeCloudBridgeService,
	logService *mockRefreshSubscribeLogService,
	storageService *mockRefreshSubscribeStorageFacadeService,
	virtualService *mockRefreshSubscribeVirtualFileService,
) error {
	t.Helper()

	handler := NewHandler(
		taskEngine,
		cloudService,
		planService,
		logService,
		storageService,
		virtualService,
	)
	req := topic.AutoIngestRefreshSubscribeRequest{PlanId: planService.plan.ID}

	payload, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal refresh request: %v", err)
	}

	processor := taskcontext.NewHandlerFuncWrapper(zap.NewNop()).Wrap(handler.RefreshSubscribe())

	return processor.Process(stdctx.Background(), payload)
}

func TestRefreshSubscribeDoesNotAdvanceOffsetWhenScanEnqueueFails(t *testing.T) {
	shareTime := time.Unix(200, 0)
	planService := &mockRefreshSubscribePlanService{
		plan:          newRefreshSubscribePlan(100, autoingest.OnConflictRename),
		updatedOffset: -1,
	}
	cloudService := &mockRefreshSubscribeCloudBridgeService{
		items: []*cloudbridge.ShareResourceInfo{{
			Name:      "movie",
			ID:        "cloud-1",
			ShareId:   88,
			ShareTime: shareTime,
			IsTop:     1,
		}},
	}
	logService := &mockRefreshSubscribeLogService{}
	storageService := &mockRefreshSubscribeStorageFacadeService{}
	virtualService := &mockRefreshSubscribeVirtualFileService{}
	taskEngine := &mockRefreshSubscribeTaskEngine{err: errors.New("queue failed")}

	if err := runRefreshSubscribeHandlerWithTaskEngine(t, taskEngine, planService, cloudService, logService, storageService, virtualService); err != nil {
		t.Fatalf("refresh subscribe: %v", err)
	}

	if planService.updatedOffset != 100 {
		t.Fatalf("expected offset kept at 100 after enqueue failure, got %d", planService.updatedOffset)
	}

	if planService.failedDelta != 1 {
		t.Fatalf("expected one failed item, got %d", planService.failedDelta)
	}

	if planService.addDelta != 0 {
		t.Fatalf("expected no added items, got %d", planService.addDelta)
	}

	if storageService.Count() != 1 {
		t.Fatalf("expected storage created once before enqueue failure, got %d", storageService.Count())
	}

	if taskEngine.Count() != 1 {
		t.Fatalf("expected one enqueue attempt, got %d", taskEngine.Count())
	}
}

func TestRefreshSubscribeDoesNotAdvanceOffsetWhenCreateFails(t *testing.T) {
	shareTime := time.Unix(200, 0)
	planService := &mockRefreshSubscribePlanService{
		plan:          newRefreshSubscribePlan(100, autoingest.OnConflictRename),
		updatedOffset: -1,
	}
	cloudService := &mockRefreshSubscribeCloudBridgeService{
		items: []*cloudbridge.ShareResourceInfo{{
			Name:      "movie",
			ID:        "cloud-1",
			ShareId:   88,
			ShareTime: shareTime,
			IsTop:     1,
		}},
	}
	logService := &mockRefreshSubscribeLogService{}
	storageService := &mockRefreshSubscribeStorageFacadeService{err: errors.New("create failed")}
	virtualService := &mockRefreshSubscribeVirtualFileService{}

	if err := runRefreshSubscribeHandler(t, planService, cloudService, logService, storageService, virtualService); err != nil {
		t.Fatalf("refresh subscribe: %v", err)
	}

	if planService.updatedOffset != 100 {
		t.Fatalf("expected offset kept at 100 after create failure, got %d", planService.updatedOffset)
	}

	if planService.failedDelta != 1 {
		t.Fatalf("expected one failed item, got %d", planService.failedDelta)
	}

	if planService.addDelta != 0 {
		t.Fatalf("expected no added items, got %d", planService.addDelta)
	}

	if storageService.Count() != 2 {
		t.Fatalf("expected initial create and one retry, got %d", storageService.Count())
	}
}

func TestRefreshSubscribeDuplicateCreateScansExistingAndAdvancesOffset(t *testing.T) {
	shareTime := time.Unix(200, 0)
	planService := &mockRefreshSubscribePlanService{
		plan:          newRefreshSubscribePlan(100, autoingest.OnConflictRename),
		updatedOffset: -1,
	}
	cloudService := &mockRefreshSubscribeCloudBridgeService{
		items: []*cloudbridge.ShareResourceInfo{{
			Name:      "movie",
			ID:        "cloud-1",
			ShareId:   88,
			ShareTime: shareTime,
			IsTop:     1,
		}},
	}
	logService := &mockRefreshSubscribeLogService{}
	storageService := &mockRefreshSubscribeStorageFacadeService{
		err: errors.New("UNIQUE constraint failed: virtual_files.parent_id, virtual_files.name"),
	}
	virtualService := &mockRefreshSubscribeVirtualFileService{
		queryFiles: []*models.VirtualFile{
			nil,
			{ID: 55, Name: "movie", CloudId: "cloud-1"},
		},
	}
	taskEngine := &mockRefreshSubscribeTaskEngine{}

	if err := runRefreshSubscribeHandlerWithTaskEngine(t, taskEngine, planService, cloudService, logService, storageService, virtualService); err != nil {
		t.Fatalf("refresh subscribe: %v", err)
	}

	if planService.updatedOffset != 200 {
		t.Fatalf("expected offset advanced to 200 after duplicate create scan, got %d", planService.updatedOffset)
	}

	if taskEngine.Count() != 1 {
		t.Fatalf("expected existing file scan enqueued once, got %d", taskEngine.Count())
	}

	if storageService.Count() != 1 {
		t.Fatalf("expected one create attempt, got %d", storageService.Count())
	}

	if virtualService.queryCalls != 2 {
		t.Fatalf("expected path queried before create and after duplicate, got %d", virtualService.queryCalls)
	}

	if planService.failedDelta != 0 {
		t.Fatalf("expected no failed items, got %d", planService.failedDelta)
	}

	if planService.addDelta != 0 {
		t.Fatalf("expected duplicate existing item not counted as newly added, got %d", planService.addDelta)
	}
}

func TestRefreshSubscribeDuplicateCreateDoesNotAdvanceOffsetWhenExistingScanFails(t *testing.T) {
	shareTime := time.Unix(200, 0)
	planService := &mockRefreshSubscribePlanService{
		plan:          newRefreshSubscribePlan(100, autoingest.OnConflictRename),
		updatedOffset: -1,
	}
	cloudService := &mockRefreshSubscribeCloudBridgeService{
		items: []*cloudbridge.ShareResourceInfo{{
			Name:      "movie",
			ID:        "cloud-1",
			ShareId:   88,
			ShareTime: shareTime,
			IsTop:     1,
		}},
	}
	logService := &mockRefreshSubscribeLogService{}
	storageService := &mockRefreshSubscribeStorageFacadeService{
		err: errors.New("duplicate key value violates unique constraint \"virtual_files_parent_name_unique\""),
	}
	virtualService := &mockRefreshSubscribeVirtualFileService{
		queryFiles: []*models.VirtualFile{
			nil,
			{ID: 55, Name: "movie", CloudId: "cloud-1"},
		},
	}
	taskEngine := &mockRefreshSubscribeTaskEngine{err: errors.New("queue failed")}

	if err := runRefreshSubscribeHandlerWithTaskEngine(t, taskEngine, planService, cloudService, logService, storageService, virtualService); err != nil {
		t.Fatalf("refresh subscribe: %v", err)
	}

	if planService.updatedOffset != 100 {
		t.Fatalf("expected offset kept at 100 after duplicate scan enqueue failure, got %d", planService.updatedOffset)
	}

	if planService.failedDelta != 1 {
		t.Fatalf("expected one failed item, got %d", planService.failedDelta)
	}

	if taskEngine.Count() != 1 {
		t.Fatalf("expected one enqueue attempt, got %d", taskEngine.Count())
	}
}

func TestRefreshSubscribeAbandonExistingAdvancesOffsetWithoutRetry(t *testing.T) {
	shareTime := time.Unix(200, 0)
	planService := &mockRefreshSubscribePlanService{
		plan:          newRefreshSubscribePlan(100, autoingest.OnConflictAbandon),
		updatedOffset: -1,
	}
	cloudService := &mockRefreshSubscribeCloudBridgeService{
		items: []*cloudbridge.ShareResourceInfo{{
			Name:      "movie",
			ID:        "cloud-1",
			ShareId:   88,
			ShareTime: shareTime,
			IsTop:     1,
		}},
	}
	logService := &mockRefreshSubscribeLogService{}
	storageService := &mockRefreshSubscribeStorageFacadeService{}
	virtualService := &mockRefreshSubscribeVirtualFileService{
		existing: &models.VirtualFile{ID: 55, Name: "movie"},
	}

	if err := runRefreshSubscribeHandler(t, planService, cloudService, logService, storageService, virtualService); err != nil {
		t.Fatalf("refresh subscribe: %v", err)
	}

	if planService.updatedOffset != 200 {
		t.Fatalf("expected offset advanced to 200 after abandon skip, got %d", planService.updatedOffset)
	}

	if virtualService.queryCalls != 1 {
		t.Fatalf("expected existing path queried once, got %d", virtualService.queryCalls)
	}

	if storageService.Count() != 0 {
		t.Fatalf("expected existing path not recreated, got %d create calls", storageService.Count())
	}

	if planService.failedDelta != 0 {
		t.Fatalf("expected no failed items, got %d", planService.failedDelta)
	}
}
