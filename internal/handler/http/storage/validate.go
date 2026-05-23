package storage

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/xxcheng123/cloudpan189-share/internal/consts"
	"github.com/xxcheng123/cloudpan189-share/internal/framework/context"
	"github.com/xxcheng123/cloudpan189-share/internal/framework/httpcontext"
	"github.com/xxcheng123/cloudpan189-share/internal/pkgs/datatypes"
	"github.com/xxcheng123/cloudpan189-share/internal/repository/models"
	cloudbridgeSvi "github.com/xxcheng123/cloudpan189-share/internal/services/cloudbridge"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

var (
	reShareLinkForAdd  = regexp.MustCompile(`cloud\.189\.cn\/t\/([a-zA-Z0-9]+)`)
	reAccessCodeForAdd = regexp.MustCompile(`(?:\S+码|code)[:：]\s*([a-zA-Z0-9]+)`)
)

func (h *handler) executeOsTypeSubscribe(ctx context.Context, req *addRequest) (datatypes.JSONMap, httpcontext.BusinessError) {
	if req.OsType != models.OsTypeSubscribe {
		return nil, busCodeStorageOsTypeNotMatch
	}

	if req.SubscribeUser == "" {
		return nil, busCodeStorageSubscribeUserEmpty
	}

	if _, err := h.cloudBridgeService.CheckSubscribeUser(ctx, req.SubscribeUser); err != nil {
		return nil, busCodeStorageQuerySubscribeUserError.WithError(err)
	}

	return datatypes.JSONMap{
		consts.FileAdditionKeyUpUserId: req.SubscribeUser,
	}, nil
}

func (h *handler) executeOsTypeSubscribeShare(ctx context.Context, req *addRequest) (datatypes.JSONMap, string, httpcontext.BusinessError) {
	if req.OsType != models.OsTypeSubscribeShareFolder {
		return nil, "", busCodeStorageOsTypeNotMatch
	}

	if req.SubscribeUser == "" || req.ShareCode == "" {
		return nil, "", busCodeStorageSubscribeShareIncomplete
	}

	shareId, isFolder, fileId, err := h.cloudBridgeService.CheckSubscribeShare(ctx, req.SubscribeUser, req.ShareCode)
	if err != nil {
		return nil, "", busCodeStorageQuerySubscribeShareError.WithError(err)
	}

	return datatypes.JSONMap{
		consts.FileAdditionKeyUpUserId: req.SubscribeUser,
		consts.FileAdditionKeyShareId:  shareId,
		consts.FileAdditionKeyIsFolder: isFolder,
	}, fileId, nil
}

func (h *handler) executeOsTypeShare(ctx context.Context, req *addRequest) (datatypes.JSONMap, string, httpcontext.BusinessError) {
	if req.OsType != models.OsTypeShareFolder {
		return nil, "", busCodeStorageOsTypeNotMatch
	}

	if req.ShareCode == "" {
		return nil, "", busCodeStorageShareCodeEmpty
	}

	// 1. 清洗输入
	cleanCode := strings.ReplaceAll(req.ShareCode, "（", "(")
	cleanCode = strings.ReplaceAll(cleanCode, "）", ")")
	cleanCode = strings.ReplaceAll(cleanCode, "：", ":")
	cleanCode = strings.TrimSpace(cleanCode)

	var pureShareCode, pureAccessCode string

	// 2. 智能提取访问码
	req.ShareAccessCode = strings.TrimSpace(req.ShareAccessCode)
	if codeMatch := reAccessCodeForAdd.FindStringSubmatch(cleanCode); len(codeMatch) > 1 {
		pureAccessCode = codeMatch[1]
	} else if req.ShareAccessCode != "" {
		pureAccessCode = req.ShareAccessCode
	} else {
		parts := strings.Fields(cleanCode)
		if len(parts) > 1 {
			lastPart := strings.Trim(parts[len(parts)-1], "()")
			if len(lastPart) == 4 {
				pureAccessCode = lastPart
			}
		}
	}

	// 3. 智能提取分享码
	if matches := reShareLinkForAdd.FindStringSubmatch(cleanCode); len(matches) > 1 {
		pureShareCode = matches[1]
	} else {
		if idx := strings.Index(cleanCode, "("); idx > -1 {
			pureShareCode = strings.TrimSpace(cleanCode[:idx])
		} else {
			parts := strings.Fields(cleanCode)
			if len(parts) > 0 {
				pureShareCode = strings.Trim(parts[0], "()")
			}
		}
	}

	// 4. 构造 SDK 调用用的分享码格式
	formattedCode := pureShareCode
	if pureShareCode != "" && pureAccessCode != "" {
		formattedCode = fmt.Sprintf("%s（访问码：%s）", pureShareCode, pureAccessCode)
		req.ShareCode = formattedCode
		req.ShareAccessCode = pureAccessCode
	} else if pureShareCode != "" {
		req.ShareCode = pureShareCode
	}

	// 5. 调用 API 验证
	ctx.Info("开始校验分享码", zap.String("try_code", formattedCode), zap.String("access_code", pureAccessCode))

	result, err := h.cloudBridgeService.CheckShare(ctx, formattedCode, pureAccessCode)

	// 6. 错误处理与重试
	if err != nil || (result != nil && result.ShareId == 0) {
		ctx.Warn("完整格式校验未通过，尝试使用纯码重试",
			zap.String("formatted_code", formattedCode),
			zap.String("pure_code", pureShareCode),
			zap.Error(err),
			zap.Any("first_result", result),
		)

		if formattedCode != pureShareCode {
			resultRetry, errRetry := h.cloudBridgeService.CheckShare(ctx, pureShareCode, pureAccessCode)
			if errRetry == nil && resultRetry != nil && resultRetry.ShareId != 0 {
				result = resultRetry
				err = nil
			} else {
				checkUrl := fmt.Sprintf("https://cloud.189.cn/t/%s", pureShareCode)

				resultUrl, errUrl := h.cloudBridgeService.CheckShare(ctx, checkUrl, pureAccessCode)
				if errUrl == nil && resultUrl != nil && resultUrl.ShareId != 0 {
					result = resultUrl
					err = nil
				}
			}
		}
	}

	// 7. 最终检查
	if err != nil {
		return nil, "", busCodeStorageQuerySubscribeShareError.WithError(err)
	}

	if result == nil || result.ShareId == 0 {
		ctx.Error("所有尝试均失败，无法获取ShareId",
			zap.String("final_result_struct", fmt.Sprintf("%+v", result)))

		return nil, "", busCodeStorageQuerySubscribeShareError.WithError(fmt.Errorf("无法获取有效的分享ID(ShareId=0)，请确认分享链接是否有效"))
	}

	return datatypes.JSONMap{
		consts.FileAdditionKeyShareId:    result.ShareId,
		consts.FileAdditionKeyIsFolder:   result.IsFolder,
		consts.FileAdditionKeyShareMode:  result.ShareMode,
		consts.FileAdditionKeyAccessCode: pureAccessCode,
	}, result.FileId, nil
}

func (h *handler) executeOsTypePersonal(ctx context.Context, req *addRequest, userID int64, isAdmin bool) httpcontext.BusinessError {
	if req.OsType != models.OsTypePersonFolder {
		return busCodeStorageOsTypeNotMatch
	}

	if req.FileId == "" {
		return busCodeStoragePersonParamsIncomplete
	}

	if req.CloudToken == 0 {
		return busCodeStorageCloudTokenEmpty
	}

	token, err := h.cloudTokenService.QueryAccessible(ctx, req.CloudToken, userID, isAdmin)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return busCodeStorageCloudTokenNotExist.WithError(err)
		}

		return busCodeStorageQueryCloudTokenError.WithError(err)
	}

	if token == nil {
		return busCodeStorageCloudTokenNotExist
	}

	if _, err = h.cloudBridgeService.CheckPerson(ctx, cloudbridgeSvi.NewAuthToken(token.AccessToken, token.ExpiresIn), req.FileId); err != nil {
		return busCodeStoragePersonFileQueryError.WithError(err)
	}

	return nil
}

func (h *handler) executeOsTypeFamily(ctx context.Context, req *addRequest, userID int64, isAdmin bool) (datatypes.JSONMap, httpcontext.BusinessError) {
	if req.OsType != models.OsTypeFamilyFolder {
		return nil, busCodeStorageOsTypeNotMatch
	}

	if req.FileId == "" || req.FamilyId == "" {
		return nil, busCodeStorageFamilyParamsIncomplete
	}

	if req.CloudToken == 0 {
		return nil, busCodeStorageCloudTokenEmpty
	}

	token, err := h.cloudTokenService.QueryAccessible(ctx, req.CloudToken, userID, isAdmin)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, busCodeStorageCloudTokenNotExist.WithError(err)
		}

		return nil, busCodeStorageQueryCloudTokenError.WithError(err)
	}

	if token == nil {
		return nil, busCodeStorageCloudTokenNotExist
	}

	if err = h.cloudBridgeService.CheckFamily(ctx, cloudbridgeSvi.NewAuthToken(token.AccessToken, token.ExpiresIn), req.FamilyId, req.FileId); err != nil {
		return nil, busCodeStorageFamilyFileQueryError.WithError(err)
	}

	return datatypes.JSONMap{
		consts.FileAdditionKeyFamilyId: req.FamilyId,
	}, nil
}
