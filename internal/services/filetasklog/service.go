package filetasklog

import (
	"time"

	"github.com/xxcheng123/cloudpan189-share/internal/bootstrap"
	"github.com/xxcheng123/cloudpan189-share/internal/framework/context"
	"github.com/xxcheng123/cloudpan189-share/internal/pkgs/utils"
	"github.com/xxcheng123/cloudpan189-share/internal/repository/models"
	"gorm.io/gorm"
)

type Service interface {
	Create(ctx context.Context, typ, title string, opts ...NewOptionFunc) (*Tracker, error)
	FlushCount(ctx context.Context, key LogKey, counters ...Counter) error

	ToggleStatus(ctx context.Context, key LogKey, status string, opts ...utils.Field) error
	Pending(ctx context.Context, key LogKey, opts ...utils.Field) error
	Running(ctx context.Context, key LogKey, opts ...utils.Field) error
	Completed(ctx context.Context, key LogKey, opts ...utils.Field) error
	Failed(ctx context.Context, key LogKey, opts ...utils.Field) error

	List(ctx context.Context, req *ListRequest) ([]*models.FileTaskLog, error)
	Count(ctx context.Context, req *ListRequest) (int64, error)
	FindStaleTasksByDuration(ctx context.Context, duration time.Duration) ([]*models.FileTaskLog, error)

	WithError(ctx context.Context, key LogKey, err error) error

	Clear(ctx context.Context) error
	ClearByDuration(ctx context.Context, duration string) error
}

type service struct {
	svc bootstrap.ServiceContext
}

// NewService 创建文件任务日志服务
func NewService(svc bootstrap.ServiceContext) Service {
	return &service{
		svc: svc,
	}
}

func (s *service) getDB(ctx context.Context) *gorm.DB {
	return s.svc.GetDB(ctx).Model(new(models.FileTaskLog))
}

type LogKey interface {
	GetID() int64
}

type LogID int64

func (l LogID) GetID() int64  { return int64(l) }
func NewLogID(id int64) LogID { return LogID(id) }
