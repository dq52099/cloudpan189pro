package storagefacade

import (
	"bytes"
	"encoding/json"
	"strings"
	"time"

	mysqlDriver "github.com/go-sql-driver/mysql"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/pkg/errors"
	"github.com/xxcheng123/cloudpan189-share/internal/bootstrap"
	"github.com/xxcheng123/cloudpan189-share/internal/consts"
	"github.com/xxcheng123/cloudpan189-share/internal/framework/context"
	"github.com/xxcheng123/cloudpan189-share/internal/pkgs/datatypes"
	"github.com/xxcheng123/cloudpan189-share/internal/pkgs/utils"
	"github.com/xxcheng123/cloudpan189-share/internal/repository/models"
	mountPointSvi "github.com/xxcheng123/cloudpan189-share/internal/services/mountpoint"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type CreateStorageRequest struct {
	LocalPath  string            // 逻辑路径，例如 /foo/bar
	OsType     string            // 协议类型
	CloudToken int64             // 云 token ID
	FileId     string            // 云文件/资源 ID（用于 Addition.CloudId 映射）
	Addition   datatypes.JSONMap // 额外元数据，由上层根据协议类型准备

	EnableAutoRefresh bool `json:"enableAutoRefresh"`
	AutoRefreshDays   int  `json:"autoRefreshDays"`
	RefreshInterval   int  `json:"refreshInterval"`
	EnableDeepRefresh bool `json:"enableDeepRefresh"`

	CreatorUserID int64 // 创建者用户ID
	IsAdmin       bool  // 创建者是否管理员

	// AllowExisting 为 true 时，若目标路径已有同一资源的挂载点，直接返回其根 VirtualFile ID；
	// 若只有同路径虚拟文件而没有挂载点，仍返回 ErrPathAlreadyExists，避免调用方误判为挂载成功。
	AllowExisting bool
}

// ErrPathAlreadyExists 路径已存在且未开启 AllowExisting 时返回。
var ErrPathAlreadyExists = errors.New("路径已被挂载，无法重复创建")

// ErrExistingPathForbidden 路径已存在但不属于当前用户时返回。
var ErrExistingPathForbidden = errors.New("路径已被其他用户挂载")

var (
	errRequestNil         = errors.New("请求对象为空")
	errInvalidPath        = errors.New("路径不合法，需要 / 开头的路径")
	errRootPathNotAllowed = errors.New("不允许挂载根路径")
	errInvalidCloudToken  = errors.New("云盘令牌不合法")
	errInvalidCreatorUser = errors.New("创建者用户 ID 必须大于 0")
)

// CreateStorage 在一个统一流程中创建 VirtualFile 顶层节点与 MountPoint 记录，返回根 VirtualFile ID。
func (s *service) CreateStorage(ctx context.Context, req *CreateStorageRequest) (int64, error) {
	// 基本校验
	if req == nil {
		return 0, errRequestNil
	}

	normalizedPath, paths, err := utils.NormalizeStoragePathParts(req.LocalPath)
	if err != nil {
		return 0, errInvalidPath
	}

	req = cloneCreateStorageRequest(req)
	req.LocalPath = normalizedPath

	if !req.IsAdmin && req.CreatorUserID <= 0 {
		return 0, errInvalidCreatorUser
	}

	if err := s.validateCloudTokenAccess(ctx, req); err != nil {
		return 0, err
	}

	if len(paths) == 0 {
		return 0, errRootPathNotAllowed
	}

	if bootstrap.HasTransactionDB(ctx) {
		return s.createStorageInTransaction(ctx, req, paths)
	}

	var (
		id    int64
		txCtx context.Context
	)

	err = s.svc.GetDB(ctx).Transaction(func(tx *gorm.DB) error {
		txCtx = bootstrap.WithTransactionDB(ctx, tx)

		createdID, txErr := s.createStorageInTransaction(txCtx, req, paths)
		if txErr != nil {
			return txErr
		}

		id = createdID

		return nil
	})
	if err != nil {
		return 0, err
	}

	bootstrap.RunAfterCommitHooks(txCtx)

	return id, nil
}

func cloneCreateStorageRequest(req *CreateStorageRequest) *CreateStorageRequest {
	cloned := *req
	cloned.FileId = strings.TrimSpace(cloned.FileId)

	return &cloned
}

func (s *service) createStorageInTransaction(ctx context.Context, req *CreateStorageRequest, paths []string) (int64, error) {
	// 防重：MountPoint 与 VirtualFile
	if mp, err := s.mountPointService.QueryByPath(ctx, req.LocalPath); err != nil {
		ctx.Error("查询挂载点路径失败", zap.Error(err), zap.String("path", req.LocalPath))

		return 0, err
	} else if mp != nil {
		if !req.AllowExisting {
			ctx.Warn("挂载点路径已存在", zap.String("path", req.LocalPath), zap.Int64("exists_id", mp.ID))

			return 0, ErrPathAlreadyExists
		}

		if req.CreatorUserID > 0 && !req.IsAdmin && mp.CreatorUserID != req.CreatorUserID {
			ctx.Warn(
				"挂载点路径已存在但不属于当前用户",
				zap.String("path", req.LocalPath),
				zap.Int64("exists_id", mp.ID),
				zap.Int64("creator_user_id", mp.CreatorUserID),
				zap.Int64("current_user_id", req.CreatorUserID),
			)

			return 0, ErrExistingPathForbidden
		}

		if mp.TokenId != req.CloudToken {
			ctx.Warn("挂载点路径已存在但令牌上下文不一致",
				zap.String("path", req.LocalPath),
				zap.Int64("exists_id", mp.ID),
				zap.Int64("existing_token_id", mp.TokenId),
				zap.Int64("request_token_id", req.CloudToken),
			)

			return 0, ErrPathAlreadyExists
		}

		existingFile, err := s.virtualFileService.Query(ctx, mp.FileId)
		if err != nil {
			ctx.Error("查询已存在挂载点根文件失败", zap.Error(err), zap.String("path", req.LocalPath), zap.Int64("file_id", mp.FileId))

			return 0, err
		}

		if existingFile == nil {
			ctx.Warn("挂载点路径已存在但根文件为空", zap.String("path", req.LocalPath), zap.Int64("exists_id", mp.ID), zap.Int64("file_id", mp.FileId))

			return 0, gorm.ErrRecordNotFound
		}

		if !sameStorageResource(existingFile, req) {
			ctx.Warn("挂载点路径已存在但资源不一致",
				zap.String("path", req.LocalPath),
				zap.Int64("exists_id", mp.ID),
				zap.String("existing_os_type", string(existingFile.OsType)),
				zap.String("request_os_type", req.OsType),
				zap.String("existing_cloud_id", existingFile.CloudId),
				zap.String("request_cloud_id", req.FileId),
			)

			return 0, ErrPathAlreadyExists
		}

		// 路径已存在，返回已存在的挂载点ID而不是报错
		ctx.Info("挂载点路径已存在，返回已存在的记录", zap.String("path", req.LocalPath), zap.Int64("exists_id", mp.ID))

		return mp.FileId, nil
	}

	if vf, err := s.virtualFileService.QueryByPath(ctx, req.LocalPath); !errors.Is(err, gorm.ErrRecordNotFound) {
		if err != nil {
			ctx.Error("查询虚拟文件路径失败", zap.Error(err), zap.String("path", req.LocalPath))

			return 0, err
		}

		ctx.Warn("虚拟文件路径已存在但不是挂载点", zap.String("path", req.LocalPath), zap.Int64("exists_id", vf.ID))

		return 0, ErrPathAlreadyExists
	}

	parentId, err := s.virtualFileService.FindOrCreateAncestors(ctx, req.LocalPath)
	if err != nil {
		ctx.Error("创建父级路径失败", zap.Error(err), zap.String("path", req.LocalPath))

		return 0, err
	}

	// 组装 VirtualFile 顶层节点
	now := time.Now()
	vf := &models.VirtualFile{
		Name:       paths[len(paths)-1],
		IsTop:      true,
		Size:       0,
		Hash:       "",
		CreateDate: now,
		ModifyDate: now,
		Rev:        now.Format(consts.RevFormat),
		OsType:     req.OsType,
		IsDir:      true,
		Addition:   req.Addition,
		CloudId:    req.FileId,
	}

	id, err := s.virtualFileService.CreateTop(ctx, parentId, vf)
	if err != nil {
		ctx.Error("创建顶层虚拟文件失败", zap.Error(err), zap.Int64("parentId", parentId))

		if isUniqueConstraintError(err) {
			return 0, ErrPathAlreadyExists
		}

		return 0, err
	}

	// 创建 MountPoint。调用方保证本函数运行在事务内，失败时会回滚前面创建的祖先与顶层虚拟文件。
	if _, err = s.mountPointService.Create(ctx, &mountPointSvi.CreateRequest{
		FullPath: req.LocalPath,
		FileId:   id,
		OsType:   req.OsType,
		TokenId:  req.CloudToken,

		EnableAutoRefresh: req.EnableAutoRefresh,
		AutoRefreshDays:   req.AutoRefreshDays,
		RefreshInterval:   req.RefreshInterval,
		EnableDeepRefresh: req.EnableDeepRefresh,
		CreatorUserID:     req.CreatorUserID,
	}); err != nil {
		ctx.Error("创建挂载点失败，回滚存储创建事务", zap.Error(err), zap.Int64("virtualFileId", id), zap.String("path", req.LocalPath))

		return 0, err
	}

	return id, nil
}

func sameStorageResource(existingFile *models.VirtualFile, req *CreateStorageRequest) bool {
	if existingFile == nil || req == nil {
		return false
	}

	if string(existingFile.OsType) != req.OsType {
		return false
	}

	existingCloudID := normalizeComparableCloudID(existingFile.CloudId)

	requestCloudID := normalizeComparableCloudID(req.FileId)
	if (existingCloudID != "" || requestCloudID != "") && existingCloudID != requestCloudID {
		return false
	}

	return jsonMapEqual(existingFile.Addition, req.Addition)
}

func normalizeComparableCloudID(cloudID string) string {
	cloudID = strings.TrimSpace(cloudID)
	if cloudID == "0" {
		return ""
	}

	return cloudID
}

func jsonMapEqual(left datatypes.JSONMap, right datatypes.JSONMap) bool {
	leftBytes, err := canonicalJSONMapBytes(left)
	if err != nil {
		return false
	}

	rightBytes, err := canonicalJSONMapBytes(right)
	if err != nil {
		return false
	}

	return bytes.Equal(leftBytes, rightBytes)
}

func canonicalJSONMapBytes(value datatypes.JSONMap) ([]byte, error) {
	if len(value) == 0 {
		return []byte("{}"), nil
	}

	return json.Marshal(map[string]interface{}(value))
}

// isUniqueConstraintError 判断错误是否为唯一约束冲突，覆盖 SQLite / MySQL / PostgreSQL。
func isUniqueConstraintError(err error) bool {
	if err == nil {
		return false
	}

	var mysqlErr *mysqlDriver.MySQLError
	if errors.As(err, &mysqlErr) && mysqlErr.Number == 1062 {
		return true
	}

	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return true
	}

	type sqliteCodeError interface {
		Code() int
	}

	var sqliteErr sqliteCodeError
	if errors.As(err, &sqliteErr) {
		switch sqliteErr.Code() {
		case 1555, 2067: // SQLITE_CONSTRAINT_PRIMARYKEY / SQLITE_CONSTRAINT_UNIQUE
			return true
		}
	}

	msg := err.Error()

	return strings.Contains(msg, "UNIQUE constraint failed") ||
		strings.Contains(msg, "Duplicate entry") ||
		strings.Contains(msg, "duplicate key value violates unique constraint")
}

func (s *service) validateCloudTokenAccess(ctx context.Context, req *CreateStorageRequest) error {
	if req.CloudToken < 0 {
		return errInvalidCloudToken
	}

	if req.CloudToken == 0 {
		return nil
	}

	if isNilDependency(s.cloudTokenService) {
		return errInvalidCloudToken
	}

	_, err := s.cloudTokenService.QueryAccessible(ctx, req.CloudToken, req.CreatorUserID, req.IsAdmin)

	return err
}
