package loginlog

import "errors"

var errInvalidLoginLogSortField = errors.New("登录日志排序字段不合法")
var errInvalidLoginLogUserID = errors.New("用户 ID 必须大于 0")
