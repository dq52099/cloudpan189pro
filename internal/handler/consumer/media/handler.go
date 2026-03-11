package media

import (
	"github.com/xxcheng123/cloudpan189-share/internal/framework/taskcontext"

	filetasklogSvi "github.com/xxcheng123/cloudpan189-share/internal/services/filetasklog"
	mediafileSvi "github.com/xxcheng123/cloudpan189-share/internal/services/mediafile"
	mountpointSvi "github.com/xxcheng123/cloudpan189-share/internal/services/mountpoint"
	verifySvi "github.com/xxcheng123/cloudpan189-share/internal/services/verify"
	virtualfileSvi "github.com/xxcheng123/cloudpan189-share/internal/services/virtualfile"
)

type Handler interface {
	Clear() taskcontext.HandlerFunc
	RebuildStrmFile() taskcontext.HandlerFunc
	RebuildStrmFileByMountPoint() taskcontext.HandlerFunc
}

type handler struct {
	mediaFileService   mediafileSvi.Service
	mountpointService  mountpointSvi.Service
	virtualfileService virtualfileSvi.Service
	verifyService      verifySvi.Service
	fileTaskLogService filetasklogSvi.Service
}

func NewHandler(
	mediaFileService mediafileSvi.Service,
	mountPointService mountpointSvi.Service,
	virtualFileService virtualfileSvi.Service,
	verifyService verifySvi.Service,
	fileTaskLogService filetasklogSvi.Service,
) Handler {
	return &handler{
		mediaFileService:   mediaFileService,
		mountpointService:  mountPointService,
		virtualfileService: virtualFileService,
		verifyService:      verifyService,
		fileTaskLogService: fileTaskLogService,
	}
}
