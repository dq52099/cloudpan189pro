package user

import (
	stdctx "context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/xxcheng123/cloudpan189-share/internal/consts"
	appContext "github.com/xxcheng123/cloudpan189-share/internal/framework/context"
	"github.com/xxcheng123/cloudpan189-share/internal/framework/httpcontext"
	"github.com/xxcheng123/cloudpan189-share/internal/pkgs/utils"
	"github.com/xxcheng123/cloudpan189-share/internal/repository/models"
	userSvi "github.com/xxcheng123/cloudpan189-share/internal/services/user"
	usergroupSvi "github.com/xxcheng123/cloudpan189-share/internal/services/usergroup"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type userNotFoundServiceStub struct {
	userSvi.Service

	queryUser     *models.User
	queryErr      error
	delErr        error
	modifyPassErr error
	bindGroupErr  error
	updateErr     error
	bindGroupCall int
}

func (s *userNotFoundServiceStub) Query(appContext.Context, int64) (*models.User, error) {
	if s.queryErr != nil {
		return nil, s.queryErr
	}

	return s.queryUser, nil
}

func (s *userNotFoundServiceStub) ModifyPass(appContext.Context, int64, string) error {
	return s.modifyPassErr
}

func (s *userNotFoundServiceStub) Del(appContext.Context, *userSvi.DelRequest) error {
	return s.delErr
}

func (s *userNotFoundServiceStub) BindGroup(appContext.Context, *userSvi.BindGroupRequest) error {
	s.bindGroupCall++

	return s.bindGroupErr
}

func (s *userNotFoundServiceStub) QueryByUsername(appContext.Context, string) (*models.User, error) {
	return nil, errors.New("not implemented")
}

func (s *userNotFoundServiceStub) Update(appContext.Context, int64, ...utils.Field) error {
	return s.updateErr
}

type userGroupNotFoundServiceStub struct {
	usergroupSvi.Service

	queryErr  error
	queryCall int
}

func (s *userGroupNotFoundServiceStub) Query(appContext.Context, int64) (*models.UserGroup, error) {
	s.queryCall++
	if s.queryErr != nil {
		return nil, s.queryErr
	}

	return &models.UserGroup{ID: 2, Name: "test-group"}, nil
}

func newUserNotFoundRouter(
	userService userSvi.Service,
	userGroupService usergroupSvi.Service,
) *gin.Engine {
	router := gin.New()
	wrapper := httpcontext.NewHandlerFuncWrapper(zap.NewNop())
	handler := NewHandler(userService, userGroupService, nil)

	router.POST("/modify_pass", wrapper.Wrap(handler.ModifyPass()))
	router.POST("/bind_group", wrapper.Wrap(handler.BindGroup()))
	router.POST("/update", wrapper.Wrap(handler.Update()))
	router.POST("/toggle_status", wrapper.Wrap(handler.ToggleStatus()))
	router.POST("/del", wrapper.Wrap(handler.Del()))
	router.GET("/info", func(ctx *gin.Context) {
		ctx.Set(consts.CtxKeyUserId, int64(99))
	}, wrapper.Wrap(handler.Info()))
	router.POST("/modify_own_pass", func(ctx *gin.Context) {
		ctx.Set(consts.CtxKeyUserId, int64(99))
	}, wrapper.Wrap(handler.ModifyOwnPass()))

	return router
}

func assertUserHTTPError(t *testing.T, recorder *httptest.ResponseRecorder, want httpcontext.BusinessError) {
	t.Helper()

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("expected http status 404, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	var response httpcontext.Response
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if response.Code != want.GetCode() {
		t.Fatalf("expected business code %d, got %d", want.GetCode(), response.Code)
	}

	if response.Msg != want.GetMessage() {
		t.Fatalf("expected message %q, got %q", want.GetMessage(), response.Msg)
	}
}

func TestModifyPassReturnsNotFoundWhenUserMissing(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := newUserNotFoundRouter(
		&userNotFoundServiceStub{modifyPassErr: gorm.ErrRecordNotFound},
		&userGroupNotFoundServiceStub{},
	)

	req := httptest.NewRequestWithContext(
		stdctx.Background(),
		http.MethodPost,
		"/modify_pass",
		strings.NewReader(`{"id":99,"password":"new-pass"}`),
	)
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	assertUserHTTPError(t, recorder, codeUserResourceMissing)
}

func TestDelReturnsNotFoundWhenUserMissing(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := newUserNotFoundRouter(
		&userNotFoundServiceStub{delErr: gorm.ErrRecordNotFound},
		&userGroupNotFoundServiceStub{},
	)

	req := httptest.NewRequestWithContext(
		stdctx.Background(),
		http.MethodPost,
		"/del",
		strings.NewReader(`{"id":99}`),
	)
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	assertUserHTTPError(t, recorder, codeUserResourceMissing)
}

func TestInfoReturnsNotFoundWhenCurrentUserMissing(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := newUserNotFoundRouter(
		&userNotFoundServiceStub{queryErr: gorm.ErrRecordNotFound},
		&userGroupNotFoundServiceStub{},
	)

	req := httptest.NewRequestWithContext(stdctx.Background(), http.MethodGet, "/info", nil)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	assertUserHTTPError(t, recorder, codeUserResourceMissing)
}

func TestUpdateReturnsNotFoundWhenUserMissing(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := newUserNotFoundRouter(
		&userNotFoundServiceStub{updateErr: gorm.ErrRecordNotFound},
		&userGroupNotFoundServiceStub{},
	)

	req := httptest.NewRequestWithContext(
		stdctx.Background(),
		http.MethodPost,
		"/update",
		strings.NewReader(`{"id":99,"status":2}`),
	)
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	assertUserHTTPError(t, recorder, codeUserResourceMissing)
}

func TestToggleStatusReturnsNotFoundWhenUserMissing(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := newUserNotFoundRouter(
		&userNotFoundServiceStub{updateErr: gorm.ErrRecordNotFound},
		&userGroupNotFoundServiceStub{},
	)

	req := httptest.NewRequestWithContext(
		stdctx.Background(),
		http.MethodPost,
		"/toggle_status",
		strings.NewReader(`{"id":99,"status":2}`),
	)
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	assertUserHTTPError(t, recorder, codeUserResourceMissing)
}

func TestBindGroupReturnsNotFoundWhenUserMissing(t *testing.T) {
	gin.SetMode(gin.TestMode)

	userGroupService := &userGroupNotFoundServiceStub{}
	userService := &userNotFoundServiceStub{queryErr: gorm.ErrRecordNotFound}
	router := newUserNotFoundRouter(userService, userGroupService)

	req := httptest.NewRequestWithContext(
		stdctx.Background(),
		http.MethodPost,
		"/bind_group",
		strings.NewReader(`{"userId":99,"groupId":2}`),
	)
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	assertUserHTTPError(t, recorder, codeUserResourceMissing)

	if userGroupService.queryCall != 0 {
		t.Fatalf("expected group lookup skipped, got %d calls", userGroupService.queryCall)
	}
}

func TestBindGroupReturnsNotFoundWhenUserGroupMissing(t *testing.T) {
	gin.SetMode(gin.TestMode)

	userService := &userNotFoundServiceStub{queryUser: &models.User{ID: 99}}
	userGroupService := &userGroupNotFoundServiceStub{queryErr: gorm.ErrRecordNotFound}
	router := newUserNotFoundRouter(userService, userGroupService)

	req := httptest.NewRequestWithContext(
		stdctx.Background(),
		http.MethodPost,
		"/bind_group",
		strings.NewReader(`{"userId":99,"groupId":2}`),
	)
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	assertUserHTTPError(t, recorder, codeUserGroupMissing)

	if userService.bindGroupCall != 0 {
		t.Fatalf("expected bind service skipped, got %d calls", userService.bindGroupCall)
	}
}

func TestModifyOwnPassReturnsNotFoundWhenCurrentUserMissing(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := newUserNotFoundRouter(
		&userNotFoundServiceStub{queryErr: gorm.ErrRecordNotFound},
		&userGroupNotFoundServiceStub{},
	)

	req := httptest.NewRequestWithContext(
		stdctx.Background(),
		http.MethodPost,
		"/modify_own_pass",
		strings.NewReader(`{"oldPassword":"old-pass","password":"new-pass"}`),
	)
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	assertUserHTTPError(t, recorder, codeUserResourceMissing)
}
