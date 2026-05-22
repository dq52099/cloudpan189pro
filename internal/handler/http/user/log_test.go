package user

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	frameworkContext "github.com/xxcheng123/cloudpan189-share/internal/framework/context"
	"github.com/xxcheng123/cloudpan189-share/internal/framework/httpcontext"
	"github.com/xxcheng123/cloudpan189-share/internal/repository/models"
	loginlogSvi "github.com/xxcheng123/cloudpan189-share/internal/services/loginlog"
	"github.com/xxcheng123/cloudpan189-share/internal/types/loginlog"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

func TestRecordLogReportsCreateFailureWithoutFailingRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)

	core, logs := observer.New(zap.ErrorLevel)
	logger := zap.New(core)
	loginLogService := &recordLogLoginLogServiceStub{err: errors.New("write login log failed")}
	handler := NewHandler(nil, nil, loginLogService)

	router := gin.New()
	wrapper := httpcontext.NewHandlerFuncWrapper(logger)
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

	if !loginLogService.createCalled {
		t.Fatal("expected login log create to be called")
	}

	if logs.FilterMessage("记录登录日志失败").Len() != 1 {
		t.Fatalf("expected one login log failure entry, got %d", logs.Len())
	}
}

type recordLogLoginLogServiceStub struct {
	err          error
	createCalled bool
}

func (s *recordLogLoginLogServiceStub) Create(_ frameworkContext.Context, _ *models.LoginLog) (int64, error) {
	s.createCalled = true

	return 0, s.err
}

func (s *recordLogLoginLogServiceStub) RecordLogin(frameworkContext.Context, *loginlogSvi.RecordLoginInput) (int64, error) {
	return 0, errors.New("not implemented")
}

func (s *recordLogLoginLogServiceStub) RecordRefreshToken(frameworkContext.Context, *loginlogSvi.RecordRefreshInput) (int64, error) {
	return 0, errors.New("not implemented")
}

func (s *recordLogLoginLogServiceStub) List(frameworkContext.Context, *loginlogSvi.ListRequest) ([]*models.LoginLog, error) {
	return nil, errors.New("not implemented")
}

func (s *recordLogLoginLogServiceStub) Count(frameworkContext.Context, *loginlogSvi.ListRequest) (int64, error) {
	return 0, errors.New("not implemented")
}

func (s *recordLogLoginLogServiceStub) ClearAll(frameworkContext.Context) (int64, error) {
	return 0, errors.New("not implemented")
}

func (s *recordLogLoginLogServiceStub) ClearBefore(frameworkContext.Context, time.Time) (int64, error) {
	return 0, errors.New("not implemented")
}
