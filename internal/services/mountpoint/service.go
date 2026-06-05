package mountpoint

import (
	"regexp"
	"strings"
	"time"

	"github.com/xxcheng123/cloudpan189-share/internal/bootstrap"
	"github.com/xxcheng123/cloudpan189-share/internal/framework/context"
	"github.com/xxcheng123/cloudpan189-share/internal/pkgs/utils"
	"github.com/xxcheng123/cloudpan189-share/internal/repository/models"

	cloudbridgeSvi "github.com/xxcheng123/cloudpan189-share/internal/services/cloudbridge"
	cloudtokenSvi "github.com/xxcheng123/cloudpan189-share/internal/services/cloudtoken"
	userMountPointTokenSvi "github.com/xxcheng123/cloudpan189-share/internal/services/userMountPointToken"
	"github.com/xxcheng123/cloudpan189-share/internal/types/topic"

	"gorm.io/gorm"
)

type Service interface {
	Create(ctx context.Context, req *CreateRequest) (int64, error)
	Query(ctx context.Context, fileId int64) (*models.MountPoint, error)
	QueryByID(ctx context.Context, id int64) (*models.MountPoint, error)
	QueryByPath(ctx context.Context, fullPath string) (*models.MountPoint, error)
	GetAccessibleMountPointIDs(ctx context.Context, userID int64, isAdmin bool, groupFileIds []int64) ([]int64, error)
	List(ctx context.Context, req *ListRequest) ([]*models.MountPoint, error)
	Count(ctx context.Context, req *ListRequest) (int64, error)
	Delete(ctx context.Context, req *DeleteRequest) error
	BatchDelete(ctx context.Context, req *BatchDeleteRequest) error
	ClearAll(ctx context.Context) (int64, error)
	EnableAutoRefresh(ctx context.Context, fileId int64, enable bool) error
	GetAutoRefreshList(ctx context.Context, req *GetAutoRefreshListRequest) ([]*models.MountPoint, error)
	UpdateRefreshConfig(ctx context.Context, fileId int64, config RefreshConfig) error
	ModifyToken(ctx context.Context, req *ModifyTokenRequest) error
	BatchParseText(ctx context.Context, req *topic.BatchParseTextRequest) ([]*topic.BatchParseItem, error)
	UpdateRefreshTime(ctx context.Context, fileId int64) error
	// UpdateLastState 写入挂载点最近一次操作的结果摘要（通常为"成功"或简短错误信息）。
	UpdateLastState(ctx context.Context, fileId int64, state string) error
}

type service struct {
	svc                        bootstrap.ServiceContext
	cloudTokenService          cloudtokenSvi.Service
	cloudBridgeService         cloudbridgeSvi.Service
	userMountPointTokenService userMountPointTokenSvi.Service
}

func NewService(
	svc bootstrap.ServiceContext,
	cloudTokenService cloudtokenSvi.Service,
	cloudBridgeService cloudbridgeSvi.Service,
	userMountPointTokenService userMountPointTokenSvi.Service,
) Service {
	return &service{
		svc:                        svc,
		cloudTokenService:          cloudTokenService,
		cloudBridgeService:         cloudBridgeService,
		userMountPointTokenService: userMountPointTokenService,
	}
}

// UpdateRefreshTime 更新挂载点的刷新时间
func (s *service) UpdateRefreshTime(ctx context.Context, fileId int64) error {
	if fileId <= 0 {
		return errInvalidMountPointFileID
	}

	result := s.getDB(ctx).Where("file_id = ?", fileId).Update("updated_at", time.Now())
	if result.Error != nil {
		return result.Error
	}

	if result.RowsAffected == 0 {
		return s.ensureMountPointFileExists(ctx, fileId)
	}

	return nil
}

// UpdateLastState 写入挂载点最近一次同步结果。
// 超过 1024 字符会被截断以适配 last_state 字段的类型。
func (s *service) UpdateLastState(ctx context.Context, fileId int64, state string) error {
	if fileId <= 0 {
		return errInvalidMountPointFileID
	}

	if state == "" {
		state = "成功"
	}

	state = sanitizeMountPointLastState(state)
	if len([]rune(state)) > 512 {
		state = string([]rune(state)[:512]) + "..."
	}

	result := s.getDB(ctx).Where("file_id = ?", fileId).Update("last_state", state)
	if result.Error != nil {
		return result.Error
	}

	if result.RowsAffected == 0 {
		return s.ensureMountPointFileExists(ctx, fileId)
	}

	return nil
}

func (s *service) getDB(ctx context.Context) *gorm.DB {
	return s.svc.GetDB(ctx).Model(new(models.MountPoint))
}

var (
	mountPointLastStateURLPattern = regexp.MustCompile(`(?i)(?:[a-z][a-z0-9+.-]*://|/)[^\s"'<>]+`)
	reFolderID                    = regexp.MustCompile(`^\d+$`)
)

type batchParseCandidateKind int

const (
	batchParseCandidateShare batchParseCandidateKind = iota + 1
	batchParseCandidateSubscribe
	batchParseCandidatePersonFolder
)

type batchParseCandidate struct {
	kind        batchParseCandidateKind
	shareCode   string
	accessCode  string
	fileID      string
	subscribeID string
}

func sanitizeMountPointLastState(state string) string {
	state = mountPointLastStateURLPattern.ReplaceAllStringFunc(state, utils.RedactURLForLog)

	return utils.RedactSensitiveText(state)
}

// 实现 BatchParseText
func (s *service) BatchParseText(ctx context.Context, req *topic.BatchParseTextRequest) ([]*topic.BatchParseItem, error) {
	if req == nil {
		return nil, errInvalidMountPointParseRequest
	}

	req.Content = strings.TrimSpace(req.Content)
	if req.Content == "" {
		return nil, errInvalidMountPointParseRequest
	}

	var candidates []*batchParseCandidate

	requiresCloudToken := false

	lines := strings.Split(req.Content, "\n")
	for _, line := range lines {
		candidate := parseBatchTextLine(line)
		if candidate == nil {
			continue
		}

		candidates = append(candidates, candidate)
		if candidate.kind == batchParseCandidatePersonFolder {
			requiresCloudToken = true
		}
	}

	var authToken cloudbridgeSvi.AuthToken

	if requiresCloudToken {
		var err error

		authToken, err = s.batchParseAuthToken(ctx, req)
		if err != nil {
			return nil, err
		}
	}

	var results []*topic.BatchParseItem

	for _, candidate := range candidates {
		switch candidate.kind {
		case batchParseCandidateShare:
			results = append(results, &topic.BatchParseItem{
				Name:            s.batchParseShareName(ctx, candidate),
				OsType:          models.OsTypeShareFolder,
				ShareCode:       candidate.shareCode,
				ShareAccessCode: candidate.accessCode,
			})
		case batchParseCandidateSubscribe:
			// 使用 subscribe 类型，只需 SubscribeUser，不需要 ShareCode
			results = append(results, &topic.BatchParseItem{
				Name:          s.batchParseSubscribeName(ctx, candidate),
				OsType:        models.OsTypeSubscribe,
				SubscribeUser: candidate.subscribeID,
			})
		case batchParseCandidatePersonFolder:
			results = append(results, &topic.BatchParseItem{
				Name:   s.batchParsePersonName(ctx, authToken, candidate),
				OsType: models.OsTypePersonFolder,
				FileId: candidate.fileID,
			})
		}
	}

	return results, nil
}

func (s *service) batchParseShareName(ctx context.Context, candidate *batchParseCandidate) string {
	if isNilDependency(s.cloudBridgeService) {
		return "未知分享_" + candidate.shareCode
	}

	info, err := s.cloudBridgeService.GetShareInfo(ctx, candidate.shareCode, candidate.accessCode)
	if err == nil && info != nil && info.Name != "" {
		return info.Name
	}

	return "未知分享_" + candidate.shareCode
}

func (s *service) batchParseSubscribeName(ctx context.Context, candidate *batchParseCandidate) string {
	if isNilDependency(s.cloudBridgeService) {
		return "未知订阅号_" + candidate.subscribeID
	}

	userInfo, err := s.cloudBridgeService.GetSubscribeUserInfo(ctx, candidate.subscribeID)
	if err == nil && userInfo != nil && userInfo.Name != "" {
		return userInfo.Name
	}

	return "未知订阅号_" + candidate.subscribeID
}

func (s *service) batchParsePersonName(ctx context.Context, authToken cloudbridgeSvi.AuthToken, candidate *batchParseCandidate) string {
	if isNilDependency(s.cloudBridgeService) {
		return "未知文件夹_" + candidate.fileID
	}

	name, err := s.cloudBridgeService.CheckPerson(ctx, authToken, candidate.fileID)
	if err == nil && name != "" {
		return name
	}

	return "未知文件夹_" + candidate.fileID
}

func (s *service) batchParseAuthToken(ctx context.Context, req *topic.BatchParseTextRequest) (cloudbridgeSvi.AuthToken, error) {
	var emptyToken cloudbridgeSvi.AuthToken

	if req.CloudToken <= 0 {
		return emptyToken, errInvalidMountPointCloudTokenID
	}

	if isNilDependency(s.cloudTokenService) {
		return emptyToken, errMountPointCloudTokenServiceUnavailable
	}

	tokenInfo, err := s.cloudTokenService.QueryAccessible(ctx, req.CloudToken, req.UserID, req.IsAdmin)
	if err != nil {
		return emptyToken, err
	}

	if tokenInfo == nil {
		return emptyToken, gorm.ErrRecordNotFound
	}

	return cloudbridgeSvi.NewAuthToken(tokenInfo.AccessToken, tokenInfo.AuthExpiresAtMillis(time.Now())), nil
}

func parseBatchTextLine(line string) *batchParseCandidate {
	line = strings.TrimSpace(line)
	if line == "" {
		return nil
	}

	// 预处理：统一中文符号。
	cleanLine := strings.ReplaceAll(line, "（", "(")
	cleanLine = strings.ReplaceAll(cleanLine, "）", ")")
	cleanLine = strings.ReplaceAll(cleanLine, "：", ":")

	if parsedSubscribeID := utils.ParseCloud189SubscribeUserID(cleanLine); parsedSubscribeID != "" {
		return &batchParseCandidate{
			kind:        batchParseCandidateSubscribe,
			subscribeID: parsedSubscribeID,
		}
	}

	if parsedShareCode, parsedAccessCode := utils.ParseCloud189ShareCode(cleanLine, ""); isCloud189ShareLink(cleanLine) &&
		isValidCloud189ShareParams(parsedShareCode, parsedAccessCode) {
		return &batchParseCandidate{
			kind:       batchParseCandidateShare,
			shareCode:  parsedShareCode,
			accessCode: parsedAccessCode,
		}
	}

	if utils.ContainsURLLike(cleanLine) {
		if parsedShareCode, parsedAccessCode := utils.ParseCloud189ShareCode(cleanLine, ""); isValidCloud189ShareParams(parsedShareCode, parsedAccessCode) {
			return &batchParseCandidate{
				kind:       batchParseCandidateShare,
				shareCode:  parsedShareCode,
				accessCode: parsedAccessCode,
			}
		}

		return nil
	}

	if reFolderID.MatchString(line) {
		return &batchParseCandidate{
			kind:   batchParseCandidatePersonFolder,
			fileID: line,
		}
	}

	parts := strings.Fields(line)
	if len(parts) > 0 && reFolderID.MatchString(parts[0]) {
		return &batchParseCandidate{
			kind:   batchParseCandidatePersonFolder,
			fileID: parts[0],
		}
	}

	if parsedShareCode, parsedAccessCode := utils.ParseCloud189ShareCode(cleanLine, ""); isValidCloud189ShareParams(parsedShareCode, parsedAccessCode) {
		return &batchParseCandidate{
			kind:       batchParseCandidateShare,
			shareCode:  parsedShareCode,
			accessCode: parsedAccessCode,
		}
	}

	return nil
}

func isCloud189ShareLink(value string) bool {
	return utils.IsCloud189ShareLink(value)
}

func isValidCloud189ShareParams(shareCode string, accessCode string) bool {
	return utils.IsCloud189ShareCode(shareCode) && (accessCode == "" || utils.IsCloud189AccessCode(accessCode))
}
