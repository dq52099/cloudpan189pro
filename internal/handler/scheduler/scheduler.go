package scheduler

import (
	stdContext "context"
	errors2 "errors"

	"github.com/pkg/errors"

	"github.com/xxcheng123/cloudpan189-share/internal/bootstrap"
	"github.com/xxcheng123/cloudpan189-share/internal/framework/context"

	autoingestlogSvi "github.com/xxcheng123/cloudpan189-share/internal/services/autoingestlog"
	autoingestplanSvi "github.com/xxcheng123/cloudpan189-share/internal/services/autoingestplan"
	cloudbridgeSvi "github.com/xxcheng123/cloudpan189-share/internal/services/cloudbridge"
	cloudtokenSvi "github.com/xxcheng123/cloudpan189-share/internal/services/cloudtoken"
	filetasklogSvi "github.com/xxcheng123/cloudpan189-share/internal/services/filetasklog"
	mountpointSvi "github.com/xxcheng123/cloudpan189-share/internal/services/mountpoint"
	subscriptionSvi "github.com/xxcheng123/cloudpan189-share/internal/services/subscription"
	userMountPointTokenSvi "github.com/xxcheng123/cloudpan189-share/internal/services/userMountPointToken"
)

type Scheduler interface {
	Start(ctx context.Context) error
	Stop()
}

var (
	ErrSchedulerRunning = errors.New("scheduler is running")
)

// Start 启动所有定时任务。ext 若非 nil，会复用其中已初始化的订阅服务。
func Start(svc bootstrap.ServiceContext, ext *bootstrap.ExtensionServices) (func(), error) {
	const handlerName = "scheduler"

	var (
		logger = svc.GetLogger(handlerName)

		errs []error

		ctx = context.NewContext(stdContext.Background(), context.WithLogger(logger))
	)

	var (
		cloudTokenService      = cloudtokenSvi.NewService(svc)
		cloudBridgeService     = cloudbridgeSvi.NewService(svc)
		fileTaskLogService     = filetasklogSvi.NewService(svc)
		userMountPointTokenSvc = userMountPointTokenSvi.NewService(svc)
		mountPointService      = mountpointSvi.NewService(svc, cloudTokenService, cloudBridgeService, userMountPointTokenSvc)
		autoIngestPlanService  = autoingestplanSvi.NewService(svc)
		autoIngestLogService   = autoingestlogSvi.NewService(svc)

		taskEngine = svc.GetTaskEngine()
	)

	fileTaskLogCheckScheduler := NewFileTaskLogCheckScheduler(fileTaskLogService)
	if err := fileTaskLogCheckScheduler.Start(ctx); err != nil {
		errs = append(errs, err)
	}

	refreshFileScheduler := NewRefreshFileScheduler(mountPointService, taskEngine)
	if err := refreshFileScheduler.Start(ctx); err != nil {
		errs = append(errs, err)
	}

	autoIngestRefreshScheduler := NewAutoIngestRefreshScheduler(taskEngine, autoIngestPlanService, autoIngestLogService)
	if err := autoIngestRefreshScheduler.Start(ctx); err != nil {
		errs = append(errs, err)
	}

	refreshCloudTokenScheduler := NewRefreshCloudTokenScheduler(cloudTokenService)
	if err := refreshCloudTokenScheduler.Start(ctx); err != nil {
		errs = append(errs, err)
	}

	rebuildStrmScheduler := NewRebuildStrmScheduler(taskEngine)
	if err := rebuildStrmScheduler.Start(ctx); err != nil {
		errs = append(errs, err)
	}

	// 订阅定时任务：优先使用 extension services 中已初始化的订阅服务（包含 TMDB/Douban/OpenAI 依赖）
	var subscriptionService subscriptionSvi.Service
	if ext != nil && ext.Subscription != nil {
		subscriptionService = ext.Subscription
	} else {
		subscriptionService = subscriptionSvi.NewService(svc.GetDBWithoutContext(), logger.Named("subscription"), nil)
	}

	subscriptionScheduler := NewSubscriptionScheduler(subscriptionService)
	if err := subscriptionScheduler.Start(ctx); err != nil {
		errs = append(errs, err)
	}

	schedulers := []Scheduler{
		fileTaskLogCheckScheduler,
		refreshFileScheduler,
		autoIngestRefreshScheduler,
		refreshCloudTokenScheduler,
		rebuildStrmScheduler,
		subscriptionScheduler,
	}

	return closeBar(schedulers), errors2.Join(errs...)
}

func closeBar(schedulers []Scheduler) func() {
	return func() {
		for _, scheduler := range schedulers {
			scheduler.Stop()
		}
	}
}
