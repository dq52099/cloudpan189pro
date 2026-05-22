package cloudtoken

import (
	"github.com/pkg/errors"
	"github.com/xxcheng123/cloudpan189-share/internal/framework/context"
	"go.uber.org/zap"
)

// ModifyNameRequest 修改云盘令牌名称请求
type ModifyNameRequest struct {
	ID      int64  `json:"id" binding:"required" example:"1"`     // 云盘令牌ID
	Name    string `json:"name" binding:"required" example:"新名称"` // 新名称
	UserID  int64  `json:"-"`                                     // 当前用户ID
	IsAdmin bool   `json:"-"`                                     // 是否管理员
}

func (s *service) ModifyName(ctx context.Context, req *ModifyNameRequest) (err error) {
	if req == nil || req.ID <= 0 {
		return errInvalidCloudTokenID
	}

	query := s.getDB(ctx).Where("id = ?", req.ID)
	if !req.IsAdmin {
		if req.UserID <= 0 {
			return errInvalidCloudTokenUserID
		}

		query = query.Where("user_id = ?", req.UserID)
	}

	result := query.Update("name", req.Name)
	if result.Error != nil {
		ctx.Error("修改云盘令牌名称失败", zap.Error(result.Error), zap.Int64("id", req.ID), zap.String("name", req.Name))

		return errors.Wrap(result.Error, "修改云盘令牌名称失败")
	}

	return s.checkCloudTokenUpdateResult(ctx, result, req.ID, req.UserID, req.IsAdmin, "令牌不存在")
}
