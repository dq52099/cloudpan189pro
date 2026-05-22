package setting

import "errors"

var errEmptySettingUpdateFields = errors.New("设置更新字段不能为空")
var errInvalidInitSystemRequest = errors.New("系统初始化请求不能为空")
