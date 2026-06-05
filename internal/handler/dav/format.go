package dav

import (
	"fmt"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/xxcheng123/cloudpan189-share/internal/repository/models"
)

// normalizeWebDAVPath 标准化 WebDAV 路径
func (e *workEngine) normalizeWebDAVPath(rawPath string) string {
	// 清理路径
	cleanPath := path.Clean(rawPath)

	// 确保以 / 开头
	if !strings.HasPrefix(cleanPath, "/") {
		cleanPath = "/" + cleanPath
	}

	return cleanPath
}

// addPropResponse 添加单个文件/文件夹的属性响应
func (e *workEngine) addPropResponse(xmlResponse *strings.Builder, file *models.VirtualFile, href string) {
	xmlResponse.WriteString(`<D:response>`)

	// 正确编码 href
	encodedHref := e.encodeWebDAVPath(href)
	fmt.Fprintf(xmlResponse, `<D:href>%s</D:href>`, escapeXML(encodedHref))

	xmlResponse.WriteString(`<D:propstat>`)
	xmlResponse.WriteString(`<D:prop>`)

	// 资源类型
	if file.IsDir {
		xmlResponse.WriteString(`<D:resourcetype><D:collection/></D:resourcetype>`)
	} else {
		xmlResponse.WriteString(`<D:resourcetype/>`)
	}

	// 显示名称
	fmt.Fprintf(xmlResponse, `<D:displayname>%s</D:displayname>`, escapeXML(file.Name))

	// 内容长度（文件大小）
	if !file.IsDir {
		fmt.Fprintf(xmlResponse, `<D:getcontentlength>%d</D:getcontentlength>`, file.Size)
	}

	// 内容类型
	if !file.IsDir {
		contentType := e.getContentType(file.Name)
		fmt.Fprintf(xmlResponse, `<D:getcontenttype>%s</D:getcontenttype>`, escapeXML(contentType))
	}

	// 最后修改时间
	rfc1123Time := file.ModifyDate.UTC().Format(time.RFC1123)
	fmt.Fprintf(xmlResponse, `<D:getlastmodified>%s</D:getlastmodified>`, escapeXML(rfc1123Time))

	// 创建时间
	rfc3339Time := file.CreateDate.UTC().Format(time.RFC3339)
	fmt.Fprintf(xmlResponse, `<D:creationdate>%s</D:creationdate>`, escapeXML(rfc3339Time))

	// ETag（使用文件哈希或修改时间）
	if file.Hash != "" {
		fmt.Fprintf(xmlResponse, `<D:getetag>"%s"</D:getetag>`, escapeXML(file.Hash))
	} else if !file.ModifyDate.IsZero() {
		// 如果没有哈希，使用修改时间作为ETag
		etag := fmt.Sprintf("%d-%s", file.ID, file.ModifyDate.Format(time.RFC3339))
		fmt.Fprintf(xmlResponse, `<D:getetag>"%s"</D:getetag>`, escapeXML(etag))
	}

	xmlResponse.WriteString(`</D:prop>`)
	xmlResponse.WriteString(`<D:status>HTTP/1.1 200 OK</D:status>`)
	xmlResponse.WriteString(`</D:propstat>`)
	xmlResponse.WriteString(`</D:response>`)
}

// escapeXML 转义XML特殊字符
func escapeXML(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, "\"", "&quot;")
	s = strings.ReplaceAll(s, "'", "&#39;")

	return s
}

// getContentType 根据文件扩展名获取MIME类型
func (e *workEngine) getContentType(filename string) string {
	ext := strings.ToLower(path.Ext(filename))
	switch ext {
	case ".txt":
		return "text/plain"
	case ".html", ".htm":
		return "text/html"
	case ".css":
		return "text/css"
	case ".js":
		return "application/javascript"
	case ".json":
		return "application/json"
	case ".xml":
		return "application/xml"
	case ".pdf":
		return "application/pdf"
	case ".zip":
		return "application/zip"
	case ".rar":
		return "application/x-rar-compressed"
	case ".7z":
		return "application/x-7z-compressed"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".gif":
		return "image/gif"
	case ".bmp":
		return "image/bmp"
	case ".webp":
		return "image/webp"
	case ".svg":
		return "image/svg+xml"
	case ".ico":
		return "image/x-icon"
	case ".mp3":
		return "audio/mpeg"
	case ".wav":
		return "audio/wav"
	case ".flac":
		return "audio/flac"
	case ".aac":
		return "audio/aac"
	case ".ogg":
		return "audio/ogg"
	case ".m4a":
		return "audio/mp4"
	case ".mp4":
		return "video/mp4"
	case ".m4v":
		return "video/x-m4v"
	case ".avi":
		return "video/x-msvideo"
	case ".mkv":
		return "video/x-matroska"
	case ".m3u8":
		return "application/vnd.apple.mpegurl"
	case ".mov":
		return "video/quicktime"
	case ".mpg", ".mpeg", ".m2v", ".m1v", ".mpv":
		return "video/mpeg"
	case ".ts", ".mts", ".m2ts", ".m2t":
		return "video/mp2t"
	case ".wmv":
		return "video/x-ms-wmv"
	case ".flv":
		return "video/x-flv"
	case ".webm":
		return "video/webm"
	case ".doc":
		return "application/msword"
	case ".docx":
		return "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
	case ".xls":
		return "application/vnd.ms-excel"
	case ".xlsx":
		return "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
	case ".ppt":
		return "application/vnd.ms-powerpoint"
	case ".pptx":
		return "application/vnd.openxmlformats-officedocument.presentationml.presentation"
	default:
		return "application/octet-stream"
	}
}

// encodeWebDAVPath 对 WebDAV 路径进行正确的 URL 编码
func (e *workEngine) encodeWebDAVPath(rawPath string) string {
	// 分割路径为各个部分
	parts := strings.Split(strings.Trim(rawPath, "/"), "/")

	// 对每个部分进行 URL 编码
	encodedParts := make([]string, len(parts))
	for i, part := range parts {
		if part != "" {
			encodedParts[i] = url.PathEscape(part)
		}
	}

	// 重新组合路径
	encodedPath := "/" + strings.Join(encodedParts, "/")

	// 如果原路径以 / 结尾（文件夹），保持这个特征
	if strings.HasSuffix(rawPath, "/") && !strings.HasSuffix(encodedPath, "/") {
		encodedPath += "/"
	}

	return encodedPath
}

// buildChildPath 构建子项路径
func (e *workEngine) buildChildPath(parentPath, childName string, isFolder bool) string {
	// 标准化父路径
	parentPath = e.normalizeWebDAVPath(parentPath)

	// 移除末尾的 /
	parentPath = strings.TrimSuffix(parentPath, "/")

	// 构建子路径
	childPath := parentPath + "/" + childName

	// 如果是文件夹，添加末尾的 /
	if isFolder {
		childPath += "/"
	}

	return childPath
}
