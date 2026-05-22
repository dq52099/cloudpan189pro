package cloudtoken

import (
	"github.com/xxcheng123/cloudpan189-share/internal/bootstrap"
	"github.com/xxcheng123/cloudpan189-share/internal/framework/context"
	"github.com/xxcheng123/cloudpan189-share/internal/repository/models"
	"gorm.io/gorm"
)

type Service interface {
	InitQrcode(ctx context.Context) (resp *InitQrcodeResponse, err error)
	CheckQrcode(ctx context.Context, req *CheckQrcodeRequest) (err error)
	ModifyName(ctx context.Context, req *ModifyNameRequest) (err error)
	Delete(ctx context.Context, req *DeleteRequest) (err error)
	List(ctx context.Context, req *ListRequest) (list []*models.CloudToken, err error)
	Count(ctx context.Context, req *ListRequest) (count int64, err error)
	UsernameLogin(ctx context.Context, req *UsernameLoginRequest) (resp *UsernameLoginResponse, err error)
	Query(ctx context.Context, id int64) (*models.CloudToken, error)
	QueryAccessible(ctx context.Context, id, userID int64, isAdmin bool) (*models.CloudToken, error)
	// ListPasswordLoginTokens 查询所有使用密码登录的令牌
	ListPasswordLoginTokens(ctx context.Context) ([]*models.CloudToken, error)
	// UpdateAddition 更新令牌的附加信息
	UpdateAddition(ctx context.Context, id int64, addition map[string]interface{}) error
}

type service struct {
	svc bootstrap.ServiceContext
}

// NewService 创建云盘令牌服务
func NewService(svc bootstrap.ServiceContext) Service {
	return &service{
		svc: svc,
	}
}

func (s *service) getDB(ctx context.Context) *gorm.DB {
	return s.svc.GetDB(ctx).Model(new(models.CloudToken))
}
