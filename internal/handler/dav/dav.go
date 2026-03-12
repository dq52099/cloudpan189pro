package dav

import (
	"github.com/xxcheng123/cloudpan189-share/internal/bootstrap"
	"github.com/xxcheng123/cloudpan189-share/internal/framework/httpcontext"

	cloudbridgeSvi "github.com/xxcheng123/cloudpan189-share/internal/services/cloudbridge"
	cloudtokenSvi "github.com/xxcheng123/cloudpan189-share/internal/services/cloudtoken"
	group2fileSvi "github.com/xxcheng123/cloudpan189-share/internal/services/group2file"
	mountpointSvi "github.com/xxcheng123/cloudpan189-share/internal/services/mountpoint"
	userSvi "github.com/xxcheng123/cloudpan189-share/internal/services/user"
	userMountPointTokenSvi "github.com/xxcheng123/cloudpan189-share/internal/services/userMountPointToken"
	verifySvi "github.com/xxcheng123/cloudpan189-share/internal/services/verify"
	virtualfileSvi "github.com/xxcheng123/cloudpan189-share/internal/services/virtualfile"
)

var (
	davMethods = []string{"GET", "HEAD", "POST", "PUT", "DELETE", "OPTIONS", "PROPFIND", "MKCOL", "MOVE", "LOCK", "UNLOCK"}
)

func Start(svc bootstrap.ServiceContext) {
	const (
		handlerName = "dav"
	)

	var (
		engine  = svc.GetHTTPEngine()
		logger  = svc.GetLogger(handlerName)
		wrapper = httpcontext.NewHandlerFuncWrapper(logger)
		wrap    = wrapper.Wrap
	)

	var (
		userService            = userSvi.NewService(svc)
		virtualFileService     = virtualfileSvi.NewService(svc)
		verifyService          = verifySvi.NewService(svc)
		group2FileService      = group2fileSvi.NewService(svc)
		userMountPointTokenSvc = userMountPointTokenSvi.NewService(svc)
		mountPointService      = mountpointSvi.NewService(svc, cloudtokenSvi.NewService(svc), cloudbridgeSvi.NewService(svc), userMountPointTokenSvc)
	)

	davRouter := engine.Group("/dav",
		wrap(newBalanceMiddleware().Balance()),
		wrap(newAuthMiddleware(userService).Auth()),
	)

	workEngine := &workEngine{
		verifyService:      verifyService,
		virtualFileService: virtualFileService,
		group2FileService:  group2FileService,
		mountPointService:  mountPointService,
	}

	for _, method := range davMethods {
		davRouter.Handle(method, "/*path", wrap(workEngine.Open()))
		davRouter.Handle(method, "", wrap(workEngine.Open()))
	}
}
