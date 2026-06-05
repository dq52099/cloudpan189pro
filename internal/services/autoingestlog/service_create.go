package autoingestlog

import (
	"github.com/xxcheng123/cloudpan189-share/internal/framework/context"
	"github.com/xxcheng123/cloudpan189-share/internal/repository/models"
	"github.com/xxcheng123/cloudpan189-share/internal/types/autoingest"
	"go.uber.org/zap"
)

// Create 写入一条自动挂载日志
func (s *service) Create(ctx context.Context, planID int64, level autoingest.LogLevel, content string) (int64, error) {
	log := &models.AutoIngestLog{
		PlanId:  planID,
		Level:   level,
		Content: sanitizeAutoIngestLogContent(content),
	}

	if err := s.getDB(ctx).Create(log).Error; err != nil {
		ctx.Error("写入自动挂载日志失败", zap.Error(err), zap.Int64("planId", planID), zap.String("level", level.String()))

		return 0, err
	}

	return log.ID, nil
}
