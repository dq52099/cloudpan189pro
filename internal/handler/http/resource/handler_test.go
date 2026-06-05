package resource

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	appContext "github.com/xxcheng123/cloudpan189-share/internal/framework/context"
	"github.com/xxcheng123/cloudpan189-share/internal/framework/httpcontext"
	"github.com/xxcheng123/cloudpan189-share/internal/pkgs/taskengine"
	"github.com/xxcheng123/cloudpan189-share/internal/repository/models"
	mediaconfigSvi "github.com/xxcheng123/cloudpan189-share/internal/services/mediaconfig"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type mockSummaryMediaConfigService struct {
	mediaconfigSvi.Service
	err error
}

func (m *mockSummaryMediaConfigService) Query(ctx appContext.Context) (*models.MediaConfig, error) {
	return nil, m.err
}

type mockSummaryTaskEngine struct {
	taskengine.TaskEngine
	stats taskengine.TaskStats
}

func (m *mockSummaryTaskEngine) GetStats() taskengine.TaskStats {
	return m.stats
}

func TestSummaryCountsDisabledUsersWithStatusTwo(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}

	if err := db.AutoMigrate(&models.User{}); err != nil {
		t.Fatalf("migrate users: %v", err)
	}

	if err := db.Create([]*models.User{
		{Username: "active", Password: "secret", Status: 1},
		{Username: "disabled", Password: "secret", Status: 2},
	}).Error; err != nil {
		t.Fatalf("seed users: %v", err)
	}

	router := gin.New()
	wrapper := httpcontext.NewHandlerFuncWrapper(zap.NewNop())
	router.GET("/summary", wrapper.Wrap(NewHandler(db, zap.NewNop(), nil, nil, nil, nil, nil, nil, nil, nil).Summary()))

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/summary", nil)
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", recorder.Code, recorder.Body.String())
	}

	var response struct {
		Code int             `json:"code"`
		Data SummaryResponse `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if response.Code != http.StatusOK {
		t.Fatalf("expected business code 200, got %d", response.Code)
	}

	if response.Data.Users.Total != 2 {
		t.Fatalf("expected total users 2, got %d", response.Data.Users.Total)
	}

	if response.Data.Users.Active != 1 {
		t.Fatalf("expected active users 1, got %d", response.Data.Users.Active)
	}

	if response.Data.Users.Disabled != 1 {
		t.Fatalf("expected disabled users 1, got %d", response.Data.Users.Disabled)
	}
}

func TestSummaryReturnsZeroCountsWhenDBMissing(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	wrapper := httpcontext.NewHandlerFuncWrapper(zap.NewNop())
	router.GET("/summary", wrapper.Wrap(NewHandler(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil).Summary()))

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/summary", nil)
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", recorder.Code, recorder.Body.String())
	}

	var response struct {
		Code int             `json:"code"`
		Data SummaryResponse `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if response.Code != http.StatusOK {
		t.Fatalf("expected business code 200, got %d", response.Code)
	}

	if response.Data.Users.Total != 0 || response.Data.MountPoints.Total != 0 || response.Data.CloudTokens.Total != 0 {
		t.Fatalf("expected zero counts without db, got users=%d mountPoints=%d cloudTokens=%d",
			response.Data.Users.Total,
			response.Data.MountPoints.Total,
			response.Data.CloudTokens.Total,
		)
	}
}

func TestSummaryToleratesCountErrorsWithoutLogger(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}

	router := gin.New()
	wrapper := httpcontext.NewHandlerFuncWrapper(zap.NewNop())
	router.GET("/summary", wrapper.Wrap(NewHandler(db, nil, nil, nil, nil, nil, nil, nil, nil, nil).Summary()))

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/summary", nil)
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", recorder.Code, recorder.Body.String())
	}

	var response struct {
		Code int             `json:"code"`
		Data SummaryResponse `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if response.Code != http.StatusOK {
		t.Fatalf("expected business code 200, got %d", response.Code)
	}

	if response.Data.Users.Total != 0 || response.Data.VirtualFiles.Files != 0 {
		t.Fatalf("expected failed counts to fall back to zero, got users=%d files=%d",
			response.Data.Users.Total,
			response.Data.VirtualFiles.Files,
		)
	}
}

func TestSummaryToleratesMediaConfigErrorsWithoutLogger(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}

	mediaConfigService := &mockSummaryMediaConfigService{err: errors.New("media config unavailable")}

	router := gin.New()
	wrapper := httpcontext.NewHandlerFuncWrapper(zap.NewNop())
	router.GET("/summary", wrapper.Wrap(NewHandler(db, nil, nil, nil, nil, nil, mediaConfigService, nil, nil, nil).Summary()))

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/summary", nil)
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", recorder.Code, recorder.Body.String())
	}

	var response struct {
		Code int             `json:"code"`
		Data SummaryResponse `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if response.Code != http.StatusOK {
		t.Fatalf("expected business code 200, got %d", response.Code)
	}

	if response.Data.Media.Enabled || response.Data.Media.StrmFiles != 0 || response.Data.Media.MediaFiles != 0 {
		t.Fatalf("expected media summary to fall back to disabled zero state, got %+v", response.Data.Media)
	}
}

func TestSummaryToleratesTypedNilMediaConfigService(t *testing.T) {
	gin.SetMode(gin.TestMode)

	var mediaConfigService *mockSummaryMediaConfigService

	router := gin.New()
	wrapper := httpcontext.NewHandlerFuncWrapper(zap.NewNop())
	router.GET("/summary", wrapper.Wrap(NewHandler(nil, zap.NewNop(), nil, nil, nil, nil, mediaConfigService, nil, nil, nil).Summary()))

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/summary", nil)
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", recorder.Code, recorder.Body.String())
	}

	var response struct {
		Code int             `json:"code"`
		Data SummaryResponse `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if response.Code != http.StatusOK {
		t.Fatalf("expected business code 200, got %d", response.Code)
	}

	if response.Data.Media.Enabled || response.Data.Media.StrmFiles != 0 || response.Data.Media.MediaFiles != 0 {
		t.Fatalf("expected typed-nil media config service to keep default media summary, got %+v", response.Data.Media)
	}
}

func TestSummaryToleratesTypedNilTaskEngine(t *testing.T) {
	gin.SetMode(gin.TestMode)

	var taskEngine *mockSummaryTaskEngine

	router := gin.New()
	wrapper := httpcontext.NewHandlerFuncWrapper(zap.NewNop())
	router.GET("/summary", wrapper.Wrap(NewHandler(nil, zap.NewNop(), nil, nil, nil, nil, nil, nil, nil, taskEngine).Summary()))

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/summary", nil)
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", recorder.Code, recorder.Body.String())
	}

	var response struct {
		Code int             `json:"code"`
		Data SummaryResponse `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if response.Code != http.StatusOK {
		t.Fatalf("expected business code 200, got %d", response.Code)
	}

	if response.Data.Tasks.Pending != 0 || response.Data.Tasks.Running != 0 ||
		response.Data.Tasks.Failed != 0 || response.Data.Tasks.Completed != 0 {
		t.Fatalf("expected typed-nil task engine to keep zero task summary, got %+v", response.Data.Tasks)
	}
}

func TestSummaryIncludesTaskStatsWhenTaskEngineAvailable(t *testing.T) {
	gin.SetMode(gin.TestMode)

	taskEngine := &mockSummaryTaskEngine{
		stats: taskengine.TaskStats{
			PendingTasks:   1,
			RunningTasks:   2,
			FailedTasks:    3,
			CompletedTasks: 4,
		},
	}

	router := gin.New()
	wrapper := httpcontext.NewHandlerFuncWrapper(zap.NewNop())
	router.GET("/summary", wrapper.Wrap(NewHandler(nil, zap.NewNop(), nil, nil, nil, nil, nil, nil, nil, taskEngine).Summary()))

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/summary", nil)
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", recorder.Code, recorder.Body.String())
	}

	var response struct {
		Code int             `json:"code"`
		Data SummaryResponse `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if response.Code != http.StatusOK {
		t.Fatalf("expected business code 200, got %d", response.Code)
	}

	if response.Data.Tasks.Pending != 1 || response.Data.Tasks.Running != 2 ||
		response.Data.Tasks.Failed != 3 || response.Data.Tasks.Completed != 4 {
		t.Fatalf("expected task stats to be included, got %+v", response.Data.Tasks)
	}
}
