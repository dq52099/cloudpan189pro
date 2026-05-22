package autoingestplan

import "errors"

var errInvalidAutoIngestPlanID = errors.New("自动挂载计划 ID 必须大于 0")
var errInvalidAutoIngestPlanUserID = errors.New("用户 ID 必须大于 0")
var errEmptyAutoIngestPlanUpdateFields = errors.New("自动挂载计划更新字段不能为空")
