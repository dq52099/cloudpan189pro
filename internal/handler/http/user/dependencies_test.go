package user

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/xxcheng123/cloudpan189-share/internal/consts"
	appContext "github.com/xxcheng123/cloudpan189-share/internal/framework/context"
	"github.com/xxcheng123/cloudpan189-share/internal/framework/httpcontext"
	"github.com/xxcheng123/cloudpan189-share/internal/repository/models"
	userSvi "github.com/xxcheng123/cloudpan189-share/internal/services/user"
	"github.com/xxcheng123/cloudpan189-share/internal/types/loginlog"
	"go.uber.org/zap"
)

func TestUserHandlersReturnBusinessErrorWhenUserServiceMissing(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name          string
		method        string
		routePath     string
		requestTarget string
		body          string
		before        []gin.HandlerFunc
		handler       func(Handler) httpcontext.HandlerFunc
		want          httpcontext.BusinessError
	}{
		{
			name:          "add",
			method:        http.MethodPost,
			routePath:     "/add",
			requestTarget: "/add",
			body:          `{"username":"alice","password":"password"}`,
			handler:       func(h Handler) httpcontext.HandlerFunc { return h.Add() },
			want:          codeAddUserFailed,
		},
		{
			name:          "login",
			method:        http.MethodPost,
			routePath:     "/login",
			requestTarget: "/login",
			body:          `{"username":"alice","password":"password"}`,
			handler:       func(h Handler) httpcontext.HandlerFunc { return h.Login() },
			want:          codeLoginFailed,
		},
		{
			name:          "refresh token",
			method:        http.MethodPost,
			routePath:     "/refresh_token",
			requestTarget: "/refresh_token",
			body:          `{"refreshToken":"old-refresh"}`,
			handler:       func(h Handler) httpcontext.HandlerFunc { return h.RefreshToken() },
			want:          codeRefreshTokenInvalid,
		},
		{
			name:          "delete",
			method:        http.MethodPost,
			routePath:     "/del",
			requestTarget: "/del",
			body:          `{"id":99}`,
			handler:       func(h Handler) httpcontext.HandlerFunc { return h.Del() },
			want:          codeDelUserFailed,
		},
		{
			name:          "update",
			method:        http.MethodPost,
			routePath:     "/update",
			requestTarget: "/update",
			body:          `{"id":99,"status":2}`,
			handler:       func(h Handler) httpcontext.HandlerFunc { return h.Update() },
			want:          codeUpdateUserFailed,
		},
		{
			name:          "toggle status",
			method:        http.MethodPost,
			routePath:     "/toggle_status",
			requestTarget: "/toggle_status",
			body:          `{"id":99,"status":2}`,
			handler:       func(h Handler) httpcontext.HandlerFunc { return h.ToggleStatus() },
			want:          codeUpdateUserFailed,
		},
		{
			name:          "modify pass",
			method:        http.MethodPost,
			routePath:     "/modify_pass",
			requestTarget: "/modify_pass",
			body:          `{"id":99,"password":"new-pass"}`,
			handler:       func(h Handler) httpcontext.HandlerFunc { return h.ModifyPass() },
			want:          codeModifyPassFailed,
		},
		{
			name:          "bind default group",
			method:        http.MethodPost,
			routePath:     "/bind_group",
			requestTarget: "/bind_group",
			body:          `{"userId":99,"groupId":0}`,
			handler:       func(h Handler) httpcontext.HandlerFunc { return h.BindGroup() },
			want:          codeBindGroupFailed,
		},
		{
			name:          "info",
			method:        http.MethodGet,
			routePath:     "/info",
			requestTarget: "/info",
			before:        []gin.HandlerFunc{setCurrentUserID(99)},
			handler:       func(h Handler) httpcontext.HandlerFunc { return h.Info() },
			want:          codeUserInfoFailed,
		},
		{
			name:          "modify own pass",
			method:        http.MethodPost,
			routePath:     "/modify_own_pass",
			requestTarget: "/modify_own_pass",
			body:          `{"oldPassword":"old-pass","password":"new-pass"}`,
			before:        []gin.HandlerFunc{setCurrentUserID(99)},
			handler:       func(h Handler) httpcontext.HandlerFunc { return h.ModifyOwnPass() },
			want:          codeModifyPassFailed,
		},
		{
			name:          "list",
			method:        http.MethodGet,
			routePath:     "/list",
			requestTarget: "/list?noPaginate=true",
			handler:       func(h Handler) httpcontext.HandlerFunc { return h.List() },
			want:          codeListUserFailed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := gin.New()
			wrapper := httpcontext.NewHandlerFuncWrapper(zap.NewNop())
			handler := NewHandler(nil, nil, nil)

			handlers := append([]gin.HandlerFunc{}, tt.before...)
			handlers = append(handlers, wrapper.Wrap(tt.handler(handler)))
			router.Handle(tt.method, tt.routePath, handlers...)

			recorder := httptest.NewRecorder()

			request := httptest.NewRequest(tt.method, tt.requestTarget, strings.NewReader(tt.body))
			if tt.body != "" {
				request.Header.Set("Content-Type", "application/json")
			}

			router.ServeHTTP(recorder, request)

			assertUserBusinessError(t, recorder, tt.want)
		})
	}
}

func TestUpdateWithoutFieldsDoesNotRequireUserService(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	wrapper := httpcontext.NewHandlerFuncWrapper(zap.NewNop())
	router.POST("/update", wrapper.Wrap(NewHandler(nil, nil, nil).Update()))

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/update", strings.NewReader(`{"id":99}`))
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, request)

	assertUserBusinessError(t, recorder, codeNoUpdateFields)
}

func TestLoginReturnsNotFoundWhenUserQueryReturnsNil(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	wrapper := httpcontext.NewHandlerFuncWrapper(zap.NewNop())
	router.POST("/login", wrapper.Wrap(NewHandler(&loginUserServiceStub{}, nil, nil).Login()))

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(
		http.MethodPost,
		"/login",
		strings.NewReader(`{"username":"alice","password":"password"}`),
	)
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, request)

	assertUserBusinessError(t, recorder, codeUserNotFound)
}

func TestListRequiresUserGroupServiceOnlyWhenGroupLookupNeeded(t *testing.T) {
	gin.SetMode(gin.TestMode)

	userSvc := &listUserServiceStub{
		users: []*models.User{
			{ID: 1, Username: "default", GroupID: 0},
		},
	}

	router := gin.New()
	wrapper := httpcontext.NewHandlerFuncWrapper(zap.NewNop())
	router.GET("/list", wrapper.Wrap(NewHandler(userSvc, nil, nil).List()))

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/list?noPaginate=true", nil)
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
}

func TestListReturnsBusinessErrorWhenUserGroupServiceMissing(t *testing.T) {
	gin.SetMode(gin.TestMode)

	userSvc := &listUserServiceStub{
		users: []*models.User{
			{ID: 2, Username: "vip-user", GroupID: 2},
		},
	}

	router := gin.New()
	wrapper := httpcontext.NewHandlerFuncWrapper(zap.NewNop())
	router.GET("/list", wrapper.Wrap(NewHandler(userSvc, nil, nil).List()))

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/list?noPaginate=true", nil)
	router.ServeHTTP(recorder, request)

	assertUserBusinessError(t, recorder, codeListUserFailed)
}

func TestListReturnsBusinessErrorWhenUserServiceTypedNil(t *testing.T) {
	gin.SetMode(gin.TestMode)

	var userSvc *listUserServiceStub

	router := gin.New()
	wrapper := httpcontext.NewHandlerFuncWrapper(zap.NewNop())
	router.GET("/list", wrapper.Wrap(NewHandler(userSvc, nil, nil).List()))

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/list?noPaginate=true", nil)
	router.ServeHTTP(recorder, request)

	assertUserBusinessError(t, recorder, codeListUserFailed)
}

func TestListReturnsBusinessErrorWhenUserGroupServiceTypedNil(t *testing.T) {
	gin.SetMode(gin.TestMode)

	userSvc := &listUserServiceStub{
		users: []*models.User{
			{ID: 2, Username: "vip-user", GroupID: 2},
		},
	}

	var userGroupSvc *listUserGroupServiceStub

	router := gin.New()
	wrapper := httpcontext.NewHandlerFuncWrapper(zap.NewNop())
	router.GET("/list", wrapper.Wrap(NewHandler(userSvc, userGroupSvc, nil).List()))

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/list?noPaginate=true", nil)
	router.ServeHTTP(recorder, request)

	assertUserBusinessError(t, recorder, codeListUserFailed)
}

func TestBindGroupReturnsBusinessErrorWhenUserGroupServiceMissing(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := newUserNotFoundRouter(
		&userNotFoundServiceStub{queryUser: &models.User{ID: 99}},
		nil,
	)

	req := httptest.NewRequest(
		http.MethodPost,
		"/bind_group",
		strings.NewReader(`{"userId":99,"groupId":2}`),
	)
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	assertUserBusinessError(t, recorder, codeBindGroupFailed)
}

func TestInfoReturnsBusinessErrorWhenUserGroupServiceMissing(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := newUserNotFoundRouter(
		&userNotFoundServiceStub{queryUser: &models.User{ID: 99, Username: "alice", GroupID: 2}},
		nil,
	)

	req := httptest.NewRequest(http.MethodGet, "/info", nil)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	assertUserBusinessError(t, recorder, codeUserInfoFailed)
}

func TestRecordLogSkipsMissingLoginLogServiceWithoutFailingRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	wrapper := httpcontext.NewHandlerFuncWrapper(zap.NewNop())
	handler := NewHandler(nil, nil, nil)
	router.GET(
		"/record",
		wrapper.Wrap(handler.RecordLog(loginlog.EventLogin)),
		wrapper.Wrap(func(ctx *httpcontext.Context) {
			ctx.Success()
		}),
	)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/record", nil)
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
}

func setCurrentUserID(uid int64) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		ctx.Set(consts.CtxKeyUserId, uid)
	}
}

type loginUserServiceStub struct {
	userSvi.Service
}

func (s *loginUserServiceStub) QueryByUsername(appContext.Context, string) (*models.User, error) {
	return nil, nil
}

func assertUserBusinessError(t *testing.T, recorder *httptest.ResponseRecorder, want httpcontext.BusinessError) {
	t.Helper()

	if recorder.Code != want.GetHTTPCode() {
		t.Fatalf("expected http status %d, got %d body=%s", want.GetHTTPCode(), recorder.Code, recorder.Body.String())
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
