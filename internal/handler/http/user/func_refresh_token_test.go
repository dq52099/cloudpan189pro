package user

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	appContext "github.com/xxcheng123/cloudpan189-share/internal/framework/context"
	"github.com/xxcheng123/cloudpan189-share/internal/framework/httpcontext"
	"github.com/xxcheng123/cloudpan189-share/internal/repository/models"
	userSvi "github.com/xxcheng123/cloudpan189-share/internal/services/user"
	loginlogType "github.com/xxcheng123/cloudpan189-share/internal/types/loginlog"
)

type refreshTokenUserServiceStub struct {
	userSvi.Service

	user *models.User

	parseUID     int64
	parseName    string
	parseVersion int
	parseErr     error

	queryErr error

	accessToken       string
	generateAccessErr error

	refreshToken       string
	generateRefreshErr error

	expire int64

	parseTokenInput      string
	queryCalls           int
	queryUID             int64
	generateAccessCalls  int
	generateRefreshCalls int
	accessTokenUserID    int64
	accessTokenUsername  string
	accessTokenVersion   int
	refreshTokenUserID   int64
	refreshTokenUsername string
	refreshTokenVersion  int
}

func (s *refreshTokenUserServiceStub) ParseRefreshToken(token string) (int64, string, int, error) {
	s.parseTokenInput = token
	if s.parseErr != nil {
		return 0, "", 0, s.parseErr
	}

	return s.parseUID, s.parseName, s.parseVersion, nil
}

func (s *refreshTokenUserServiceStub) Query(_ appContext.Context, uid int64) (*models.User, error) {
	s.queryCalls++

	s.queryUID = uid
	if s.queryErr != nil {
		return nil, s.queryErr
	}

	return s.user, nil
}

func (s *refreshTokenUserServiceStub) GenerateAccessToken(userID int64, username string, version int) (string, error) {
	s.generateAccessCalls++
	s.accessTokenUserID = userID
	s.accessTokenUsername = username

	s.accessTokenVersion = version
	if s.generateAccessErr != nil {
		return "", s.generateAccessErr
	}

	return s.accessToken, nil
}

func (s *refreshTokenUserServiceStub) GenerateRefreshToken(userID int64, username string, version int) (string, error) {
	s.generateRefreshCalls++
	s.refreshTokenUserID = userID
	s.refreshTokenUsername = username

	s.refreshTokenVersion = version
	if s.generateRefreshErr != nil {
		return "", s.generateRefreshErr
	}

	return s.refreshToken, nil
}

func (s *refreshTokenUserServiceStub) GetExpire() int64 {
	return s.expire
}

func newRefreshTokenTestRouter(userService userSvi.Service) *gin.Engine {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	wrapper := httpcontext.NewHandlerFuncWrapper(zap.NewNop())
	router.POST("/refresh_token", wrapper.Wrap(NewHandler(userService, nil, nil).RefreshToken()))

	return router
}

func newRefreshTokenLogTestRouter(
	userService userSvi.Service,
	loginLogService *recordLogLoginLogServiceStub,
) *gin.Engine {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	wrapper := httpcontext.NewHandlerFuncWrapper(zap.NewNop())
	handler := NewHandler(userService, nil, loginLogService)
	router.POST(
		"/refresh_token",
		wrapper.Wrap(handler.RecordLog(loginlogType.EventRefreshToken)),
		wrapper.Wrap(handler.RefreshToken()),
	)

	return router
}

func postRefreshToken(router *gin.Engine, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/refresh_token", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	return recorder
}

func assertRefreshTokenBusinessError(t *testing.T, recorder *httptest.ResponseRecorder, want httpcontext.BusinessError) {
	t.Helper()

	if recorder.Code != want.GetHTTPCode() {
		t.Fatalf("expected status %d, got %d body=%s", want.GetHTTPCode(), recorder.Code, recorder.Body.String())
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

func TestRefreshTokenReturnsInvalidWhenRefreshTokenParseFails(t *testing.T) {
	stub := &refreshTokenUserServiceStub{parseErr: errors.New("bad refresh token")}
	router := newRefreshTokenTestRouter(stub)

	recorder := postRefreshToken(router, `{"refreshToken":"old-refresh"}`)

	assertRefreshTokenBusinessError(t, recorder, codeRefreshTokenInvalid)

	if stub.parseTokenInput != "old-refresh" {
		t.Fatalf("expected parsed token old-refresh, got %q", stub.parseTokenInput)
	}

	if stub.queryCalls != 0 {
		t.Fatalf("expected Query not to be called, got %d calls", stub.queryCalls)
	}

	if stub.generateAccessCalls != 0 || stub.generateRefreshCalls != 0 {
		t.Fatalf("expected no token generation, got access=%d refresh=%d", stub.generateAccessCalls, stub.generateRefreshCalls)
	}
}

func TestRefreshTokenReturnsInvalidWhenUserQueryFails(t *testing.T) {
	stub := &refreshTokenUserServiceStub{
		parseUID:     7,
		parseName:    "alice",
		parseVersion: 2,
		queryErr:     errors.New("user missing"),
	}
	router := newRefreshTokenTestRouter(stub)

	recorder := postRefreshToken(router, `{"refreshToken":"old-refresh"}`)

	assertRefreshTokenBusinessError(t, recorder, codeRefreshTokenInvalid)

	if stub.queryCalls != 1 || stub.queryUID != 7 {
		t.Fatalf("expected Query uid 7 once, got uid=%d calls=%d", stub.queryUID, stub.queryCalls)
	}

	if stub.generateAccessCalls != 0 || stub.generateRefreshCalls != 0 {
		t.Fatalf("expected no token generation, got access=%d refresh=%d", stub.generateAccessCalls, stub.generateRefreshCalls)
	}
}

func TestRefreshTokenReturnsInvalidWhenUserQueryReturnsNil(t *testing.T) {
	stub := &refreshTokenUserServiceStub{
		parseUID:     7,
		parseName:    "alice",
		parseVersion: 2,
	}
	router := newRefreshTokenTestRouter(stub)

	recorder := postRefreshToken(router, `{"refreshToken":"old-refresh"}`)

	assertRefreshTokenBusinessError(t, recorder, codeRefreshTokenInvalid)

	if stub.queryCalls != 1 || stub.queryUID != 7 {
		t.Fatalf("expected Query uid 7 once, got uid=%d calls=%d", stub.queryUID, stub.queryCalls)
	}

	if stub.generateAccessCalls != 0 || stub.generateRefreshCalls != 0 {
		t.Fatalf("expected no token generation, got access=%d refresh=%d", stub.generateAccessCalls, stub.generateRefreshCalls)
	}
}

func TestRefreshTokenFailureLogKeepsParsedUsernameWhenUserQueryFails(t *testing.T) {
	stub := &refreshTokenUserServiceStub{
		parseUID:     7,
		parseName:    "claim-alice",
		parseVersion: 2,
		queryErr:     errors.New("user missing"),
	}
	loginLogService := &recordLogLoginLogServiceStub{}
	router := newRefreshTokenLogTestRouter(stub, loginLogService)

	recorder := postRefreshToken(router, `{"refreshToken":"old-refresh"}`)

	assertRefreshTokenBusinessError(t, recorder, codeRefreshTokenInvalid)

	if loginLogService.createdLog == nil {
		t.Fatal("expected login log to be created")
	}

	if loginLogService.createdLog.UserId != 7 {
		t.Fatalf("expected login log user id 7, got %d", loginLogService.createdLog.UserId)
	}

	if loginLogService.createdLog.Username != "claim-alice" {
		t.Fatalf("expected login log username claim-alice, got %q", loginLogService.createdLog.Username)
	}

	if loginLogService.createdLog.Event != loginlogType.EventRefreshToken {
		t.Fatalf("expected refresh token event, got %s", loginLogService.createdLog.Event)
	}

	if loginLogService.createdLog.Status != loginlogType.StatusFailed {
		t.Fatalf("expected failed login log status, got %s", loginLogService.createdLog.Status)
	}

	if loginLogService.createdLog.Reason != codeRefreshTokenInvalid.GetMessage() {
		t.Fatalf("expected login log reason %q, got %q", codeRefreshTokenInvalid.GetMessage(), loginLogService.createdLog.Reason)
	}
}

func TestRefreshTokenRejectsDisabledOrStaleUser(t *testing.T) {
	tests := []struct {
		name string
		user *models.User
		want httpcontext.BusinessError
	}{
		{
			name: "disabled user",
			user: &models.User{ID: 7, Username: "alice", Version: 2, Status: 0},
			want: codeUserDisabled,
		},
		{
			name: "stale user version",
			user: &models.User{ID: 7, Username: "alice", Version: 3, Status: 1},
			want: codeUserInfoUpdated,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stub := &refreshTokenUserServiceStub{
				user:         tt.user,
				parseUID:     7,
				parseName:    "alice",
				parseVersion: 2,
			}
			router := newRefreshTokenTestRouter(stub)

			recorder := postRefreshToken(router, `{"refreshToken":"old-refresh"}`)

			assertRefreshTokenBusinessError(t, recorder, tt.want)

			if stub.generateAccessCalls != 0 || stub.generateRefreshCalls != 0 {
				t.Fatalf("expected no token generation, got access=%d refresh=%d", stub.generateAccessCalls, stub.generateRefreshCalls)
			}
		})
	}
}

func TestRefreshTokenReturnsGenerateFailedWhenSigningFails(t *testing.T) {
	tests := []struct {
		name               string
		generateAccessErr  error
		generateRefreshErr error
		wantAccessCalls    int
		wantRefreshCalls   int
	}{
		{
			name:              "access token generation fails",
			generateAccessErr: errors.New("access signing failed"),
			wantAccessCalls:   1,
		},
		{
			name:               "refresh token generation fails",
			generateRefreshErr: errors.New("refresh signing failed"),
			wantAccessCalls:    1,
			wantRefreshCalls:   1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stub := &refreshTokenUserServiceStub{
				user:               &models.User{ID: 7, Username: "alice", Version: 2, Status: 1},
				parseUID:           7,
				parseName:          "alice",
				parseVersion:       2,
				generateAccessErr:  tt.generateAccessErr,
				generateRefreshErr: tt.generateRefreshErr,
				accessToken:        "new-access",
			}
			router := newRefreshTokenTestRouter(stub)

			recorder := postRefreshToken(router, `{"refreshToken":"old-refresh"}`)

			assertRefreshTokenBusinessError(t, recorder, codeTokenGenerateFailed)

			if stub.generateAccessCalls != tt.wantAccessCalls {
				t.Fatalf("expected GenerateAccessToken calls %d, got %d", tt.wantAccessCalls, stub.generateAccessCalls)
			}

			if stub.generateRefreshCalls != tt.wantRefreshCalls {
				t.Fatalf("expected GenerateRefreshToken calls %d, got %d", tt.wantRefreshCalls, stub.generateRefreshCalls)
			}
		})
	}
}

func TestRefreshTokenSuccessUsesPersistedUserForNewTokens(t *testing.T) {
	stub := &refreshTokenUserServiceStub{
		user:         &models.User{ID: 7, Username: "db-alice", Version: 4, Status: 1},
		parseUID:     7,
		parseName:    "claim-alice",
		parseVersion: 4,
		accessToken:  "new-access",
		refreshToken: "new-refresh",
		expire:       3600,
	}
	router := newRefreshTokenTestRouter(stub)

	recorder := postRefreshToken(router, `{"refreshToken":"old-refresh"}`)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusOK, recorder.Code, recorder.Body.String())
	}

	var response struct {
		Code int `json:"code"`
		Data struct {
			AccessToken  string       `json:"accessToken"`
			RefreshToken string       `json:"refreshToken"`
			TokenType    string       `json:"tokenType"`
			ExpiresIn    int64        `json:"expiresIn"`
			User         *models.User `json:"user"`
		} `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if response.Code != http.StatusOK {
		t.Fatalf("expected business code %d, got %d", http.StatusOK, response.Code)
	}

	if response.Data.AccessToken != "new-access" || response.Data.RefreshToken != "new-refresh" {
		t.Fatalf("unexpected token response: %#v", response.Data)
	}

	if response.Data.TokenType != "Bearer" || response.Data.ExpiresIn != 3600 {
		t.Fatalf("unexpected token metadata: tokenType=%q expiresIn=%d", response.Data.TokenType, response.Data.ExpiresIn)
	}

	if response.Data.User == nil || response.Data.User.Username != "db-alice" {
		t.Fatalf("expected persisted user db-alice, got %#v", response.Data.User)
	}

	if stub.accessTokenUserID != 7 || stub.accessTokenUsername != "db-alice" || stub.accessTokenVersion != 4 {
		t.Fatalf(
			"expected access token generated from persisted user, got userID=%d username=%q version=%d",
			stub.accessTokenUserID,
			stub.accessTokenUsername,
			stub.accessTokenVersion,
		)
	}

	if stub.refreshTokenUserID != 7 || stub.refreshTokenUsername != "db-alice" || stub.refreshTokenVersion != 4 {
		t.Fatalf(
			"expected refresh token generated from persisted user, got userID=%d username=%q version=%d",
			stub.refreshTokenUserID,
			stub.refreshTokenUsername,
			stub.refreshTokenVersion,
		)
	}
}
