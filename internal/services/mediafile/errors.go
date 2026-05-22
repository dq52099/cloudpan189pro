package mediafile

import "errors"

var errInvalidMediaFileFID = errors.New("媒体文件 FID 必须大于 0")
var errInvalidMediaFilePath = errors.New("媒体文件路径不合法")
