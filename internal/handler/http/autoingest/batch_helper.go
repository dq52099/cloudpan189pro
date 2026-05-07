package autoingest

// maxBatchConcurrency 限制批量操作同时执行的 goroutine 数，避免大批量请求打满数据库/任务引擎。
const maxBatchConcurrency = 16
