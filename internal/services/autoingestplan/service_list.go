package autoingestplan

import (
	"github.com/xxcheng123/cloudpan189-share/internal/framework/context"
	"github.com/xxcheng123/cloudpan189-share/internal/repository/models"
	"go.uber.org/zap"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
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
	query := s.getListQuery(ctx, req)

	// 默认按 id 倒序
	query = query.Order(clause.OrderByColumn{Column: clause.Column{Name: "id"}, Desc: true})

	// 分页（仅在传入正数页码与页大小时生效）
	if req != nil && !req.NoPaginate && req.CurrentPage > 0 && req.PageSize > 0 {
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
	var count int64
	if err := s.getListQuery(ctx, req).Count(&count).Error; err != nil {
		ctx.Error("统计自动挂载计划数量失败", zap.Error(err))

		return 0, err
	}

	return count, nil
}

func (s *service) getListQuery(ctx context.Context, req *ListRequest) *gorm.DB {
	query := s.getDB(ctx)

	if req == nil {
		return query
	}

	if req.Name != "" {
		query = query.Where("name LIKE ?", "%"+req.Name+"%")
	}

	// 非管理员只能查看自己的计划
	if !req.IsAdmin && req.UserID > 0 {
		query = query.Where("user_id = ?", req.UserID)
	}

	return query
}
