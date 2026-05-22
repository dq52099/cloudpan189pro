package autoingest

import (
	"errors"

	"github.com/xxcheng123/cloudpan189-share/internal/consts"
	"github.com/xxcheng123/cloudpan189-share/internal/framework/httpcontext"
)

func (h *handler) validateCloudTokenAccess(ctx *httpcontext.Context, tokenID int64) error {
	if tokenID < 0 {
		return errors.New("tokenId 必须大于等于 0")
	}

	if tokenID == 0 {
		return nil
	}

	if h.cloudTokenService == nil {
		return errors.New("云盘令牌服务不可用")
	}

	_, err := h.cloudTokenService.QueryAccessible(
		ctx.GetContext(),
		tokenID,
		ctx.GetInt64(consts.CtxKeyUserId),
		ctx.GetBool(consts.CtxKeyIsAdmin),
	)

	return err
}
