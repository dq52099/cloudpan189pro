package mediaconfig

import "errors"

var (
	errEmptyMediaConfigUpdateFields = errors.New("媒体配置更新字段不能为空")
	errInvalidAutoRebuildCron       = errors.New("自动重建 cron 表达式无效")
)
