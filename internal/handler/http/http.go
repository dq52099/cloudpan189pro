package http

import (
	"io/fs"
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
	embed "github.com/xxcheng123/cloudpan189-share"
	"github.com/xxcheng123/cloudpan189-share/internal/handler/http/taskstate"
	"github.com/xxcheng123/cloudpan189-share/internal/types/loginlog"

	"github.com/xxcheng123/cloudpan189-share/internal/handler/http/autoingest"
	"github.com/xxcheng123/cloudpan189-share/internal/handler/http/cloudtoken"
	"github.com/xxcheng123/cloudpan189-share/internal/handler/http/file"
	loginlogHandler "github.com/xxcheng123/cloudpan189-share/internal/handler/http/loginlog"
	"github.com/xxcheng123/cloudpan189-share/internal/handler/http/media"
	"github.com/xxcheng123/cloudpan189-share/internal/handler/http/setting"
	"github.com/xxcheng123/cloudpan189-share/internal/handler/http/storage"
	"github.com/xxcheng123/cloudpan189-share/internal/handler/http/storage/advance"
	"github.com/xxcheng123/cloudpan189-share/internal/handler/http/usergroup"

	"github.com/xxcheng123/cloudpan189-share/internal/bootstrap"
	"github.com/xxcheng123/cloudpan189-share/internal/framework/httpcontext"
	"github.com/xxcheng123/cloudpan189-share/internal/handler/http/user"

	resourceHandlerPkg "github.com/xxcheng123/cloudpan189-share/internal/handler/http/resource"
	subscriptionHandler "github.com/xxcheng123/cloudpan189-share/internal/handler/http/subscription"
	telegramHandler "github.com/xxcheng123/cloudpan189-share/internal/handler/http/telegram"
	autoingestlogSvi "github.com/xxcheng123/cloudpan189-share/internal/services/autoingestlog"
	autoingestplanSvi "github.com/xxcheng123/cloudpan189-share/internal/services/autoingestplan"
	cloudbridgeSvi "github.com/xxcheng123/cloudpan189-share/internal/services/cloudbridge"
	cloudtokenSvi "github.com/xxcheng123/cloudpan189-share/internal/services/cloudtoken"
	doubanSvi "github.com/xxcheng123/cloudpan189-share/internal/services/douban"
	filetasklogSvi "github.com/xxcheng123/cloudpan189-share/internal/services/filetasklog"
	group2fileSvi "github.com/xxcheng123/cloudpan189-share/internal/services/group2file"
	loginlogSvi "github.com/xxcheng123/cloudpan189-share/internal/services/loginlog"
	mediaconfigSvi "github.com/xxcheng123/cloudpan189-share/internal/services/mediaconfig"
	mediafileSvi "github.com/xxcheng123/cloudpan189-share/internal/services/mediafile"
	mountPointSvi "github.com/xxcheng123/cloudpan189-share/internal/services/mountpoint"
	settingSvi "github.com/xxcheng123/cloudpan189-share/internal/services/setting"
	storagefacadeSvi "github.com/xxcheng123/cloudpan189-share/internal/services/storagefacade"
	telegramSvi "github.com/xxcheng123/cloudpan189-share/internal/services/telegram"
	tmdbSvi "github.com/xxcheng123/cloudpan189-share/internal/services/tmdb"
	userSvi "github.com/xxcheng123/cloudpan189-share/internal/services/user"
	userGroupSvi "github.com/xxcheng123/cloudpan189-share/internal/services/usergroup"
	verifySvi "github.com/xxcheng123/cloudpan189-share/internal/services/verify"
	virtualfileSvi "github.com/xxcheng123/cloudpan189-share/internal/services/virtualfile"
)

func Start(svc bootstrap.ServiceContext, extServices *bootstrap.ExtensionServices) {
	const (
		handlerName = "http"
	)

	var (
		engine = svc.GetHTTPEngine()

		logger     = svc.GetLogger(handlerName)
		taskEngine = svc.GetTaskEngine()

		wrapper = httpcontext.NewHandlerFuncWrapper(logger)
		wrap    = wrapper.Wrap
	)

	var (
		userService           = userSvi.NewService(svc)
		userGroupService      = userGroupSvi.NewService(svc)
		group2FileService     = group2fileSvi.NewService(svc)
		settingService        = settingSvi.NewService(svc)
		virtualFileService    = virtualfileSvi.NewService(svc)
		cloudBridgeService    = cloudbridgeSvi.NewService(svc)
		cloudTokenService     = cloudtokenSvi.NewService(svc)
		mountPointService     = mountPointSvi.NewService(svc, cloudTokenService, cloudBridgeService)
		fileTaskLogService    = filetasklogSvi.NewService(svc)
		storageFacadeService  = storagefacadeSvi.NewService(svc)
		verifyService         = verifySvi.NewService(svc)
		autoIngestPlanService = autoingestplanSvi.NewService(svc)
		autoIngestLogService  = autoingestlogSvi.NewService(svc)
		loginLogService       = loginlogSvi.NewService(svc)
		mediaConfigService    = mediaconfigSvi.NewService(svc)
		mediaFileService      = mediafileSvi.NewService(svc)
	)

	// Telegram 服务：优先使用 extensionServices 中已启动的服务
	var telegramService telegramSvi.Service
	if extServices != nil && extServices.Telegram != nil {
		telegramService = extServices.Telegram
	} else {
		// 备用：使用环境变量创建
		telegramService = telegramSvi.NewService(
			os.Getenv("TG_BOT_TOKEN"),
			os.Getenv("TG_CHAT_ID"),
			os.Getenv("TG_PROXY"),
			os.Getenv("TG_PROXY_TYPE"),
			os.Getenv("TG_API_URL"),
			svc.GetLogger("telegram"),
		)
	}

	// 获取原始 GORM DB
	db := svc.GetDBWithoutContext()

	var (
		userHandler           = user.NewHandler(userService, userGroupService, loginLogService)
		settingHandler        = setting.NewHandler(userService, settingService)
		userGroupHandler      = usergroup.NewHandler(userGroupService, group2FileService, userService)
		storageHandler        = storage.NewHandler(taskEngine, virtualFileService, cloudBridgeService, cloudTokenService, mountPointService, fileTaskLogService, storageFacadeService)
		storageAdvanceHandler = advance.NewHandler(cloudBridgeService, cloudTokenService)
		cloudTokenHandler     = cloudtoken.NewHandler(cloudTokenService, mountPointService)
		fileHandler           = file.NewHandler(virtualFileService, verifyService, cloudTokenService, cloudBridgeService, mountPointService, group2FileService, taskEngine)

		taskStateHandler    = taskstate.NewHandler(taskEngine, fileTaskLogService)
		autoIngestHandler   = autoingest.NewHandler(taskEngine, autoIngestPlanService, autoIngestLogService, cloudBridgeService)
		loginLogHandler     = loginlogHandler.NewHandler(loginLogService)
		mediaHandler        = media.NewHandler(mediaConfigService, mediaFileService, mountPointService, virtualFileService, verifyService, taskEngine)
		telegramHTTPHandler = telegramHandler.NewHandler(db, telegramService, svc.GetLogger("telegram-http"))
		resourceHandler     = resourceHandlerPkg.NewHandler(db, svc.GetLogger("resource"), userService, userGroupService, mountPointService, cloudTokenService, mediaConfigService, mediaFileService, taskEngine)
	)

	var tmdbService tmdbSvi.Service
	var doubanService doubanSvi.Service
	var openaiService interface {
		GenerateUpgradeKeyword(title, category string) (string, error)
	}
	if extServices != nil {
		tmdbService = extServices.TMDB
		doubanService = extServices.Douban
		openaiService = extServices.OpenAI
	}
	subscriptionHTTPHandler := subscriptionHandler.NewHandler(db, tmdbService, doubanService, storageFacadeService, svc.GetLogger("subscription-http"), openaiService)

	var (
		userMiddleware = newAuthMiddleware(userService)
	)

	openapiRouter := engine.Group("/api", httpcontext.LoggerHandler(logger))

	{
		userRouter := openapiRouter.Group("/user")
		{
			userRouter.POST("/login", wrap(userHandler.RecordLog(loginlog.EventLogin)), wrap(userHandler.Login()))
			userRouter.POST("/refresh_token", wrap(userHandler.RecordLog(loginlog.EventLogin)), wrap(userHandler.RefreshToken()))
		}

		userRouterWithAdminAuth := openapiRouter.Group("/user", wrap(userMiddleware.Auth(true)))
		{
			userRouterWithAdminAuth.POST("/add", wrap(userHandler.Add()))
			userRouterWithAdminAuth.POST("/del", wrap(userHandler.Del()))
			userRouterWithAdminAuth.POST("/update", wrap(userHandler.Update()))
			userRouterWithAdminAuth.POST("/toggle_status", wrap(userHandler.ToggleStatus()))
			userRouterWithAdminAuth.GET("/list", wrap(userHandler.List()))
			userRouterWithAdminAuth.POST("/modify_pass", wrap(userHandler.ModifyPass()))
			userRouterWithAdminAuth.POST("/bind_group", wrap(userHandler.BindGroup()))
		}

		userRouterWithBaseAuth := openapiRouter.Group("/user", wrap(userMiddleware.Auth()))
		{
			userRouterWithBaseAuth.GET("/info", wrap(userHandler.Info()))
			userRouterWithBaseAuth.POST("/modify_own_pass", wrap(userHandler.ModifyOwnPass()))
		}
	}

	{
		userGroupRouter := openapiRouter.Group("/user_group", wrap(userMiddleware.Auth(true)))
		{
			userGroupRouter.POST("/add", wrap(userGroupHandler.Add()))
			userGroupRouter.POST("/delete", wrap(userGroupHandler.Delete()))
			userGroupRouter.POST("/modify_name", wrap(userGroupHandler.ModifyName()))
			userGroupRouter.GET("/list", wrap(userGroupHandler.List()))
			userGroupRouter.POST("/batch_bind_files", wrap(userGroupHandler.BatchBindFiles()))
			userGroupRouter.GET("/bind_files", wrap(userGroupHandler.GetBindFiles()))
		}
	}

	{
		storageRouter := openapiRouter.Group("/storage", wrap(userMiddleware.Auth()))
		{
			storageRouter.POST("/add", wrap(storageHandler.Add()))
			storageRouter.POST("/batch_add", wrap(storageHandler.BatchAdd()))
			storageRouter.POST("/delete", wrap(storageHandler.Delete()))
			storageRouter.POST("/batch_delete", wrap(storageHandler.BatchDelete()))
			storageRouter.POST("/batch_parse_text", wrap(storageHandler.BatchParseFromText()))
			storageRouter.POST("/batch_refresh", wrap(storageHandler.BatchRefresh()))
			storageRouter.POST("/batch_modify_token", wrap(storageHandler.BatchModifyToken()))
			storageRouter.GET("/list", wrap(storageHandler.List()))
			storageRouter.GET("/select_list", wrap(storageHandler.SelectList()))
			storageRouter.POST("/refresh", wrap(storageHandler.Refresh()))
			storageRouter.POST("/toggle_auto_refresh", wrap(storageHandler.ToggleAutoRefresh()))
			storageRouter.POST("/modify_token", wrap(storageHandler.ModifyToken()))
		}

		storageAdvanceRouter := openapiRouter.Group("/storage/advance", wrap(userMiddleware.Auth()))
		{
			storageAdvanceRouter.GET("/person/files", wrap(storageAdvanceHandler.GetPersonFiles()))
			storageAdvanceRouter.GET("/family/files", wrap(storageAdvanceHandler.GetFamilyFiles()))
			storageAdvanceRouter.GET("/family/list", wrap(storageAdvanceHandler.FamilyList()))
			storageAdvanceRouter.GET("/get_subscribe_user", wrap(storageAdvanceHandler.GetSubscribeUser()))
			storageAdvanceRouter.GET("/get_subscribe_user_all", wrap(storageAdvanceHandler.GetSubscribeUserAll()))
			storageAdvanceRouter.GET("/share_info", wrap(storageAdvanceHandler.GetShareInfo()))
		}
	}

	{
		fileRouter := openapiRouter.Group("/file", wrap(userMiddleware.Auth()))
		{
			fileRouter.GET("/search", wrap(fileHandler.Search()))
			fileRouter.POST("/create_download_url", wrap(fileHandler.CreateDownloadURL()))
			fileRouter.GET("/open/*fullPath", wrap(fileHandler.Open()))
			fileRouter.POST("/batch_delete", wrap(fileHandler.BatchDelete()))
		}

		{
			openapiRouter.GET("/file/download/:fileId", wrap(fileHandler.Download()))
		}
	}

	{
		cloudTokenRouter := openapiRouter.Group("/cloud_token", wrap(userMiddleware.Auth(true)))
		{
			cloudTokenRouter.POST("/init_qrcode", wrap(cloudTokenHandler.InitQrcode()))
			cloudTokenRouter.POST("/check_qrcode", wrap(cloudTokenHandler.CheckQrcode()))
			cloudTokenRouter.POST("/username_login", wrap(cloudTokenHandler.UsernameLogin()))
			cloudTokenRouter.POST("/modify_name", wrap(cloudTokenHandler.ModifyName()))
			cloudTokenRouter.POST("/delete", wrap(cloudTokenHandler.Delete()))
			cloudTokenRouter.GET("/list", wrap(cloudTokenHandler.List()))
			cloudTokenRouter.GET("/:id", wrap(cloudTokenHandler.Query()))
		}
	}

	{
		taskStateRouter := openapiRouter.Group("/task_state", wrap(userMiddleware.Auth(true)))
		{
			taskStateRouter.GET("/file_log/list", wrap(taskStateHandler.FileLogList()))
			taskStateRouter.GET("/task_engine/list", wrap(taskStateHandler.TaskEngineList()))
			taskStateRouter.POST("/file_log/clear", wrap(taskStateHandler.ClearTaskLogs()))
		}
	}

	{
		openapiRouter.POST("/setting/init_system", wrap(settingHandler.InitSystem()))
		openapiRouter.GET("/setting/info", wrap(settingHandler.Info()))
	}
	{
		settingBaseRouter := openapiRouter.Group("/setting", wrap(userMiddleware.Auth()))
		{
			settingBaseRouter.GET("/addition", wrap(settingHandler.Addition()))
		}
	}

	{
		settingAdminRouter := openapiRouter.Group("/setting", wrap(userMiddleware.Auth(true)))
		{
			settingAdminRouter.POST("/modify_title", wrap(settingHandler.ModifyTitle()))
			settingAdminRouter.POST("/modify_base_url", wrap(settingHandler.ModifyBaseURL()))
			settingAdminRouter.POST("/toggle_enable_auth", wrap(settingHandler.ToggleEnableAuth()))
			settingAdminRouter.POST("/modify_addition", wrap(settingHandler.ModifyAddition()))
		}
	}

	{
		autoIngestRouter := openapiRouter.Group("/auto_ingest", wrap(userMiddleware.Auth(true)))
		{
			autoIngestRouter.POST("/plan/create_subscribe", wrap(autoIngestHandler.CreateSubscribePlan()))
			autoIngestRouter.GET("/plan/list", wrap(autoIngestHandler.PlanList()))
			autoIngestRouter.POST("/plan/enable", wrap(autoIngestHandler.EnablePlan()))
			autoIngestRouter.POST("/plan/disable", wrap(autoIngestHandler.DisablePlan()))
			autoIngestRouter.POST("/plan/refresh", wrap(autoIngestHandler.Refresh()))
			autoIngestRouter.POST("/plan/retry_failed", wrap(autoIngestHandler.RetryFailed()))
			autoIngestRouter.POST("/plan/delete", wrap(autoIngestHandler.DeletePlan()))
			autoIngestRouter.POST("/plan/update", wrap(autoIngestHandler.UpdatePlan()))
			autoIngestRouter.POST("/plan/retry", wrap(autoIngestHandler.RetryPlan()))
			autoIngestRouter.GET("/log/list", wrap(autoIngestHandler.LogList()))
			autoIngestRouter.POST("/log/delete_error", wrap(autoIngestHandler.DeleteErrorLogs()))
			autoIngestRouter.POST("/plan/batch_retry", wrap(autoIngestHandler.BatchRetry()))
			autoIngestRouter.POST("/plan/batch_refresh", wrap(autoIngestHandler.BatchRefresh()))
			autoIngestRouter.POST("/plan/batch_delete", wrap(autoIngestHandler.BatchDelete()))
			autoIngestRouter.POST("/plan/batch_disable", wrap(autoIngestHandler.BatchDisable()))
		}
	}

	{
		loginLogRouter := openapiRouter.Group("/login_log", wrap(userMiddleware.Auth(true)))
		{
			loginLogRouter.GET("/list", wrap(loginLogHandler.List()))
		}
	}

	{
		mediaRouter := openapiRouter.Group("/media", wrap(userMiddleware.Auth(true)))
		{
			mediaRouter.GET("/config/info", wrap(mediaHandler.ConfigInfo()))
			mediaRouter.POST("/config/init", wrap(mediaHandler.ConfigInit()))
			mediaRouter.POST("/config/update", wrap(mediaHandler.ConfigUpdate()))
			mediaRouter.POST("/config/toggle", wrap(mediaHandler.ConfigToggle()))
			mediaRouter.POST("/clear", wrap(mediaHandler.Clear()))
			mediaRouter.POST("/rebuild_strm_file", wrap(mediaHandler.RebuildStrmFile()))
		}
	}

	{
		telegramRouter := openapiRouter.Group("/telegram", wrap(userMiddleware.Auth(true)))
		{
			telegramRouter.GET("/setting", wrap(telegramHTTPHandler.GetSetting()))
			telegramRouter.POST("/setting", wrap(telegramHTTPHandler.UpdateSetting()))
			telegramRouter.POST("/test", wrap(telegramHTTPHandler.TestConnection()))
			telegramRouter.GET("/users", wrap(telegramHTTPHandler.GetUserList()))
			telegramRouter.POST("/user", wrap(telegramHTTPHandler.UpdateUser()))
			telegramRouter.POST("/send", wrap(telegramHTTPHandler.SendMessage()))
			telegramRouter.POST("/process_share", wrap(telegramHTTPHandler.ProcessShareLink()))
		}
	}

	{
		subscriptionRouter := openapiRouter.Group("/subscription", wrap(userMiddleware.Auth(true)))
		{
			subscriptionRouter.GET("/categories", wrap(subscriptionHTTPHandler.GetCategories()))
			subscriptionRouter.GET("/tmdb/movies", wrap(subscriptionHTTPHandler.GetTMDbMovies()))
			subscriptionRouter.GET("/tmdb/tvs", wrap(subscriptionHTTPHandler.GetTMDbTVs()))
			subscriptionRouter.GET("/douban/movies", wrap(subscriptionHTTPHandler.GetDoubanMovies()))
			subscriptionRouter.GET("/config", wrap(subscriptionHTTPHandler.GetConfig()))
			subscriptionRouter.POST("/config", wrap(subscriptionHTTPHandler.UpdateConfig()))
			subscriptionRouter.GET("/search", wrap(subscriptionHTTPHandler.SearchPan()))
			subscriptionRouter.GET("/search/ai", wrap(subscriptionHTTPHandler.SearchPanWithAI()))
			subscriptionRouter.POST("/mount", wrap(subscriptionHTTPHandler.MountSubscription()))
		}
	}

	{
		resourceRouter := openapiRouter.Group("/resource")
		{
			resourceRouter.GET("/summary", wrap(resourceHandler.Summary()))
		}
	}

	{
		staticFS, ok := embed.StaticFS()
		if ok {
			assetsFS, _ := fs.Sub(staticFS, "assets")

			engine.StaticFS("/assets", http.FS(assetsFS))

			engine.NoRoute(func(c *gin.Context) {
				if strings.HasPrefix(c.Request.URL.Path, "/api") {
					c.Status(404)

					return
				}

				// 返回 index.html
				file, err := staticFS.Open("index.html")
				if err != nil {
					c.Status(404)

					return
				}

				defer func() {
					_ = file.Close()
				}()

				stat, _ := file.Stat()

				c.Header("Content-Type", "text/html")
				c.DataFromReader(200, stat.Size(), "text/html", file, nil)
			})
		}
	}
}
