package cloudbridge

import (
	"fmt"
	"strings"
	"time"

	"github.com/xxcheng123/cloudpan189-interface/client"
	"github.com/xxcheng123/cloudpan189-share/internal/framework/context"
	"github.com/xxcheng123/cloudpan189-share/internal/pkgs/utils"
	"go.uber.org/zap"
)

type ShareInfo struct {
	Name       string    `json:"name"`
	IsFolder   bool      `json:"isFolder"`
	AccessCode string    `json:"accessCode"`
	ShareId    int64     `json:"shareId"`
	ShareMode  int       `json:"shareMode"`
	ID         string    `json:"id"`
	ShareTime  time.Time `json:"shareTime"`
}

func validateCloud189ShareCode(shareCode string) (string, error) {
	shareCode = strings.TrimSpace(shareCode)
	if !utils.IsCloud189ShareCode(shareCode) {
		return "", fmt.Errorf("分享码格式无效")
	}

	return shareCode, nil
}

func validateCloud189AccessCode(accessCode string) (string, error) {
	accessCode = strings.TrimSpace(accessCode)
	if accessCode == "" {
		return "", nil
	}

	if !utils.IsCloud189AccessCode(accessCode) {
		return "", fmt.Errorf("访问码格式无效")
	}

	return accessCode, nil
}

func validateCloud189SubscribeUserID(userID string) (string, error) {
	userID = utils.NormalizeCloud189SubscribeUserID(userID)
	if userID == "" {
		return "", fmt.Errorf("订阅用户ID格式无效")
	}

	return userID, nil
}

func (s *service) GetShareInfo(ctx context.Context, shareCode string, accessCode string) (*ShareInfo, error) {
	shareCode, err := validateCloud189ShareCode(shareCode)
	if err != nil {
		return nil, err
	}

	accessCode, err = validateCloud189AccessCode(accessCode)
	if err != nil {
		return nil, err
	}

	info, err := s.getClient(ctx).GetShareInfo(ctx, shareCode, func(gsir *client.GetShareInfoRequest) {
		gsir.AccessCode = accessCode
	})
	if err != nil {
		return nil, logCloudbridgeError(ctx, "获取分享详情失败", err,
			zap.String("shareCode", utils.MaskShareCodeForLog(shareCode)),
			zap.Bool("has_access_code", accessCode != ""))
	}

	return &ShareInfo{
		Name:       info.FileName,
		IsFolder:   info.IsFolder,
		AccessCode: info.AccessCode,
		ShareId:    info.ShareId,
		ShareMode:  info.ShareMode,
		ID:         string(info.FileId),
		ShareTime:  parseShareTime(info.FileCreateDate),
	}, nil
}
