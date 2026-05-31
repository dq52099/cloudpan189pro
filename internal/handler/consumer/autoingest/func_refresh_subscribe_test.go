package autoingest

import (
	stdctx "context"
	"encoding/json"
	"errors"
	"fmt"
	"path"
	"strings"
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
	payloads  [][]byte
}

func (m *mockRefreshSubscribeTaskEngine) PushMessage(ctx stdctx.Context, taskTopic taskengine.Topic, payload []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.pushCount++
	m.payloads = append(m.payloads, append([]byte(nil), payload...))

	return m.err
}

func (m *mockRefreshSubscribeTaskEngine) Count() int {
	m.mu.Lock()
	defer m.mu.Unlock()

	return m.pushCount
}

func (m *mockRefreshSubscribeTaskEngine) ScanRequests(t *testing.T) []topic.FileScanFileRequest {
	t.Helper()

	m.mu.Lock()
	defer m.mu.Unlock()

	result := make([]topic.FileScanFileRequest, 0, len(m.payloads))
	for _, payload := range m.payloads {
		var req topic.FileScanFileRequest
		if err := json.Unmarshal(payload, &req); err != nil {
			t.Fatalf("unmarshal scan request: %v", err)
		}

		result = append(result, req)
	}

	return result
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
	updateReqs    []*autoingestplan.UpdateRequest
	updatedFields [][]utils.Field
}

func (m *mockRefreshSubscribePlanService) Query(ctx context.Context, id int64) (*models.AutoIngestPlan, error) {
	return m.plan, nil
}

func (m *mockRefreshSubscribePlanService) UpdateOffset(ctx context.Context, id int64, offset int64) error {
	m.updatedOffset = offset

	return nil
}

func (m *mockRefreshSubscribePlanService) UpdateByOwner(ctx context.Context, req *autoingestplan.UpdateRequest, fields ...utils.Field) error {
	if req != nil {
		copied := *req
		m.updateReqs = append(m.updateReqs, &copied)
	}

	m.updatedFields = append(m.updatedFields, append([]utils.Field(nil), fields...))

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

	levels   []autoingest.LogLevel
	contents []string
}

func (m *mockRefreshSubscribeLogService) Create(ctx context.Context, planID int64, level autoingest.LogLevel, content string) (int64, error) {
	m.levels = append(m.levels, level)
	m.contents = append(m.contents, content)

	return int64(len(m.levels)), nil
}

type mockRefreshSubscribeStorageFacadeService struct {
	storagefacade.Service

	mu          sync.Mutex
	err         error
	errs        []error
	ids         []int64
	createCalls int
	requests    []*storagefacade.CreateStorageRequest
}

func (m *mockRefreshSubscribeStorageFacadeService) CreateStorage(ctx context.Context, req *storagefacade.CreateStorageRequest) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	callIndex := m.createCalls
	m.createCalls++

	if req != nil {
		copied := *req
		m.requests = append(m.requests, &copied)
	}

	if callIndex < len(m.errs) && m.errs[callIndex] != nil {
		return 0, m.errs[callIndex]
	}

	if m.err != nil {
		return 0, m.err
	}

	if callIndex < len(m.ids) && m.ids[callIndex] != 0 {
		return m.ids[callIndex], nil
	}

	return 9000 + int64(m.createCalls), nil
}

func (m *mockRefreshSubscribeStorageFacadeService) Count() int {
	m.mu.Lock()
	defer m.mu.Unlock()

	return m.createCalls
}

func (m *mockRefreshSubscribeStorageFacadeService) Requests() []*storagefacade.CreateStorageRequest {
	m.mu.Lock()
	defer m.mu.Unlock()

	result := make([]*storagefacade.CreateStorageRequest, 0, len(m.requests))
	for _, req := range m.requests {
		copied := *req
		result = append(result, &copied)
	}

	return result
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

	return runRefreshSubscribeHandlerWithRequest(t, handler, req)
}

func runRefreshSubscribeHandlerWithRequest(
	t *testing.T,
	handler Handler,
	req topic.AutoIngestRefreshSubscribeRequest,
) error {
	t.Helper()

	payload, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal refresh request: %v", err)
	}

	processor := taskcontext.NewHandlerFuncWrapper(zap.NewNop()).Wrap(handler.RefreshSubscribe())

	return processor.Process(stdctx.Background(), payload)
}

func refreshSubscribeFieldsToMap(fields []utils.Field) map[string]any {
	result := make(map[string]any, len(fields))
	for _, field := range fields {
		result[field.Key] = field.Value
	}

	return result
}

func TestRefreshSubscribeSkipsManualTaskWhenOwnerChanged(t *testing.T) {
	planService := &mockRefreshSubscribePlanService{
		plan:          newRefreshSubscribePlan(100, autoingest.OnConflictRename),
		updatedOffset: -1,
	}
	planService.plan.UserID = 8

	cloudService := &mockRefreshSubscribeCloudBridgeService{}
	logService := &mockRefreshSubscribeLogService{}
	storageService := &mockRefreshSubscribeStorageFacadeService{}
	virtualService := &mockRefreshSubscribeVirtualFileService{}
	taskEngine := &mockRefreshSubscribeTaskEngine{}
	handler := NewHandler(
		taskEngine,
		cloudService,
		planService,
		logService,
		storageService,
		virtualService,
	)

	err := runRefreshSubscribeHandlerWithRequest(t, handler, topic.AutoIngestRefreshSubscribeRequest{
		PlanId:         planService.plan.ID,
		ExpectedUserID: 7,
	})
	if err != nil {
		t.Fatalf("refresh subscribe: %v", err)
	}

	if planService.updatedOffset != -1 {
		t.Fatalf("expected owner mismatch task not to update offset, got %d", planService.updatedOffset)
	}

	if planService.addDelta != 0 || planService.failedDelta != 0 {
		t.Fatalf("expected owner mismatch task not to update counters, add=%d failed=%d", planService.addDelta, planService.failedDelta)
	}

	if len(cloudService.pageCalls) != 0 {
		t.Fatalf("expected owner mismatch task not to call cloud bridge, got %v", cloudService.pageCalls)
	}

	if storageService.Count() != 0 {
		t.Fatalf("expected owner mismatch task not to create storage, got %d", storageService.Count())
	}
}

func TestRefreshSubscribeSkipsRetryResetWhenOwnerChanged(t *testing.T) {
	planService := &mockRefreshSubscribePlanService{
		plan:          newRefreshSubscribePlan(100, autoingest.OnConflictRename),
		updatedOffset: -1,
	}
	planService.plan.UserID = 8

	cloudService := &mockRefreshSubscribeCloudBridgeService{}
	logService := &mockRefreshSubscribeLogService{}
	storageService := &mockRefreshSubscribeStorageFacadeService{}
	virtualService := &mockRefreshSubscribeVirtualFileService{}
	taskEngine := &mockRefreshSubscribeTaskEngine{}
	handler := NewHandler(
		taskEngine,
		cloudService,
		planService,
		logService,
		storageService,
		virtualService,
	)

	err := runRefreshSubscribeHandlerWithRequest(t, handler, topic.AutoIngestRefreshSubscribeRequest{
		PlanId:         planService.plan.ID,
		ExpectedUserID: 7,
		RetryReset: &topic.AutoIngestRetryStateReset{
			Offset:        1,
			ResetCounters: true,
		},
	})
	if err != nil {
		t.Fatalf("refresh subscribe: %v", err)
	}

	if len(planService.updateReqs) != 0 || len(planService.updatedFields) != 0 {
		t.Fatalf("expected owner mismatch task not to reset retry state, reqs=%d fields=%d", len(planService.updateReqs), len(planService.updatedFields))
	}

	if planService.updatedOffset != -1 || planService.addDelta != 0 || planService.failedDelta != 0 {
		t.Fatalf("expected owner mismatch task not to update runtime state, offset=%d add=%d failed=%d",
			planService.updatedOffset, planService.addDelta, planService.failedDelta)
	}
}

func TestRefreshSubscribeAppliesRetryResetBeforeScanning(t *testing.T) {
	planService := &mockRefreshSubscribePlanService{
		plan:          newRefreshSubscribePlan(200, autoingest.OnConflictRename),
		updatedOffset: -1,
	}
	cloudService := &mockRefreshSubscribeCloudBridgeService{
		items: []*cloudbridge.ShareResourceInfo{{
			Name:      "movie",
			ID:        "cloud-1",
			ShareId:   88,
			ShareTime: time.Unix(150, 0),
			IsTop:     1,
		}},
	}
	logService := &mockRefreshSubscribeLogService{}
	storageService := &mockRefreshSubscribeStorageFacadeService{}
	virtualService := &mockRefreshSubscribeVirtualFileService{}
	taskEngine := &mockRefreshSubscribeTaskEngine{}
	handler := NewHandler(
		taskEngine,
		cloudService,
		planService,
		logService,
		storageService,
		virtualService,
	)

	err := runRefreshSubscribeHandlerWithRequest(t, handler, topic.AutoIngestRefreshSubscribeRequest{
		PlanId:         planService.plan.ID,
		ExpectedUserID: 7,
		RetryReset: &topic.AutoIngestRetryStateReset{
			Offset:        1,
			ResetCounters: true,
		},
	})
	if err != nil {
		t.Fatalf("refresh subscribe: %v", err)
	}

	if len(planService.updateReqs) != 1 {
		t.Fatalf("expected one owner-scoped retry reset, got %d", len(planService.updateReqs))
	}

	updateReq := planService.updateReqs[0]
	if updateReq.ID != planService.plan.ID || updateReq.UserID != 7 || updateReq.IsAdmin {
		t.Fatalf("unexpected retry reset request: %+v", updateReq)
	}

	if len(planService.updatedFields) != 1 {
		t.Fatalf("expected one retry reset field set, got %d", len(planService.updatedFields))
	}

	resetFields := refreshSubscribeFieldsToMap(planService.updatedFields[0])
	if resetFields["offset"] != int64(1) || resetFields["add_count"] != int64(0) || resetFields["failed_count"] != int64(0) {
		t.Fatalf("unexpected retry reset fields: %+v", resetFields)
	}

	if planService.updatedOffset != 150 {
		t.Fatalf("expected offset to advance from reset offset to 150, got %d", planService.updatedOffset)
	}

	if planService.addDelta != 1 || planService.failedDelta != 0 {
		t.Fatalf("expected one item added after reset, add=%d failed=%d", planService.addDelta, planService.failedDelta)
	}

	if storageService.Count() != 1 {
		t.Fatalf("expected item after reset to be created, got %d", storageService.Count())
	}

	scanReqs := taskEngine.ScanRequests(t)
	if len(scanReqs) != 1 {
		t.Fatalf("expected one scan request after reset, got %d", len(scanReqs))
	}

	if scanReqs[0].ExpectedUserID != planService.plan.UserID {
		t.Fatalf("expected scan request owner snapshot %d, got %d", planService.plan.UserID, scanReqs[0].ExpectedUserID)
	}
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

func TestRefreshSubscribeRenamesPathWhenCreateStoragePathExists(t *testing.T) {
	shareTime := time.Unix(200, 0)
	createdFileID := int64(777)
	planService := &mockRefreshSubscribePlanService{
		plan:          newRefreshSubscribePlan(100, autoingest.OnConflictRename),
		updatedOffset: -1,
	}
	planService.plan.MaxRetryCount = 2

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
		errs: []error{
			storagefacade.ErrPathAlreadyExists,
			storagefacade.ErrPathAlreadyExists,
			nil,
		},
		ids: []int64{
			0,
			0,
			createdFileID,
		},
	}
	virtualService := &mockRefreshSubscribeVirtualFileService{}
	taskEngine := &mockRefreshSubscribeTaskEngine{}

	if err := runRefreshSubscribeHandlerWithTaskEngine(t, taskEngine, planService, cloudService, logService, storageService, virtualService); err != nil {
		t.Fatalf("refresh subscribe: %v", err)
	}

	if storageService.Count() != 3 {
		t.Fatalf("expected create retried with renamed path, got %d calls", storageService.Count())
	}

	createReqs := storageService.Requests()
	if len(createReqs) != 3 {
		t.Fatalf("expected three create requests, got %d", len(createReqs))
	}

	if createReqs[0].LocalPath != "/subscriptions/movie" {
		t.Fatalf("unexpected initial local path: %s", createReqs[0].LocalPath)
	}

	renamedPaths := []string{createReqs[1].LocalPath, createReqs[2].LocalPath}
	for _, renamedPath := range renamedPaths {
		if path.Dir(renamedPath) != "/subscriptions" {
			t.Fatalf("expected renamed path to stay under /subscriptions, got %s", renamedPath)
		}

		if renamedPath == createReqs[0].LocalPath || !strings.HasPrefix(path.Base(renamedPath), "movie_") {
			t.Fatalf("expected create path to be renamed from movie, got %s", renamedPath)
		}
	}

	if renamedPaths[0] == renamedPaths[1] {
		t.Fatalf("expected repeated conflict renames to produce different paths, got %s", renamedPaths[0])
	}

	if planService.updatedOffset != shareTime.Unix() {
		t.Fatalf("expected offset advanced to %d after renamed create succeeds, got %d", shareTime.Unix(), planService.updatedOffset)
	}

	scanReqs := taskEngine.ScanRequests(t)
	if len(scanReqs) != 1 {
		t.Fatalf("expected one scan request after renamed create succeeds, got %d", len(scanReqs))
	}

	if scanReqs[0].FileId != createdFileID {
		t.Fatalf("expected scan task to use created file id %d, got %+v", createdFileID, scanReqs[0])
	}
}

func TestRefreshSubscribeLogsPathQueryFailure(t *testing.T) {
	planService := &mockRefreshSubscribePlanService{
		plan:          newRefreshSubscribePlan(100, autoingest.OnConflictRename),
		updatedOffset: -1,
	}
	cloudService := &mockRefreshSubscribeCloudBridgeService{
		items: []*cloudbridge.ShareResourceInfo{{
			Name:      "query-fail",
			ID:        "cloud-1",
			ShareId:   88,
			ShareTime: time.Unix(200, 0),
			IsTop:     1,
		}},
	}
	logService := &mockRefreshSubscribeLogService{}
	storageService := &mockRefreshSubscribeStorageFacadeService{}
	virtualService := &mockRefreshSubscribeVirtualFileService{err: errors.New("query failed")}

	if err := runRefreshSubscribeHandler(t, planService, cloudService, logService, storageService, virtualService); err != nil {
		t.Fatalf("refresh subscribe: %v", err)
	}

	if planService.updatedOffset != 100 {
		t.Fatalf("expected offset kept at 100 after path query failure, got %d", planService.updatedOffset)
	}

	if planService.failedDelta != 1 {
		t.Fatalf("expected one failed item, got %d", planService.failedDelta)
	}

	if storageService.Count() != 0 {
		t.Fatalf("expected no storage create after path query failure, got %d", storageService.Count())
	}

	if len(logService.contents) != 1 || !strings.Contains(logService.contents[0], "查询虚拟文件路径失败") {
		t.Fatalf("expected path query failure log, got %#v", logService.contents)
	}
}

func TestRefreshSubscribeRetryLogsExistingScanEnqueueFailure(t *testing.T) {
	planService := &mockRefreshSubscribePlanService{
		plan:          newRefreshSubscribePlan(100, autoingest.OnConflictRename),
		updatedOffset: -1,
	}
	cloudService := &mockRefreshSubscribeCloudBridgeService{
		items: []*cloudbridge.ShareResourceInfo{{
			Name:      "created-before",
			ID:        "cloud-created",
			ShareId:   88,
			ShareTime: time.Unix(200, 0),
			IsTop:     1,
		}},
	}
	logService := &mockRefreshSubscribeLogService{}
	storageService := &mockRefreshSubscribeStorageFacadeService{}
	virtualService := &mockRefreshSubscribeVirtualFileService{
		existing: &models.VirtualFile{ID: 55, Name: "created-before", CloudId: "cloud-created"},
	}
	taskEngine := &mockRefreshSubscribeTaskEngine{err: errors.New("queue failed")}
	handler := NewHandler(
		taskEngine,
		cloudService,
		planService,
		logService,
		storageService,
		virtualService,
	)

	err := runRefreshSubscribeHandlerWithRequest(t, handler, topic.AutoIngestRefreshSubscribeRequest{
		PlanId:  planService.plan.ID,
		IsRetry: true,
	})
	if err != nil {
		t.Fatalf("refresh subscribe: %v", err)
	}

	if planService.updatedOffset != 100 {
		t.Fatalf("expected offset kept at 100 after retry enqueue failure, got %d", planService.updatedOffset)
	}

	if planService.failedDelta != 1 {
		t.Fatalf("expected one failed item, got %d", planService.failedDelta)
	}

	if len(logService.contents) != 1 || !strings.Contains(logService.contents[0], "下发已存在文件扫描任务失败") {
		t.Fatalf("expected existing scan enqueue failure log, got %#v", logService.contents)
	}
}

func TestRefreshSubscribeSkipsExistingSameCloudIDWithoutRetryScanWhenOtherItemFails(t *testing.T) {
	planService := &mockRefreshSubscribePlanService{
		plan:          newRefreshSubscribePlan(100, autoingest.OnConflictRename),
		updatedOffset: -1,
	}
	cloudService := &mockRefreshSubscribeCloudBridgeService{
		items: []*cloudbridge.ShareResourceInfo{
			{
				Name:      "created-before",
				ID:        "cloud-created",
				ShareId:   88,
				ShareTime: time.Unix(200, 0),
				IsTop:     1,
			},
			{
				Name:      "will-fail",
				ID:        "cloud-fail",
				ShareId:   89,
				ShareTime: time.Unix(201, 0),
				IsTop:     1,
			},
		},
	}
	logService := &mockRefreshSubscribeLogService{}
	storageService := &mockRefreshSubscribeStorageFacadeService{err: errors.New("create failed")}
	virtualService := &mockRefreshSubscribeVirtualFileService{
		queryFiles: []*models.VirtualFile{
			{ID: 55, Name: "created-before", CloudId: "cloud-created"},
			nil,
			nil,
		},
	}
	taskEngine := &mockRefreshSubscribeTaskEngine{}

	if err := runRefreshSubscribeHandlerWithTaskEngine(t, taskEngine, planService, cloudService, logService, storageService, virtualService); err != nil {
		t.Fatalf("refresh subscribe: %v", err)
	}

	if taskEngine.Count() != 0 {
		t.Fatalf("expected existing completed item not to enqueue scan during ordinary refresh, got %d", taskEngine.Count())
	}

	if planService.updatedOffset != 100 {
		t.Fatalf("expected offset kept at 100 while another item fails, got %d", planService.updatedOffset)
	}

	if planService.failedDelta != 1 {
		t.Fatalf("expected only failing item counted failed, got %d", planService.failedDelta)
	}

	if storageService.Count() != 2 {
		t.Fatalf("expected failing item create and retry only, got %d", storageService.Count())
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

	scanReqs := taskEngine.ScanRequests(t)
	if len(scanReqs) != 1 {
		t.Fatalf("expected one scan request, got %d", len(scanReqs))
	}

	if scanReqs[0].FileId != 55 || scanReqs[0].ExpectedUserID != planService.plan.UserID {
		t.Fatalf("unexpected existing scan request: %+v", scanReqs[0])
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
