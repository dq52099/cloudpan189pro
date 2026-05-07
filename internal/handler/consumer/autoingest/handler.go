package autoingest

import (
	"github.com/xxcheng123/cloudpan189-share/internal/framework/taskcontext"
	"github.com/xxcheng123/cloudpan189-share/internal/pkgs/taskengine"

	autoingestlogSvi "github.com/xxcheng123/cloudpan189-share/internal/services/autoingestlog"
	autoingestplanSvi "github.com/xxcheng123/cloudpan189-share/internal/services/autoingestplan"
	cloudbridgeSvi "github.com/xxcheng123/cloudpan189-share/internal/services/cloudbridge"
	storagefacadeSvi "github.com/xxcheng123/cloudpan189-share/internal/services/storagefacade"
	virtualfileSvi "github.com/xxcheng123/cloudpan189-share/internal/services/virtualfile"
)

type Handler interface {
	RefreshSubscribe() taskcontext.HandlerFunc
}

type handler struct {
	taskEngine            taskengine.TaskEngine
	cloudbridgeService    cloudbridgeSvi.Service
	autoIngestPlanService autoingestplanSvi.Service
	autoIngestLogService  autoingestlogSvi.Service
	storageFacadeService  storagefacadeSvi.Service
	virtualFileService    virtualfileSvi.Service
}

func NewHandler(
	taskEngine taskengine.TaskEngine,
	cloudbridgeService cloudbridgeSvi.Service,
	autoIngestPlanService autoingestplanSvi.Service,
	autoIngestLogService autoingestlogSvi.Service,
	storageFacadeService storagefacadeSvi.Service,
	virtualFileService virtualfileSvi.Service,
) Handler {
	return &handler{
		taskEngine:            taskEngine,
		cloudbridgeService:    cloudbridgeService,
		autoIngestPlanService: autoIngestPlanService,
		autoIngestLogService:  autoIngestLogService,
		storageFacadeService:  storageFacadeService,
		virtualFileService:    virtualFileService,
	}
}
