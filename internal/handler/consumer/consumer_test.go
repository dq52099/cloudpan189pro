package consumer

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/xxcheng123/cloudpan189-share/internal/bootstrap"
	appContext "github.com/xxcheng123/cloudpan189-share/internal/framework/context"
	"github.com/xxcheng123/cloudpan189-share/internal/pkgs/taskengine"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type consumerStartServiceContext struct {
	taskEngine taskengine.TaskEngine
	logger     *zap.Logger
}

func (m *consumerStartServiceContext) GetDB(appContext.Context) *gorm.DB {
	return nil
}

func (m *consumerStartServiceContext) GetDBWithoutContext() *gorm.DB {
	return nil
}

func (m *consumerStartServiceContext) GetLogger(string, ...zap.Field) *zap.Logger {
	if m.logger != nil {
		return m.logger
	}

	return zap.NewNop()
}

func (m *consumerStartServiceContext) Close() {}

func (m *consumerStartServiceContext) GetPort() int {
	return 0
}

func (m *consumerStartServiceContext) GetTaskEngine() taskengine.TaskEngine {
	return m.taskEngine
}

func (m *consumerStartServiceContext) GetHTTPEngine() *gin.Engine {
	return nil
}

type consumerStartTaskEngine struct {
	taskengine.TaskEngine
	mu               sync.Mutex
	registerErr      error
	registerFailAt   int
	startErr         error
	registeredTopics []taskengine.Topic
	registrations    []registeredConsumerProcessor
	unregistered     []registeredConsumerProcessor
	running          bool
	startCalled      bool
	startCalls       int
	startEntered     chan struct{}
	startEnteredOnce sync.Once
	releaseStart     chan struct{}
}

func (m *consumerStartTaskEngine) RegisterProcessor(topic taskengine.Topic, processor taskengine.MessageProcessor) error {
	if processor == nil {
		return errors.New("processor is nil")
	}

	processorID := processor.ProcessorID()

	m.mu.Lock()
	defer m.mu.Unlock()

	nextRegistration := len(m.registrations) + 1
	if m.registerErr != nil && (m.registerFailAt == 0 || nextRegistration == m.registerFailAt) {
		return m.registerErr
	}

	m.registeredTopics = append(m.registeredTopics, topic)
	m.registrations = append(m.registrations, registeredConsumerProcessor{
		topic:       topic,
		processorID: processorID,
	})

	return nil
}

func (m *consumerStartTaskEngine) Start() error {
	m.mu.Lock()
	m.startCalled = true

	m.startCalls++
	if m.startEntered != nil {
		m.startEnteredOnce.Do(func() {
			close(m.startEntered)
		})
	}

	releaseStart := m.releaseStart
	m.mu.Unlock()

	if releaseStart != nil {
		<-releaseStart
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if m.startErr != nil {
		return m.startErr
	}

	m.running = true

	return nil
}

func (m *consumerStartTaskEngine) Stop() error {
	return nil
}

func (m *consumerStartTaskEngine) UnregisterProcessor(topic taskengine.Topic, processorID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for idx, registration := range m.registrations {
		if registration.topic != topic || registration.processorID != processorID {
			continue
		}

		m.registrations = append(m.registrations[:idx], m.registrations[idx+1:]...)
		m.registeredTopics = append(m.registeredTopics[:idx], m.registeredTopics[idx+1:]...)
		m.unregistered = append(m.unregistered, registration)

		return nil
	}

	return taskengine.ErrProcessorNotFound
}

func (m *consumerStartTaskEngine) IsRunning() bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	return m.running
}

func (m *consumerStartTaskEngine) PushMessage(context.Context, taskengine.Topic, []byte) error {
	return nil
}

func (m *consumerStartTaskEngine) GetStats() taskengine.TaskStats {
	return taskengine.TaskStats{}
}

func (m *consumerStartTaskEngine) GetRunningTasks() []*taskengine.TaskInfo {
	return nil
}

func (m *consumerStartTaskEngine) GetPendingTasks() []*taskengine.TaskInfo {
	return nil
}

func (m *consumerStartTaskEngine) SetWorkerCount(int) error {
	return nil
}

func (m *consumerStartTaskEngine) registeredTopicCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()

	return len(m.registrations)
}

func (m *consumerStartTaskEngine) unregisteredTopicCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()

	return len(m.unregistered)
}

func (m *consumerStartTaskEngine) startCallCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()

	return m.startCalls
}

func TestStartRejectsNilServiceContext(t *testing.T) {
	var svc bootstrap.ServiceContext

	if err := Start(svc); !errors.Is(err, ErrConsumerServiceContextMissing) {
		t.Fatalf("expected missing service context error, got %v", err)
	}
}

func TestStartRejectsNilTaskEngine(t *testing.T) {
	if err := Start(bootstrap.NewMockServiceContext()); !errors.Is(err, ErrConsumerTaskEngineMissing) {
		t.Fatalf("expected missing task engine error, got %v", err)
	}
}

func TestStartRejectsTypedNilTaskEngine(t *testing.T) {
	var taskEngine *consumerStartTaskEngine

	svc := &consumerStartServiceContext{taskEngine: taskEngine}

	if err := Start(svc); !errors.Is(err, ErrConsumerTaskEngineMissing) {
		t.Fatalf("expected missing task engine error, got %v", err)
	}
}

func TestStartRejectsRunningTaskEngineBeforeRegisteringProcessors(t *testing.T) {
	taskEngine := &consumerStartTaskEngine{running: true}
	svc := &consumerStartServiceContext{taskEngine: taskEngine, logger: zap.NewNop()}

	if err := Start(svc); !errors.Is(err, taskengine.ErrEngineAlreadyRunning) {
		t.Fatalf("expected already running task engine error, got %v", err)
	}

	if taskEngine.startCalled {
		t.Fatal("expected already running task engine not to be started again")
	}

	if got := taskEngine.registeredTopicCount(); got != 0 {
		t.Fatalf("expected no processors to be registered, got %d", got)
	}
}

func TestStartSerializesConcurrentStartsBeforeRegisteringProcessors(t *testing.T) {
	taskEngine := &consumerStartTaskEngine{
		startEntered: make(chan struct{}),
		releaseStart: make(chan struct{}),
	}

	defer func() {
		select {
		case <-taskEngine.releaseStart:
		default:
			close(taskEngine.releaseStart)
		}
	}()

	svc := &consumerStartServiceContext{taskEngine: taskEngine, logger: zap.NewNop()}
	firstErr := make(chan error, 1)
	secondErr := make(chan error, 1)

	go func() {
		firstErr <- Start(svc)
	}()

	select {
	case <-taskEngine.startEntered:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for first start to enter task engine start")
	}

	go func() {
		secondErr <- Start(svc)
	}()

	time.Sleep(50 * time.Millisecond)

	if got := taskEngine.registeredTopicCount(); got != 9 {
		t.Fatalf("expected only the first start to register processors while blocked, got %d", got)
	}

	close(taskEngine.releaseStart)

	if err := <-firstErr; err != nil {
		t.Fatalf("first start: %v", err)
	}

	if err := <-secondErr; !errors.Is(err, taskengine.ErrEngineAlreadyRunning) {
		t.Fatalf("expected second start to see running task engine, got %v", err)
	}

	if got := taskEngine.registeredTopicCount(); got != 9 {
		t.Fatalf("expected no duplicate processor registrations, got %d", got)
	}

	if got := taskEngine.startCallCount(); got != 1 {
		t.Fatalf("expected task engine Start to be called once, got %d", got)
	}
}

func TestStartRollsBackProcessorsWhenRegistrationFails(t *testing.T) {
	wantErr := errors.New("register failed")
	taskEngine := &consumerStartTaskEngine{
		registerErr:    wantErr,
		registerFailAt: 3,
	}
	svc := &consumerStartServiceContext{taskEngine: taskEngine, logger: zap.NewNop()}

	if err := Start(svc); !errors.Is(err, wantErr) {
		t.Fatalf("expected register failure, got %v", err)
	}

	if taskEngine.startCalled {
		t.Fatal("expected task engine not to start after registration failure")
	}

	if got := taskEngine.registeredTopicCount(); got != 0 {
		t.Fatalf("expected successful registrations to be rolled back, got %d", got)
	}

	if got := taskEngine.unregisteredTopicCount(); got != 2 {
		t.Fatalf("expected two processors to be unregistered, got %d", got)
	}
}

func TestStartRollsBackProcessorsWhenTaskEngineStartFails(t *testing.T) {
	wantErr := errors.New("start failed")
	taskEngine := &consumerStartTaskEngine{startErr: wantErr}
	svc := &consumerStartServiceContext{taskEngine: taskEngine, logger: zap.NewNop()}

	if err := Start(svc); !errors.Is(err, wantErr) {
		t.Fatalf("expected start failure, got %v", err)
	}

	if !taskEngine.startCalled {
		t.Fatal("expected task engine start to be called")
	}

	if got := taskEngine.registeredTopicCount(); got != 0 {
		t.Fatalf("expected processor registrations to be rolled back, got %d", got)
	}

	if got := taskEngine.unregisteredTopicCount(); got != 9 {
		t.Fatalf("expected all processors to be unregistered, got %d", got)
	}
}

func TestStartRegistersProcessorsAndStartsTaskEngine(t *testing.T) {
	taskEngine := &consumerStartTaskEngine{}
	svc := &consumerStartServiceContext{taskEngine: taskEngine, logger: zap.NewNop()}

	if err := Start(svc); err != nil {
		t.Fatalf("start consumer: %v", err)
	}

	if !taskEngine.startCalled {
		t.Fatal("expected task engine to start")
	}

	if got := taskEngine.registeredTopicCount(); got != 9 {
		t.Fatalf("expected 9 registered topics, got %d", got)
	}
}
