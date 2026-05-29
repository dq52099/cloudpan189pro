package autoingestplan

import (
	"time"

	"github.com/xxcheng123/cloudpan189-share/internal/bootstrap"
	"github.com/xxcheng123/cloudpan189-share/internal/framework/context"
	"github.com/xxcheng123/cloudpan189-share/internal/pkgs/utils"
	"github.com/xxcheng123/cloudpan189-share/internal/repository/models"
	"gorm.io/gorm"
)

// Service 面向 AutoIngestPlan 的计划服务接口（不含日志相关方法）
type Service interface {
	// 基础 CRUD
	Create(ctx context.Context, plan *models.AutoIngestPlan) (int64, error)
	Update(ctx context.Context, id int64, fields ...utils.Field) error
	UpdateByOwner(ctx context.Context, req *UpdateRequest, fields ...utils.Field) error
	Query(ctx context.Context, id int64) (*models.AutoIngestPlan, error)
	Delete(ctx context.Context, req *DeleteRequest) error

	// 查询
	List(ctx context.Context, req *ListRequest) ([]*models.AutoIngestPlan, error)
	Count(ctx context.Context, req *ListRequest) (int64, error)
	ListByIDs(ctx context.Context, ids []int64) ([]*models.AutoIngestPlan, error)

	// 状态/配置
	Enable(ctx context.Context, id int64) error
	Disable(ctx context.Context, id int64) error
	EnableByOwner(ctx context.Context, req *UpdateRequest) error
	DisableByOwner(ctx context.Context, req *UpdateRequest) error

	// 运行相关数据
	UpdateOffset(ctx context.Context, id int64, offset int64) error
	IncrAddCount(ctx context.Context, id int64, delta int64) error
	IncrFailedCount(ctx context.Context, id int64, delta int64) error
	ResetCounters(ctx context.Context, id int64) error

	// 调度辅助：查找到期需要执行的计划
	FindDue(ctx context.Context, now time.Time) ([]*models.AutoIngestPlan, error)
}

// service 与构造函数，保持与 mountpoint/filetasklog 风格一致
type service struct {
	svc bootstrap.ServiceContext
}

func NewService(svc bootstrap.ServiceContext) Service {
	return &service{
		svc: svc,
	}
}

func (s *service) getDB(ctx context.Context) *gorm.DB {
	return s.svc.GetDB(ctx).Model(new(models.AutoIngestPlan))
}
