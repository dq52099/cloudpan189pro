package taskengine

import (
	"context"
	"fmt"
	"regexp"
	"runtime/debug"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/pkg/errors"

	"github.com/google/uuid"
	"github.com/xxcheng123/cloudpan189-share/internal/pkgs/utils"
	"go.uber.org/zap"
)

type TaskEngine interface {
	Start() error
	Stop() error
	IsRunning() bool

	RegisterProcessor(topic Topic, processor MessageProcessor) error
	PushMessage(ctx context.Context, topic Topic, payload []byte) error

	GetStats() TaskStats
	GetRunningTasks() []*TaskInfo
	GetPendingTasks() []*TaskInfo

	SetWorkerCount(count int) error
}

var processorPanicURLPattern = regexp.MustCompile(`(?i)(?:[a-z][a-z0-9+.-]*://|/)[^\s"'<>]+`)

const (
	maxProcessorLogTextSize         = 4 * 1024
	truncatedProcessorLogTextSuffix = "...[truncated]"
	oversizeProcessorLogText        = "[processor message omitted: exceeds log limit]"
)

type taskEngine struct {
	topicProcessors map[Topic][]MessageProcessor
	processorMu     sync.RWMutex
	mu              sync.RWMutex
	running         bool
	stopping        bool
	options         *Options
	logger          *zap.Logger

	taskChan        chan *TaskInfo
	topCtx          context.Context
	topCancel       context.CancelFunc
	workerGroup     sync.WaitGroup
	workerCancels   map[string]context.CancelFunc
	retiringWorkers map[string]struct{}
	nextWorkerID    int

	stats        *TaskStats
	runningTasks map[string]*TaskInfo
	pendingTasks map[string]*TaskInfo
	tasksMu      sync.RWMutex
}

type EngineOption struct {
	Logger  *zap.Logger
	Options []OptionFunc
}

func WithLogger(logger *zap.Logger) EngineOption {
	return EngineOption{
		Logger: logger,
	}
}

func NewTaskEngine(opts ...EngineOption) TaskEngine {
	logger := zap.NewNop()
	options := defaultOptions()

	for _, opt := range opts {
		if opt.Logger != nil {
			logger = opt.Logger
		}

		for _, optFunc := range opt.Options {
			optFunc(options)
		}
	}

	return &taskEngine{
		topicProcessors: make(map[Topic][]MessageProcessor),
		options:         options,
		logger:          logger,
		stats: &TaskStats{
			mu: &sync.RWMutex{},
		},
		runningTasks: make(map[string]*TaskInfo),
		pendingTasks: make(map[string]*TaskInfo),
	}
}

func (t *taskEngine) Start() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.running || t.stopping {
		return ErrEngineAlreadyRunning
	}

	t.running = true
	t.taskChan = make(chan *TaskInfo, t.options.BufferSize)
	t.topCtx, t.topCancel = context.WithCancel(context.Background())
	t.workerCancels = make(map[string]context.CancelFunc, t.options.WorkerCount)
	t.retiringWorkers = make(map[string]struct{})
	t.nextWorkerID = 0

	for idx := 0; idx < t.options.WorkerCount; idx++ {
		t.startWorkerLocked()
	}

	t.logger.Info("task engine started",
		zap.Int("worker_count", t.options.WorkerCount),
		zap.Int("buffer_size", t.options.BufferSize))

	return nil
}

func (t *taskEngine) Stop() error {
	t.mu.Lock()

	if !t.running {
		t.mu.Unlock()

		return ErrEngineNotRunning
	}

	t.logger.Info("stopping task engine...")

	t.running = false
	t.stopping = true

	// 停止接收新消息
	t.topCancel()

	// 关闭消息通道
	close(t.taskChan)
	t.cancelQueuedTasks()

	t.mu.Unlock()

	// 等待所有工作协程结束
	t.workerGroup.Wait()

	t.mu.Lock()
	t.stopping = false
	t.workerCancels = nil
	t.retiringWorkers = nil
	t.mu.Unlock()

	t.logger.Info("task engine stopped")

	return nil
}

func (t *taskEngine) IsRunning() bool {
	t.mu.RLock()
	defer t.mu.RUnlock()

	return t.running
}

func (t *taskEngine) startWorkerLocked() {
	workerID := fmt.Sprintf("worker_%d", t.nextWorkerID)
	t.nextWorkerID++

	workerCtx, cancel := context.WithCancel(t.topCtx)
	t.workerCancels[workerID] = cancel

	t.workerGroup.Add(1)

	go t.worker(workerID, workerCtx)
}

func (t *taskEngine) retireWorkersLocked(count int) {
	if count <= 0 {
		return
	}

	for workerID, cancel := range t.workerCancels {
		if _, retiring := t.retiringWorkers[workerID]; retiring {
			continue
		}

		t.retiringWorkers[workerID] = struct{}{}

		cancel()

		count--
		if count == 0 {
			return
		}
	}
}

func (t *taskEngine) activeWorkerCountLocked() int {
	return len(t.workerCancels) - len(t.retiringWorkers)
}

func (t *taskEngine) unregisterWorker(workerID string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	delete(t.workerCancels, workerID)
	delete(t.retiringWorkers, workerID)
}

func (t *taskEngine) worker(workerId string, workerCtx context.Context) {
	defer t.workerGroup.Done()
	defer t.unregisterWorker(workerId)

	t.logger.Debug("worker started", zap.String("worker_id", workerId))

	for {
		select {
		case <-workerCtx.Done():
			t.logger.Debug("worker stopped", zap.String("worker_id", workerId))

			return
		case taskInfo, ok := <-t.taskChan:
			if !ok {
				t.logger.Debug("worker stopped due to channel closed", zap.String("worker_id", workerId))

				return
			}

			if errors.Is(t.topCtx.Err(), context.Canceled) {
				t.cancelPendingTask(taskInfo)

				return
			}

			t.processMessage(taskInfo, workerId)
		}
	}
}

func (t *taskEngine) processMessage(taskInfo *TaskInfo, workerId string) {
	// 从待处理任务中删除
	t.tasksMu.Lock()
	delete(t.pendingTasks, taskInfo.ID)
	t.tasksMu.Unlock()

	// 获取处理器
	processors, ok := t.processorsForTopic(taskInfo.Topic)
	if !ok {
		t.logger.Warn("no processor found for topic", zap.String("topic", string(taskInfo.Topic)))

		taskInfo.markStarted(workerId, time.Now())
		taskInfo.SetStatus(TaskStatusFailed)

		if t.options.EnableStats {
			t.stats.DecrementPending()
			t.stats.IncrementFailed()
		}

		return
	}

	// 创建任务上下文
	taskCtx := t.newTaskContext(taskInfo, workerId)

	go func() {
		select {
		case <-t.topCtx.Done():
			t.logger.Debug("worker stopped due to engine stop", zap.String("task_id", taskCtx.taskId))

			taskCtx.Cancel()
		case <-taskCtx.ctx.Done():
			// 任务自然完成或被取消，无需额外操作
			t.logger.Debug("task completed or cancelled", zap.String("task_id", taskCtx.taskId))
		}
	}()

	// 记录运行中的任务
	t.addRunningTask(taskCtx.taskInfo)
	defer t.removeRunningTask(taskCtx.taskId)

	// 更新统计
	if t.options.EnableStats {
		t.stats.IncrementRunning()

		t.stats.DecrementPending()
		defer t.stats.DecrementRunning()
	}

	// 设置任务状态为运行中
	taskCtx.taskInfo.SetStatus(TaskStatusRunning)

	processorsToRun := processors
	hasError := false
	wasCancelled := false

	maxRetry := t.options.MaxRetry
	if maxRetry < 0 {
		maxRetry = 0
	}

	for attempt := 0; ; attempt++ {
		failedProcessors, attemptHasError, attemptWasCancelled := t.processAttempt(taskCtx, processorsToRun)
		if attemptWasCancelled {
			wasCancelled = true

			break
		}

		if !attemptHasError {
			hasError = false

			break
		}

		hasError = true

		if attempt >= maxRetry {
			break
		}

		t.logger.Warn("retrying failed task processors",
			zap.String("task_id", taskCtx.taskId),
			zap.Int("failed_processors", len(failedProcessors)),
			zap.Int("next_attempt", attempt+2),
			zap.Int("max_retry", maxRetry))

		if !t.waitRetryDelay(taskCtx.ctx) {
			wasCancelled = true

			break
		}

		processorsToRun = failedProcessors
	}

	// 等待所有处理器完成后再判断状态，避免主动取消导致的误判。
	wasCancelled = wasCancelled || taskCtx.ctx.Err() != nil

	// 确保取消上下文
	taskCtx.Cancel()

	// 根据执行结果设置最终状态
	if wasCancelled {
		taskCtx.taskInfo.SetStatus(TaskStatusCancelled)

		if t.options.EnableStats {
			t.stats.IncrementCancelled()
		}
	} else if hasError {
		taskCtx.taskInfo.SetStatus(TaskStatusFailed)

		if t.options.EnableStats {
			t.stats.IncrementFailed()
		}
	} else {
		taskCtx.taskInfo.SetStatus(TaskStatusCompleted)

		if t.options.EnableStats {
			t.stats.IncrementCompleted()
		}
	}

	t.logger.Debug("task completed",
		zap.String("task_id", taskCtx.taskId),
		zap.String("status", taskCtx.taskInfo.GetStatus()),
		zap.Duration("duration", time.Since(taskCtx.taskInfo.ReceiveAt)))
}

func (t *taskEngine) processAttempt(taskCtx *TaskContext, processors []MessageProcessor) ([]MessageProcessor, bool, bool) {
	var (
		wg               sync.WaitGroup
		hasError         bool
		wasCancelled     bool
		failedProcessors = make([]MessageProcessor, 0)
		mu               sync.Mutex
		attemptAbandoned atomic.Bool
	)

	for _, processor := range processors {
		wg.Add(1)

		go func(processor MessageProcessor) {
			defer wg.Done()

			startTime := time.Now()
			result := ProcessorResult{
				StartTime: startTime,
				Status:    TaskStatusCompleted,
			}

			defer func() {
				if recovered := recover(); recovered != nil {
					panicValue := sanitizeProcessorPanicValue(recovered)
					result.Status = TaskStatusFailed
					result.Error = fmt.Sprintf("processor panic: %s", panicValue)

					t.logger.Error("processor panic recovery",
						zap.String("task_id", taskCtx.taskId),
						zap.String("processor_id", result.ProcessorID),
						zap.String("panic", panicValue),
						zap.String("stack", string(debug.Stack())))
				}

				if attemptAbandoned.Load() {
					return
				}

				result.EndTime = time.Now()
				result.Duration = result.EndTime.Sub(result.StartTime)

				mu.Lock()
				defer mu.Unlock()

				if attemptAbandoned.Load() {
					return
				}

				switch result.Status {
				case TaskStatusFailed:
					hasError = true

					failedProcessors = append(failedProcessors, processor)
				case TaskStatusCancelled:
					wasCancelled = true
				}

				// 添加处理结果
				taskCtx.taskInfo.AddResult(result)
			}()

			result.ProcessorID = processor.ProcessorID()

			// 执行处理器
			if err := processor.Process(taskCtx.ctx, taskCtx.taskInfo.payloadSnapshot()); err != nil {
				result.Status = TaskStatusFailed
				result.Error = sanitizeProcessorError(err)

				if taskCtx.ctx.Err() != nil {
					result.Status = TaskStatusCancelled
				}

				t.logger.Error("processor failed",
					zap.String("task_id", taskCtx.taskId),
					zap.String("processor_id", result.ProcessorID),
					zap.String("error", result.Error))
			}
		}(processor)
	}

	done := make(chan struct{})

	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-taskCtx.ctx.Done():
		mu.Lock()
		attemptAbandoned.Store(true)
		mu.Unlock()

		t.logger.Warn("task processors cancelled before all processors returned",
			zap.String("task_id", taskCtx.taskId),
			zap.String("error", sanitizeProcessorError(taskCtx.ctx.Err())))

		return failedProcessors, hasError, true
	}

	return failedProcessors, hasError, wasCancelled
}

func (t *taskEngine) waitRetryDelay(ctx context.Context) bool {
	if t.options.RetryDelay <= 0 {
		return true
	}

	timer := time.NewTimer(t.options.RetryDelay)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

func (t *taskEngine) newTaskContext(taskInfo *TaskInfo, workerId string) *TaskContext {
	// 应用超时控制
	ctx, cancel := context.WithTimeout(context.WithoutCancel(taskInfo.Context), t.options.ProcessTimeout)

	taskInfo.markStarted(workerId, time.Now())

	return &TaskContext{
		ctx:      ctx,
		cancel:   cancel,
		taskId:   taskInfo.ID,
		taskInfo: taskInfo,
	}
}

func sanitizeProcessorPanicValue(value interface{}) string {
	return sanitizeProcessorText(fmt.Sprint(value))
}

func sanitizeProcessorError(err error) string {
	if err == nil {
		return ""
	}

	return sanitizeProcessorText(err.Error())
}

func sanitizeProcessorText(message string) string {
	if len(message) > maxProcessorLogTextSize {
		return oversizeProcessorLogText + truncatedProcessorLogTextSuffix
	}

	message = processorPanicURLPattern.ReplaceAllStringFunc(message, utils.RedactURLForLog)

	sanitized := utils.RedactSensitiveText(message)
	if len(sanitized) > maxProcessorLogTextSize {
		return sanitized[:maxProcessorLogTextSize] + truncatedProcessorLogTextSuffix
	}

	return sanitized
}

func (t *taskEngine) addRunningTask(taskInfo *TaskInfo) {
	t.tasksMu.Lock()
	defer t.tasksMu.Unlock()

	t.runningTasks[taskInfo.ID] = taskInfo
}

func (t *taskEngine) removeRunningTask(taskId string) {
	t.tasksMu.Lock()

	defer t.tasksMu.Unlock()

	delete(t.runningTasks, taskId)
}

func (t *taskEngine) cancelQueuedTasks() {
	for {
		select {
		case taskInfo, ok := <-t.taskChan:
			if !ok {
				return
			}

			t.cancelPendingTask(taskInfo)
		default:
			return
		}
	}
}

func (t *taskEngine) cancelPendingTask(taskInfo *TaskInfo) {
	t.tasksMu.Lock()
	if _, ok := t.pendingTasks[taskInfo.ID]; !ok {
		t.tasksMu.Unlock()

		return
	}

	delete(t.pendingTasks, taskInfo.ID)
	t.tasksMu.Unlock()

	taskInfo.SetStatus(TaskStatusCancelled)

	if t.options.EnableStats {
		t.stats.DecrementPending()
		t.stats.IncrementCancelled()
	}
}

func (t *taskEngine) RegisterProcessor(topic Topic, processor MessageProcessor) error {
	if isNilDependency(processor) {
		return ErrProcessorMissing
	}

	processorID, err := processorIDForRegistration(processor)
	if err != nil {
		return err
	}

	t.processorMu.Lock()
	defer t.processorMu.Unlock()

	for _, registered := range t.topicProcessors[topic] {
		registeredID, err := processorIDForRegistration(registered)
		if err != nil {
			return err
		}

		if registeredID == processorID {
			return fmt.Errorf("%w: topic=%s processor_id=%s", ErrProcessorRegistered, topic, processorID)
		}
	}

	t.topicProcessors[topic] = append(t.topicProcessors[topic], processor)

	t.logger.Info("processor registered",
		zap.String("topic", string(topic)),
		zap.String("processor_id", processorID))

	return nil
}

func processorIDForRegistration(processor MessageProcessor) (processorID string, err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("%w: processor id panic: %s", ErrProcessorInvalid, sanitizeProcessorPanicValue(recovered))
		}
	}()

	processorID = processor.ProcessorID()
	if strings.TrimSpace(processorID) == "" {
		return "", fmt.Errorf("%w: processor id is empty", ErrProcessorInvalid)
	}

	return processorID, nil
}

func (t *taskEngine) UnregisterProcessor(topic Topic, processorID string) error {
	t.processorMu.Lock()
	defer t.processorMu.Unlock()

	processors := t.topicProcessors[topic]
	for idx, processor := range processors {
		registeredID, err := processorIDForRegistration(processor)
		if err != nil {
			return err
		}

		if registeredID != processorID {
			continue
		}

		updated := make([]MessageProcessor, 0, len(processors)-1)
		updated = append(updated, processors[:idx]...)

		updated = append(updated, processors[idx+1:]...)
		if len(updated) == 0 {
			delete(t.topicProcessors, topic)
		} else {
			t.topicProcessors[topic] = updated
		}

		t.logger.Info("processor unregistered",
			zap.String("topic", string(topic)),
			zap.String("processor_id", processorID))

		return nil
	}

	return ErrProcessorNotFound
}

func (t *taskEngine) processorsForTopic(topic Topic) ([]MessageProcessor, bool) {
	t.processorMu.RLock()
	defer t.processorMu.RUnlock()

	processors, ok := t.topicProcessors[topic]
	if !ok || len(processors) == 0 {
		return nil, false
	}

	return append([]MessageProcessor(nil), processors...), true
}

// PushMessage 推送消息，使用引擎配置的重试策略处理失败任务。
func (t *taskEngine) PushMessage(ctx context.Context, topic Topic, payload []byte) error {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if !t.running {
		return ErrEngineNotRunning
	}

	if isNilDependency(ctx) {
		return ErrTaskContextMissing
	}

	taskInfo := &TaskInfo{
		Context:   ctx,
		ID:        uuid.New().String(),
		Payload:   append([]byte(nil), payload...),
		Topic:     topic,
		ReceiveAt: time.Now(),
		Status:    TaskStatusPending,
		Results:   make([]ProcessorResult, 0),
	}

	t.tasksMu.Lock()
	defer t.tasksMu.Unlock()

	select {
	case t.taskChan <- taskInfo:
		t.pendingTasks[taskInfo.ID] = taskInfo
		if t.options.EnableStats {
			t.stats.IncrementTotal()
			t.stats.IncrementPending()
		}

		return nil
	default:
		return ErrBufferFull
	}
}

func (t *taskEngine) GetStats() TaskStats {
	if !t.options.EnableStats {
		return TaskStats{}
	}

	return t.stats.GetStats()
}

func (t *taskEngine) GetRunningTasks() []*TaskInfo {
	t.tasksMu.RLock()
	defer t.tasksMu.RUnlock()

	tasks := make([]*TaskInfo, 0, len(t.runningTasks))
	for _, task := range t.runningTasks {
		tasks = append(tasks, task.snapshot())
	}

	return tasks
}

func (t *taskEngine) GetPendingTasks() []*TaskInfo {
	t.tasksMu.RLock()
	defer t.tasksMu.RUnlock()

	tasks := make([]*TaskInfo, 0, len(t.pendingTasks))
	for _, task := range t.pendingTasks {
		tasks = append(tasks, task.snapshot())
	}

	return tasks
}

func (t *taskEngine) SetWorkerCount(count int) error {
	if count <= 0 || count > 64 {
		return fmt.Errorf("invalid worker count: %d", count)
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	// 总是更新 WorkerCount 配置
	oldCount := t.options.WorkerCount
	t.options.WorkerCount = count

	// 如果 engine 已启动，需要动态调整 workers。
	if t.running {
		activeWorkers := t.activeWorkerCountLocked()
		if count > activeWorkers {
			needAdd := count - activeWorkers
			for i := 0; i < needAdd; i++ {
				t.startWorkerLocked()
			}

			t.logger.Info("added workers", zap.Int("added", needAdd), zap.Int("total", count))
		} else if count < activeWorkers {
			needRetire := activeWorkers - count
			t.retireWorkersLocked(needRetire)
			t.logger.Info("retiring workers", zap.Int("retiring", needRetire), zap.Int("old", oldCount), zap.Int("new", count))
		}
	}

	return nil
}
