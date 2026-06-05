package file

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/xxcheng123/cloudpan189-share/internal/consts"
	"github.com/xxcheng123/cloudpan189-share/internal/pkgs/utils"
	"github.com/xxcheng123/cloudpan189-share/internal/repository/models"
	"github.com/xxcheng123/cloudpan189-share/internal/shared"
	"github.com/xxcheng123/multistreamer"
	"go.uber.org/zap"

	"github.com/xxcheng123/cloudpan189-share/internal/framework/httpcontext"
	"gorm.io/gorm"

	cloudbridgeSvi "github.com/xxcheng123/cloudpan189-share/internal/services/cloudbridge"
)

type downloadRequest struct {
	Sign      string `form:"sign" binding:"required" example:"abc123"`
	UUID      string `form:"uuid" binding:"required" example:"def456"`
	Timestamp string `form:"timestamp" binding:"required" example:"1234567890"`
	Signer    string `form:"signer,default=v1" binding:"required" example:"v1"`
}

type downloadFileIdRequest struct {
	FileId int64 `uri:"fileId" binding:"required,min=1" example:"123456"`
}

// Download 下载文件
// @Summary 下载文件
// @Description 通过文件ID和签名验证下载文件，支持多种传输方式（重定向、本地代理、多线程流）
// @Tags 文件管理
// @Accept json
// @Produce application/octet-stream
// @Param fileId path int true "文件ID" example(123456)
// @Param sign query string true "签名" example("abc123")
// @Param uuid query string true "UUID" example("def456")
// @Param timestamp query string true "时间戳" example("1234567890")
// @Param signer query string true "签名版本" default("v1") example("v1")
// @Param Range header string false "HTTP Range 请求头，支持断点续传" example("bytes=0-1023")
// @Success 200 {file} binary "文件下载成功（本地代理或多线程流模式）"
// @Success 302 {string} string "重定向到下载链接（重定向模式）"
// @Failure 400 {object} httpcontext.Response "参数验证失败，code=99998"
// @Failure 400 {object} httpcontext.Response "签名验证失败，code=6005"
// @Failure 400 {object} httpcontext.Response "查询文件失败，code=6004"
// @Failure 400 {object} httpcontext.Response "查询文件令牌失败，code=6008"
// @Failure 400 {object} httpcontext.Response "获取下载链接失败，code=6009"
// @Failure 400 {object} httpcontext.Response "目录文件不支持下载，code=6006"
// @Failure 400 {object} httpcontext.Response "缺少family_id参数，code=6010"
// @Failure 400 {object} httpcontext.Response "缺少share_id参数，code=6011"
// @Failure 400 {object} httpcontext.Response "不支持的文件类型，code=6012"
// @Failure 400 {object} httpcontext.Response "创建本地代理请求失败，code=6013"
// @Failure 400 {object} httpcontext.Response "创建多线程流请求失败，code=6014"
// @Failure 404 {object} httpcontext.Response "文件令牌未绑定，code=6007"
// @Failure 401 {object} httpcontext.Response "未授权访问"
// @Failure 403 {object} httpcontext.Response "权限不足"
// @Router /api/file/download/{fileId} [get]
func (h *handler) Download() httpcontext.HandlerFunc {
	return func(ctx *httpcontext.Context) {
		req := new(downloadRequest)
		if err := ctx.ShouldBindQuery(req); err != nil {
			ctx.AbortWithInvalidParams(err)

			return
		}

		fileIdReq := new(downloadFileIdRequest)
		if err := ctx.ShouldBindUri(fileIdReq); err != nil {
			ctx.AbortWithInvalidParams(err)

			return
		}

		if !h.ensureVerifyService(ctx, busCodeFileVerifyError) {
			return
		}

		if err := h.verifyService.VerifyV1(ctx.GetContext(), fileIdReq.FileId, req.Sign, req.UUID, req.Timestamp, req.Signer); err != nil {
			ctx.Fail(busCodeFileVerifyError.WithError(err))

			return
		}

		if !h.ensureVirtualFileService(ctx, busCodeFileQueryError) {
			return
		}

		// 查询挂载文件
		file, err := h.virtualFileService.Query(ctx.GetContext(), fileIdReq.FileId)
		if err != nil {
			ctx.Fail(busCodeFileQueryError.WithError(err))

			return
		}

		if file == nil {
			ctx.Fail(busCodeFileNotFound.WithError(gorm.ErrRecordNotFound))

			return
		}

		if file.IsDir {
			ctx.Fail(busCodeFileIsDirNotSupport)

			return
		}

		if !h.ensureMountPointService(ctx, busCodeFileQueryError) {
			return
		}

		mountFile, err := h.mountPointService.Query(ctx.GetContext(), file.TopId)
		if err != nil {
			ctx.Fail(busCodeFileQueryError.WithError(err))

			return
		}

		if mountFile == nil {
			ctx.Fail(busCodeFileQueryError.WithError(gorm.ErrRecordNotFound))

			return
		}

		userID := ctx.GetInt64(consts.CtxKeyUserId)
		isAdmin := ctx.GetBool(consts.CtxKeyIsAdmin)
		tokenID := mountFile.TokenId

		if userID > 0 && h.hasUserMountPointTokenService() {
			boundTokenID, bindErr := h.userMountPointTokenService.GetTokenID(ctx.GetContext(), userID, mountFile.ID)
			if bindErr != nil {
				ctx.Fail(busCodeTokenQueryError.WithError(bindErr))

				return
			}

			if boundTokenID > 0 {
				tokenID = boundTokenID
			} else if isAdmin {
				anyTokenMap, anyErr := h.userMountPointTokenService.GetAnyUserTokens(ctx.GetContext(), []int64{mountFile.ID})
				if anyErr != nil {
					ctx.Fail(busCodeTokenQueryError.WithError(anyErr))

					return
				}

				if anyTokenID, ok := anyTokenMap[mountFile.ID]; ok && anyTokenID > 0 {
					tokenID = anyTokenID
				}
			}
		}

		if tokenID == 0 && h.hasUserMountPointTokenService() && userID == 0 {
			anyTokenMap, anyErr := h.userMountPointTokenService.GetAnyUserTokens(ctx.GetContext(), []int64{mountFile.ID})
			if anyErr != nil {
				ctx.Fail(busCodeTokenQueryError.WithError(anyErr))

				return
			}

			if anyTokenID, ok := anyTokenMap[mountFile.ID]; ok && anyTokenID > 0 {
				tokenID = anyTokenID
			}
		}

		if tokenID == 0 {
			ctx.Fail(busCodeTokenNotBind)

			return
		}

		// 获取令牌
		if !h.ensureCloudTokenService(ctx, busCodeTokenQueryError) {
			return
		}

		token, err := h.cloudTokenService.Query(ctx.GetContext(), tokenID)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				ctx.Fail(busCodeTokenNotBind.WithError(err))
			} else {
				ctx.Fail(busCodeTokenQueryError.WithError(err))
			}

			return
		}

		if token == nil {
			ctx.Fail(busCodeTokenNotBind)

			return
		}

		authToken := cloudbridgeSvi.NewAuthToken(token.AccessToken, token.AuthExpiresAtMillis(time.Now()))

		// 获取下载链接
		var (
			downloadLink string
		)

		if !h.ensureCloudBridgeService(ctx, busCodeGetDownloadLinkError) {
			return
		}

		switch file.OsType {
		case models.OsTypePersonFile:
			downloadLink, err = h.cloudBridgeService.PersonDownloadLink(ctx.GetContext(), authToken, file.CloudId)
			if err != nil {
				ctx.Fail(busCodeGetDownloadLinkError.WithError(err))

				return
			}
		case models.OsTypeFamilyFile:
			familyId, ok := file.Addition.String(consts.FileAdditionKeyFamilyId)
			if !ok {
				ctx.Fail(busCodeMissFamilyId)

				return
			}

			downloadLink, err = h.cloudBridgeService.FamilyDownloadLink(ctx.GetContext(), authToken, familyId, file.CloudId)
			if err != nil {
				ctx.Fail(busCodeGetDownloadLinkError.WithError(err))

				return
			}
		case models.OsTypeShareFile, models.OsTypeSubscribeShareFile:
			shareId, ok := file.Addition.Int64(consts.FileAdditionKeyShareId)
			if !ok {
				ctx.Fail(busCodeMissShareId)

				return
			}

			downloadLink, err = h.cloudBridgeService.ShareDownloadLink(ctx.GetContext(), authToken, shareId, file.CloudId)
			if err != nil {
				ctx.Fail(busCodeGetDownloadLinkError.WithError(err))

				return
			}
		default:
			ctx.Fail(busCodeUnsupportedOsType)

			return
		}

		settingAddition := shared.GetSettingAddition()
		if settingAddition.MultipleStream {
			ctx.Header(consts.HeaderKeyTransferType, consts.HeaderValueTransferTypeMultiStream)
			ctx.Header(consts.HeaderKeyTransferChunkSize, fmt.Sprintf("%d", settingAddition.MultipleStreamChunkSize))
			ctx.Header(consts.HeaderKeyTransferThreadCount, fmt.Sprintf("%d", settingAddition.MultipleStreamThreadCount))
			ctx.Header(consts.HeaderKeyTransferChunkSizeFormat, utils.FormatBytes(settingAddition.MultipleStreamChunkSize))

			h.doMultiStream(ctx, downloadLink, settingAddition.MultipleStreamThreadCount, settingAddition.MultipleStreamChunkSize)
		} else if settingAddition.LocalProxy {
			ctx.Header(consts.HeaderKeyTransferType, consts.HeaderValueTransferTypeLocalProxy)

			h.doProxy(ctx, downloadLink, settingAddition.LocalProxyURL)
		} else {
			ctx.Header(consts.HeaderKeyTransferType, consts.HeaderValueTransferTypeRedirect)
			ctx.Redirect(http.StatusFound, downloadLink)
		}
	}
}

var globalHTTPTransport = &http.Transport{
	DisableKeepAlives:     false,
	MaxIdleConns:          200,
	MaxIdleConnsPerHost:   20,
	MaxConnsPerHost:       50,
	IdleConnTimeout:       120 * time.Second,
	TLSHandshakeTimeout:   5 * time.Second,
	ExpectContinueTimeout: 1 * time.Second,
	DisableCompression:    true,
	WriteBufferSize:       128 * 1024,
	ReadBufferSize:        128 * 1024,
	ForceAttemptHTTP2:     false,
}

// 全局HTTP客户端，复用连接
var globalHTTPClient = &http.Client{
	Timeout:   0,
	Transport: globalHTTPTransport,
}

var downloadProxyLogURLPattern = regexp.MustCompile(`(?i)(?:[a-z][a-z0-9+.-]*://|/)[^\s"'<>]+`)

func sanitizeDownloadProxyError(err error) string {
	if err == nil {
		return ""
	}

	message := err.Error()
	message = downloadProxyLogURLPattern.ReplaceAllStringFunc(message, utils.RedactURLForLog)

	return utils.RedactSensitiveText(message)
}

func (h *handler) doProxy(ctx *httpcontext.Context, link string, proxyURL string) {
	start := time.Now()
	logger := ctx.GetContext().Logger
	logLink := utils.RedactURLForLog(link)

	client, err := localProxyHTTPClient(proxyURL)
	if err != nil {
		logger.Error("本地代理地址无效",
			zap.String("proxy", utils.RedactURLForLog(proxyURL)),
			zap.String("error", sanitizeDownloadProxyError(err)),
		)

		ctx.Fail(busCodeCreateLocalProxyRequestError.WithError(err))

		return
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, link, nil)
	if err != nil {
		logger.Error("创建本地代理请求失败", zap.String("error", sanitizeDownloadProxyError(err)), zap.String("link", logLink))

		ctx.Fail(busCodeCreateLocalProxyRequestError.WithError(err))

		return
	}

	h.copyOptimizedHeaders(ctx.Request.Header, req.Header)

	rangeHeader := ctx.Request.Header.Get("Range")
	isRangeRequest := rangeHeader != ""

	if isRangeRequest {
		req.Header.Set("Connection", "keep-alive")
	}

	resp, err := client.Do(req)
	if err != nil {
		if ctx.Request.Context().Err() != nil {
			logger.Error("客户端断开连接", zap.String("link", logLink))

			return
		}

		logger.Error("本地代理请求失败", zap.String("error", sanitizeDownloadProxyError(err)), zap.String("link", logLink))

		ctx.Fail(busCodeCreateLocalProxyRequestError.WithError(err))

		return
	}

	defer func() {
		_ = resp.Body.Close()
	}()

	h.copyOptimizedResponseHeaders(resp.Header, ctx)
	ctx.Status(resp.StatusCode)

	ctx.Stream(func(w io.Writer) bool {
		n, err := io.Copy(w, resp.Body)
		if err != nil {
			logger.Error("本地代理响应写入失败", zap.String("error", sanitizeDownloadProxyError(err)), zap.String("link", logLink))

			return false
		}

		logger.Debug("本地代理响应写入成功", zap.Int64("n", n), zap.String("link", logLink), zap.Duration("cost", time.Since(start)))

		return true
	})
}

func localProxyHTTPClient(proxyURL string) (*http.Client, error) {
	transport, err := localProxyHTTPTransport(proxyURL)
	if err != nil {
		return nil, err
	}

	if transport == nil {
		return globalHTTPClient, nil
	}

	return &http.Client{
		Timeout:   globalHTTPClient.Timeout,
		Transport: transport,
	}, nil
}

func localProxyHTTPTransport(proxyURL string) (*http.Transport, error) {
	normalizedProxyURL, err := utils.NormalizeHTTPProxyURL(proxyURL)
	if err != nil {
		return nil, err
	}

	if normalizedProxyURL == "" {
		return nil, nil
	}

	parsedProxyURL, _ := url.Parse(normalizedProxyURL)

	transport := globalHTTPTransport.Clone()
	transport.Proxy = http.ProxyURL(parsedProxyURL)

	return transport, nil
}

func (h *handler) doMultiStream(ctx *httpcontext.Context, link string, threadCount int, chunkSize int64) {
	var (
		logger  = ctx.GetContext().Logger
		logLink = utils.RedactURLForLog(link)
	)

	httpReq := http.Header{}
	h.copyOptimizedHeaders(ctx.Request.Header, httpReq)
	httpReq.Set("Accept-Encoding", "identity")
	httpReq.Del("Content-Type")

	streamer, err := multistreamer.NewStreamer(ctx,
		link,
		httpReq,
		multistreamer.WithLogger(logger),
		multistreamer.WithThreads(threadCount),
		multistreamer.WithChunkSize(chunkSize),
	)
	if err != nil {
		logger.Error("多线程流初始化失败", zap.String("error", sanitizeDownloadProxyError(err)), zap.String("url", logLink))

		ctx.Fail(busCodeCreateMultiStreamProxyError.WithError(err))

		return
	}

	h.copyOptimizedResponseHeaders(streamer.GetResponseHeader(), ctx)

	ctx.Status(streamer.HTTPCode())

	if err = streamer.Transfer(ctx, ctx.Writer); err != nil {
		if h.isConnectionError(err) {
			logger.Info("客户端连接断开", zap.String("link", logLink))
		} else {
			logger.Error("多线程流文件传输失败", zap.String("error", sanitizeDownloadProxyError(err)), zap.String("link", logLink))
		}
	}
}

func (h *handler) copyOptimizedHeaders(src, dst http.Header) {
	importantHeaders := []string{
		"Range", "If-Range", "If-Modified-Since", "If-None-Match",
		"User-Agent", "Accept", "Accept-Encoding",
		"Referer", "Origin",
	}

	for _, header := range importantHeaders {
		if value := src.Get(header); value != "" {
			dst.Set(header, value)
		}
	}
}

func (h *handler) copyOptimizedResponseHeaders(src http.Header, ctx *httpcontext.Context) {
	importantHeaders := []string{
		"Content-Type", "Content-Length", "Content-Range",
		"Accept-Ranges", "Last-Modified", "ETag", "Cache-Control",
		"Content-Disposition", "Content-Encoding",
	}

	for _, header := range importantHeaders {
		if value := src.Get(header); value != "" {
			ctx.Header(header, value)
		}
	}
}

func (h *handler) isConnectionError(err error) bool {
	if err == nil {
		return false
	}

	errStr := strings.ToLower(err.Error())
	connectionErrors := []string{
		"connection was forcibly closed",
		"wsasend",
		"broken pipe",
		"connection reset by peer",
		"client disconnected",
		"context canceled",
		"context deadline exceeded",
		"use of closed network connection",
		"connection refused",
		"no route to host",
	}

	for _, connErr := range connectionErrors {
		if strings.Contains(errStr, connErr) {
			return true
		}
	}

	return false
}
