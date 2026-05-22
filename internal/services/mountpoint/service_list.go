package mountpoint

import (
	"strings"

	"github.com/xxcheng123/cloudpan189-share/internal/framework/context"
	"github.com/xxcheng123/cloudpan189-share/internal/repository/models"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

const (
	defaultMountPointCurrentPage = 1
	defaultMountPointPageSize    = 10
	maxMountPointPageSize        = 500
)

type ListRequest struct {
	TokenId           *int64  `form:"tokenId" binding:"omitempty" example:"1"`                               // 云盘令牌ID，可选
	CurrentPage       int     `form:"currentPage,omitempty,default=1" binding:"omitempty,min=1" example:"1"` // 当前页码，默认为1
	PageSize          int     `form:"pageSize,omitempty,default=10" binding:"omitempty,min=1" example:"10"`  // 每页大小，默认为10
	NoPaginate        bool    `form:"noPaginate" binding:"omitempty" example:"false"`                        // 是否不分页，默认false
	Name              string  `form:"name" binding:"omitempty" example:"挂载点名称"`                              // 挂载点名称模糊搜索，可选
	FullPath          string  `form:"fullPath" binding:"omitempty" example:"/path/to/mount"`                 // 完整路径模糊搜索，可选
	FileId            *int64  `form:"fileId" binding:"omitempty" example:"1"`                                // 文件ID
	FileIdList        []int64 `form:"-"`                                                                     // 文件ID列表，用于内部筛选
	EnableAutoRefresh *bool   `form:"enableAutoRefresh" binding:"omitempty" example:"true"`                  // 自动刷新
	LastState         string  `form:"lastState" binding:"omitempty" example:"成功"`                            // 按状态筛选：成功、失败等
	UserID            int64   `form:"-"`                                                                     // 当前用户ID，用于权限过滤
	IsAdmin           bool    `form:"-"`                                                                     // 是否管理员
	UserGroupId       int64   `form:"-"`                                                                     // 用户组ID，用于权限过滤
	GroupFileIds      []int64 `form:"-"`                                                                     // 用户组绑定的文件ID列表
}

func (s *service) List(ctx context.Context, req *ListRequest) (list []*models.MountPoint, err error) {
	if req == nil {
		req = &ListRequest{}
	}

	query, err := s.getListQuery(ctx, req)
	if err != nil {
		ctx.Error("构建挂载点列表查询失败", zap.Error(err))

		return nil, err
	}

	// 应用分页
	if !req.NoPaginate {
		normalizeMountPointPagination(req)
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
	if req == nil {
		req = &ListRequest{}
	}

	query, err := s.getListQuery(ctx, req)
	if err != nil {
		ctx.Error("构建挂载点数量查询失败", zap.Error(err))

		return 0, err
	}

	if err = query.Count(&count).Error; err != nil {
		ctx.Error("查询挂载点数量失败", zap.Error(err))

		return 0, err
	}

	return count, nil
}

func (s *service) getListQuery(ctx context.Context, req *ListRequest) (*gorm.DB, error) {
	query := s.getDB(ctx)

	if !req.IsAdmin && req.UserID <= 0 {
		return nil, errInvalidMountPointUserID
	}

	if req.Name != "" {
		query = query.Where("name LIKE ?", "%"+req.Name+"%")
	}

	if req.FullPath != "" {
		query = query.Where("full_path LIKE ?", "%"+req.FullPath+"%")
	}

	if req.FileId != nil {
		if *req.FileId <= 0 {
			return nil, errInvalidMountPointFileID
		}

		query = query.Where("file_id = ?", *req.FileId)
	}

	if req.FileIdList != nil {
		fileIDs, err := normalizeMountPointFileIDs(req.FileIdList)
		if err != nil {
			return nil, err
		}

		if len(fileIDs) == 0 {
			query = query.Where("1 = 0")
		} else {
			query = query.Where("file_id IN ?", fileIDs)
		}
	}

	if req.TokenId != nil {
		if *req.TokenId < 0 {
			return nil, errInvalidMountPointTokenID
		}

		query = query.Where("token_id = ?", *req.TokenId)
	}

	if req.EnableAutoRefresh != nil {
		query = query.Where("enable_auto_refresh = ?", *req.EnableAutoRefresh)
	}

	if req.LastState != "" {
		query = query.Where("last_state = ?", req.LastState)
	}

	// 非管理员：查看自己创建的 + 用户组分享的 + 自己绑定了令牌的挂载点。
	if !req.IsAdmin {
		conditions := []string{"creator_user_id = ?"}
		args := []any{req.UserID}

		if len(req.GroupFileIds) > 0 {
			groupFileIDs, err := normalizeMountPointFileIDs(req.GroupFileIds)
			if err != nil {
				return nil, err
			}

			conditions = append(conditions, "file_id IN ?")
			args = append(args, groupFileIDs)
		}

		if s.userMountPointTokenService != nil {
			boundMountPointIDs, err := s.userMountPointTokenService.GetUserMountPointIDs(ctx, req.UserID)
			if err != nil {
				return nil, err
			}

			if len(boundMountPointIDs) > 0 {
				conditions = append(conditions, "id IN ?")
				args = append(args, boundMountPointIDs)
			}
		}

		query = query.Where("("+strings.Join(conditions, " OR ")+")", args...)
	}

	return query, nil
}

func normalizeMountPointPagination(req *ListRequest) {
	if req.CurrentPage <= 0 {
		req.CurrentPage = defaultMountPointCurrentPage
	}

	if req.PageSize <= 0 {
		req.PageSize = defaultMountPointPageSize
	}

	if req.PageSize > maxMountPointPageSize {
		req.PageSize = maxMountPointPageSize
	}
}

func normalizeMountPointFileIDs(fileIDs []int64) ([]int64, error) {
	seen := make(map[int64]struct{}, len(fileIDs))

	normalized := make([]int64, 0, len(fileIDs))
	for _, fileID := range fileIDs {
		if fileID <= 0 {
			return nil, errInvalidMountPointFileID
		}

		if _, ok := seen[fileID]; ok {
			continue
		}

		seen[fileID] = struct{}{}
		normalized = append(normalized, fileID)
	}

	return normalized, nil
}
