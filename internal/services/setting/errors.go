package setting

import "errors"

var errEmptySettingUpdateFields = errors.New("设置更新字段不能为空")
var errInvalidInitSystemRequest = errors.New("系统初始化请求不能为空")
var errInvalidSettingBaseURL = errors.New("系统基础 URL 必须是有效的 http/https 地址")
var errInvalidSettingAddition = errors.New("系统附加设置格式不正确")
var errInvalidSettingLocalProxyURL = errors.New("本地代理地址必须是有效的 http/https 地址")
