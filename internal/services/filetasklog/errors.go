package filetasklog

import "errors"

var ErrFileTaskLogTerminalState = errors.New("文件任务日志已处于终态")

var errInvalidFileTaskLogID = errors.New("文件任务日志 ID 必须大于 0")
var errInvalidFileTaskLogFileID = errors.New("文件 ID 必须大于 0")
var errInvalidFileTaskLogUserID = errors.New("用户 ID 必须大于 0")
var errInvalidFileTaskLogSortField = errors.New("文件任务日志排序字段不合法")
var errInvalidFileTaskLogCounter = errors.New("文件任务日志计数器不合法")
var errFileTaskLogProgressNotDone = errors.New("文件任务日志状态或进度未满足自动完成条件")
