package autoingestplan

import "errors"

var errInvalidAutoIngestPlanID = errors.New("自动挂载计划 ID 必须大于 0")
var errInvalidAutoIngestPlanUserID = errors.New("用户 ID 必须大于 0")
var errInvalidAutoIngestPlanOffset = errors.New("自动挂载计划 offset 不能为负数")
var errInvalidAutoIngestPlanCounterDelta = errors.New("自动挂载计划计数增量不能为负数")
var errEmptyAutoIngestPlanUpdateFields = errors.New("自动挂载计划更新字段不能为空")
