package mountpoint

import (
	"time"

	"github.com/xxcheng123/cloudpan189-share/internal/framework/context"
	"github.com/xxcheng123/cloudpan189-share/internal/repository/models"
	"go.uber.org/zap"
)

// GetAutoRefreshListRequest 获取需要自动刷新的挂载点列表请求
type GetAutoRefreshListRequest struct {
	TokenId *int64 `form:"tokenId" binding:"omitempty"` // 可选的token过滤
}

// GetAutoRefreshList 获取需要自动刷新的挂载点列表
func (s *service) GetAutoRefreshList(ctx context.Context, req *GetAutoRefreshListRequest) ([]*models.MountPoint, error) {
	now := time.Now()

	query := s.getDB(ctx).Where("enable_auto_refresh = ?", true)

	if req.TokenId != nil {
		query = query.Where("token_id = ?", *req.TokenId)
	}

	query = query.Where("auto_refresh_begin_at IS NOT NULL").
		Where("auto_refresh_begin_at <= ?", now).
		Where("date(auto_refresh_begin_at, '+' || auto_refresh_days || ' days') >= ?", now.Format("2006-01-02"))

	list := make([]*models.MountPoint, 0)

	if err := query.Find(&list).Error; err != nil {
		ctx.Error("查询需要自动刷新的挂载点列表失败", zap.Error(err))
		return nil, err
	}

	ctx.Info("查询到需要自动刷新的挂载点", zap.Int("count", len(list)))

	return list, nil
}
