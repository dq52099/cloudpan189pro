package consumer

import (
	"github.com/xxcheng123/cloudpan189-share/internal/bootstrap"
	"github.com/xxcheng123/cloudpan189-share/internal/framework/taskcontext"
	"github.com/xxcheng123/cloudpan189-share/internal/handler/consumer/autoingest"
	"github.com/xxcheng123/cloudpan189-share/internal/handler/consumer/file"
	"github.com/xxcheng123/cloudpan189-share/internal/handler/consumer/media"

	autoingestlogSvi "github.com/xxcheng123/cloudpan189-share/internal/services/autoingestlog"
	autoingestplanSvi "github.com/xxcheng123/cloudpan189-share/internal/services/autoingestplan"
	cloudbridgeSvi "github.com/xxcheng123/cloudpan189-share/internal/services/cloudbridge"
	cloudtokenSvi "github.com/xxcheng123/cloudpan189-share/internal/services/cloudtoken"
	filetasklogSvi "github.com/xxcheng123/cloudpan189-share/internal/services/filetasklog"
	mediafileSvi "github.com/xxcheng123/cloudpan189-share/internal/services/mediafile"
	mountPointSvi "github.com/xxcheng123/cloudpan189-share/internal/services/mountpoint"
	storageFacadeSvi "github.com/xxcheng123/cloudpan189-share/internal/services/storagefacade"
	userMountPointTokenSvi "github.com/xxcheng123/cloudpan189-share/internal/services/userMountPointToken"
	verifySvi "github.com/xxcheng123/cloudpan189-share/internal/services/verify"
	virtualfileSvi "github.com/xxcheng123/cloudpan189-share/internal/services/virtualfile"

	"github.com/xxcheng123/cloudpan189-share/internal/types/topic"
)

func Start(svc bootstrap.ServiceContext) error {
	var (
		handlerName = "consumer"
		logger      = svc.GetLogger(handlerName)

		wrapper = taskcontext.NewHandlerFuncWrapper(logger)
		wrap    = wrapper.Wrap

		taskEngine = svc.GetTaskEngine()
	)

	var (
		virtualFileService     = virtualfileSvi.NewService(svc)
		cloudBridgeService     = cloudbridgeSvi.NewService(svc)
		cloudTokenService      = cloudtokenSvi.NewService(svc)
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
		fileHandler       = file.NewHandler(logger, virtualFileService, cloudBridgeService, cloudTokenService, mountPointService, fileTaskLogService, mediaFileService, verifyService)
		autoIngestHandler = autoingest.NewHandler(taskEngine, cloudBridgeService, autoIngestPlanService, authIngestLogService, storageFacadeService, virtualFileService)
		mediaHandler      = media.NewHandler(mediaFileService, mountPointService, virtualFileService, verifyService, fileTaskLogService)
	)

	{
		if err := taskEngine.RegisterProcessor(new(topic.FileScanFileRequest).Topic(), wrap(fileHandler.ScanFile())); err != nil {
			logger.Error("注册文件扫描处理器失败")

			return err
		}

		if err := taskEngine.RegisterProcessor(new(topic.FileBatchDeleteRequest).Topic(), wrap(fileHandler.HandleBatchDelete())); err != nil {
			logger.Error("注册文件批量删除处理器失败")

			return err
		}

		if err := taskEngine.RegisterProcessor(new(topic.FileDeleteRequest).Topic(), wrap(fileHandler.HandleDelete())); err != nil {
			logger.Error("注册文件删除处理器失败")

			return err
		}

		if err := taskEngine.RegisterProcessor(new(topic.FileClearFileRequest).Topic(), wrap(fileHandler.ClearFile())); err != nil {
			logger.Error("注册文件清理处理器失败")

			return err
		}
	}

	{
		if err := taskEngine.RegisterProcessor(new(topic.AutoIngestRefreshSubscribeRequest).Topic(), wrap(autoIngestHandler.RefreshSubscribe())); err != nil {
			logger.Error("注册订阅号自动入库刷新处理器失败")

			return err
		}
	}

	{
		if err := taskEngine.RegisterProcessor(new(topic.MediaClearRequest).Topic(), wrap(mediaHandler.Clear())); err != nil {
			logger.Error("注册媒体文件清理处理器失败")

			return err
		}

		if err := taskEngine.RegisterProcessor(new(topic.MediaRebuildStrmFileRequest).Topic(), wrap(mediaHandler.RebuildStrmFile())); err != nil {
			logger.Error("注册媒体文件STRM重建处理器失败")

			return err
		}

		if err := taskEngine.RegisterProcessor(new(topic.MediaRebuildStrmFileByMountPointRequest).Topic(), wrap(mediaHandler.RebuildStrmFileByMountPoint())); err != nil {
			logger.Error("注册媒体文件STRM重建处理器(按挂载点)失败")

			return err
		}
	}

	logger.Info("consumer handler start")

	return taskEngine.Start()
}
