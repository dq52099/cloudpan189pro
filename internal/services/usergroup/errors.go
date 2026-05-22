package usergroup

import "errors"

var errInvalidUserGroupID = errors.New("用户组 ID 必须大于 0")
var errInvalidUserGroupName = errors.New("用户组名称不能为空")
