package autoingestlog

import (
	"time"

	"github.com/xxcheng123/cloudpan189-share/internal/framework/context"
	"github.com/xxcheng123/cloudpan189-share/internal/repository/models"
	"go.uber.org/zap"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	defaultAutoIngestLogCurrentPage = 1
	defaultAutoIngestLogPageSize    = 10
	maxAutoIngestLogPageSize        = 500
)

// ListRequest 自动挂载日志列表查询请求
type ListRequest struct {
	PlanId      int64     `form:"planId" binding:"omitempty,min=1" example:"1"`
	PlanIdList  []int64   `form:"-"`
	Level       string    `form:"level" binding:"omitempty" example:"info"`
	Content     string    `form:"content" binding:"omitempty"` // 内容模糊匹配
	BeginAt     time.Time `form:"beginAt" binding:"omitempty"`
	EndAt       time.Time `form:"endAt" binding:"omitempty"`
	CurrentPage int       `form:"currentPage,omitempty,default=1" binding:"omitempty,min=1" example:"1"`
	PageSize    int       `form:"pageSize,omitempty,default=10" binding:"omitempty,min=1" example:"10"`
}

// List 列出自动挂载日志
func (s *service) List(ctx context.Context, req *ListRequest) ([]*models.AutoIngestLog, error) {
	if req == nil {
		req = &ListRequest{}
	}

	query, err := s.getListQuery(ctx, req)
	if err != nil {
		return nil, err
	}

	// 默认按 id 倒序
	query = query.Order(clause.OrderByColumn{Column: clause.Column{Name: "id"}, Desc: true})

	normalizeAutoIngestLogPagination(req)
	query = query.Offset((req.CurrentPage - 1) * req.PageSize).Limit(req.PageSize)

	list := make([]*models.AutoIngestLog, 0)
	if err := query.Find(&list).Error; err != nil {
		ctx.Error("查询自动挂载日志列表失败", zap.Error(err))

		return nil, err
	}

	return list, nil
}

// Count 统计自动挂载日志数量
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
		ctx.Error("统计自动挂载日志数量失败", zap.Error(err))

		return 0, err
	}

	return count, nil
}

func (s *service) getListQuery(ctx context.Context, req *ListRequest) (*gorm.DB, error) {
	query := s.getDB(ctx)

	if req == nil {
		return query, nil
	}

	if req.PlanId > 0 {
		query = query.Where("plan_id = ?", req.PlanId)
	} else if req.PlanId < 0 {
		return nil, errInvalidAutoIngestLogPlanID
	}

	if req.PlanIdList != nil {
		planIDs, err := normalizeAutoIngestLogIDs(req.PlanIdList, errInvalidAutoIngestLogPlanID)
		if err != nil {
			return nil, err
		}

		if len(planIDs) == 0 {
			query = query.Where("1 = 0")
		} else {
			query = query.Where("plan_id IN ?", planIDs)
		}
	}

	if req.Level != "" {
		query = query.Where("level = ?", req.Level)
	}

	if req.Content != "" {
		query = query.Where("content LIKE ?", "%"+req.Content+"%")
	}

	if !req.BeginAt.IsZero() {
		query = query.Where("created_at >= ?", req.BeginAt)
	}

	if !req.EndAt.IsZero() {
		query = query.Where("created_at <= ?", req.EndAt)
	}

	return query, nil
}

func normalizeAutoIngestLogPagination(req *ListRequest) {
	if req.CurrentPage <= 0 {
		req.CurrentPage = defaultAutoIngestLogCurrentPage
	}

	if req.PageSize <= 0 {
		req.PageSize = defaultAutoIngestLogPageSize
	}

	if req.PageSize > maxAutoIngestLogPageSize {
		req.PageSize = maxAutoIngestLogPageSize
	}
}
