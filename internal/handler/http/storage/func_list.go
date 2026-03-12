package storage

import (
	"github.com/samber/lo"
	"github.com/xxcheng123/cloudpan189-share/internal/consts"
	"github.com/xxcheng123/cloudpan189-share/internal/repository/models"

	cloudtokenSvi "github.com/xxcheng123/cloudpan189-share/internal/services/cloudtoken"
	filetasklogSvi "github.com/xxcheng123/cloudpan189-share/internal/services/filetasklog"
	mountpointSvi "github.com/xxcheng123/cloudpan189-share/internal/services/mountpoint"
	"github.com/xxcheng123/cloudpan189-share/internal/services/virtualfile"

	"github.com/xxcheng123/cloudpan189-share/internal/framework/httpcontext"
)

type listRequest struct {
	CurrentPage int    `form:"currentPage,omitempty,default=1" binding:"omitempty,min=1" example:"1"` // 当前页码，默认为1
	PageSize    int    `form:"pageSize,omitempty,default=10" binding:"omitempty,min=1" example:"10"`  // 每页大小，默认为10
	Path        string `form:"path" example:"/aaa"`
	// LastState     string `form:"lastState" example:"成功"`       // 按状态筛选：成功、失败等
	TaskLogStatus string `form:"taskLogStatus" example:"failed"` // 按任务日志状态筛选：failed, completed等
}

type storageDTO struct {
	ID                    int64                 `json:"id"`
	TaskLogs              []*models.FileTaskLog `json:"taskLogs"`
	TokenName             string                `json:"tokenName"`
	IsInAutoRefreshPeriod bool                  `json:"isInAutoRefreshPeriod"` // 是否在自动刷新时间范围内
	FileCount             int64                 `json:"fileCount"`
	*models.MountPoint
}

type listResponse struct {
	Total       int64         `json:"total" example:"100"`     // 总记录数
	CurrentPage int           `json:"currentPage" example:"1"` // 当前页码
	PageSize    int           `json:"pageSize" example:"10"`   // 每页大小
	Data        []*storageDTO `json:"data"`                    // 列表数据
}

// List 获取存储挂载点列表
// @Summary 获取存储挂载点列表
// @Description 分页获取存储挂载点列表，支持按路径过滤
// @Tags 存储管理
// @Accept json
// @Produce json
// @Param Authorization header string true "Bearer token"
// @Param currentPage query int false "当前页码，默认为1" default(1)
// @Param pageSize query int false "每页大小，默认为10" default(10)
// @Param path query string false "路径过滤" example("/aaa")
// @Success 200 {object} httpcontext.Response{data=listResponse} "获取存储挂载点列表成功"
// @Failure 400 {object} httpcontext.Response "参数验证失败，code=99998"
// @Failure 400 {object} httpcontext.Response "查询挂载点失败，code=3019"
// @Failure 401 {object} httpcontext.Response "未授权访问"
// @Failure 403 {object} httpcontext.Response "权限不足"
// @Router /api/storage/list [get]
func (h *handler) List() httpcontext.HandlerFunc {
	return func(ctx *httpcontext.Context) {
		req := new(listRequest)
		if err := ctx.ShouldBindQuery(req); err != nil {
			ctx.AbortWithInvalidParams(err)
			return
		}

		// 获取当前用户信息用于权限控制
		userID := ctx.GetInt64(consts.CtxKeyUserId)
		isAdmin := ctx.GetBool(consts.CtxKeyIsAdmin)
		userGroupId := ctx.GetInt64(consts.CtxKeyUserGroupId)

		// 获取用户组绑定的文件ID（用于获取用户组分享的挂载点）
		var groupFileIds []int64
		if userGroupId > 0 {
			groupFileIds, _ = h.group2FileService.GetBindFiles(ctx.GetContext(), userGroupId)
		}

		// 管理员显示所有挂载点，普通用户显示自己创建的+用户组分享的
		var list []*models.MountPoint
		var count int64
		var err error
		taskLogMapList := make(map[int64][]*models.FileTaskLog)

		if req.TaskLogStatus != "" {
			// 带日志筛选：先取出所有符合权限的挂载点
			allList, err := h.mountPointService.List(ctx.GetContext(), &mountpointSvi.ListRequest{
				FullPath:     req.Path,
				NoPaginate:   true,
				UserID:       userID,
				IsAdmin:      isAdmin,
				GroupFileIds: groupFileIds,
			})
			if err != nil {
				ctx.Fail(busCodeStorageQueryMountPointError.WithError(err))
				return
			}

			// 获取所有挂载点的日志
			fileIdList := make([]int64, 0, len(allList))
			for _, item := range allList {
				fileIdList = append(fileIdList, item.FileId)
			}
			fileIdList = lo.Uniq(fileIdList)

			if len(fileIdList) > 0 {
				logs, err := h.fileTaskLogService.List(ctx.GetContext(), &filetasklogSvi.ListRequest{
					PageSize:    10000,
					CurrentPage: 1,
					FileIdList:  fileIdList,
				})
				if err != nil {
					ctx.Fail(busCodeStorageQueryFileTaskLogError.WithError(err))
					return
				}

				taskLogMapList = make(map[int64][]*models.FileTaskLog)
				for _, taskLog := range logs {
					if taskLog.FileId == 0 {
						continue
					}
					taskLogMapList[taskLog.FileId] = append(taskLogMapList[taskLog.FileId], taskLog)
				}
			}

			// 内存中按日志状态过滤
			filteredList := make([]*models.MountPoint, 0)
			for _, item := range allList {
				logs := taskLogMapList[item.FileId]
				match := false
				if len(logs) > 0 {
					if logs[0].Status == req.TaskLogStatus {
						match = true
					}
				}
				if match {
					filteredList = append(filteredList, item)
				}
			}

			// 手动分页
			start := (req.CurrentPage - 1) * req.PageSize
			end := start + req.PageSize
			if start > len(filteredList) {
				list = []*models.MountPoint{}
			} else {
				if end > len(filteredList) {
					end = len(filteredList)
				}
				list = filteredList[start:end]
			}
			count = int64(len(filteredList))
		} else {
			// 不带日志筛选：直接数据库分页
			mountReq := &mountpointSvi.ListRequest{
				CurrentPage:  req.CurrentPage,
				PageSize:     req.PageSize,
				FullPath:     req.Path,
				UserID:       userID,
				IsAdmin:      isAdmin,
				GroupFileIds: groupFileIds,
			}

			list, err = h.mountPointService.List(ctx.GetContext(), mountReq)
			if err != nil {
				ctx.Fail(busCodeStorageQueryMountPointError.WithError(err))
				return
			}

			count, err = h.mountPointService.Count(ctx.GetContext(), mountReq)
			if err != nil {
				ctx.Fail(busCodeStorageQueryMountPointError.WithError(err))
				return
			}
		}

		// 获取当前用户对这些挂载点的令牌绑定
		mountPointIds := make([]int64, 0, len(list))
		for _, mp := range list {
			mountPointIds = append(mountPointIds, mp.ID)
		}
		userTokenMap := make(map[int64]int64)
		if len(mountPointIds) > 0 {
			userTokenMap, _ = h.userMountPointTokenService.GetUserTokens(ctx.GetContext(), userID, mountPointIds)
		}

		// 替换令牌：用户组分享的挂载点强制为空，自己创建的保留原令牌
		for _, mp := range list {
			isGroupShare := false
			for _, fid := range groupFileIds {
				if fid == mp.FileId {
					isGroupShare = true
					break
				}
			}

			if isGroupShare {
				// 用户组分享的挂载点：只有用户自己绑定了才显示，否则为空
				if userTokenId, ok := userTokenMap[mp.ID]; ok && userTokenId > 0 {
					mp.TokenId = userTokenId
				} else {
					mp.TokenId = 0
				}
			} else {
				// 自己创建的挂载点：优先用自己的，没有就用原令牌
				if userTokenId, ok := userTokenMap[mp.ID]; ok && userTokenId > 0 {
					mp.TokenId = userTokenId
				}
				// 否则保留原令牌
			}
		}

		var (
			tokenMap     map[int64]string
			fileCountMap map[int64]int64
		)

		// 查询令牌名字
		{
			cloudTokenList := make([]int64, 0, len(list))
			for _, item := range list {
				if item.TokenId > 0 {
					cloudTokenList = append(cloudTokenList, item.TokenId)
				}
			}
			cloudTokenList = lo.Uniq(cloudTokenList)

			tokenList, err := h.cloudTokenService.List(ctx.GetContext(), &cloudtokenSvi.ListRequest{
				IdList:     cloudTokenList,
				NoPaginate: true,
			})
			if err != nil {
				ctx.Fail(busCodeStorageQueryCloudTokenError.WithError(err))
				return
			}

			tokenMap = lo.SliceToMap(tokenList, func(item *models.CloudToken) (int64, string) { return item.ID, item.Name })
		}

		// 补查日志：如果 taskLogMapList 为空（说明走了else分支），则需要查当前页的日志
		if taskLogMapList == nil && len(list) > 0 {
			fileIdList := make([]int64, 0, len(list))
			for _, item := range list {
				fileIdList = append(fileIdList, item.FileId)
			}
			fileIdList = lo.Uniq(fileIdList)

			taskLogList, err := h.fileTaskLogService.List(ctx.GetContext(), &filetasklogSvi.ListRequest{
				PageSize:    200,
				CurrentPage: 1,
				FileIdList:  fileIdList,
			})
			if err != nil {
				ctx.Fail(busCodeStorageQueryFileTaskLogError.WithError(err))
				return
			}

			taskLogMapList = make(map[int64][]*models.FileTaskLog)
			for _, taskLog := range taskLogList {
				if taskLog.FileId == 0 {
					continue
				}
				taskLogMapList[taskLog.FileId] = append(taskLogMapList[taskLog.FileId], taskLog)
			}
		}

		// 查询文件数量
		{
			fileCountList, err := h.virtualFileService.GroupCountByTopId(ctx.GetContext(), &virtualfile.GroupCountByTopIdRequest{})
			if err != nil {
				ctx.Fail(busCodeStorageQueryFileCountError.WithError(err))
				return
			}

			fileCountMap = lo.SliceToMap(fileCountList, func(item *virtualfile.GroupCountByTopId) (int64, int64) { return item.TopId, item.Count })
		}

		dtoList := make([]*storageDTO, 0, len(list))

		for _, item := range list {
			tokenName := "令牌未绑定"

			if item.TokenId > 0 {
				if tkName, ok := tokenMap[item.TokenId]; ok {
					tokenName = tkName
				} else {
					tokenName = "令牌不存在"
				}
			}

			taskLogs := make([]*models.FileTaskLog, 0)
			if taskLogList, ok := taskLogMapList[item.FileId]; ok {
				taskLogs = taskLogList
			}

			dtoList = append(dtoList, &storageDTO{
				ID:                    item.FileId,
				TaskLogs:              taskLogs,
				TokenName:             tokenName,
				MountPoint:            item,
				IsInAutoRefreshPeriod: item.IsInAutoRefreshPeriod(),
				FileCount:             fileCountMap[item.FileId],
			})
		}

		ctx.Success(&listResponse{
			Total:       count,
			CurrentPage: req.CurrentPage,
			PageSize:    req.PageSize,
			Data:        dtoList,
		})
	}
}
