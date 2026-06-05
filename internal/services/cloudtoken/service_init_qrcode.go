package cloudtoken

import (
	"fmt"

	"github.com/xxcheng123/cloudpan189-interface/client"
	"github.com/xxcheng123/cloudpan189-share/internal/framework/context"
	"go.uber.org/zap"
)

var loginInit = client.LoginInit

// InitQrcodeResponse 初始化二维码响应
type InitQrcodeResponse struct {
	UUID string `json:"uuid" example:"550e8400-e29b-41d4-a716-446655440000"` // 二维码UUID
}

func (s *service) InitQrcode(ctx context.Context) (resp *InitQrcodeResponse, err error) {
	respData, err := loginInit()
	if err != nil {
		safeErr := sanitizeCloudTokenLogError(err)
		ctx.Error("登录初始化失败", zap.String("error", safeErr))

		return nil, fmt.Errorf("登录初始化失败: %s", safeErr)
	}

	return &InitQrcodeResponse{
		UUID: respData.UUID,
	}, nil
}
