package storage

import (
	"encoding/json"
	"errors"

	"github.com/xxcheng123/cloudpan189-share/internal/consts"
	"github.com/xxcheng123/cloudpan189-share/internal/framework/httpcontext"
	"github.com/xxcheng123/cloudpan189-share/internal/pkgs/datatypes"
	storagefacadeSvi "github.com/xxcheng123/cloudpan189-share/internal/services/storagefacade"
	"github.com/xxcheng123/cloudpan189-share/internal/types/topic"
	"go.uber.org/zap"
)

type (
	addRequest struct {
		LocalPath       string `json:"localPath" binding:"required" example:"/test"`
		OsType          string `json:"osType" binding:"required,oneof=subscribe subscribe_share_folder share_folder person_folder family_folder" example:"subscribe"`
		SubscribeUser   string `json:"subscribeUser" example:"user123"`
		ShareCode       string `json:"shareCode" example:"abc123"`
		ShareAccessCode string `json:"shareAccessCode" example:"1234"`
		CloudToken      int64  `json:"cloudToken" example:"1"`
		FileId          string `json:"fileId" example:"file123"`
		FamilyId        string `json:"familyId" example:"family123"`

		EnableAutoRefresh bool `json:"enableAutoRefresh" binding:"omitempty" example:"true"`
		AutoRefreshDays   int  `json:"autoRefreshDays" binding:"omitempty,min=1,max=365" example:"7"`
		RefreshInterval   int  `json:"refreshInterval" binding:"omitempty,min=30,max=1440" example:"30"`
		EnableDeepRefresh bool `json:"enableDeepRefresh" example:"true"`
	}

	addResponse struct {
		ID         int64  `json:"id" example:"1001"`         // 存储ID
		Path       string `json:"path" example:"/test"`      // 存储路径
		ScanQueued bool   `json:"scanQueued" example:"true"` // 初始化扫描任务是否已入队
		ScanError  string `json:"scanError,omitempty"`       // 初始化扫描任务入队失败原因
	}
)

// Add 添加存储挂载
// @Summary 添加存储挂载
// @Description 添加新的存储挂载点，支持多种协议类型（订阅、分享、个人云盘、家庭云盘等）
// @Tags 存储管理
// @Accept json
// @Produce json
// @Param Authorization header string true "Bearer token"
// @Param request body addRequest true "存储挂载信息"
// @Success 200 {object} httpcontext.Response{data=addResponse} "存储挂载创建成功"
// @Failure 400 {object} httpcontext.Response "参数验证失败，code=99998"
// @Failure 400 {object} httpcontext.Response "路径分割失败，code=4001"
// @Failure 400 {object} httpcontext.Response "不允许挂载根路径，code=4002"
// @Failure 400 {object} httpcontext.Response "路径不合法，需要 / 开头的路径，code=4003"
// @Failure 400 {object} httpcontext.Response "路径已存在，code=4004"
// @Failure 400 {object} httpcontext.Response "查询路径失败，code=4005"
// @Failure 400 {object} httpcontext.Response "订阅用户不能为空，code=4006"
// @Failure 400 {object} httpcontext.Response "订阅分享参数不完整，code=4008"
// @Failure 400 {object} httpcontext.Response "分享码不能为空，code=4009"
// @Failure 400 {object} httpcontext.Response "个人云盘参数不完整，code=4010"
// @Failure 400 {object} httpcontext.Response "家庭云盘参数不完整，code=4011"
// @Failure 401 {object} httpcontext.Response "未授权访问"
// @Failure 403 {object} httpcontext.Response "权限不足"
// @Router /api/storage/add [post]
func (h *handler) Add() httpcontext.HandlerFunc {
	return func(ctx *httpcontext.Context) {
		req := new(addRequest)
		if err := ctx.ShouldBindJSON(req); err != nil {
			ctx.AbortWithInvalidParams(err)

			return
		}

		var (
			addition datatypes.JSONMap
			err      error
		)

		fileId := req.FileId
		userID := ctx.GetInt64(consts.CtxKeyUserId)
		isAdmin := ctx.GetBool(consts.CtxKeyIsAdmin)

		switch req.OsType {
		case protocolSubscribe:
			addition, err = h.executeOsTypeSubscribe(ctx.GetContext(), req)
		case protocolSubscribeShare:
			addition, fileId, err = h.executeOsTypeSubscribeShare(ctx.GetContext(), req)
		case protocolShare:
			addition, fileId, err = h.executeOsTypeShare(ctx.GetContext(), req)
		case protocolPerson:
			err = h.executeOsTypePersonal(ctx.GetContext(), req, userID, isAdmin)
			addition = datatypes.JSONMap{}
		case protocolFamily:
			addition, err = h.executeOsTypeFamily(ctx.GetContext(), req, userID, isAdmin)
		default:
			ctx.Fail(busCodeStorageOsTypeUnsupport)

			return
		}

		if err != nil {
			var busErr httpcontext.BusinessError
			if errors.As(err, &busErr) {
				ctx.Fail(busErr)

				return
			}

			ctx.Fail(busCodeStorageQueryPathFailed.WithError(err))

			return
		}

		// 使用组合服务创建存储（内部完成校验、父级创建、虚拟文件与挂载点创建与补偿）
		id, err := h.storageFacadeService.CreateStorage(ctx.GetContext(), &storagefacadeSvi.CreateStorageRequest{
			LocalPath:         req.LocalPath,
			OsType:            req.OsType,
			CloudToken:        req.CloudToken,
			FileId:            fileId,
			Addition:          addition,
			EnableAutoRefresh: req.EnableAutoRefresh,
			AutoRefreshDays:   req.AutoRefreshDays,
			RefreshInterval:   req.RefreshInterval,
			EnableDeepRefresh: req.EnableDeepRefresh,
			CreatorUserID:     userID,
			IsAdmin:           isAdmin,
			AllowExisting:     false,
		})
		if err != nil {
			ctx.Fail(busCodeStorageAddMountPointFailed.WithError(err))

			return
		}

		taskReq := &topic.FileScanFileRequest{
			FileId:           id,
			Deep:             true,
			ExpectedUserID:   userID,
			TriggeredByAdmin: isAdmin,
		}

		resp := &addResponse{
			ID:   id,
			Path: req.LocalPath,
		}

		body, err := json.Marshal(taskReq)
		if err != nil {
			ctx.GetContext().Warn("序列化创建初始化扫描任务失败", zap.Int64("file_id", id), zap.Error(err))
			h.markInitialScanFailed(ctx.GetContext(), id, "序列化失败", err)
			resp.ScanError = err.Error()
			ctx.Success(resp)

			return
		}

		if err = h.taskEngine.PushMessage(
			ctx.GetContext().
				WithValue(consts.CtxKeyFullPath, req.LocalPath).
				WithValue(consts.CtxKeyInvokeHandlerName, "创建初始化执行器"),
			taskReq.Topic(), body); err != nil {
			ctx.GetContext().Warn("推送创建初始化扫描任务失败", zap.Int64("file_id", id), zap.Error(err))
			h.markInitialScanFailed(ctx.GetContext(), id, "入队失败", err)
			resp.ScanError = err.Error()
			ctx.Success(resp)

			return
		}

		resp.ScanQueued = true
		ctx.Success(resp)
	}
}
