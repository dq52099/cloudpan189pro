package advance

import (
	"errors"

	"github.com/xxcheng123/cloudpan189-share/internal/consts"
	"github.com/xxcheng123/cloudpan189-share/internal/framework/httpcontext"
	"github.com/xxcheng123/cloudpan189-share/internal/repository/models"
	"gorm.io/gorm"
)

func (h *handler) queryAccessibleCloudToken(ctx *httpcontext.Context, cloudTokenID int64) (*models.CloudToken, bool) {
	if isNilDependency(h.cloudTokenService) {
		ctx.Fail(codeStorageAdvanceQueryCloudTokenError.WithError(errors.New("云盘令牌服务未初始化")))

		return nil, false
	}

	token, err := h.cloudTokenService.QueryAccessible(
		ctx.GetContext(),
		cloudTokenID,
		ctx.GetInt64(consts.CtxKeyUserId),
		ctx.GetBool(consts.CtxKeyIsAdmin),
	)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			ctx.Fail(codeStorageAdvanceCloudTokenNotExist.WithError(err))
		} else {
			ctx.Fail(codeStorageAdvanceQueryCloudTokenError.WithError(err))
		}

		return nil, false
	}

	if token == nil {
		ctx.Fail(codeStorageAdvanceCloudTokenNotExist)

		return nil, false
	}

	return token, true
}
