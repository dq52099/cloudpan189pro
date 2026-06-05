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

// allowedCounterColumns 白名单，限制可通过 FlushCount 更新的真实计数字段，避免 SQL 注入。
var allowedCounterColumns = map[string]struct{}{
	"completed": {},
	"total":     {},
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

// WithProcessedCounter 保留历史构造器；当前 file_task_logs 无 processed 列，FlushCount 会拒绝该计数器。
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
	if len(counters) == 0 {
		return nil
	}

	id, err := validateLogKey(key)
	if err != nil {
		return err
	}

	ctx.Debug("刷新文件任务日志计数", zap.Int64("task_id", id))

	countMp := map[string]int{}

	for _, ct := range counters {
		name, count, counterErr := validateFlushCounter(ct)
		if counterErr != nil {
			ctx.Error("文件任务日志计数器不合法", zap.Error(counterErr), zap.Int64("task_id", id))

			return counterErr
		}

		countMp[name] += count
	}

	if len(countMp) == 0 {
		return nil
	}

	mp := map[string]any{}
	for k, v := range countMp {
		// 使用参数化表达式防止 SQL 注入；k 已经在白名单内
		mp[k] = gorm.Expr(gormColumnExpr(k), v)
	}

	result := s.getDB(ctx).
		Where("id = ?", id).
		Updates(mp)
	if err = checkTaskLogUpdateResult(ctx, result, id, "文件任务日志不存在"); err != nil {
		ctx.Error("刷新文件任务日志计数失败", zap.Error(err))
	}

	return
}

// gormColumnExpr 构造形如 "col + ?" 的表达式字符串。
// col 由白名单校验，不存在注入风险。
func gormColumnExpr(col string) string {
	return col + " + ?"
}

func validateFlushCounter(ct Counter) (name string, count int, err error) {
	if ct == nil {
		return "", 0, fmt.Errorf("%w: counter 为空", errInvalidFileTaskLogCounter)
	}

	defer func() {
		if recover() != nil {
			name = ""
			count = 0
			err = fmt.Errorf("%w: counter 读取失败", errInvalidFileTaskLogCounter)
		}
	}()

	name = ct.Name()
	count = ct.Count()

	if _, ok := allowedCounterColumns[name]; !ok {
		return "", 0, fmt.Errorf("%w: 未知计数列 %q", errInvalidFileTaskLogCounter, name)
	}

	if count < 0 {
		return "", 0, fmt.Errorf("%w: %s 增量不能为负数", errInvalidFileTaskLogCounter, name)
	}

	return name, count, nil
}
