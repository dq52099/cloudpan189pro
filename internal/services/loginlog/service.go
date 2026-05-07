package loginlog

import (
	"time"

	"github.com/xxcheng123/cloudpan189-share/internal/bootstrap"
	"github.com/xxcheng123/cloudpan189-share/internal/framework/context"
	"github.com/xxcheng123/cloudpan189-share/internal/repository/models"
	"gorm.io/gorm"
)

// Service 面向 LoginLog 的服务接口
type Service interface {
	// Create 通用创建（如已有完整字段，直接落库）
	Create(ctx context.Context, log *models.LoginLog) (int64, error)

	// RecordLogin 记录登录事件
	RecordLogin(ctx context.Context, in *RecordLoginInput) (int64, error)

	// RecordRefreshToken 记录刷新令牌事件
	RecordRefreshToken(ctx context.Context, in *RecordRefreshInput) (int64, error)

	// List 查询日志
	List(ctx context.Context, req *ListRequest) ([]*models.LoginLog, error)

	// Count 统计数量
	Count(ctx context.Context, req *ListRequest) (int64, error)

	// ClearAll 清空全部日志
	ClearAll(ctx context.Context) (int64, error)

	// ClearBefore 清理指定时间之前的日志
	ClearBefore(ctx context.Context, before time.Time) (int64, error)
}

type service struct {
	svc bootstrap.ServiceContext
}

func NewService(svc bootstrap.ServiceContext) Service {
	return &service{
		svc: svc,
	}
}

func (s *service) getDB(ctx context.Context) *gorm.DB {
	return s.svc.GetDB(ctx).Model(new(models.LoginLog))
}
