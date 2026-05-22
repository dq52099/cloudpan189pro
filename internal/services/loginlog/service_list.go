package loginlog

import (
	"time"

	"github.com/xxcheng123/cloudpan189-share/internal/framework/context"
	"github.com/xxcheng123/cloudpan189-share/internal/repository/models"
	"github.com/xxcheng123/cloudpan189-share/internal/types/loginlog"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	defaultLoginLogCurrentPage = 1
	defaultLoginLogPageSize    = 10
	maxLoginLogPageSize        = 500
)

var allowedLoginLogSortColumns = map[string]struct{}{
	"id":         {},
	"user_id":    {},
	"username":   {},
	"addr":       {},
	"method":     {},
	"event":      {},
	"status":     {},
	"trace_id":   {},
	"created_at": {},
	"updated_at": {},
}

// ListRequest 登录日志列表查询请求
type ListRequest struct {
	UserId   int64           `form:"userId"  binding:"omitempty" example:"1"`
	Username string          `form:"username" binding:"omitempty"` // 模糊匹配
	Addr     string          `form:"addr"    binding:"omitempty"`  // 模糊匹配
	Method   loginlog.Method `form:"method"  binding:"omitempty"`
	Event    loginlog.Event  `form:"event"   binding:"omitempty"`
	Status   loginlog.Status `form:"status"  binding:"omitempty"`

	BeginAt time.Time `form:"beginAt" binding:"omitempty"`
	EndAt   time.Time `form:"endAt"   binding:"omitempty"`

	CurrentPage int  `form:"currentPage,omitempty,default=1" binding:"omitempty,min=1" example:"1"` // 当前页码，默认为1
	PageSize    int  `form:"pageSize,omitempty,default=10" binding:"omitempty,min=1,max=500" example:"10"`
	NoPaginate  bool `form:"noPaginate" binding:"omitempty" example:"false"`

	AscList  []string `form:"-"`
	DescList []string `form:"-"`
}

// List 查询日志
func (s *service) List(ctx context.Context, req *ListRequest) ([]*models.LoginLog, error) {
	if req == nil {
		req = &ListRequest{}
	}

	query := s.getListQuery(ctx, req)

	// 排序
	if len(req.AscList) > 0 {
		for _, k := range req.AscList {
			if _, ok := allowedLoginLogSortColumns[k]; !ok {
				return nil, errInvalidLoginLogSortField
			}

			query = query.Order(clause.OrderByColumn{Column: clause.Column{Name: k}, Desc: false})
		}
	} else {
		// 默认按 id 降序
		query = query.Order(clause.OrderByColumn{Column: clause.Column{Name: "id"}, Desc: true})
	}

	for _, k := range req.DescList {
		if _, ok := allowedLoginLogSortColumns[k]; !ok {
			return nil, errInvalidLoginLogSortField
		}

		query = query.Order(clause.OrderByColumn{Column: clause.Column{Name: k}, Desc: true})
	}

	// 分页
	if !req.NoPaginate {
		normalizeLoginLogPagination(req)
		query = query.Offset((req.CurrentPage - 1) * req.PageSize).Limit(req.PageSize)
	}

	list := make([]*models.LoginLog, 0)
	if err := query.Find(&list).Error; err != nil {
		return nil, err
	}

	return list, nil
}

func normalizeLoginLogPagination(req *ListRequest) {
	if req.CurrentPage <= 0 {
		req.CurrentPage = defaultLoginLogCurrentPage
	}

	if req.PageSize <= 0 {
		req.PageSize = defaultLoginLogPageSize
	}

	if req.PageSize > maxLoginLogPageSize {
		req.PageSize = maxLoginLogPageSize
	}
}

// Count 统计数量
func (s *service) Count(ctx context.Context, req *ListRequest) (int64, error) {
	var count int64

	err := s.getListQuery(ctx, req).Count(&count).Error

	return count, err
}

// getListQuery 构建列表查询
func (s *service) getListQuery(ctx context.Context, req *ListRequest) *gorm.DB {
	query := s.getDB(ctx)

	if req == nil {
		return query
	}

	if req.UserId > 0 {
		query = query.Where("user_id = ?", req.UserId)
	}

	if req.Username != "" {
		query = query.Where("username LIKE ?", "%"+req.Username+"%")
	}

	if req.Addr != "" {
		query = query.Where("addr LIKE ?", "%"+req.Addr+"%")
	}

	if req.Method != "" {
		query = query.Where("method = ?", req.Method)
	}

	if req.Event != "" {
		query = query.Where("event = ?", req.Event)
	}

	if req.Status != "" {
		query = query.Where("status = ?", req.Status)
	}

	if !req.BeginAt.IsZero() {
		query = query.Where("created_at >= ?", req.BeginAt)
	}

	if !req.EndAt.IsZero() {
		query = query.Where("created_at <= ?", req.EndAt)
	}

	return query
}
