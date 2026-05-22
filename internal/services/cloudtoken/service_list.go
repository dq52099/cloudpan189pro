package cloudtoken

import (
	"github.com/xxcheng123/cloudpan189-share/internal/framework/context"
	"github.com/xxcheng123/cloudpan189-share/internal/repository/models"
	"go.uber.org/zap"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

const (
	defaultCloudTokenCurrentPage = 1
	defaultCloudTokenPageSize    = 10
	maxCloudTokenPageSize        = 500
)

type ListRequest struct {
	CurrentPage int     `form:"currentPage,omitempty,default=1" binding:"omitempty,min=1" example:"1"` // 当前页码，默认为1
	PageSize    int     `form:"pageSize,omitempty,default=10" binding:"omitempty,min=1" example:"10"`  // 每页大小，默认为10
	NoPaginate  bool    `form:"noPaginate" binding:"omitempty" example:"false"`                        // 是否不分页，默认false
	Name        string  `form:"name" binding:"omitempty" example:"名称模糊搜索"`                             // 名称模糊搜索
	IdList      []int64 `form:"-"`
	UserID      int64   `form:"-"` // 用户ID，用于权限过滤
	IsAdmin     bool    `form:"-"` // 是否管理员，管理员可查看所有
}

func (s *service) List(ctx context.Context, req *ListRequest) (list []*models.CloudToken, err error) {
	if req == nil {
		req = &ListRequest{}
	}

	if req.IdList != nil && len(req.IdList) == 0 {
		return []*models.CloudToken{}, nil
	}

	query, err := s.getListQuery(ctx, req)
	if err != nil {
		return nil, err
	}

	query = query.Order("created_at desc")

	// 应用分页
	if !req.NoPaginate {
		normalizeCloudTokenPagination(req)
		query = query.Offset((req.CurrentPage - 1) * req.PageSize).Limit(req.PageSize)
	}

	// 查询云盘令牌列表
	list = make([]*models.CloudToken, 0)
	if err = query.Find(&list).Error; err != nil {
		ctx.Error("查询云盘令牌列表失败", zap.Error(err))

		return nil, err
	}

	return list, nil
}

func (s *service) Count(ctx context.Context, req *ListRequest) (count int64, err error) {
	if req == nil {
		req = &ListRequest{}
	}

	if req.IdList != nil && len(req.IdList) == 0 {
		return 0, nil
	}

	query, err := s.getListQuery(ctx, req)
	if err != nil {
		return 0, err
	}

	if err = query.Count(&count).Error; err != nil {
		ctx.Error("查询云盘令牌数量失败", zap.Error(err))

		return 0, err
	}

	return count, nil
}

func (s *service) getListQuery(ctx context.Context, req *ListRequest) (*gorm.DB, error) {
	query := s.getDB(ctx)
	if req.Name != "" {
		query = query.Where("name LIKE ?", "%"+req.Name+"%")
	}

	if len(req.IdList) > 0 {
		ids, err := normalizeCloudTokenIDs(req.IdList)
		if err != nil {
			return nil, err
		}

		query = query.Where("id IN ?", ids)
	}

	// 非管理员只能查看自己的令牌
	if !req.IsAdmin {
		if req.UserID <= 0 {
			return nil, errInvalidCloudTokenUserID
		}

		query = query.Where("user_id = ?", req.UserID)
	}

	return query, nil
}

func normalizeCloudTokenPagination(req *ListRequest) {
	if req.CurrentPage <= 0 {
		req.CurrentPage = defaultCloudTokenCurrentPage
	}

	if req.PageSize <= 0 {
		req.PageSize = defaultCloudTokenPageSize
	}

	if req.PageSize > maxCloudTokenPageSize {
		req.PageSize = maxCloudTokenPageSize
	}
}

func normalizeCloudTokenIDs(ids []int64) ([]int64, error) {
	seen := make(map[int64]struct{}, len(ids))

	normalized := make([]int64, 0, len(ids))
	for _, id := range ids {
		if id <= 0 {
			return nil, errInvalidCloudTokenID
		}

		if _, ok := seen[id]; ok {
			continue
		}

		seen[id] = struct{}{}
		normalized = append(normalized, id)
	}

	return normalized, nil
}

// ListPasswordLoginTokens 查询所有使用密码登录的令牌
func (s *service) ListPasswordLoginTokens(ctx context.Context) ([]*models.CloudToken, error) {
	var tokens []*models.CloudToken

	// 查询所有使用密码登录的令牌（login_type = 2）
	query := s.getDB(ctx).Where("login_type = ?", models.LoginTypePassword)

	if err := query.Find(&tokens).Error; err != nil {
		ctx.Error("查询密码登录令牌失败", zap.Error(err))

		return nil, err
	}

	return tokens, nil
}

// UpdateAddition 更新令牌的附加信息
func (s *service) UpdateAddition(ctx context.Context, id int64, addition map[string]interface{}) error {
	if id <= 0 {
		return errInvalidCloudTokenID
	}

	result := s.getDB(ctx).Where("id = ?", id).Update("addition", datatypes.JSONMap(addition))
	if result.Error != nil {
		ctx.Error("更新令牌附加信息失败", zap.Error(result.Error), zap.Int64("id", id))

		return result.Error
	}

	return s.checkCloudTokenUpdateResult(ctx, result, id, 0, true, "令牌不存在")
}
