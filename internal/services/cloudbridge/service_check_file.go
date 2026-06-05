package cloudbridge

import (
	"github.com/xxcheng123/cloudpan189-interface/client"
	"github.com/xxcheng123/cloudpan189-share/internal/framework/context"
	"github.com/xxcheng123/cloudpan189-share/internal/pkgs/utils"
	"go.uber.org/zap"
)

func (s *service) CheckSubscribeUser(ctx context.Context, subscribeUser string) (string, error) {
	subscribeUser, err := validateCloud189SubscribeUserID(subscribeUser)
	if err != nil {
		return "", err
	}

	resp, err := client.New().WithClient(ctx.HTTPClient()).SubscribeGetUser(ctx, subscribeUser)
	if err != nil {
		return "", logCloudbridgeError(ctx, "查询订阅用户信息失败", err, zap.String("up_user_id", subscribeUser))
	}

	return resp.Data.Name, nil
}

func (s *service) CheckSubscribeShare(ctx context.Context, subscribeUser, shareCode, accessCode string) (shareId int64, isFolder bool, fileId string, shareMode int, resolvedAccessCode string, err error) {
	subscribeUser, err = validateCloud189SubscribeUserID(subscribeUser)
	if err != nil {
		return 0, false, "", 0, "", err
	}

	shareCode, err = validateCloud189ShareCode(shareCode)
	if err != nil {
		return 0, false, "", 0, "", err
	}

	accessCode, err = validateCloud189AccessCode(accessCode)
	if err != nil {
		return 0, false, "", 0, "", err
	}

	resp, err := client.New().WithClient(ctx.HTTPClient()).GetShareInfo(ctx, shareCode, func(gsir *client.GetShareInfoRequest) {
		gsir.AccessCode = accessCode
	})
	if err != nil {
		return 0, false, "", 0, "", logCloudbridgeError(ctx, "查询订阅分享信息失败", err,
			zap.String("up_user_id", subscribeUser),
			zap.String("share_code", utils.MaskShareCodeForLog(shareCode)),
			zap.Bool("has_access_code", accessCode != ""))
	}

	resolvedAccessCode = accessCode
	if resp.AccessCode != "" {
		resolvedAccessCode = resp.AccessCode
	}

	return resp.ShareId, resp.IsFolder, string(resp.FileId), resp.ShareMode, resolvedAccessCode, nil
}

type CheckShareResult struct {
	ShareId    int64
	IsFolder   bool
	AccessCode string
	ShareMode  int
	FileId     string
}

func (s *service) CheckShare(ctx context.Context, shareCode string, accessCode string) (result *CheckShareResult, err error) {
	shareCode, err = validateCloud189ShareCode(shareCode)
	if err != nil {
		return nil, err
	}

	accessCode, err = validateCloud189AccessCode(accessCode)
	if err != nil {
		return nil, err
	}

	cli := client.New().WithClient(ctx.HTTPClient())

	resp, err := cli.GetShareInfo(ctx, shareCode, func(gsir *client.GetShareInfoRequest) {
		gsir.AccessCode = accessCode
	})
	if err != nil {
		return nil, logCloudbridgeError(ctx, "查询分享信息失败", err,
			zap.String("share_code", utils.MaskShareCodeForLog(shareCode)),
			zap.Bool("has_access_code", accessCode != ""))
	}

	return &CheckShareResult{
		ShareId:    resp.ShareId,
		IsFolder:   resp.IsFolder,
		AccessCode: resp.AccessCode,
		ShareMode:  resp.ShareMode,
		FileId:     string(resp.FileId),
	}, nil
}

func (s *service) CheckPerson(ctx context.Context, token AuthToken, fileId string) (string, error) {
	resp, err := client.New().WithClient(ctx.HTTPClient()).WithToken(token).GetFolderInfo(ctx, client.String(fileId))
	if err != nil {
		return "", logCloudbridgeError(ctx, "查询文件信息失败", err, zap.String("file_id", fileId))
	}

	return resp.FileName, nil
}

func (s *service) CheckFamily(ctx context.Context, token AuthToken, familyId, fileId string) error {
	_, err := client.New().WithClient(ctx.HTTPClient()).WithToken(token).FamilyListFiles(ctx, client.String(familyId), client.String(fileId))
	if err != nil {
		return logCloudbridgeError(ctx, "查询文件信息失败", err, zap.String("family_id", familyId), zap.String("file_id", fileId))
	}

	return nil
}
