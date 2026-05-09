package utils

import (
	"net/url"
	"path"
	"regexp"
	"strings"
)

func SplitPath(path string) ([]string, error) {
	// 按照路径分隔符分割路径
	parts := strings.Split(path, "/")

	// 创建结果切片
	result := make([]string, 0)

	for _, part := range parts {
		// 跳过空字符串
		if part == "" {
			continue
		}

		// 对特殊字符进行转义
		escaped, err := url.PathUnescape(part)
		if err != nil {
			return nil, err
		}

		result = append(result, escaped)
	}

	return result, nil
}

// 路径最大长度，避免极端输入占用大量存储。
const maxPathLength = 4096

// CheckIsPath 检查字符串是否为合法的存储路径：
// - 必须以 / 开头
// - 不能包含反斜杠、换行符
// - 不能包含 ".." 段（防止路径穿越）
// - 长度不超过 4096
func CheckIsPath(p string) bool {
	if p == "" {
		return false
	}

	if len(p) > maxPathLength {
		return false
	}

	// 检查是否以 / 开头
	if !strings.HasPrefix(p, "/") {
		return false
	}

	// 检查路径是否包含非法字符
	if strings.ContainsAny(p, "\\\n\r\x00") {
		return false
	}

	// 防止路径穿越
	for _, seg := range strings.Split(p, "/") {
		if seg == ".." {
			return false
		}
	}

	return true
}

func PathEscape(elem ...string) string {
	if len(elem) == 0 {
		return ""
	}

	// 先拼接路径
	joined := path.Join(elem...)

	// 检查是否需要前导斜杠
	needsLeadingSlash := len(elem) > 0 && strings.HasPrefix(elem[0], "/")

	// 分割并转义每个部分
	ss := strings.Split(joined, "/")

	var ns = make([]string, 0, len(ss))

	for _, s := range ss {
		if s != "" { // 过滤空字符串
			ns = append(ns, url.PathEscape(s))
		}
	}

	result := path.Join(ns...)

	// 添加前导斜杠（如果需要）
	if needsLeadingSlash && !strings.HasPrefix(result, "/") {
		result = "/" + result
	}

	return result
}

// sanitizeFileNameRegex 匹配 Windows 保留的非法文件名字符（不含 |，| 将被替换为中文字符）。
var sanitizeFileNameRegex = regexp.MustCompile(`[<>:"?*\\/]`)

// SanitizeFileName 清理 Windows 非法文件名字符
func SanitizeFileName(name string) string {
	// 先把 | 换成中文竖线，避免被通用规则直接替换为下划线
	cleaned := strings.ReplaceAll(name, "|", "丨")

	// 再替换其他 Windows 禁用字符为下划线
	cleaned = sanitizeFileNameRegex.ReplaceAllString(cleaned, "_")

	return strings.TrimSpace(cleaned)
}
