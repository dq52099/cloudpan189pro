package autoingestplan

import (
	"github.com/xxcheng123/cloudpan189-share/internal/framework/context"
	"github.com/xxcheng123/cloudpan189-share/internal/repository/models"
	"go.uber.org/zap"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	defaultAutoIngestPlanCurrentPage = 1
	defaultAutoIngestPlanPageSize    = 10
	maxAutoIngestPlanPageSize        = 500
)

// ListRequest 自动挂载计划列表查询请求（与列表实现写在同一文件）
type ListRequest struct {
	Name        string `form:"name" binding:"omitempty"`
	CurrentPage int    `form:"currentPage,omitempty,default=1" binding:"omitempty,min=1" example:"1"`
	PageSize    int    `form:"pageSize,omitempty,default=10" binding:"omitempty,min=1" example:"10"`
	NoPaginate  bool   `form:"-"`
	UserID      int64  `form:"-"` // 用户ID，用于权限过滤
	IsAdmin     bool   `form:"-"` // 是否管理员
}

// List 列出自动挂载计划
func (s *service) List(ctx context.Context, req *ListRequest) ([]*models.AutoIngestPlan, error) {
	if req == nil {
		req = &ListRequest{}
	}

	query, err := s.getListQuery(ctx, req)
	if err != nil {
		return nil, err
	}

	// 默认按 id 倒序
	query = query.Order(clause.OrderByColumn{Column: clause.Column{Name: "id"}, Desc: true})

	if !req.NoPaginate {
		normalizeAutoIngestPlanPagination(req)
		query = query.Offset((req.CurrentPage - 1) * req.PageSize).Limit(req.PageSize)
	}

	list := make([]*models.AutoIngestPlan, 0)
	if err := query.Find(&list).Error; err != nil {
		ctx.Error("查询自动挂载计划列表失败", zap.Error(err))

		return nil, err
	}

	return list, nil
}

// Count 统计自动挂载计划数量
func (s *service) Count(ctx context.Context, req *ListRequest) (int64, error) {
	if req == nil {
		req = &ListRequest{}
	}

	query, err := s.getListQuery(ctx, req)
	if err != nil {
		return 0, err
	}

	var count int64
	if err := query.Count(&count).Error; err != nil {
		ctx.Error("统计自动挂载计划数量失败", zap.Error(err))

		return 0, err
	}

	return count, nil
}

func (s *service) getListQuery(ctx context.Context, req *ListRequest) (*gorm.DB, error) {
	query := s.getDB(ctx)

	if req == nil {
		return query, nil
	}

	if req.Name != "" {
		query = query.Where("name LIKE ?", "%"+req.Name+"%")
	}

	// 非管理员只能查看自己的计划
	if !req.IsAdmin {
		if req.UserID <= 0 {
			return nil, errInvalidAutoIngestPlanUserID
		}

		query = query.Where("user_id = ?", req.UserID)
	}

	return query, nil
}

func normalizeAutoIngestPlanPagination(req *ListRequest) {
	if req.CurrentPage <= 0 {
		req.CurrentPage = defaultAutoIngestPlanCurrentPage
	}

	if req.PageSize <= 0 {
		req.PageSize = defaultAutoIngestPlanPageSize
	}

	if req.PageSize > maxAutoIngestPlanPageSize {
		req.PageSize = maxAutoIngestPlanPageSize
	}
}
