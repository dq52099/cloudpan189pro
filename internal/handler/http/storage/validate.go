package storage

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/xxcheng123/cloudpan189-share/internal/consts"
	"github.com/xxcheng123/cloudpan189-share/internal/framework/context"
	"github.com/xxcheng123/cloudpan189-share/internal/framework/httpcontext"
	"github.com/xxcheng123/cloudpan189-share/internal/pkgs/datatypes"
	"github.com/xxcheng123/cloudpan189-share/internal/pkgs/utils"
	"github.com/xxcheng123/cloudpan189-share/internal/repository/models"
	cloudbridgeSvi "github.com/xxcheng123/cloudpan189-share/internal/services/cloudbridge"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

func normalizeLocalPathForAdd(localPath string) (string, error) {
	normalizedPath, err := utils.NormalizeStoragePath(localPath)
	if err != nil {
		return "", fmt.Errorf("路径不合法，需要 / 开头的路径")
	}

	paths := utils.SplitNormalizedStoragePath(normalizedPath)
	if len(paths) == 0 {
		return "", fmt.Errorf("不允许挂载根路径")
	}

	return normalizedPath, nil
}

func (h *handler) executeOsTypeSubscribe(ctx context.Context, req *addRequest) (datatypes.JSONMap, httpcontext.BusinessError) {
	if req.OsType != models.OsTypeSubscribe {
		return nil, busCodeStorageOsTypeNotMatch
	}

	req.SubscribeUser = normalizeSubscribeUserID(req.SubscribeUser)
	if req.SubscribeUser == "" {
		return nil, busCodeStorageSubscribeUserEmpty
	}

	if busErr := h.missingCloudBridgeBusinessError(busCodeStorageQuerySubscribeUserError); busErr != nil {
		return nil, busErr
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

	req.SubscribeUser = normalizeSubscribeUserID(req.SubscribeUser)
	if req.SubscribeUser == "" || req.ShareCode == "" {
		return nil, "", busCodeStorageSubscribeShareIncomplete
	}

	shareCode, accessCode := normalizeSubscribeShareParams(req)
	if !utils.IsCloud189ShareCode(shareCode) {
		return nil, "", busCodeStorageSubscribeShareIncomplete
	}

	if accessCode != "" && !utils.IsCloud189AccessCode(accessCode) {
		return nil, "", busCodeStorageShareAccessCodeInvalid
	}

	if busErr := h.missingCloudBridgeBusinessError(busCodeStorageQuerySubscribeShareError); busErr != nil {
		return nil, "", busErr
	}

	shareId, isFolder, fileId, shareMode, resolvedAccessCode, err := h.cloudBridgeService.CheckSubscribeShare(ctx, req.SubscribeUser, shareCode, accessCode)
	if err != nil {
		return nil, "", busCodeStorageQuerySubscribeShareError.WithError(err)
	}

	if shareMode <= 0 {
		shareMode = 5
	}

	if resolvedAccessCode != "" {
		accessCode = resolvedAccessCode
	}

	return datatypes.JSONMap{
		consts.FileAdditionKeyUpUserId:   req.SubscribeUser,
		consts.FileAdditionKeyShareId:    shareId,
		consts.FileAdditionKeyIsFolder:   isFolder,
		consts.FileAdditionKeyShareMode:  shareMode,
		consts.FileAdditionKeyAccessCode: accessCode,
	}, fileId, nil
}

func normalizeSubscribeShareParams(req *addRequest) (string, string) {
	if req == nil {
		return "", ""
	}

	return utils.ParseCloud189ShareCode(req.ShareCode, req.ShareAccessCode)
}

func normalizeSubscribeUserID(value string) string {
	return utils.NormalizeCloud189SubscribeUserID(value)
}

func (h *handler) executeOsTypeShare(ctx context.Context, req *addRequest) (datatypes.JSONMap, string, httpcontext.BusinessError) {
	if req.OsType != models.OsTypeShareFolder {
		return nil, "", busCodeStorageOsTypeNotMatch
	}

	if req.ShareCode == "" {
		return nil, "", busCodeStorageShareCodeEmpty
	}

	// 1. 清洗并提取分享码/访问码。单独填写的访问码优先级更高，便于覆盖链接里的旧访问码。
	pureShareCode, pureAccessCode := utils.ParseCloud189ShareCode(req.ShareCode, req.ShareAccessCode)
	if !utils.IsCloud189ShareCode(pureShareCode) {
		return nil, "", busCodeStorageShareCodeInvalid
	}

	if pureAccessCode != "" && !utils.IsCloud189AccessCode(pureAccessCode) {
		return nil, "", busCodeStorageShareAccessCodeInvalid
	}

	req.ShareCode = pureShareCode
	req.ShareAccessCode = pureAccessCode

	if busErr := h.missingCloudBridgeBusinessError(busCodeStorageQuerySubscribeShareError); busErr != nil {
		return nil, "", busErr
	}

	// 2. 调用 API 验证
	ctx.Info("开始校验分享码",
		zap.String("share_code", utils.MaskShareCodeForLog(pureShareCode)),
		zap.Bool("has_access_code", pureAccessCode != ""))

	result, err := h.cloudBridgeService.CheckShare(ctx, pureShareCode, pureAccessCode)

	// 3. 最终检查
	if err != nil {
		return nil, "", busCodeStorageQuerySubscribeShareError.WithError(err)
	}

	if result == nil || result.ShareId == 0 {
		ctx.Error("所有尝试均失败，无法获取ShareId", checkShareResultLogFields("final", result)...)

		return nil, "", busCodeStorageQuerySubscribeShareError.WithError(fmt.Errorf("无法获取有效的分享ID(ShareId=0)，请确认分享链接是否有效"))
	}

	resolvedAccessCode := pureAccessCode
	if result.AccessCode != "" {
		resolvedAccessCode = result.AccessCode
	}

	return datatypes.JSONMap{
		consts.FileAdditionKeyShareId:    result.ShareId,
		consts.FileAdditionKeyIsFolder:   result.IsFolder,
		consts.FileAdditionKeyShareMode:  result.ShareMode,
		consts.FileAdditionKeyAccessCode: resolvedAccessCode,
	}, result.FileId, nil
}

func checkShareResultLogFields(prefix string, result *cloudbridgeSvi.CheckShareResult) []zap.Field {
	if result == nil {
		return []zap.Field{zap.Bool(prefix+"_result_nil", true)}
	}

	return []zap.Field{
		zap.Bool(prefix+"_result_nil", false),
		zap.Int64(prefix+"_share_id", result.ShareId),
		zap.Bool(prefix+"_is_folder", result.IsFolder),
		zap.Int(prefix+"_share_mode", result.ShareMode),
		zap.Bool(prefix+"_has_access_code", result.AccessCode != ""),
	}
}

func (h *handler) executeOsTypePersonal(ctx context.Context, req *addRequest, userID int64, isAdmin bool) httpcontext.BusinessError {
	if req.OsType != models.OsTypePersonFolder {
		return busCodeStorageOsTypeNotMatch
	}

	req.FileId = strings.TrimSpace(req.FileId)
	if req.FileId == "" {
		return busCodeStoragePersonParamsIncomplete
	}

	if req.CloudToken == 0 {
		return busCodeStorageCloudTokenEmpty
	}

	if busErr := h.missingCloudTokenBusinessError(busCodeStorageQueryCloudTokenError); busErr != nil {
		return busErr
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

	if busErr := h.missingCloudBridgeBusinessError(busCodeStoragePersonFileQueryError); busErr != nil {
		return busErr
	}

	if _, err = h.cloudBridgeService.CheckPerson(ctx, cloudbridgeSvi.NewAuthToken(token.AccessToken, token.AuthExpiresAtMillis(time.Now())), req.FileId); err != nil {
		return busCodeStoragePersonFileQueryError.WithError(err)
	}

	return nil
}

func (h *handler) executeOsTypeFamily(ctx context.Context, req *addRequest, userID int64, isAdmin bool) (datatypes.JSONMap, httpcontext.BusinessError) {
	if req.OsType != models.OsTypeFamilyFolder {
		return nil, busCodeStorageOsTypeNotMatch
	}

	req.FileId = strings.TrimSpace(req.FileId)

	req.FamilyId = strings.TrimSpace(req.FamilyId)
	if req.FileId == "" || req.FamilyId == "" {
		return nil, busCodeStorageFamilyParamsIncomplete
	}

	if req.CloudToken == 0 {
		return nil, busCodeStorageCloudTokenEmpty
	}

	if busErr := h.missingCloudTokenBusinessError(busCodeStorageQueryCloudTokenError); busErr != nil {
		return nil, busErr
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

	if busErr := h.missingCloudBridgeBusinessError(busCodeStorageFamilyFileQueryError); busErr != nil {
		return nil, busErr
	}

	if err = h.cloudBridgeService.CheckFamily(ctx, cloudbridgeSvi.NewAuthToken(token.AccessToken, token.AuthExpiresAtMillis(time.Now())), req.FamilyId, req.FileId); err != nil {
		return nil, busCodeStorageFamilyFileQueryError.WithError(err)
	}

	return datatypes.JSONMap{
		consts.FileAdditionKeyFamilyId: req.FamilyId,
	}, nil
}
