package setting

import (
	stdctx "context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/xxcheng123/cloudpan189-share/internal/bootstrap"
	appContext "github.com/xxcheng123/cloudpan189-share/internal/framework/context"
	"github.com/xxcheng123/cloudpan189-share/internal/framework/httpcontext"
	"github.com/xxcheng123/cloudpan189-share/internal/pkgs/taskengine"
	"github.com/xxcheng123/cloudpan189-share/internal/repository/models"
	settingSvc "github.com/xxcheng123/cloudpan189-share/internal/services/setting"
	userSvc "github.com/xxcheng123/cloudpan189-share/internal/services/user"
	"github.com/xxcheng123/cloudpan189-share/internal/shared"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type mockInitSettingService struct {
	settingSvc.Service
	req                 *settingSvc.InitSystemRequest
	runTransactionCalls int
}

func (m *mockInitSettingService) InitSystem(ctx appContext.Context, req *settingSvc.InitSystemRequest) error {
	m.req = req

	return nil
}

func (m *mockInitSettingService) RunInTransaction(ctx appContext.Context, run func(appContext.Context) error) error {
	m.runTransactionCalls++

	return run(ctx)
}

type mockInitUserService struct {
	userSvc.Service
	req     *userSvc.AddRequest
	isAdmin bool
}

func (m *mockInitUserService) Add(ctx appContext.Context, req *userSvc.AddRequest, opts ...userSvc.AddOptionFunc) (*userSvc.AddResponse, error) {
	m.req = req

	user := &models.User{}
	for _, opt := range opts {
		opt(user)
	}

	m.isAdmin = user.IsAdmin

	return &userSvc.AddResponse{ID: 1}, nil
}

type initSystemIntegrationDB struct {
	db *gorm.DB
}

func (t *initSystemIntegrationDB) GetDB(ctx appContext.Context) *gorm.DB {
	return bootstrap.DBFromContext(ctx, t.db)
}

func (t *initSystemIntegrationDB) GetDBWithoutContext() *gorm.DB {
	return t.db
}

func (t *initSystemIntegrationDB) GetLogger(name string, fields ...zap.Field) *zap.Logger {
	return zap.NewNop()
}

func (t *initSystemIntegrationDB) Close() {}

func (t *initSystemIntegrationDB) GetPort() int {
	return 9999
}

func (t *initSystemIntegrationDB) GetTaskEngine() taskengine.TaskEngine {
	return nil
}

func (t *initSystemIntegrationDB) GetHTTPEngine() *gin.Engine {
	return nil
}

var _ bootstrap.ServiceContext = (*initSystemIntegrationDB)(nil)

func setupInitSystemIntegrationDB(t *testing.T) *initSystemIntegrationDB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}

	if err := db.AutoMigrate(&models.Setting{}, &models.User{}); err != nil {
		t.Fatalf("migrate test db: %v", err)
	}

	if err := db.Create(&models.Setting{
		Title:       "old title",
		EnableAuth:  true,
		SaltKey:     "init-salt",
		BaseURL:     "http://old.example.test",
		Initialized: false,
	}).Error; err != nil {
		t.Fatalf("create setting: %v", err)
	}

	return &initSystemIntegrationDB{db: db}
}

func restoreInitSystemSharedSetting(t *testing.T) {
	t.Helper()

	oldSaltKey := shared.SaltKey
	oldBaseURL := shared.BaseURL
	oldEnableAuth := shared.EnableAuth
	oldAddition := shared.SettingAddition

	t.Cleanup(func() {
		shared.SaltKey = oldSaltKey
		shared.BaseURL = oldBaseURL
		shared.EnableAuth = oldEnableAuth
		shared.SettingAddition = oldAddition
	})
}

func performInitSystemRequest(t *testing.T, settingService settingSvc.Service, userService userSvc.Service, body string) *httptest.ResponseRecorder {
	t.Helper()

	gin.SetMode(gin.TestMode)

	router := gin.New()
	wrapper := httpcontext.NewHandlerFuncWrapper(zap.NewNop())
	router.POST("/init_system", wrapper.Wrap(NewHandler(userService, settingService, nil).InitSystem()))

	req := httptest.NewRequestWithContext(stdctx.Background(), http.MethodPost, "/init_system", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	return recorder
}

func TestInitSystemAcceptsExplicitDisableAuth(t *testing.T) {
	settingService := &mockInitSettingService{}
	userService := &mockInitUserService{}

	recorder := performInitSystemRequest(t, settingService, userService, `{"title":"CloudPan","enableAuth":false,"baseURL":"http://example.test","superUsername":"admin","superPassword":"123456"}`)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected success, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	if settingService.req == nil {
		t.Fatal("expected init system service to be called")
	}

	if settingService.req.EnableAuth {
		t.Fatal("expected enableAuth false to be passed to service")
	}

	if userService.req == nil {
		t.Fatal("expected super user to be created")
	}

	if !userService.isAdmin {
		t.Fatal("expected super user to be created as admin")
	}

	if settingService.runTransactionCalls != 1 {
		t.Fatalf("expected init to run in one transaction, got %d", settingService.runTransactionCalls)
	}
}

func TestInitSystemRejectsMissingEnableAuth(t *testing.T) {
	settingService := &mockInitSettingService{}
	userService := &mockInitUserService{}

	recorder := performInitSystemRequest(t, settingService, userService, `{"title":"CloudPan","baseURL":"http://example.test","superUsername":"admin","superPassword":"123456"}`)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected bad request, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	if settingService.req != nil {
		t.Fatal("expected missing enableAuth not to call init system service")
	}

	if userService.req != nil {
		t.Fatal("expected missing enableAuth not to create super user")
	}
}

func TestInitSystemRollsBackSettingWhenSuperUserCreateFails(t *testing.T) {
	restoreInitSystemSharedSetting(t)

	tDB := setupInitSystemIntegrationDB(t)
	settingService := settingSvc.NewService(tDB)
	userService := userSvc.NewService(tDB)

	if _, err := userService.Add(appContext.NewContext(stdctx.Background()), &userSvc.AddRequest{
		Username: "admin",
		Password: "existing-password",
	}); err != nil {
		t.Fatalf("seed existing user: %v", err)
	}

	shared.BaseURL = "http://shared-before.example.test"
	shared.EnableAuth = true

	recorder := performInitSystemRequest(t, settingService, userService, `{"title":"CloudPan","enableAuth":false,"baseURL":"http://example.test","superUsername":"admin","superPassword":"123456"}`)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected super user failure, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	var setting models.Setting
	if err := tDB.db.First(&setting).Error; err != nil {
		t.Fatalf("query setting: %v", err)
	}

	if setting.Initialized {
		t.Fatal("expected setting initialization to be rolled back")
	}

	if setting.Title != "old title" || setting.BaseURL != "http://old.example.test" || !setting.EnableAuth {
		t.Fatalf("expected setting fields unchanged after rollback, got %+v", setting)
	}

	var userCount int64
	if err := tDB.db.Model(&models.User{}).Where("username = ?", "admin").Count(&userCount).Error; err != nil {
		t.Fatalf("count admin users: %v", err)
	}

	if userCount != 1 {
		t.Fatalf("expected only seeded admin user after rollback, got %d", userCount)
	}

	if shared.BaseURL != "http://shared-before.example.test" {
		t.Fatalf("expected shared base url unchanged after rollback, got %q", shared.BaseURL)
	}

	if !shared.EnableAuth {
		t.Fatal("expected shared enable auth unchanged after rollback")
	}
}

func TestInitSystemCommitsSettingAndSuperUserTogether(t *testing.T) {
	restoreInitSystemSharedSetting(t)

	tDB := setupInitSystemIntegrationDB(t)
	settingService := settingSvc.NewService(tDB)
	userService := userSvc.NewService(tDB)

	shared.BaseURL = "http://shared-before.example.test"
	shared.EnableAuth = true

	recorder := performInitSystemRequest(t, settingService, userService, `{"title":"CloudPan","enableAuth":false,"baseURL":"http://example.test","superUsername":"admin","superPassword":"123456"}`)
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected success, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	var setting models.Setting
	if err := tDB.db.First(&setting).Error; err != nil {
		t.Fatalf("query setting: %v", err)
	}

	if !setting.Initialized || setting.Title != "CloudPan" || setting.BaseURL != "http://example.test" || setting.EnableAuth {
		t.Fatalf("expected initialized setting to be committed, got %+v", setting)
	}

	var admin models.User
	if err := tDB.db.Where("username = ?", "admin").First(&admin).Error; err != nil {
		t.Fatalf("query admin user: %v", err)
	}

	if !admin.IsAdmin {
		t.Fatal("expected created user to be admin")
	}

	if shared.BaseURL != "http://example.test" {
		t.Fatalf("expected shared base url synced after commit, got %q", shared.BaseURL)
	}

	if shared.EnableAuth {
		t.Fatal("expected shared enable auth synced after commit")
	}
}
