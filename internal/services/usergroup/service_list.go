package usergroup

import (
	"github.com/xxcheng123/cloudpan189-share/internal/framework/context"
	"github.com/xxcheng123/cloudpan189-share/internal/repository/models"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

const (
	defaultUserGroupCurrentPage = 1
	defaultUserGroupPageSize    = 10
	maxUserGroupPageSize        = 500
)

type ListRequest struct {
	CurrentPage int    `binding:"omitempty,min=1" form:"currentPage,omitempty,default=1" example:"1"` // 当前页码，默认为1
	PageSize    int    `binding:"omitempty,min=1" form:"pageSize,omitempty,default=10" example:"10"`  // 每页大小，默认为10
	NoPaginate  bool   `form:"noPaginate" binding:"omitempty" example:"false"`                        // 是否不分页，默认false
	Name        string `form:"name" binding:"omitempty" example:"管理员组"`                               // 用户组名称模糊搜索，可选
}

func (s *service) List(ctx context.Context, req *ListRequest) (list []*models.UserGroup, err error) {
	if req == nil {
		req = &ListRequest{}
	}

	query := s.getListQuery(ctx, req).Order("created_at DESC")

	// 应用分页
	if !req.NoPaginate {
		normalizeUserGroupPagination(req)
		query = query.Offset((req.CurrentPage - 1) * req.PageSize).Limit(req.PageSize)
	}

	// 查询用户组列表
	list = make([]*models.UserGroup, 0)
	if err = query.Find(&list).Error; err != nil {
		ctx.Error("查询用户组列表失败", zap.Error(err))

		return nil, err
	}

	return list, nil
}

func (s *service) Count(ctx context.Context, req *ListRequest) (count int64, err error) {
	if req == nil {
		req = &ListRequest{}
	}

	if err = s.getListQuery(ctx, req).Count(&count).Error; err != nil {
		ctx.Error("查询用户组数量失败", zap.Error(err))

		return 0, err
	}

	return count, nil
}

func (s *service) getListQuery(ctx context.Context, req *ListRequest) *gorm.DB {
	query := s.getDB(ctx)
	if req.Name != "" {
		query = query.Where("name LIKE ?", "%"+req.Name+"%")
	}

	return query
}

func normalizeUserGroupPagination(req *ListRequest) {
	if req.CurrentPage <= 0 {
		req.CurrentPage = defaultUserGroupCurrentPage
	}

	if req.PageSize <= 0 {
		req.PageSize = defaultUserGroupPageSize
	}

	if req.PageSize > maxUserGroupPageSize {
		req.PageSize = maxUserGroupPageSize
	}
}
