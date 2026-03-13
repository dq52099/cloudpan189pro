package filetasklog

import (
	"fmt"

	"github.com/xxcheng123/cloudpan189-share/internal/framework/context"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type Counter interface {
	Name() string
	Count() int
}

type counter struct {
	name  string
	count int
}

func (c *counter) Name() string {
	return c.name
}

func (c *counter) Count() int {
	return c.count
}

func WithCompletedCounter(count int) Counter {
	return &counter{
		name:  "completed",
		count: count,
	}
}

func WithCompletedOneCounter() Counter {
	return WithCompletedCounter(1)
}

func WithTotalCounter(count int) Counter {
	return &counter{
		name:  "total",
		count: count,
	}
}

func WithAllCounter(count int) []Counter {
	return []Counter{
		WithCompletedCounter(count),
		WithTotalCounter(count),
	}
}

// WithProcessedCounter 处理文件数量计数器
func WithProcessedCounter(count int) Counter {
	return &counter{
		name:  "processed",
		count: count,
	}
}

// WithFailedCounter 失败文件数量计数器
func WithFailedCounter(count int) Counter {
	return &counter{
		name:  "failed",
		count: count,
	}
}

func (s *service) FlushCount(ctx context.Context, key LogKey, counters ...Counter) (err error) {
	ctx.Debug("刷新文件任务日志计数", zap.Int64("task_id", key.GetID()))

	if len(counters) == 0 {
		return nil
	}

	countMp := map[string]int{}

	for _, ct := range counters {
		if v, ok := countMp[ct.Name()]; ok {
			countMp[ct.Name()] = v + ct.Count()
		} else {
			countMp[ct.Name()] = ct.Count()
		}
	}

	mp := map[string]any{}
	for k, v := range countMp {
		mp[k] = gorm.Expr(fmt.Sprintf("%s + %d", k, v))
	}

	if err = s.getDB(ctx).
		Where("id = ?", key.GetID()).
		Updates(mp).Error; err != nil {
		ctx.Error("刷新文件任务日志计数失败", zap.Error(err))
	}

	return
}
