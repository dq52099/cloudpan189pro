package virtualfile

import "errors"

var errInvalidVirtualFileID = errors.New("文件 ID 必须大于 0")
var errInvalidVirtualFilePath = errors.New("文件路径不合法")
var errEmptyVirtualFileUpdateFields = errors.New("文件更新字段不能为空")
var errEmptyVirtualFileUpdateConditions = errors.New("文件更新条件不能为空")
var errVirtualFilePathCycle = errors.New("文件路径存在循环引用")
var errVirtualFilePathTooDeep = errors.New("文件路径深度超过限制")
var errInvalidVirtualFileSortField = errors.New("文件排序字段不合法")
var errUnsupportedVirtualFileLinkID = errors.New("文件 link_id 查询不支持")
