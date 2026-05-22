package user

import (
	"github.com/xxcheng123/cloudpan189-share/internal/framework/context"
	"github.com/xxcheng123/cloudpan189-share/internal/repository/models"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

const (
	defaultUserCurrentPage = 1
	defaultUserPageSize    = 10
	maxUserPageSize        = 500
)

type ListRequest struct {
	CurrentPage int    `form:"currentPage,omitempty,default=1" binding:"omitempty,min=1" example:"1"` // 当前页码，默认为1
	PageSize    int    `form:"pageSize,omitempty,default=10" binding:"omitempty,min=1" example:"10"`  // 每页大小，默认为10
	NoPaginate  bool   `form:"noPaginate" binding:"omitempty" example:"false"`                        // 是否不分页，默认false
	Username    string `form:"username" binding:"omitempty" example:"admin"`                          // 用户名模糊搜索，可选
	GroupId     *int64 `form:"groupId" binding:"omitempty" example:"1"`                               // 用户组ID
}

func (s *service) List(ctx context.Context, req *ListRequest) (list []*models.User, err error) {
	if req == nil {
		req = &ListRequest{}
	}

	query, err := s.getListQuery(ctx, req)
	if err != nil {
		return nil, err
	}

	query = query.Order("created_at desc")

	// 应用分页
	if !req.NoPaginate {
		normalizeUserPagination(req)
		query = query.Offset((req.CurrentPage - 1) * req.PageSize).Limit(req.PageSize)
	}

	// 查询用户列表
	list = make([]*models.User, 0)
	if err = query.Find(&list).Error; err != nil {
		ctx.Error("查询用户列表失败", zap.Error(err))

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
		return 0, err
	}

	if err = query.Count(&count).Error; err != nil {
		ctx.Error("查询用户数量失败", zap.Error(err))

		return 0, err
	}

	return count, nil
}

func (s *service) getListQuery(ctx context.Context, req *ListRequest) (*gorm.DB, error) {
	query := s.getDB(ctx)
	if req.Username != "" {
		query = query.Where("username LIKE ?", "%"+req.Username+"%")
	}

	if req.GroupId != nil {
		if *req.GroupId < 0 {
			return nil, errInvalidUserGroupID
		}

		query = query.Where("group_id = ?", *req.GroupId)
	}

	return query, nil
}

func normalizeUserPagination(req *ListRequest) {
	if req.CurrentPage <= 0 {
		req.CurrentPage = defaultUserCurrentPage
	}

	if req.PageSize <= 0 {
		req.PageSize = defaultUserPageSize
	}

	if req.PageSize > maxUserPageSize {
		req.PageSize = maxUserPageSize
	}
}
