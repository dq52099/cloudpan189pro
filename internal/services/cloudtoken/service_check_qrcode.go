package cloudtoken

import (
	"time"

	"github.com/pkg/errors"
	"github.com/xxcheng123/cloudpan189-interface/client"
	"github.com/xxcheng123/cloudpan189-share/internal/framework/context"
	"github.com/xxcheng123/cloudpan189-share/internal/repository/models"
	"go.uber.org/zap"
	"gorm.io/datatypes"
)

var loginQuery = client.LoginQuery

// CheckQrcodeRequest 检查二维码请求
type CheckQrcodeRequest struct {
	ID      int64  `json:"id" binding:"omitempty" example:"1"`                                     // 云盘令牌ID，可选
	UUID    string `json:"uuid" binding:"required" example:"550e8400-e29b-41d4-a716-446655440000"` // 二维码UUID
	UserID  int64  // 创建令牌的用户ID
	IsAdmin bool   // 是否管理员
}

// CheckQrcode 检查二维码状态
func (s *service) CheckQrcode(ctx context.Context, req *CheckQrcodeRequest) (err error) {
	if req == nil {
		return errInvalidCloudTokenUserID
	}

	if req.ID < 0 {
		return errInvalidCloudTokenID
	}

	if req.ID == 0 && req.UserID <= 0 {
		return errInvalidCloudTokenUserID
	}

	var oldToken *models.CloudToken

	if req.ID != 0 {
		oldToken, err = s.QueryAccessible(ctx, req.ID, req.UserID, req.IsAdmin)
		if err != nil {
			return err
		}
	}

	respData, err := loginQuery(req.UUID)
	if err != nil {
		safeErr := sanitizeCloudTokenLogError(err)
		ctx.Error("登录查询失败", zap.String("error", safeErr))

		return errors.Errorf("登录查询失败: %s", safeErr)
	}

	if req.ID != 0 {
		// 更新现有记录
		updateMap := map[string]interface{}{
			"status":       1,
			"access_token": respData.AccessToken,
			"expires_in":   respData.ExpiresIn,
			"addition": func() datatypes.JSONMap {
				addition := oldToken.Addition
				if addition == nil {
					addition = make(map[string]interface{})
				}

				addition[models.CloudTokenAdditionTokenIssuedAt] = time.Now().UnixMilli()

				return addition
			}(),
		}

		query := s.getDB(ctx).Where("id = ?", req.ID)
		if !req.IsAdmin {
			query = query.Where("user_id = ?", req.UserID)
		}

		result := query.Updates(updateMap)
		if result.Error != nil {
			ctx.Error("更新云盘令牌失败", zap.Error(result.Error), zap.Int64("id", req.ID))

			return errors.Wrap(result.Error, "更新云盘令牌失败")
		}

		if err := s.checkCloudTokenUpdateResult(ctx, result, req.ID, req.UserID, req.IsAdmin, "令牌不存在"); err != nil {
			return err
		}
	} else {
		// 创建新记录
		cloudToken := &models.CloudToken{
			Name:        "云盘令牌",
			Status:      1,
			AccessToken: respData.AccessToken,
			ExpiresIn:   respData.ExpiresIn,
			LoginType:   models.LoginTypeScan,
			Addition: map[string]interface{}{
				models.CloudTokenAdditionTokenIssuedAt: time.Now().UnixMilli(),
			},
			UserID: req.UserID,
		}

		if err = s.getDB(ctx).Create(cloudToken).Error; err != nil {
			ctx.Error("创建云盘令牌失败", zap.Error(err))

			return errors.Wrap(err, "创建云盘令牌失败")
		}
	}

	return nil
}
