package mountpoint

import (
	"github.com/xxcheng123/cloudpan189-share/internal/framework/context"
	"github.com/xxcheng123/cloudpan189-share/internal/repository/models"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type ListRequest struct {
	TokenId           *int64  `form:"tokenId" binding:"omitempty" example:"1"`                               // 云盘令牌ID，可选
	CurrentPage       int     `form:"currentPage,omitempty,default=1" binding:"omitempty,min=1" example:"1"` // 当前页码，默认为1
	PageSize          int     `form:"pageSize,omitempty,default=10" binding:"omitempty,min=1" example:"10"`  // 每页大小，默认为10
	NoPaginate        bool    `form:"noPaginate" binding:"omitempty" example:"false"`                        // 是否不分页，默认false
	Name              string  `form:"name" binding:"omitempty" example:"挂载点名称"`                              // 挂载点名称模糊搜索，可选
	FullPath          string  `form:"fullPath" binding:"omitempty" example:"/path/to/mount"`                 // 完整路径模糊搜索，可选
	FileId            *int64  `form:"fileId" binding:"omitempty" example:"1"`                                // 文件ID
	EnableAutoRefresh *bool   `form:"enableAutoRefresh" binding:"omitempty" example:"true"`                  // 自动刷新
	LastState         string  `form:"lastState" binding:"omitempty" example:"成功"`                            // 按状态筛选：成功、失败等
	UserID            int64   `form:"-"`                                                                     // 当前用户ID，用于权限过滤
	IsAdmin           bool    `form:"-"`                                                                     // 是否管理员
	UserGroupId       int64   `form:"-"`                                                                     // 用户组ID，用于权限过滤
	GroupFileIds      []int64 `form:"-"`                                                                     // 用户组绑定的文件ID列表
}

func (s *service) List(ctx context.Context, req *ListRequest) (list []*models.MountPoint, err error) {
	query := s.getListQuery(ctx, req)

	// 应用分页
	if !req.NoPaginate {
		if req.CurrentPage <= 0 {
			req.CurrentPage = 1
		}

		if req.PageSize <= 0 {
			req.PageSize = 10
		}

		query = query.Offset((req.CurrentPage - 1) * req.PageSize).Limit(req.PageSize)
	}

	// 查询挂载点列表
	list = make([]*models.MountPoint, 0)
	if err = query.Find(&list).Error; err != nil {
		ctx.Error("查询挂载点列表失败", zap.Error(err))

		return nil, err
	}

	return list, nil
}

func (s *service) Count(ctx context.Context, req *ListRequest) (count int64, err error) {
	if err = s.getListQuery(ctx, req).Count(&count).Error; err != nil {
		ctx.Error("查询挂载点数量失败", zap.Error(err))

		return 0, err
	}

	return count, nil
}

func (s *service) getListQuery(ctx context.Context, req *ListRequest) *gorm.DB {
	query := s.getDB(ctx)

	if req.Name != "" {
		query = query.Where("name LIKE ?", "%"+req.Name+"%")
	}

	if req.FullPath != "" {
		query = query.Where("full_path LIKE ?", "%"+req.FullPath+"%")
	}

	if req.FileId != nil {
		query = query.Where("file_id = ?", *req.FileId)
	}

	if req.TokenId != nil {
		query = query.Where("token_id = ?", *req.TokenId)
	}

	if req.EnableAutoRefresh != nil {
		query = query.Where("enable_auto_refresh = ?", *req.EnableAutoRefresh)
	}

	if req.LastState != "" {
		query = query.Where("last_state = ?", req.LastState)
	}

	// 非管理员：查看自己创建的 + 用户组分享的挂载点
	if !req.IsAdmin && req.UserID > 0 {
		if len(req.GroupFileIds) > 0 {
			// 自己创建的 OR 用户组分享的
			query = query.Where("creator_user_id = ? OR file_id IN ?", req.UserID, req.GroupFileIds)
		} else {
			query = query.Where("creator_user_id = ?", req.UserID)
		}
	}

	return query
}
