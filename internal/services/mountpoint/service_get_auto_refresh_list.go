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

	// 过滤已到期的挂载点
	query = query.Where("auto_refresh_begin_at IS NOT NULL").
		Where("auto_refresh_begin_at <= ?", now)

	// 在 Go 层过滤到期时间，避免 SQL 方言问题
	list := make([]*models.MountPoint, 0)

	if err := query.Find(&list).Error; err != nil {
		ctx.Error("查询需要自动刷新的挂载点列表失败", zap.Error(err))
		return nil, err
	}

	// 过滤掉已过期的挂载点
	filtered := make([]*models.MountPoint, 0)
	for _, mp := range list {
		if mp.AutoRefreshDays > 0 {
			expireDate := mp.AutoRefreshBeginAt.AddDate(0, 0, mp.AutoRefreshDays)
			if expireDate.After(now) || expireDate.Equal(now) {
				filtered = append(filtered, mp)
			}
		} else {
			filtered = append(filtered, mp)
		}
	}

	ctx.Info("查询到需要自动刷新的挂载点", zap.Int("count", len(filtered)))

	return filtered, nil
}
