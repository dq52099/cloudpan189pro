package autoingest

import (
	"errors"
	"net/http"

	"github.com/xxcheng123/cloudpan189-share/internal/consts"
	"github.com/xxcheng123/cloudpan189-share/internal/framework/httpcontext"
	"gorm.io/gorm"
)

var codeCloudTokenNotFound = bi.Next("云盘令牌不存在").WithHTTPCode(http.StatusNotFound)

func (h *handler) validateCloudTokenAccess(ctx *httpcontext.Context, tokenID int64) error {
	if tokenID < 0 {
		return errors.New("tokenId 必须大于等于 0")
	}

	if tokenID == 0 {
		return nil
	}

	if isNilDependency(h.cloudTokenService) {
		return errors.New("云盘令牌服务不可用")
	}

	_, err := h.cloudTokenService.QueryAccessible(
		ctx.GetContext(),
		tokenID,
		ctx.GetInt64(consts.CtxKeyUserId),
		ctx.GetBool(consts.CtxKeyIsAdmin),
	)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return codeCloudTokenNotFound.WithError(err)
	}

	return err
}

func failCloudTokenAccessError(ctx *httpcontext.Context, err error, fallback httpcontext.BusinessError) {
	var busErr httpcontext.BusinessError
	if errors.As(err, &busErr) {
		ctx.Fail(busErr)

		return
	}

	ctx.Fail(fallback.WithError(err))
}
