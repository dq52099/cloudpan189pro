package cloudbridge

import (
	"fmt"
	"net/http"
	"time"

	"github.com/pkg/errors"

	"github.com/xxcheng123/cloudpan189-interface/client"
	"github.com/xxcheng123/cloudpan189-share/internal/framework/context"
	"github.com/xxcheng123/cloudpan189-share/internal/shared"
	"go.uber.org/zap"
)

const (
	personDownloadFormat = "person_download_link:%s"
	familyDownloadFormat = "family_download_link:%s:%s"
	shareDownloadFormat  = "share_download_link:%d:%s"
)

func (s *service) PersonDownloadLink(ctx context.Context, token AuthToken, fileId string) (string, error) {
	return s.loadOrFetch(ctx, fmt.Sprintf(personDownloadFormat, fileId), func() (string, error) {
		link, err := client.New().WithClient(ctx.HTTPClient()).WithToken(token).GetFileDownload(ctx, client.String(fileId))
		if err != nil {
			ctx.Error("获取个人文件下载地址失败",
				zap.String("file_id", fileId), zap.Error(err))

			return "", err
		}

		return link.FileDownloadUrl, nil
	})
}

func (s *service) FamilyDownloadLink(ctx context.Context, token AuthToken, familyId, fileId string) (string, error) {
	return s.loadOrFetch(ctx, fmt.Sprintf(familyDownloadFormat, familyId, fileId), func() (string, error) {
		link, err := client.New().WithClient(ctx.HTTPClient()).WithToken(token).FamilyGetFileDownload(ctx, client.String(familyId), client.String(fileId))
		if err != nil {
			ctx.Error("获取家庭文件下载地址失败",
				zap.String("file_id", fileId), zap.Error(err))

			return "", err
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
			ctx.Error("获取分享文件下载地址失败",
				zap.String("file_id", fileId), zap.Error(err))

			return "", err
		}

		return link.FileDownloadUrl, nil
	})
}

var (
	noFollowRedirectHttpClient = &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/114.0.0.0 Safari/537.36")

			return http.ErrUseLastResponse // 停止跟随重定向
		},
	}
)

// fetchRealDownloadLink 获取真实的下载地址 自带的跳转域名有泄露 token 风险
func (s *service) fetchRealDownloadLink(ctx context.Context, link string) (string, error) {
	resp, err := noFollowRedirectHttpClient.Get(link)
	if err != nil {
		ctx.Error("请求云盘下载链接失败",
			zap.String("downloadUrl", link),
			zap.Error(err))

		return "", err
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
		ctx.Debug("从缓存中获取个人文件下载地址", zap.String("file_id", cacheKey))

		return v.(string), nil
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

	ctx.Debug("真实获取个人文件下载地址", zap.String("file_id", cacheKey), zap.String("download_link", realLink))

	return realLink, nil
}
