package utils

import (
	"fmt"
	"net/url"
	"path"
	"regexp"
	"strings"
	"unicode/utf8"
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

		decodedPart, err := decodePathSegment(part)
		if err != nil {
			return nil, err
		}

		result = append(result, decodedPart)
	}

	return result, nil
}

func NormalizeStoragePath(p string) (string, error) {
	normalizedPath, _, err := NormalizeStoragePathParts(p)

	return normalizedPath, err
}

func NormalizeStoragePathParts(p string) (string, []string, error) {
	if !CheckIsPath(p) {
		return "", nil, fmt.Errorf("路径不合法，需要 / 开头的路径")
	}

	paths, err := SplitPath(p)
	if err != nil {
		return "", nil, err
	}

	normalizedParts := make([]string, 0, len(paths))

	displayParts := make([]string, 0, len(paths))
	for _, part := range paths {
		normalizedPart := SanitizeFileName(part)
		if normalizedPart == "" {
			return "", nil, fmt.Errorf("路径段不能为空")
		}

		displayParts = append(displayParts, normalizedPart)
		normalizedParts = append(normalizedParts, escapeStoragePathPercent(normalizedPart))
	}

	if len(normalizedParts) == 0 {
		return "/", nil, nil
	}

	return "/" + strings.Join(normalizedParts, "/"), displayParts, nil
}

// JoinStoragePath 将已解码的展示名拼接到存储路径上，并返回可持久化的规范化路径。
func JoinStoragePath(parentPath string, names ...string) (string, error) {
	normalizedParent, err := NormalizeStoragePath(parentPath)
	if err != nil {
		return "", err
	}

	normalizedParts := SplitNormalizedStoragePath(normalizedParent)

	for _, name := range names {
		normalizedName, displayParts, err := NormalizeStoragePathParts("/" + escapeStoragePathPercent(SanitizeFileName(name)))
		if err != nil {
			return "", err
		}

		if len(displayParts) != 1 {
			return "", fmt.Errorf("路径段不能为空")
		}

		normalizedParts = append(normalizedParts, strings.TrimPrefix(normalizedName, "/"))
	}

	if len(normalizedParts) == 0 {
		return "/", nil
	}

	return "/" + strings.Join(normalizedParts, "/"), nil
}

// SplitNormalizedStoragePath 拆分 NormalizeStoragePath 返回的规范化路径。
// 规范化路径已经完成 URL 解码，不能再次使用 SplitPath 做 PathUnescape。
func SplitNormalizedStoragePath(p string) []string {
	parts := strings.Split(p, "/")
	result := make([]string, 0, len(parts))

	for _, part := range parts {
		if part == "" {
			continue
		}

		result = append(result, part)
	}

	return result
}

// escapeStoragePathPercent 让规范化后的路径可以再次传入 NormalizeStoragePath。
// 例如用户输入 a%25b 表示文件名 a%b，规范化后保存为 a%25b。
func escapeStoragePathPercent(segment string) string {
	return strings.ReplaceAll(segment, "%", "%25")
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

	// 防止路径穿越。这里必须在 URL 解码后检查，避免 /%2e%2e/x 绕过。
	for _, seg := range strings.Split(p, "/") {
		if seg == "" {
			continue
		}

		unescapedSeg, err := decodePathSegment(seg)
		if err != nil {
			return false
		}

		if unescapedSeg == "." || unescapedSeg == ".." {
			return false
		}

		if strings.ContainsAny(unescapedSeg, "/\\\n\r\x00") {
			return false
		}
	}

	return true
}

// decodePathSegment 只解码合法的 %XX 序列，非法 % 保留为普通文件名字符。
func decodePathSegment(segment string) (string, error) {
	if !strings.Contains(segment, "%") {
		if !utf8.ValidString(segment) {
			return "", fmt.Errorf("路径段包含不合法 UTF-8 编码")
		}

		return segment, nil
	}

	decoded := make([]byte, 0, len(segment))
	for i := 0; i < len(segment); i++ {
		if segment[i] == '%' && i+2 < len(segment) {
			high, okHigh := fromHex(segment[i+1])

			low, okLow := fromHex(segment[i+2])
			if okHigh && okLow {
				decoded = append(decoded, high<<4|low)
				i += 2

				continue
			}
		}

		decoded = append(decoded, segment[i])
	}

	if !utf8.Valid(decoded) {
		return "", fmt.Errorf("路径段包含不合法 UTF-8 编码")
	}

	return string(decoded), nil
}

func fromHex(value byte) (byte, bool) {
	switch {
	case value >= '0' && value <= '9':
		return value - '0', true
	case value >= 'a' && value <= 'f':
		return value - 'a' + 10, true
	case value >= 'A' && value <= 'F':
		return value - 'A' + 10, true
	default:
		return 0, false
	}
}

func PathEscape(elem ...string) string {
	if len(elem) == 0 {
		return ""
	}

	// 先拼接路径
	joined := path.Join(elem...)
	if joined == "/" {
		return "/"
	}

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
