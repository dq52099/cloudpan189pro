package main

import (
	"fmt"

	stdContext "context"
	"github.com/xxcheng123/cloudpan189-share/internal/bootstrap"
	"github.com/xxcheng123/cloudpan189-share/internal/configs"
	"github.com/xxcheng123/cloudpan189-share/internal/framework/context"
	"github.com/xxcheng123/cloudpan189-share/internal/handler/consumer"
	"github.com/xxcheng123/cloudpan189-share/internal/handler/dav"
	"github.com/xxcheng123/cloudpan189-share/internal/handler/http"
	"github.com/xxcheng123/cloudpan189-share/internal/handler/scheduler"
	"github.com/xxcheng123/cloudpan189-share/internal/pkgs/shutdown"
	"go.uber.org/zap"
)

func main() {
	cfg := configs.Get()

	svc, err := bootstrap.New(cfg)
	if err != nil {
		panic(err)
	}

	defer svc.Close()

	logger := svc.GetLogger("main")
	taskEngine := svc.GetTaskEngine()
	httpEngine := svc.GetHTTPEngine()
	port := svc.GetPort()

	if err = consumer.Start(svc); err != nil {
		panic(err)
	}

	var (
		closeBar func()
	)

	// 启动 scheduler
	if closeBar, err = scheduler.Start(svc); err != nil {
		panic(err)
	}

	// 初始化扩展服务（Telegram, TMDB, Douban, OpenAI, Subscription）
	ctx := context.NewContext(stdContext.Background())
	extServices, err := bootstrap.InitExtensionServices(svc.GetDB(ctx), svc.GetLogger("extension"), cfg.Config)
	if err != nil {
		logger.Warn("初始化扩展服务失败", zap.Error(err))
		extServices = nil
	} else {
		logger.Info("扩展服务初始化成功")
	}

	// 启动 HTTP 服务
	http.Start(svc, extServices)
	dav.Start(svc)

	// 启动 Telegram Bot
	if extServices != nil {
		if err = extServices.StartTelegramBot(); err != nil {
			logger.Warn("启动 Telegram Bot 失败", zap.Error(err))
		} else {
			logger.Info("Telegram Bot 已启动")
		}
	}

	go func() {
		if err = httpEngine.Run(fmt.Sprintf(":%d", port)); err != nil {
			panic(err)
		}
	}()

	logger.Info("system running....")

	shutdown.Close(func() {
		logger.Info("close shutdown")

		// 停止扩展服务
		if extServices != nil {
			extServices.StopTelegramBot()
		}

		if err = taskEngine.Stop(); err != nil {
			logger.Error("close task engine failed", zap.Error(err))
		}

		closeBar()

		logger.Info("close successful, bye~")
	})
}
