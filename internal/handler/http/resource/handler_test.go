package resource

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/xxcheng123/cloudpan189-share/internal/framework/httpcontext"
	"github.com/xxcheng123/cloudpan189-share/internal/repository/models"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

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
