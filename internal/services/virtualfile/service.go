package virtualfile

import (
	"sync"

	"github.com/xxcheng123/cloudpan189-share/internal/bootstrap"
	"github.com/xxcheng123/cloudpan189-share/internal/framework/context"
	"github.com/xxcheng123/cloudpan189-share/internal/pkgs/utils"
	"github.com/xxcheng123/cloudpan189-share/internal/repository/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Service interface {
	BatchQueryParentFiles(ctx context.Context, id int64) ([]*models.VirtualFile, error)

	GetMaxId(ctx context.Context) (maxId int64, err error)
	CalFullPath(ctx context.Context, id int64) (path string, err error)
	CalFilePath(ctx context.Context, id int64) (path string, err error)

	List(ctx context.Context, req *ListRequest) ([]*models.VirtualFile, error)
	Count(ctx context.Context, req *ListRequest) (count int64, err error)
	Query(ctx context.Context, fid int64) (*models.VirtualFile, error)
	QueryByPath(ctx context.Context, path string) (*models.VirtualFile, error)
	QueryTop(ctx context.Context, fid int64) (*models.VirtualFile, error)
	FindOrCreateAncestors(ctx context.Context, path string) (int64, error)
	Create(ctx context.Context, parentId int64, file *models.VirtualFile) (int64, error)
	CreateTop(ctx context.Context, parentId int64, file *models.VirtualFile) (int64, error)
	BatchCreate(ctx context.Context, parentId int64, files []*models.VirtualFile, hooks ...BatchCreateHook) (int64, error)
	Delete(ctx context.Context, id int64, hooks ...DeleteHook) error
	BatchDelete(ctx context.Context, ids []int64, hooks ...BatchDeleteHook) (deletedIdList []int64, err error)
	Update(ctx context.Context, id int64, opts []utils.Field, hooks ...UpdateHook) error
	ModifyAddition(ctx context.Context, id int64, key string, value any) error
	BatchUpdate(ctx context.Context, filesToUpdate map[int64][]utils.Field) error
	BatchUpdatePlus(ctx context.Context, values []utils.Field, exps []clause.Expression) error
	GroupCountByTopId(ctx context.Context, req *GroupCountByTopIdRequest) ([]*GroupCountByTopId, error)
	ClearUnusedAncestorFolder(ctx context.Context, subId int64) error
	ClearAll(ctx context.Context) error
}

type service struct {
	svc    bootstrap.ServiceContext
	dbLock sync.Mutex
}

func NewService(svc bootstrap.ServiceContext) Service {
	return &service{
		svc: svc,
	}
}

func (s *service) getDB(ctx context.Context) *gorm.DB {
	return s.svc.GetDB(ctx).Model(new(models.VirtualFile))
}

// withLock 默认使用 sqlite，性能差
func (s *service) withLock(ctx context.Context, fn func(db *gorm.DB) *gorm.DB) *gorm.DB {
	s.dbLock.Lock()
	defer s.dbLock.Unlock()

	return fn(s.getDB(ctx))
}
