package scheduler

import (
	stdContext "context"
	"errors"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/xxcheng123/cloudpan189-share/internal/bootstrap"
	appContext "github.com/xxcheng123/cloudpan189-share/internal/framework/context"
	"github.com/xxcheng123/cloudpan189-share/internal/pkgs/taskengine"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type schedulerStartServiceContext struct {
	taskEngine taskengine.TaskEngine
	logger     *zap.Logger
}

func (m *schedulerStartServiceContext) GetDB(appContext.Context) *gorm.DB {
	return nil
}

func (m *schedulerStartServiceContext) GetDBWithoutContext() *gorm.DB {
	return nil
}

func (m *schedulerStartServiceContext) GetLogger(string, ...zap.Field) *zap.Logger {
	return m.logger
}

func (m *schedulerStartServiceContext) Close() {}

func (m *schedulerStartServiceContext) GetPort() int {
	return 0
}

func (m *schedulerStartServiceContext) GetTaskEngine() taskengine.TaskEngine {
	return m.taskEngine
}

func (m *schedulerStartServiceContext) GetHTTPEngine() *gin.Engine {
	return nil
}

var _ bootstrap.ServiceContext = (*schedulerStartServiceContext)(nil)

type schedulerStartTaskEngine struct {
	taskengine.TaskEngine
}

func (m *schedulerStartTaskEngine) Start() error {
	return nil
}

func (m *schedulerStartTaskEngine) Stop() error {
	return nil
}

func (m *schedulerStartTaskEngine) IsRunning() bool {
	return true
}

func (m *schedulerStartTaskEngine) RegisterProcessor(taskengine.Topic, taskengine.MessageProcessor) error {
	return nil
}

func (m *schedulerStartTaskEngine) PushMessage(stdContext.Context, taskengine.Topic, []byte) error {
	return nil
}

func (m *schedulerStartTaskEngine) GetStats() taskengine.TaskStats {
	return taskengine.TaskStats{}
}

func (m *schedulerStartTaskEngine) GetRunningTasks() []*taskengine.TaskInfo {
	return nil
}

func (m *schedulerStartTaskEngine) GetPendingTasks() []*taskengine.TaskInfo {
	return nil
}

func (m *schedulerStartTaskEngine) SetWorkerCount(int) error {
	return nil
}

func TestStartRejectsNilServiceContext(t *testing.T) {
	var svc bootstrap.ServiceContext

	stop, err := Start(svc, nil)
	if !errors.Is(err, ErrSchedulerServiceContextMissing) {
		t.Fatalf("expected missing service context error, got %v", err)
	}

	if stop == nil {
		t.Fatal("expected no-op stop function")
	}
}

func TestStartRejectsTypedNilServiceContext(t *testing.T) {
	var (
		typedSvc *schedulerStartServiceContext
		svc      bootstrap.ServiceContext = typedSvc
	)

	stop, err := Start(svc, nil)
	if !errors.Is(err, ErrSchedulerServiceContextMissing) {
		t.Fatalf("expected missing service context error, got %v", err)
	}

	if stop == nil {
		t.Fatal("expected no-op stop function")
	}
}

func TestStartRejectsNilTaskEngine(t *testing.T) {
	stop, err := Start(&schedulerStartServiceContext{logger: zap.NewNop()}, nil)
	if !errors.Is(err, ErrSchedulerTaskEngineMissing) {
		t.Fatalf("expected missing task engine error, got %v", err)
	}

	if stop == nil {
		t.Fatal("expected no-op stop function")
	}
}

func TestStartRejectsTypedNilTaskEngine(t *testing.T) {
	var taskEngine *schedulerStartTaskEngine

	stop, err := Start(&schedulerStartServiceContext{taskEngine: taskEngine, logger: zap.NewNop()}, nil)
	if !errors.Is(err, ErrSchedulerTaskEngineMissing) {
		t.Fatalf("expected missing task engine error, got %v", err)
	}

	if stop == nil {
		t.Fatal("expected no-op stop function")
	}
}

func TestStartHandlesNilLoggerWhenTaskEngineMissing(t *testing.T) {
	stop, err := Start(&schedulerStartServiceContext{}, nil)
	if !errors.Is(err, ErrSchedulerTaskEngineMissing) {
		t.Fatalf("expected missing task engine error, got %v", err)
	}

	if stop == nil {
		t.Fatal("expected no-op stop function")
	}
}
