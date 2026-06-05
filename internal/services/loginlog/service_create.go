package loginlog

import (
	"errors"

	"github.com/xxcheng123/cloudpan189-share/internal/framework/context"
	"github.com/xxcheng123/cloudpan189-share/internal/repository/models"
)

// Create 通用创建（如已有完整字段，直接落库）
func (s *service) Create(ctx context.Context, log *models.LoginLog) (int64, error) {
	if log == nil {
		return 0, errors.New("login log is nil")
	}

	normalizeLoginLogForCreate(log)

	// 补充 TraceId
	if log.TraceId == "" && ctx.Trace != nil {
		log.TraceId = ctx.ID()
	}

	if err := s.getDB(ctx).Create(log).Error; err != nil {
		return 0, err
	}

	return log.ID, nil
}
