package http

import (
	stdContext "context"
	"encoding/json"
	"errors"
	"fmt"

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
		Name:       info.Name,
		ShareId:    info.ShareId,
		ShareMode:  info.ShareMode,
		FileId:     info.ID,
		IsFolder:   info.IsFolder,
		AccessCode: info.AccessCode,
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
	if req == nil {
		return 0, errors.New("telegram 挂载请求为空")
	}

	if req.ShareID <= 0 {
		return 0, errors.New("telegram 分享ID缺失，无法创建可扫描的挂载点")
	}

	shareMode := req.ShareMode
	if shareMode <= 0 {
		shareMode = 1
	}

	addition := datatypes.JSONMap{
		consts.FileAdditionKeyShareId:    req.ShareID,
		consts.FileAdditionKeyIsFolder:   req.IsFolder,
		consts.FileAdditionKeyShareMode:  shareMode,
		consts.FileAdditionKeyAccessCode: req.AccessCode,
	}

	bgCtx := context.NewContext(ctx)

	id, err := a.inner.CreateStorage(bgCtx, &storagefacade.CreateStorageRequest{
		LocalPath:         req.LocalPath,
		OsType:            req.OsType,
		CloudToken:        0,
		FileId:            req.FileID,
		Addition:          addition,
		EnableDeepRefresh: req.EnableDeepRefresh,
		CreatorUserID:     1,
		IsAdmin:           true,
		// Telegram 重复收到同一分享链接也视作成功
		AllowExisting: true,
	})
	if err != nil {
		return 0, err
	}

	// 下发扫描任务，与 BatchAdd 行为保持一致
	if isNilDependency(a.mountPointService) || isNilDependency(a.taskEngine) {
		return id, errors.New("telegram 挂载扫描依赖未初始化")
	}

	mp, queryErr := a.mountPointService.Query(bgCtx, id)
	if queryErr != nil {
		return id, fmt.Errorf("查询挂载点以下发扫描任务失败: %w", queryErr)
	}

	if mp == nil {
		return id, errors.New("查询挂载点以下发扫描任务失败: 挂载点为空")
	}

	taskReq := &topic.FileScanFileRequest{
		FileId: id,
		Deep:   true,
	}

	body, err := json.Marshal(taskReq)
	if err != nil {
		return id, fmt.Errorf("序列化 Telegram 挂载扫描任务失败: %w", err)
	}

	fullPath := mp.FullPath
	if fullPath == "" {
		fullPath = mp.Name
	}

	if err := a.taskEngine.PushMessage(
		bgCtx.WithValue(consts.CtxKeyFullPath, fullPath).
			WithValue(consts.CtxKeyInvokeHandlerName, "Telegram Bot 挂载扫描"),
		taskReq.Topic(), body,
	); err != nil {
		return id, fmt.Errorf("下发 Telegram 挂载扫描任务失败: %w", err)
	}

	return id, nil
}
