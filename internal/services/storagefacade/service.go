package storagefacade

import (
	"github.com/xxcheng123/cloudpan189-share/internal/bootstrap"
	"github.com/xxcheng123/cloudpan189-share/internal/framework/context"
	cloudbridgeSvi "github.com/xxcheng123/cloudpan189-share/internal/services/cloudbridge"
	cloudtokenSvi "github.com/xxcheng123/cloudpan189-share/internal/services/cloudtoken"
	mountPointSvi "github.com/xxcheng123/cloudpan189-share/internal/services/mountpoint"
	userMountPointTokenSvi "github.com/xxcheng123/cloudpan189-share/internal/services/userMountPointToken"
	virtualFileSvi "github.com/xxcheng123/cloudpan189-share/internal/services/virtualfile"
)

// Service 仅实现创建存储的组合能力（接口与构造定义）。
type Service interface {
	// CreateStorage 创建存储挂载，返回根 VirtualFile ID
	CreateStorage(ctx context.Context, req *CreateStorageRequest) (int64, error)
}

type service struct {
	svc                bootstrap.ServiceContext
	mountPointService  mountPointSvi.Service
	virtualFileService virtualFileSvi.Service
}

func NewService(svc bootstrap.ServiceContext) Service {
	cloudTokenService := cloudtokenSvi.NewService(svc)
	cloudBridgeService := cloudbridgeSvi.NewService(svc)
	userMountPointTokenService := userMountPointTokenSvi.NewService(svc)

	return &service{
		svc:                svc,
		mountPointService:  mountPointSvi.NewService(svc, cloudTokenService, cloudBridgeService, userMountPointTokenService),
		virtualFileService: virtualFileSvi.NewService(svc),
	}
}
