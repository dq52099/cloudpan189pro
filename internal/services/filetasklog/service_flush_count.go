package filetasklog

import (
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

// allowedCounterColumns 白名单，限制可通过 FlushCount 更新的列名，避免 SQL 注入。
var allowedCounterColumns = map[string]struct{}{
	"completed": {},
	"total":     {},
	"processed": {},
	"failed":    {},
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

// FlushCount 刷新计数列（在已有值基础上做原子递增）。
// 列名通过白名单校验，不受外部 Counter 实现污染影响。
func (s *service) FlushCount(ctx context.Context, key LogKey, counters ...Counter) (err error) {
	ctx.Debug("刷新文件任务日志计数", zap.Int64("task_id", key.GetID()))

	if len(counters) == 0 {
		return nil
	}

	countMp := map[string]int{}

	for _, ct := range counters {
		name := ct.Name()
		if _, ok := allowedCounterColumns[name]; !ok {
			ctx.Warn("忽略未知的计数列", zap.String("column", name))
			continue
		}

		countMp[name] += ct.Count()
	}

	if len(countMp) == 0 {
		return nil
	}

	mp := map[string]any{}
	for k, v := range countMp {
		// 使用参数化表达式防止 SQL 注入；k 已经在白名单内
		mp[k] = gorm.Expr(gormColumnExpr(k), v)
	}

	if err = s.getDB(ctx).
		Where("id = ?", key.GetID()).
		Updates(mp).Error; err != nil {
		ctx.Error("刷新文件任务日志计数失败", zap.Error(err))
	}

	return
}

// gormColumnExpr 构造形如 "col + ?" 的表达式字符串。
// col 由白名单校验，不存在注入风险。
func gormColumnExpr(col string) string {
	return col + " + ?"
}
