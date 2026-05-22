package cloudtoken

import "errors"

var errInvalidCloudTokenID = errors.New("令牌 ID 必须大于 0")
var errInvalidCloudTokenUserID = errors.New("用户 ID 必须大于 0")
var errInvalidUsernameLoginCredentials = errors.New("用户名和密码不能为空")
