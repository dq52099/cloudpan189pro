package user

import "errors"

var errInvalidUserID = errors.New("用户 ID 必须大于 0")
var errInvalidUserGroupID = errors.New("用户组 ID 不能小于 0")
var errInvalidUsername = errors.New("用户名不能为空")
var errInvalidUserPassword = errors.New("密码不能为空")
var errEmptyUserUpdateFields = errors.New("更新字段不能为空")
