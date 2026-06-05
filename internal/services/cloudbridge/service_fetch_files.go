package cloudbridge

import (
	"fmt"

	"github.com/xxcheng123/cloudpan189-interface/client"
	"github.com/xxcheng123/cloudpan189-share/internal/consts"
	"github.com/xxcheng123/cloudpan189-share/internal/framework/context"
	"github.com/xxcheng123/cloudpan189-share/internal/repository/models"
	"github.com/xxcheng123/cloudpan189-share/internal/types/converter"
	"go.uber.org/zap"
)

type fetchFunc func(ctx context.Context) ([]converter.VirtualFileConverter, error)

func (s *service) doFetch(ctx context.Context, fn fetchFunc) ([]converter.VirtualFileConverter, error) {
	var (
		idx           = 0
		maxRetryTimes = 3
		lastErr       error
	)

	for idx < maxRetryTimes {
		files, err := fn(ctx)
		if err != nil {
			ctx.Error("获取数据失败", zap.String("error", sanitizeCloudbridgeError(err)))
			lastErr = err

			idx++

			continue
		}

		return files, nil
	}

	return nil, lastErr
}

type simplyFunc func(ctx context.Context, pageNum int64) (list []converter.VirtualFileConverter, hasNext bool, err error)

func (s *service) simplyFetch(ctx context.Context, fn simplyFunc) (list []converter.VirtualFileConverter, err error) {
	list = make([]converter.VirtualFileConverter, 0)

	var (
		hasNext       = true
		pageNum int64 = 1
	)

	for hasNext {
		pageFiles, nextPage, fnErr := fn(ctx, pageNum)
		if fnErr != nil {
			return nil, fnErr
		}

		list = append(list, pageFiles...)
		hasNext = nextPage
		pageNum++
	}

	return list, nil
}

// GetSubscribeUserFiles 获取订阅用户分享的文件（获取全部）
func (s *service) GetSubscribeUserFiles(ctx context.Context, userId string) ([]converter.VirtualFileConverter, error) {
	userId, err := validateCloud189SubscribeUserID(userId)
	if err != nil {
		return nil, err
	}

	pageSize := int64(200)

	return s.doFetch(ctx, func(ctx context.Context) ([]converter.VirtualFileConverter, error) {
		return s.simplyFetch(ctx, func(ctx context.Context, pageNum int64) ([]converter.VirtualFileConverter, bool, error) {
			resp, err := s.getClient(ctx).GetUpResourceShare(ctx, userId, pageNum, pageSize)
			if err != nil {
				return nil, false, logAndReturnCloudbridgeError(ctx, "获取数据失败", fmt.Sprintf("获取第%d页数据失败", pageNum), err,
					zap.String("user_id", userId),
					zap.Int64("page_num", pageNum))
			}

			if resp.Data != nil {
				files := make([]converter.VirtualFileConverter, 0)

				for _, v := range resp.Data.FileList {
					files = append(files, converter.NewShareFileInfo(v, userId))
				}

				totalPages := (resp.Data.Count + pageSize - 1) / pageSize

				return files, pageNum < totalPages, nil
			}

			return make([]converter.VirtualFileConverter, 0), false, nil
		})
	})
}

const defaultSubscribeShareListMode = 5

// GetSubscribeShareFiles 获取订阅分享的文件。
func (s *service) GetSubscribeShareFiles(ctx context.Context, upUserId string, shareId int64, fileId string, isFolder bool, shareMode int, accessCode string) ([]converter.VirtualFileConverter, error) {
	upUserId, err := validateCloud189SubscribeUserID(upUserId)
	if err != nil {
		return nil, err
	}

	accessCode, err = validateCloud189AccessCode(accessCode)
	if err != nil {
		return nil, err
	}

	pageSize := int64(200)

	if shareMode <= 0 {
		shareMode = defaultSubscribeShareListMode
	}

	return s.doFetch(ctx, func(ctx context.Context) ([]converter.VirtualFileConverter, error) {
		return s.simplyFetch(ctx, func(ctx context.Context, pageNum int64) ([]converter.VirtualFileConverter, bool, error) {
			resp, err := s.getClient(ctx).ListResourceShareDir(ctx, upUserId, shareId, client.String(fileId), func(req *client.ListResourceShareFileRequest) {
				req.IsFolder = isFolder
				req.IconOption = 5
				req.OrderBy = "lastOpTime"
				req.Descending = true
				req.PageNum = int(pageNum)
				req.PageSize = int(pageSize)
			})
			if err != nil {
				return nil, false, logAndReturnCloudbridgeError(ctx, "获取共享文件失败", fmt.Sprintf("获取第%d页共享文件失败", pageNum), err,
					zap.Int64("share_id", shareId),
					zap.String("file_id", fileId),
					zap.Int64("page_num", pageNum))
			}

			files := make([]converter.VirtualFileConverter, 0)

			for _, v := range resp.FileListAO.FolderList {
				files = append(files, converter.NewFolderInfo(v, models.OsTypeSubscribeShareFolder, newSubscribeShareAddition(upUserId, shareId, true, shareMode, accessCode)))
			}

			for _, v := range resp.FileListAO.FileList {
				files = append(files, converter.NewFileInfo(v, models.OsTypeSubscribeShareFile, newSubscribeShareAddition(upUserId, shareId, false, shareMode, accessCode)))
			}

			totalPages := (resp.FileListAO.Count + pageSize - 1) / pageSize

			return files, pageNum < totalPages, nil
		})
	})
}

func newSubscribeShareAddition(upUserId string, shareId int64, isFolder bool, shareMode int, accessCode string) map[string]interface{} {
	return map[string]interface{}{
		consts.FileAdditionKeyUpUserId:   upUserId,
		consts.FileAdditionKeyShareId:    shareId,
		consts.FileAdditionKeyIsFolder:   isFolder,
		consts.FileAdditionKeyShareMode:  shareMode,
		consts.FileAdditionKeyAccessCode: accessCode,
	}
}

// GetShareFiles 获取普通分享的文件
func (s *service) GetShareFiles(ctx context.Context, shareId int64, fileId string, shareMode int, accessCode string, isFolder bool) ([]converter.VirtualFileConverter, error) {
	accessCode, err := validateCloud189AccessCode(accessCode)
	if err != nil {
		return nil, err
	}

	var (
		pageSize int64 = 200
		addMpFn        = func(mp map[string]interface{}) map[string]interface{} {
			mp[consts.FileAdditionKeyShareId] = shareId
			mp[consts.FileAdditionKeyShareMode] = shareMode
			mp[consts.FileAdditionKeyAccessCode] = accessCode

			return mp
		}
	)

	return s.doFetch(ctx, func(ctx context.Context) ([]converter.VirtualFileConverter, error) {
		return s.simplyFetch(ctx, func(ctx context.Context, pageNum int64) ([]converter.VirtualFileConverter, bool, error) {
			resp, err := s.getClient(ctx).ListShareDir(ctx, shareId, client.String(fileId), func(req *client.ListShareFileRequest) {
				req.PageNum = int(pageNum)
				req.PageSize = int(pageSize)
				req.AccessCode = accessCode
				req.ShareMode = shareMode
				req.IconOption = 5
				req.IsFolder = isFolder
			})
			if err != nil {
				return nil, false, logAndReturnCloudbridgeError(ctx, "获取分享文件失败", fmt.Sprintf("获取第%d页分享文件失败", pageNum), err,
					zap.Int64("share_id", shareId),
					zap.String("file_id", fileId),
					zap.Int64("page_num", pageNum))
			}

			files := make([]converter.VirtualFileConverter, 0)

			for _, v := range resp.FileListAO.FolderList {
				files = append(files, converter.NewFolderInfo(v, models.OsTypeShareFolder, addMpFn(map[string]interface{}{
					consts.FileAdditionKeyIsFolder: true,
				})))
			}

			for _, v := range resp.FileListAO.FileList {
				files = append(files, converter.NewFileInfo(v, models.OsTypeShareFile, addMpFn(map[string]interface{}{
					consts.FileAdditionKeyIsFolder: false,
				})))
			}

			totalPages := (resp.FileListAO.Count + pageSize - 1) / pageSize

			return files, pageNum < totalPages, nil
		})
	})
}

func (s *service) GetCloudFiles(ctx context.Context, cc AuthToken, fileId string) ([]converter.VirtualFileConverter, error) {
	var (
		ct             = client.New().WithToken(cc)
		pageSize int64 = 200
		addMpFn        = func(mp map[string]interface{}) map[string]interface{} {
			return mp
		}
	)

	return s.doFetch(ctx, func(ctx context.Context) ([]converter.VirtualFileConverter, error) {
		return s.simplyFetch(ctx, func(ctx context.Context, pageNum int64) ([]converter.VirtualFileConverter, bool, error) {
			resp, err := ct.ListFiles(ctx, client.String(fileId), func(req *client.ListFilesRequest) {
				req.PageNum = int(pageNum)
				req.PageSize = int(pageSize)
				req.IconOption = 5
				req.Descending = true
				req.OrderBy = "lastOpTime"
			})
			if err != nil {
				return nil, false, logAndReturnCloudbridgeError(ctx, "获取云盘文件失败", fmt.Sprintf("获取第%d页云盘文件失败", pageNum), err,
					zap.String("file_id", fileId),
					zap.Int64("page_num", pageNum))
			}

			files := make([]converter.VirtualFileConverter, 0)

			for _, v := range resp.FileListAO.FolderList {
				files = append(files, converter.NewFolderInfo(v, models.OsTypePersonFolder, addMpFn(map[string]interface{}{})))
			}

			for _, v := range resp.FileListAO.FileList {
				files = append(files, converter.NewFileInfo(v, models.OsTypePersonFile, addMpFn(map[string]interface{}{})))
			}

			totalPages := (resp.FileListAO.Count + pageSize - 1) / pageSize

			return files, pageNum < totalPages, nil
		})
	})
}

func (s *service) GetCloudFamilyFiles(ctx context.Context, cc AuthToken, familyId string, fileId string) ([]converter.VirtualFileConverter, error) {
	var (
		ct             = client.New().WithToken(cc)
		pageSize int64 = 200
		addMpFn        = func(mp map[string]interface{}) map[string]interface{} {
			mp[consts.FileAdditionKeyFamilyId] = familyId

			return mp
		}
	)

	return s.doFetch(ctx, func(ctx context.Context) ([]converter.VirtualFileConverter, error) {
		return s.simplyFetch(ctx, func(ctx context.Context, pageNum int64) ([]converter.VirtualFileConverter, bool, error) {
			resp, err := ct.FamilyListFiles(ctx, client.String(familyId), client.String(fileId), func(req *client.FamilyListFilesRequest) {
				req.PageNum = int(pageNum)
				req.PageSize = int(pageSize)
				req.IconOption = 5
				req.Descending = true
				req.OrderBy = "lastOpTime"
			})
			if err != nil {
				return nil, false, logAndReturnCloudbridgeError(ctx, "获取家庭云文件失败", fmt.Sprintf("获取第%d页家庭云文件失败", pageNum), err,
					zap.String("family_id", familyId),
					zap.String("file_id", fileId),
					zap.Int64("page_num", pageNum))
			}

			files := make([]converter.VirtualFileConverter, 0)

			for _, v := range resp.FileListAO.FolderList {
				files = append(files, converter.NewFolderInfo(v, models.OsTypeFamilyFolder, addMpFn(map[string]interface{}{})))
			}

			for _, v := range resp.FileListAO.FileList {
				files = append(files, converter.NewFileInfo(v, models.OsTypeFamilyFile, addMpFn(map[string]interface{}{})))
			}

			totalPages := (resp.FileListAO.Count + pageSize - 1) / pageSize

			return files, pageNum < totalPages, nil
		})
	})
}
