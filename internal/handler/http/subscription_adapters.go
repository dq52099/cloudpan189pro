package http

import (
	"github.com/xxcheng123/cloudpan189-share/internal/framework/context"
	"github.com/xxcheng123/cloudpan189-share/internal/services/cloudbridge"
	"github.com/xxcheng123/cloudpan189-share/internal/services/storagefacade"
	"github.com/xxcheng123/cloudpan189-share/internal/services/subscription"
)

// subscriptionMountAdapter 将 storagefacade.Service 适配为 subscription.MountService，
// 在保持订阅模块与具体实现解耦的同时，避免 cloudbridge -> bootstrap 的导入循环。
type subscriptionMountAdapter struct {
	inner storagefacade.Service
}

func (a *subscriptionMountAdapter) CreateStorage(ctx context.Context, req *subscription.MountStorageRequest) (int64, error) {
	return a.inner.CreateStorage(ctx, &storagefacade.CreateStorageRequest{
		LocalPath:         req.LocalPath,
		OsType:            req.OsType,
		CloudToken:        req.CloudToken,
		FileId:            req.FileId,
		Addition:          req.Addition,
		EnableAutoRefresh: req.EnableAutoRefresh,
		AutoRefreshDays:   req.AutoRefreshDays,
		RefreshInterval:   req.RefreshInterval,
		EnableDeepRefresh: req.EnableDeepRefresh,
		CreatorUserID:     req.CreatorUserID,
		// 订阅定时任务下 idempotent，允许路径已存在
		AllowExisting: true,
	})
}

// subscriptionShareAdapter 将 cloudbridge.Service 适配为 subscription.ShareInfoFetcher。
type subscriptionShareAdapter struct {
	inner cloudbridge.Service
}

func (a *subscriptionShareAdapter) GetShareInfo(ctx context.Context, shareCode string, accessCode string) (*subscription.ShareInfo, error) {
	info, err := a.inner.GetShareInfo(ctx, shareCode, accessCode)
	if err != nil {
		return nil, err
	}

	return &subscription.ShareInfo{
		Name:       info.Name,
		IsFolder:   info.IsFolder,
		ShareId:    info.ShareId,
		ID:         info.ID,
		AccessCode: info.AccessCode,
	}, nil
}
