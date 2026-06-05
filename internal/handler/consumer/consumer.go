package consumer

import (
	"errors"
	"sync"

	"github.com/xxcheng123/cloudpan189-share/internal/bootstrap"
	"github.com/xxcheng123/cloudpan189-share/internal/framework/taskcontext"
	"github.com/xxcheng123/cloudpan189-share/internal/handler/consumer/autoingest"
	"github.com/xxcheng123/cloudpan189-share/internal/handler/consumer/file"
	"github.com/xxcheng123/cloudpan189-share/internal/handler/consumer/media"
	"github.com/xxcheng123/cloudpan189-share/internal/pkgs/taskengine"

	autoingestlogSvi "github.com/xxcheng123/cloudpan189-share/internal/services/autoingestlog"
	autoingestplanSvi "github.com/xxcheng123/cloudpan189-share/internal/services/autoingestplan"
	cloudbridgeSvi "github.com/xxcheng123/cloudpan189-share/internal/services/cloudbridge"
	cloudtokenSvi "github.com/xxcheng123/cloudpan189-share/internal/services/cloudtoken"
	filetasklogSvi "github.com/xxcheng123/cloudpan189-share/internal/services/filetasklog"
	group2fileSvi "github.com/xxcheng123/cloudpan189-share/internal/services/group2file"
	mediafileSvi "github.com/xxcheng123/cloudpan189-share/internal/services/mediafile"
	mountPointSvi "github.com/xxcheng123/cloudpan189-share/internal/services/mountpoint"
	storageFacadeSvi "github.com/xxcheng123/cloudpan189-share/internal/services/storagefacade"
	userMountPointTokenSvi "github.com/xxcheng123/cloudpan189-share/internal/services/userMountPointToken"
	verifySvi "github.com/xxcheng123/cloudpan189-share/internal/services/verify"
	virtualfileSvi "github.com/xxcheng123/cloudpan189-share/internal/services/virtualfile"

	"github.com/xxcheng123/cloudpan189-share/internal/types/topic"
	"go.uber.org/zap"
)

var (
	ErrConsumerServiceContextMissing = errors.New("consumer service context is nil")
	ErrConsumerTaskEngineMissing     = errors.New("consumer task engine is nil")

	startMu sync.Mutex
)

type consumerProcessorRegistration struct {
	topic     taskengine.Topic
	processor taskengine.MessageProcessor
	errorLog  string
}

type registeredConsumerProcessor struct {
	topic       taskengine.Topic
	processorID string
}

type consumerProcessorUnregisterer interface {
	UnregisterProcessor(topic taskengine.Topic, processorID string) error
}

func Start(svc bootstrap.ServiceContext) error {
	startMu.Lock()
	defer startMu.Unlock()

	if isNilDependency(svc) {
		return ErrConsumerServiceContextMissing
	}

	var (
		handlerName = "consumer"
		logger      = svc.GetLogger(handlerName)
	)
	if logger == nil {
		logger = zap.NewNop()
	}

	var (
		wrapper = taskcontext.NewHandlerFuncWrapper(logger)
		wrap    = wrapper.Wrap

		taskEngine = svc.GetTaskEngine()
	)
	if isNilDependency(taskEngine) {
		logger.Error("任务引擎未初始化")

		return ErrConsumerTaskEngineMissing
	}

	if taskEngine.IsRunning() {
		logger.Error("任务引擎已启动，跳过重复注册消费者")

		return taskengine.ErrEngineAlreadyRunning
	}

	var (
		virtualFileService     = virtualfileSvi.NewService(svc)
		cloudBridgeService     = cloudbridgeSvi.NewService(svc)
		cloudTokenService      = cloudtokenSvi.NewService(svc)
		group2FileService      = group2fileSvi.NewService(svc)
		userMountPointTokenSvc = userMountPointTokenSvi.NewService(svc)
		mountPointService      = mountPointSvi.NewService(svc, cloudTokenService, cloudBridgeService, userMountPointTokenSvc)
		fileTaskLogService     = filetasklogSvi.NewService(svc)
		authIngestLogService   = autoingestlogSvi.NewService(svc)
		autoIngestPlanService  = autoingestplanSvi.NewService(svc)
		storageFacadeService   = storageFacadeSvi.NewService(svc)
		mediaFileService       = mediafileSvi.NewService(svc)
		verifyService          = verifySvi.NewService(svc)
	)

	var (
		fileHandler       = file.NewHandler(logger, virtualFileService, cloudBridgeService, cloudTokenService, mountPointService, fileTaskLogService, mediaFileService, verifyService, group2FileService, userMountPointTokenSvc)
		autoIngestHandler = autoingest.NewHandler(taskEngine, cloudBridgeService, autoIngestPlanService, authIngestLogService, storageFacadeService, virtualFileService)
		mediaHandler      = media.NewHandler(mediaFileService, mountPointService, virtualFileService, verifyService, fileTaskLogService)
	)

	registrations := []consumerProcessorRegistration{
		{
			topic:     new(topic.FileScanFileRequest).Topic(),
			processor: wrap(fileHandler.ScanFile()),
			errorLog:  "注册文件扫描处理器失败",
		},
		{
			topic:     new(topic.FileBatchDeleteRequest).Topic(),
			processor: wrap(fileHandler.HandleBatchDelete()),
			errorLog:  "注册文件批量删除处理器失败",
		},
		{
			topic:     new(topic.FileDeleteRequest).Topic(),
			processor: wrap(fileHandler.HandleDelete()),
			errorLog:  "注册文件删除处理器失败",
		},
		{
			topic:     new(topic.FileBatchModifyTokenRequest).Topic(),
			processor: wrap(fileHandler.HandleBatchModifyToken()),
			errorLog:  "注册批量修改令牌处理器失败",
		},
		{
			topic:     new(topic.FileClearFileRequest).Topic(),
			processor: wrap(fileHandler.ClearFile()),
			errorLog:  "注册文件清理处理器失败",
		},
		{
			topic:     new(topic.AutoIngestRefreshSubscribeRequest).Topic(),
			processor: wrap(autoIngestHandler.RefreshSubscribe()),
			errorLog:  "注册订阅号自动入库刷新处理器失败",
		},
		{
			topic:     new(topic.MediaClearRequest).Topic(),
			processor: wrap(mediaHandler.Clear()),
			errorLog:  "注册媒体文件清理处理器失败",
		},
		{
			topic:     new(topic.MediaRebuildStrmFileRequest).Topic(),
			processor: wrap(mediaHandler.RebuildStrmFile()),
			errorLog:  "注册媒体文件 STRM 重建处理器失败",
		},
		{
			topic:     new(topic.MediaRebuildStrmFileByMountPointRequest).Topic(),
			processor: wrap(mediaHandler.RebuildStrmFileByMountPoint()),
			errorLog:  "注册按挂载点重建 STRM 处理器失败",
		},
	}

	registered := make([]registeredConsumerProcessor, 0, len(registrations))
	for _, registration := range registrations {
		processorID := registration.processor.ProcessorID()
		if err := taskEngine.RegisterProcessor(registration.topic, registration.processor); err != nil {
			logger.Error(registration.errorLog)
			rollbackConsumerProcessors(logger, taskEngine, registered)

			return err
		}

		registered = append(registered, registeredConsumerProcessor{
			topic:       registration.topic,
			processorID: processorID,
		})
	}

	logger.Info("consumer handler start")

	if err := taskEngine.Start(); err != nil {
		rollbackConsumerProcessors(logger, taskEngine, registered)

		return err
	}

	return nil
}

func rollbackConsumerProcessors(logger *zap.Logger, taskEngine taskengine.TaskEngine, registered []registeredConsumerProcessor) {
	if len(registered) == 0 {
		return
	}

	unregisterer, ok := taskEngine.(consumerProcessorUnregisterer)
	if !ok {
		logger.Warn("任务引擎不支持撤销消费者处理器注册")

		return
	}

	for idx := len(registered) - 1; idx >= 0; idx-- {
		registration := registered[idx]
		if err := unregisterer.UnregisterProcessor(registration.topic, registration.processorID); err != nil && !errors.Is(err, taskengine.ErrProcessorNotFound) {
			logger.Error("撤销消费者处理器注册失败",
				zap.String("topic", string(registration.topic)),
				zap.String("processor_id", registration.processorID),
				zap.Error(err))
		}
	}
}
