package verify

import (
	"net/url"

	"github.com/xxcheng123/cloudpan189-share/internal/bootstrap"
	"github.com/xxcheng123/cloudpan189-share/internal/framework/context"
)

type Service interface {
	// SignV1 生成下载文件令牌V1
	SignV1(ctx context.Context, fileId int64, opts ...SignV1OptionFunc) (values url.Values, err error)
	// VerifyV1 验证下载文件令牌V1
	VerifyV1(ctx context.Context, fileId int64, sign, uuidValue, timestamp, signer string) error
}

type service struct {
	svc bootstrap.ServiceContext
}

func NewService(svc bootstrap.ServiceContext) Service {
	return &service{
		svc: svc,
	}
}
