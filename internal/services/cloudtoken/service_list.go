package cloudtoken

import (
	"github.com/xxcheng123/cloudpan189-share/internal/framework/context"
	"github.com/xxcheng123/cloudpan189-share/internal/repository/models"
	"go.uber.org/zap"
	"gorm.io/gorm"
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
	query := s.getListQuery(ctx, req).Order("created_at desc")

	// 应用分页
	if !req.NoPaginate {
		if req.CurrentPage <= 0 {
			req.CurrentPage = 1
		}

		if req.PageSize <= 0 {
			req.PageSize = 10
		}

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
	if err = s.getListQuery(ctx, req).Count(&count).Error; err != nil {
		ctx.Error("查询云盘令牌数量失败", zap.Error(err))

		return 0, err
	}

	return count, nil
}

func (s *service) getListQuery(ctx context.Context, req *ListRequest) *gorm.DB {
	query := s.getDB(ctx)
	if req.Name != "" {
		query = query.Where("name LIKE ?", "%"+req.Name+"%")
	}

	if len(req.IdList) > 0 {
		query = query.Where("id IN ?", req.IdList)
	}

	// 非管理员只能查看自己的令牌
	if !req.IsAdmin && req.UserID > 0 {
		query = query.Where("user_id = ?", req.UserID)
	}

	return query
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
	// 首先查询现有的令牌
	var token models.CloudToken
	if err := s.getDB(ctx).Where("id = ?", id).First(&token).Error; err != nil {
		ctx.Error("查询令牌失败", zap.Error(err), zap.Int64("id", id))
		return err
	}

	// 更新addition字段
	token.Addition = addition

	// 保存更新
	if err := s.getDB(ctx).Save(&token).Error; err != nil {
		ctx.Error("更新令牌附加信息失败", zap.Error(err), zap.Int64("id", id))
		return err
	}
	return nil
}
