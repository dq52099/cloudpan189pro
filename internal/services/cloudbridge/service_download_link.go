package cloudbridge

import (
	"fmt"
	"net/http"
	"regexp"
	"time"

	"github.com/pkg/errors"

	"github.com/xxcheng123/cloudpan189-interface/client"
	"github.com/xxcheng123/cloudpan189-share/internal/framework/context"
	"github.com/xxcheng123/cloudpan189-share/internal/pkgs/utils"
	"github.com/xxcheng123/cloudpan189-share/internal/shared"
	"go.uber.org/zap"
)

const (
	personDownloadFormat = "person_download_link:%s"
	familyDownloadFormat = "family_download_link:%s:%s"
	shareDownloadFormat  = "share_download_link:%d:%s"
)

var cloudbridgeLogURLPattern = regexp.MustCompile(`(?i)(?:[a-z][a-z0-9+.-]*://|/)[^\s"'<>]+`)

func sanitizeCloudbridgeError(err error) string {
	if err == nil {
		return ""
	}

	message := err.Error()
	message = cloudbridgeLogURLPattern.ReplaceAllStringFunc(message, utils.RedactURLForLog)

	return utils.RedactSensitiveText(message)
}

func logCloudbridgeError(ctx context.Context, message string, err error, fields ...zap.Field) error {
	return logAndReturnCloudbridgeError(ctx, message, message, err, fields...)
}

func logAndReturnCloudbridgeError(ctx context.Context, logMessage string, returnMessage string, err error, fields ...zap.Field) error {
	safeErr := sanitizeCloudbridgeError(err)
	fields = append(fields, zap.String("error", safeErr))
	ctx.Error(logMessage, fields...)

	return fmt.Errorf("%s: %s", returnMessage, safeErr)
}

func logAndReturnDownloadLinkError(ctx context.Context, message string, fileId string, err error) (string, error) {
	return "", logCloudbridgeError(ctx, message, err, zap.String("file_id", fileId))
}

func (s *service) PersonDownloadLink(ctx context.Context, token AuthToken, fileId string) (string, error) {
	return s.loadOrFetch(ctx, fmt.Sprintf(personDownloadFormat, fileId), func() (string, error) {
		link, err := client.New().WithClient(ctx.HTTPClient()).WithToken(token).GetFileDownload(ctx, client.String(fileId))
		if err != nil {
			return logAndReturnDownloadLinkError(ctx, "获取个人文件下载地址失败", fileId, err)
		}

		return link.FileDownloadUrl, nil
	})
}

func (s *service) FamilyDownloadLink(ctx context.Context, token AuthToken, familyId, fileId string) (string, error) {
	return s.loadOrFetch(ctx, fmt.Sprintf(familyDownloadFormat, familyId, fileId), func() (string, error) {
		link, err := client.New().WithClient(ctx.HTTPClient()).WithToken(token).FamilyGetFileDownload(ctx, client.String(familyId), client.String(fileId))
		if err != nil {
			return logAndReturnDownloadLinkError(ctx, "获取家庭文件下载地址失败", fileId, err)
		}

		return link.FileDownloadUrl, nil
	})
}

func (s *service) ShareDownloadLink(ctx context.Context, token AuthToken, shareId int64, fileId string) (string, error) {
	return s.loadOrFetch(ctx, fmt.Sprintf(shareDownloadFormat, shareId, fileId), func() (string, error) {
		link, err := client.New().WithClient(ctx.HTTPClient()).WithToken(token).GetFileDownload(ctx, client.String(fileId), func(req *client.GetFileDownloadRequest) {
			req.ShareId = shareId
		})
		if err != nil {
			return logAndReturnDownloadLinkError(ctx, "获取分享文件下载地址失败", fileId, err)
		}

		return link.FileDownloadUrl, nil
	})
}

var (
	noFollowRedirectHttpClient = &http.Client{
		Timeout: 30 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/114.0.0.0 Safari/537.36")

			return http.ErrUseLastResponse // 停止跟随重定向
		},
	}
)

// fetchRealDownloadLink 获取真实的下载地址 自带的跳转域名有泄露 token 风险
func (s *service) fetchRealDownloadLink(ctx context.Context, link string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, link, nil)
	if err != nil {
		safeErr := sanitizeCloudbridgeError(err)
		ctx.Error("创建云盘下载链接请求失败",
			zap.String("downloadUrl", utils.RedactURLForLog(link)),
			zap.String("error", safeErr))

		return "", fmt.Errorf("创建云盘下载链接请求失败: %s", safeErr)
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/114.0.0.0 Safari/537.36")

	resp, err := noFollowRedirectHttpClient.Do(req)
	if err != nil {
		safeErr := sanitizeCloudbridgeError(err)
		ctx.Error("请求云盘下载链接失败",
			zap.String("downloadUrl", utils.RedactURLForLog(link)),
			zap.String("error", safeErr))

		return "", fmt.Errorf("请求云盘下载链接失败: %s", safeErr)
	}

	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode >= 300 && resp.StatusCode < 400 {
		// 从Location头获取重定向地址（内网地址）
		location := resp.Header.Get("Location")
		if location == "" {
			return "", errors.New("重定向响应但没有Location头")
		}

		return location, nil
	}

	return "", fmt.Errorf("获取重定向地址失败: 期望重定向响应，实际状态码 %d", resp.StatusCode)
}

func (s *service) loadOrFetch(ctx context.Context, cacheKey string, fn func() (string, error)) (string, error) {
	if v, ok := shared.ShareCache.Get(cacheKey); ok {
		link, ok := v.(string)
		if ok {
			ctx.Debug("从缓存中获取个人文件下载地址", zap.String("file_id", cacheKey))

			return link, nil
		}

		shared.ShareCache.Delete(cacheKey)
		ctx.Warn("下载链接缓存类型异常，已删除", zap.String("cache_key", cacheKey), zap.String("type", fmt.Sprintf("%T", v)))
	}

	v, err := fn()
	if err != nil {
		return "", err
	}

	realLink, err := s.fetchRealDownloadLink(ctx, v)
	if err != nil {
		return "", err
	}

	shared.ShareCache.Set(cacheKey, realLink, time.Minute*2)

	ctx.Debug("真实获取个人文件下载地址", zap.String("file_id", cacheKey), zap.String("download_link", utils.RedactURLForLog(realLink)))

	return realLink, nil
}
