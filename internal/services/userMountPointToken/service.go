package userMountPointToken

import (
	"errors"
	"strings"

	mysqlDriver "github.com/go-sql-driver/mysql"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/xxcheng123/cloudpan189-share/internal/bootstrap"
	"github.com/xxcheng123/cloudpan189-share/internal/framework/context"
	"github.com/xxcheng123/cloudpan189-share/internal/repository/models"
	"go.uber.org/zap"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Service interface {
	BindToken(ctx context.Context, userID, mountPointID, tokenID int64) error
	UnbindToken(ctx context.Context, userID, mountPointID int64) error
	UnbindTokenWithResult(ctx context.Context, userID, mountPointID int64) (bool, error)
	GetTokenID(ctx context.Context, userID, mountPointID int64) (int64, error)
	GetUserTokens(ctx context.Context, userID int64, mountPointIDs []int64) (map[int64]int64, error)
	GetUserMountPointIDs(ctx context.Context, userID int64) ([]int64, error)
	GetAnyUserTokens(ctx context.Context, mountPointIDs []int64) (map[int64]int64, error)
	CountByToken(ctx context.Context, userID, tokenID int64) (int64, error)
	DeleteByMountPoint(ctx context.Context, mountPointID int64) error
	DeleteByToken(ctx context.Context, tokenID int64) error
}

type service struct {
	svc bootstrap.ServiceContext
}

var errInvalidBindTokenID = errors.New("userID、mountPointID 必须大于 0，tokenID 不能小于 0")
var errInvalidUnbindTokenID = errors.New("userID、mountPointID 必须全部大于 0")
var errInvalidUserID = errors.New("userID 必须大于 0")
var errInvalidMountPointID = errors.New("mountPointID 必须大于 0")
var errInvalidTokenID = errors.New("tokenID 必须大于 0")

func NewService(svc bootstrap.ServiceContext) Service {
	return &service{
		svc: svc,
	}
}

func (s *service) getDB(ctx context.Context) *gorm.DB {
	return s.svc.GetDB(ctx).Model(new(models.UserMountPointToken))
}

func (s *service) BindToken(ctx context.Context, userID, mountPointID, tokenID int64) error {
	if userID <= 0 || mountPointID <= 0 || tokenID < 0 {
		return errInvalidBindTokenID
	}

	err := s.svc.GetDB(ctx).Transaction(func(tx *gorm.DB) error {
		bindingDB := tx.Model(new(models.UserMountPointToken))

		var mountPoint models.MountPoint
		if err := tx.Select("id").Take(&mountPoint, "id = ?", mountPointID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}

			ctx.Error("校验挂载点存在失败", zap.Error(err), zap.Int64("mount_point_id", mountPointID))

			return err
		}

		if tokenID == 0 {
			if err := bindingDB.
				Where("user_id = ? AND mount_point_id = ?", userID, mountPointID).
				Delete(new(models.UserMountPointToken)).Error; err != nil {
				ctx.Error("删除用户挂载点旧令牌绑定失败", zap.Error(err))

				return err
			}

			return nil
		}

		var token models.CloudToken
		if err := tx.Select("id").Take(&token, "id = ?", tokenID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}

			ctx.Error("校验云盘令牌存在失败", zap.Error(err), zap.Int64("token_id", tokenID))

			return err
		}

		if err := bindingDB.Clauses(clause.OnConflict{
			Columns: []clause.Column{
				{Name: "user_id"},
				{Name: "mount_point_id"},
			},
			DoUpdates: clause.AssignmentColumns([]string{"token_id", "updated_at"}),
		}).Create(&models.UserMountPointToken{
			UserID:       userID,
			MountPointID: mountPointID,
			TokenID:      tokenID,
		}).Error; err != nil {
			ctx.Error("绑定用户挂载点令牌失败", zap.Error(err))

			return err
		}

		return nil
	})
	if err != nil {
		return err
	}

	return nil
}

func (s *service) UnbindToken(ctx context.Context, userID, mountPointID int64) error {
	_, err := s.UnbindTokenWithResult(ctx, userID, mountPointID)

	return err
}

func (s *service) UnbindTokenWithResult(ctx context.Context, userID, mountPointID int64) (bool, error) {
	if userID <= 0 || mountPointID <= 0 {
		return false, errInvalidUnbindTokenID
	}

	result := s.getDB(ctx).
		Where("user_id = ? AND mount_point_id = ?", userID, mountPointID).
		Delete(new(models.UserMountPointToken))
	if result.Error != nil {
		return false, result.Error
	}

	return result.RowsAffected > 0, nil
}

func (s *service) GetTokenID(ctx context.Context, userID, mountPointID int64) (int64, error) {
	if userID <= 0 || mountPointID <= 0 {
		return 0, errInvalidUnbindTokenID
	}

	var binding models.UserMountPointToken

	err := s.getDB(ctx).Where("user_id = ? AND mount_point_id = ?", userID, mountPointID).First(&binding).Error
	if errors.Is(err, gorm.ErrRecordNotFound) || isMissingTableError(err) {
		return 0, nil
	}

	return binding.TokenID, err
}

func (s *service) GetUserTokens(ctx context.Context, userID int64, mountPointIDs []int64) (map[int64]int64, error) {
	if userID <= 0 {
		return nil, errInvalidUserID
	}

	if len(mountPointIDs) == 0 {
		return make(map[int64]int64), nil
	}

	normalizedMountPointIDs, err := normalizeMountPointIDs(mountPointIDs)
	if err != nil {
		return nil, err
	}

	var bindings []models.UserMountPointToken

	err = s.getDB(ctx).Where("user_id = ? AND mount_point_id IN ?", userID, normalizedMountPointIDs).Find(&bindings).Error
	if err != nil {
		if isMissingTableError(err) {
			return make(map[int64]int64), nil
		}

		ctx.Error("查询用户挂载点令牌绑定列表失败", zap.Error(err))

		return nil, err
	}

	result := make(map[int64]int64)
	for _, b := range bindings {
		result[b.MountPointID] = b.TokenID
	}

	return result, nil
}

func (s *service) GetUserMountPointIDs(ctx context.Context, userID int64) ([]int64, error) {
	if userID <= 0 {
		return nil, errInvalidUserID
	}

	var bindings []models.UserMountPointToken

	err := s.getDB(ctx).Where("user_id = ?", userID).Find(&bindings).Error
	if err != nil {
		if isMissingTableError(err) {
			return []int64{}, nil
		}

		ctx.Error("查询用户挂载点绑定失败", zap.Error(err))

		return nil, err
	}

	ids := make([]int64, 0, len(bindings))
	for _, binding := range bindings {
		ids = append(ids, binding.MountPointID)
	}

	return ids, nil
}

func (s *service) GetAnyUserTokens(ctx context.Context, mountPointIDs []int64) (map[int64]int64, error) {
	if len(mountPointIDs) == 0 {
		return make(map[int64]int64), nil
	}

	normalizedMountPointIDs, err := normalizeMountPointIDs(mountPointIDs)
	if err != nil {
		return nil, err
	}

	var bindings []models.UserMountPointToken

	err = s.getDB(ctx).Where("mount_point_id IN ?", normalizedMountPointIDs).Order("updated_at DESC, id DESC").Find(&bindings).Error
	if err != nil {
		if isMissingTableError(err) {
			return make(map[int64]int64), nil
		}

		ctx.Error("查询挂载点已绑定令牌失败", zap.Error(err))

		return nil, err
	}

	result := make(map[int64]int64)
	for _, binding := range bindings {
		if _, ok := result[binding.MountPointID]; ok {
			continue
		}

		result[binding.MountPointID] = binding.TokenID
	}

	return result, nil
}

func (s *service) CountByToken(ctx context.Context, userID, tokenID int64) (int64, error) {
	if userID <= 0 {
		return 0, errInvalidUserID
	}

	if tokenID <= 0 {
		return 0, errInvalidTokenID
	}

	var count int64

	err := s.getDB(ctx).Where("user_id = ? AND token_id = ?", userID, tokenID).Count(&count).Error
	if err != nil {
		if isMissingTableError(err) {
			return 0, nil
		}

		ctx.Error("查询用户挂载点令牌绑定数量失败", zap.Error(err))

		return 0, err
	}

	return count, nil
}

func normalizeMountPointIDs(mountPointIDs []int64) ([]int64, error) {
	seen := make(map[int64]struct{}, len(mountPointIDs))
	normalized := make([]int64, 0, len(mountPointIDs))

	for _, mountPointID := range mountPointIDs {
		if mountPointID <= 0 {
			return nil, errInvalidMountPointID
		}

		if _, ok := seen[mountPointID]; ok {
			continue
		}

		seen[mountPointID] = struct{}{}
		normalized = append(normalized, mountPointID)
	}

	return normalized, nil
}

func (s *service) DeleteByMountPoint(ctx context.Context, mountPointID int64) error {
	if mountPointID <= 0 {
		return errInvalidMountPointID
	}

	return s.getDB(ctx).
		Where("mount_point_id = ?", mountPointID).
		Delete(new(models.UserMountPointToken)).Error
}

func (s *service) DeleteByToken(ctx context.Context, tokenID int64) error {
	if tokenID <= 0 {
		return errInvalidTokenID
	}

	return s.getDB(ctx).
		Where("token_id = ?", tokenID).
		Delete(new(models.UserMountPointToken)).Error
}

func isMissingTableError(err error) bool {
	if err == nil {
		return false
	}

	var mysqlErr *mysqlDriver.MySQLError
	if errors.As(err, &mysqlErr) && mysqlErr.Number == 1146 {
		return true
	}

	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "42P01" {
		return true
	}

	msg := strings.ToLower(err.Error())

	return strings.Contains(msg, "no such table: user_mount_point_tokens") ||
		(strings.Contains(msg, "user_mount_point_tokens") &&
			(strings.Contains(msg, "does not exist") || strings.Contains(msg, "doesn't exist")))
}
