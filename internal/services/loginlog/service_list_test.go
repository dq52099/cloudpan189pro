package loginlog

import (
	stdctx "context"
	"errors"
	"fmt"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/xxcheng123/cloudpan189-share/internal/bootstrap"
	"github.com/xxcheng123/cloudpan189-share/internal/framework/context"
	"github.com/xxcheng123/cloudpan189-share/internal/pkgs/taskengine"
	"github.com/xxcheng123/cloudpan189-share/internal/repository/models"
	loginlogType "github.com/xxcheng123/cloudpan189-share/internal/types/loginlog"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type loginLogTestDB struct {
	db *gorm.DB
}

func (t *loginLogTestDB) GetDB(ctx context.Context) *gorm.DB {
	return t.db.WithContext(ctx)
}

func (t *loginLogTestDB) GetDBWithoutContext() *gorm.DB {
	return t.db
}

func (t *loginLogTestDB) GetLogger(name string, fields ...zap.Field) *zap.Logger {
	return zap.NewNop()
}

func (t *loginLogTestDB) Close() {}

func (t *loginLogTestDB) GetPort() int {
	return 9999
}

func (t *loginLogTestDB) GetTaskEngine() taskengine.TaskEngine {
	return nil
}

func (t *loginLogTestDB) GetHTTPEngine() *gin.Engine {
	return nil
}

var _ bootstrap.ServiceContext = (*loginLogTestDB)(nil)

func setupLoginLogTestDB(t *testing.T) *loginLogTestDB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}

	if err := db.AutoMigrate(&models.LoginLog{}); err != nil {
		t.Fatalf("migrate test db: %v", err)
	}

	return &loginLogTestDB{db: db}
}

func createLoginLog(t *testing.T, db *gorm.DB, index int) *models.LoginLog {
	t.Helper()

	log := &models.LoginLog{
		UserId:   int64(index + 1),
		Username: fmt.Sprintf("user-%03d", index),
		Addr:     "127.0.0.1",
		Method:   loginlogType.MethodWeb,
		Event:    loginlogType.EventLogin,
		Status:   loginlogType.StatusSuccess,
		TraceId:  fmt.Sprintf("trace-%03d", index),
	}
	if err := db.Create(log).Error; err != nil {
		t.Fatalf("create login log: %v", err)
	}

	return log
}

func TestListDefaultsPaginationWhenRequestNil(t *testing.T) {
	tDB := setupLoginLogTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())

	for i := 0; i < 12; i++ {
		createLoginLog(t, tDB.db, i)
	}

	list, err := svc.List(ctx, nil)
	if err != nil {
		t.Fatalf("list login logs: %v", err)
	}

	if len(list) != defaultLoginLogPageSize {
		t.Fatalf("expected default page size %d, got %d", defaultLoginLogPageSize, len(list))
	}
}

func TestListDefaultsPaginationWhenPageValuesMissing(t *testing.T) {
	tDB := setupLoginLogTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())

	for i := 0; i < 12; i++ {
		createLoginLog(t, tDB.db, i)
	}

	req := &ListRequest{}

	list, err := svc.List(ctx, req)
	if err != nil {
		t.Fatalf("list login logs: %v", err)
	}

	if len(list) != defaultLoginLogPageSize {
		t.Fatalf("expected default page size %d, got %d", defaultLoginLogPageSize, len(list))
	}

	if req.CurrentPage != defaultLoginLogCurrentPage {
		t.Fatalf("expected current page normalized to %d, got %d", defaultLoginLogCurrentPage, req.CurrentPage)
	}

	if req.PageSize != defaultLoginLogPageSize {
		t.Fatalf("expected page size normalized to %d, got %d", defaultLoginLogPageSize, req.PageSize)
	}
}

func TestListCapsPageSize(t *testing.T) {
	tDB := setupLoginLogTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())

	for i := 0; i < 3; i++ {
		createLoginLog(t, tDB.db, i)
	}

	req := &ListRequest{
		CurrentPage: 1,
		PageSize:    maxLoginLogPageSize + 100,
	}

	if _, err := svc.List(ctx, req); err != nil {
		t.Fatalf("list login logs: %v", err)
	}

	if req.PageSize != maxLoginLogPageSize {
		t.Fatalf("expected page size capped to %d, got %d", maxLoginLogPageSize, req.PageSize)
	}
}

func TestListNoPaginateKeepsAllRows(t *testing.T) {
	tDB := setupLoginLogTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())

	for i := 0; i < 12; i++ {
		createLoginLog(t, tDB.db, i)
	}

	list, err := svc.List(ctx, &ListRequest{NoPaginate: true})
	if err != nil {
		t.Fatalf("list login logs: %v", err)
	}

	if len(list) != 12 {
		t.Fatalf("expected all rows with no paginate, got %d", len(list))
	}
}

func TestListRejectsInvalidSortFields(t *testing.T) {
	tDB := setupLoginLogTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())

	tests := []struct {
		name string
		req  *ListRequest
	}{
		{name: "invalid asc", req: &ListRequest{AscList: []string{"username; DROP TABLE login_logs"}}},
		{name: "invalid desc", req: &ListRequest{DescList: []string{"password"}}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := svc.List(ctx, tt.req)
			if !errors.Is(err, errInvalidLoginLogSortField) {
				t.Fatalf("expected invalid login log sort field, got %v", err)
			}
		})
	}
}

func TestListAllowsWhitelistedSortFields(t *testing.T) {
	tDB := setupLoginLogTestDB(t)
	svc := NewService(tDB)
	ctx := context.NewContext(stdctx.Background())

	createLoginLog(t, tDB.db, 2)
	createLoginLog(t, tDB.db, 1)

	list, err := svc.List(ctx, &ListRequest{
		NoPaginate: true,
		AscList:    []string{"username"},
	})
	if err != nil {
		t.Fatalf("list login logs: %v", err)
	}

	if len(list) != 2 {
		t.Fatalf("expected 2 logs, got %d", len(list))
	}

	if list[0].Username > list[1].Username {
		t.Fatalf("expected username asc order, got %q then %q", list[0].Username, list[1].Username)
	}
}
