package cloudtoken

import (
	"github.com/pkg/errors"
	"github.com/xxcheng123/cloudpan189-interface/client"
	"github.com/xxcheng123/cloudpan189-share/internal/framework/context"
	"github.com/xxcheng123/cloudpan189-share/internal/repository/models"
	"go.uber.org/zap"
)

// CheckQrcodeRequest 检查二维码请求
type CheckQrcodeRequest struct {
	ID     int64  `json:"id" binding:"omitempty" example:"1"`                                     // 云盘令牌ID，可选
	UUID   string `json:"uuid" binding:"required" example:"550e8400-e29b-41d4-a716-446655440000"` // 二维码UUID
	UserID int64  // 创建令牌的用户ID
}

// CheckQrcode 检查二维码状态
func (s *service) CheckQrcode(ctx context.Context, req *CheckQrcodeRequest) (err error) {
	respData, err := client.LoginQuery(req.UUID)
	if err != nil {
		ctx.Error("登录查询失败", zap.Error(err))

		return errors.Wrap(err, "登录查询失败")
	}

	if req.ID != 0 {
		// 更新现有记录
		updateMap := map[string]interface{}{
			"status":       1,
			"access_token": respData.AccessToken,
			"expires_in":   respData.ExpiresIn,
		}

		if err = s.getDB(ctx).Where("id = ?", req.ID).Updates(updateMap).Error; err != nil {
			ctx.Error("更新云盘令牌失败", zap.Error(err), zap.Int64("id", req.ID))

			return errors.Wrap(err, "更新云盘令牌失败")
		}
	} else {
		// 创建新记录
		cloudToken := &models.CloudToken{
			Name:        "云盘令牌",
			Status:      1,
			AccessToken: respData.AccessToken,
			ExpiresIn:   respData.ExpiresIn,
			LoginType:   models.LoginTypeScan,
			Addition:    map[string]interface{}{},
			UserID:      req.UserID,
		}

		if err = s.getDB(ctx).Create(cloudToken).Error; err != nil {
			ctx.Error("创建云盘令牌失败", zap.Error(err))

			return errors.Wrap(err, "创建云盘令牌失败")
		}
	}

	return nil
}
