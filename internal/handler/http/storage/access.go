package storage

import (
	"errors"

	"github.com/xxcheng123/cloudpan189-share/internal/consts"
	"github.com/xxcheng123/cloudpan189-share/internal/framework/httpcontext"
	"github.com/xxcheng123/cloudpan189-share/internal/repository/models"
	"gorm.io/gorm"
)

func (h *handler) queryOwnedMountPoint(ctx *httpcontext.Context, id int64) (*models.MountPoint, bool) {
	if !h.ensureMountPointService(ctx, busCodeStorageQueryMountPointError) {
		return nil, false
	}

	mountPoint, err := h.mountPointService.QueryByID(ctx.GetContext(), id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			ctx.Fail(busCodeStorageMountPointNotFound.WithError(err))
		} else {
			ctx.Fail(busCodeStorageQueryMountPointError.WithError(err))
		}

		return nil, false
	}

	if mountPoint == nil {
		ctx.Fail(busCodeStorageMountPointNotFound.WithError(gorm.ErrRecordNotFound))

		return nil, false
	}

	userID := ctx.GetInt64(consts.CtxKeyUserId)

	isAdmin := ctx.GetBool(consts.CtxKeyIsAdmin)
	if !isAdmin && (userID <= 0 || mountPoint.CreatorUserID != userID) {
		ctx.Fail(busCodeStorageMountPointNotFound)

		return nil, false
	}

	return mountPoint, true
}
