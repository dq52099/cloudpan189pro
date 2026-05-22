package cloudbridge

import (
	"github.com/pkg/errors"
	"github.com/xxcheng123/cloudpan189-interface/client"
	"github.com/xxcheng123/cloudpan189-share/internal/framework/context"
	"go.uber.org/zap"
)

func (s *service) CheckSubscribeUser(ctx context.Context, subscribeUser string) (string, error) {
	resp, err := client.New().WithClient(ctx.HTTPClient()).SubscribeGetUser(ctx, subscribeUser)
	if err != nil {
		ctx.Error("查询订阅用户信息失败", zap.Error(err), zap.String("up_user_id", subscribeUser))

		return "", errors.WithStack(err)
	}

	return resp.Data.Name, nil
}

func (s *service) CheckSubscribeShare(ctx context.Context, subscribeUser, shareCode string) (shareId int64, isFolder bool, fileId string, err error) {
	resp, err := client.New().WithClient(ctx.HTTPClient()).GetShareInfo(ctx, shareCode)
	if err != nil {
		ctx.Error("查询订阅分享信息失败", zap.Error(err), zap.String("up_user_id", subscribeUser), zap.String("share_code", shareCode))

		return 0, false, "", errors.WithStack(err)
	}

	return resp.ShareId, resp.IsFolder, string(resp.FileId), nil
}

type CheckShareResult struct {
	ShareId    int64
	IsFolder   bool
	AccessCode string
	ShareMode  int
	FileId     string
}

func (s *service) CheckShare(ctx context.Context, shareCode string, accessCode string) (result *CheckShareResult, err error) {
	cli := client.New().WithClient(ctx.HTTPClient())

	resp, err := cli.GetShareInfo(ctx, shareCode, func(gsir *client.GetShareInfoRequest) {
		gsir.AccessCode = accessCode
	})
	if err != nil {
		ctx.Error("查询分享信息失败", zap.Error(err), zap.String("share_code", shareCode), zap.String("access_code", accessCode))

		return nil, errors.WithStack(err)
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
		ctx.Error("查询文件信息失败", zap.Error(err), zap.String("file_id", fileId))

		return "", errors.WithStack(err)
	}

	return resp.FileName, nil
}

func (s *service) CheckFamily(ctx context.Context, token AuthToken, familyId, fileId string) error {
	_, err := client.New().WithClient(ctx.HTTPClient()).WithToken(token).FamilyListFiles(ctx, client.String(familyId), client.String(fileId))
	if err != nil {
		ctx.Error("查询文件信息失败", zap.Error(err), zap.String("file_id", fileId))

		return errors.WithStack(err)
	}

	return nil
}
