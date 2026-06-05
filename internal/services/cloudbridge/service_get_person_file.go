package cloudbridge

import (
	"github.com/xxcheng123/cloudpan189-interface/client"
	"github.com/xxcheng123/cloudpan189-share/internal/framework/context"
	"go.uber.org/zap"
)

func (s *service) PersonFileList(ctx context.Context, token client.AuthToken, parentId string, pageNum, pageSize int) (*PersonFileListResponse, error) {
	resp, err := client.New().
		WithClient(ctx.HTTPClient()).
		WithToken(token).
		ListFiles(ctx, client.String(parentId), func(req *client.ListFilesRequest) {
			req.PageSize = pageSize
			req.PageNum = pageNum
		})
	if err != nil {
		return nil, logCloudbridgeError(ctx, "获取个人文件列表失败", err,
			zap.String("parent_id", parentId),
			zap.Int("page_num", pageNum),
			zap.Int("page_size", pageSize))
	}

	list := make([]*FileNode, 0)

	// 添加文件夹
	for _, file := range resp.FileListAO.FolderList {
		list = append(list, &FileNode{
			ID:       string(file.Id),
			IsFolder: 1,
			Name:     file.Name,
			ParentId: parentId,
		})
	}

	// 添加文件
	for _, file := range resp.FileListAO.FileList {
		list = append(list, &FileNode{
			ID:       string(file.Id),
			IsFolder: 0,
			Name:     file.Name,
			ParentId: parentId,
		})
	}

	return &PersonFileListResponse{
		Data:        list,
		Total:       resp.FileListAO.FileListSize,
		CurrentPage: pageNum,
		PageSize:    pageSize,
	}, nil
}

func (s *service) PersonFileCount(ctx context.Context, token client.AuthToken, parentId string) (int64, error) {
	resp, err := client.New().
		WithClient(ctx.HTTPClient()).
		WithToken(token).
		ListFiles(ctx, client.String(parentId), func(req *client.ListFilesRequest) {
			req.PageSize = 1 // 只需要获取总数，设置最小页面大小
			req.PageNum = 1
		})
	if err != nil {
		return 0, logCloudbridgeError(ctx, "获取个人文件总数失败", err,
			zap.String("parent_id", parentId))
	}

	return resp.FileListAO.Count, nil
}
