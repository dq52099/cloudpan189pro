package group2file

import (
	"sync"

	"github.com/xxcheng123/cloudpan189-share/internal/bootstrap"
	"github.com/xxcheng123/cloudpan189-share/internal/framework/context"
	"github.com/xxcheng123/cloudpan189-share/internal/repository/models"
	"gorm.io/gorm"
)

type Service interface {
	// BatchBindFiles 批量绑定文件权限到用户组 先删除 再绑定
	BatchBindFiles(ctx context.Context, groupId int64, fileIds []int64) error
	// GetBindFiles 获取组的所有文件ID
	GetBindFiles(ctx context.Context, groupId int64) ([]int64, error)
	// CheckPermission 检查用户组是否有文件访问权限
	CheckPermission(ctx context.Context, groupId int64, fileId int64) (bool, error)
}

type service struct {
	svc           bootstrap.ServiceContext
	dbLock        sync.Mutex
	usesLocalLock bool
}

func NewService(svc bootstrap.ServiceContext) Service {
	return &service{
		svc:           svc,
		usesLocalLock: group2FileDBNeedsLocalLock(svc),
	}
}

func (s *service) getDB(ctx context.Context) *gorm.DB {
	return s.svc.GetDB(ctx).Model(new(models.Group2File))
}

func group2FileDBNeedsLocalLock(svc bootstrap.ServiceContext) bool {
	db := svc.GetDBWithoutContext()
	if db == nil || db.Dialector == nil {
		return true
	}

	dialector := db.Dialector
	switch dialector.Name() {
	case "sqlite", "sqlite3":
		return true
	default:
		return false
	}
}

func (s *service) withWriteLock(ctx context.Context, fn func(db *gorm.DB) error) error {
	if !s.usesLocalLock {
		return fn(s.getDB(ctx))
	}

	s.dbLock.Lock()
	defer s.dbLock.Unlock()

	return fn(s.getDB(ctx))
}
