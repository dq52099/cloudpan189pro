package http

import (
	stdContext "context"
	"encoding/json"

	"github.com/xxcheng123/cloudpan189-share/internal/consts"
	"github.com/xxcheng123/cloudpan189-share/internal/framework/context"
	"github.com/xxcheng123/cloudpan189-share/internal/pkgs/datatypes"
	"github.com/xxcheng123/cloudpan189-share/internal/pkgs/taskengine"
	"github.com/xxcheng123/cloudpan189-share/internal/services/cloudbridge"
	"github.com/xxcheng123/cloudpan189-share/internal/services/mountpoint"
	"github.com/xxcheng123/cloudpan189-share/internal/services/storagefacade"
	"github.com/xxcheng123/cloudpan189-share/internal/services/telegram"
	"github.com/xxcheng123/cloudpan189-share/internal/types/topic"
)

// telegramShareAdapter 将 cloudbridge.Service 适配为 telegram.ShareInfoFetcher。
type telegramShareAdapter struct {
	inner cloudbridge.Service
}

func (a *telegramShareAdapter) GetShareInfo(ctx stdContext.Context, shareCode, accessCode string) (*telegram.ShareInfo, error) {
	bgCtx := context.NewContext(ctx)

	info, err := a.inner.GetShareInfo(bgCtx, shareCode, accessCode)
	if err != nil {
		return nil, err
	}

	return &telegram.ShareInfo{
		Name:     info.Name,
		ShareId:  info.ShareId,
		FileId:   info.ID,
		IsFolder: info.IsFolder,
	}, nil
}

// telegramMountAdapter 将 storagefacade.Service 适配为 telegram.StorageMounter，
// 除了创建挂载点本身，还会下发扫描任务（对齐 /api/storage/batch_add 的语义）。
type telegramMountAdapter struct {
	inner             storagefacade.Service
	taskEngine        taskengine.TaskEngine
	mountPointService mountpoint.Service
}

func (a *telegramMountAdapter) CreateMountPoint(ctx stdContext.Context, req *telegram.MountRequest) (int64, error) {
	addition := datatypes.JSONMap{
		consts.FileAdditionKeyShareId:  req.ShareCode,
		consts.FileAdditionKeyIsFolder: true,
	}

	bgCtx := context.NewContext(ctx)

	id, err := a.inner.CreateStorage(bgCtx, &storagefacade.CreateStorageRequest{
		LocalPath:         req.LocalPath,
		OsType:            req.OsType,
		CloudToken:        0,
		FileId:            req.FileID,
		Addition:          addition,
		EnableDeepRefresh: req.EnableDeepRefresh,
		// Telegram 重复收到同一分享链接也视作成功
		AllowExisting: true,
	})
	if err != nil {
		return 0, err
	}

	// 下发扫描任务，与 BatchAdd 行为保持一致
	mp, queryErr := a.mountPointService.Query(bgCtx, id)
	if queryErr == nil && mp != nil {
		taskReq := &topic.FileScanFileRequest{
			FileId: id,
			Deep:   true,
		}

		body, _ := json.Marshal(taskReq)

		fullPath := mp.FullPath
		if fullPath == "" {
			fullPath = mp.Name
		}

		_ = a.taskEngine.PushMessage(
			bgCtx.WithValue(consts.CtxKeyFullPath, fullPath).
				WithValue(consts.CtxKeyInvokeHandlerName, "Telegram Bot 挂载扫描"),
			taskReq.Topic(), body,
		)
	}

	return id, nil
}
