package mountpoint

import "errors"

var errInvalidMountPointFileID = errors.New("挂载点文件 ID 必须大于 0")
var errInvalidMountPointTokenID = errors.New("挂载点令牌 ID 不能小于 0")
var errInvalidMountPointUserID = errors.New("用户 ID 必须大于 0")
var errInvalidMountPointParseRequest = errors.New("解析请求不能为空")
var errInvalidMountPointCloudTokenID = errors.New("云盘令牌 ID 必须大于 0")
