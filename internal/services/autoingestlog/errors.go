package autoingestlog

import "errors"

var errInvalidAutoIngestLogID = errors.New("自动入库日志 ID 必须大于 0")
var errInvalidAutoIngestLogPlanID = errors.New("自动入库计划 ID 必须大于 0")
