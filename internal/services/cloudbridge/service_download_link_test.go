package cloudbridge

import (
	stdctx "context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/patrickmn/go-cache"
	"github.com/xxcheng123/cloudpan189-share/internal/framework/context"
	"github.com/xxcheng123/cloudpan189-share/internal/pkgs/utils"
	"github.com/xxcheng123/cloudpan189-share/internal/shared"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

func isolateShareCache(t *testing.T) {
	t.Helper()

	oldCache := shared.ShareCache
	shared.ShareCache = cache.New(5*time.Minute, 10*time.Minute)

	t.Cleanup(func() {
		shared.ShareCache = oldCache
	})
}

func TestLoadOrFetchDeletesInvalidCachedDownloadLinkAndRefetches(t *testing.T) {
	isolateShareCache(t)

	svc := &service{}
	ctx := context.NewContext(stdctx.Background())
	cacheKey := "person_download_link:test-file"
	realLink := "https://download.example.test/file"

	shared.ShareCache.Set(cacheKey, 123, time.Minute)

	var fetchCount int

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Location", realLink)
		w.WriteHeader(http.StatusFound)
	}))
	defer server.Close()

	link, err := svc.loadOrFetch(ctx, cacheKey, func() (string, error) {
		fetchCount++

		return server.URL, nil
	})
	if err != nil {
		t.Fatalf("load download link: %v", err)
	}

	if link != realLink {
		t.Fatalf("expected real download link %q, got %q", realLink, link)
	}

	if fetchCount != 1 {
		t.Fatalf("expected fetch to run once, got %d", fetchCount)
	}

	cached, ok := shared.ShareCache.Get(cacheKey)
	if !ok {
		t.Fatal("expected refreshed link to be cached")
	}

	if cached != realLink {
		t.Fatalf("expected refreshed cache value %q, got %#v", realLink, cached)
	}
}

func TestLoadOrFetchRedactsDownloadLinkInLogs(t *testing.T) {
	isolateShareCache(t)

	core, logs := observer.New(zap.DebugLevel)
	logger := zap.New(core)
	svc := &service{}
	ctx := context.NewContext(stdctx.Background(), context.WithLogger(logger), context.WithTraceId("trace-test"))
	cacheKey := "person_download_link:secret-file"
	realLink := "https://download.example.test/file.mkv?sign=real-secret-sign&filename=private-name.mkv"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Location", realLink)
		w.WriteHeader(http.StatusFound)
	}))
	defer server.Close()

	link, err := svc.loadOrFetch(ctx, cacheKey, func() (string, error) {
		return server.URL + "?sign=origin-secret-sign&filename=origin-private-name.mkv", nil
	})
	if err != nil {
		t.Fatalf("load download link: %v", err)
	}

	if link != realLink {
		t.Fatalf("expected raw real download link %q, got %q", realLink, link)
	}

	entries := logs.FilterMessage("真实获取个人文件下载地址").All()
	if len(entries) != 1 {
		t.Fatalf("expected one download link log, got %d", len(entries))
	}

	logText := entries[0].Message + fmt.Sprint(entries[0].Context)
	for _, leaked := range []string{"real-secret-sign", "private-name.mkv", "origin-secret-sign", "origin-private-name.mkv"} {
		if strings.Contains(logText, leaked) {
			t.Fatalf("expected %q to be redacted from log %s", leaked, logText)
		}
	}

	if !strings.Contains(logText, utils.RedactedSecret) {
		t.Fatalf("expected redacted marker in log %s", logText)
	}
}

func TestFetchRealDownloadLinkRedactsRequestError(t *testing.T) {
	core, logs := observer.New(zap.ErrorLevel)
	logger := zap.New(core)
	svc := &service{}
	ctx := context.NewContext(stdctx.Background(), context.WithLogger(logger), context.WithTraceId("trace-test"))

	oldClient := noFollowRedirectHttpClient
	noFollowRedirectHttpClient = &http.Client{
		Transport:     failingDownloadLinkRoundTripper{},
		CheckRedirect: oldClient.CheckRedirect,
	}

	t.Cleanup(func() {
		noFollowRedirectHttpClient = oldClient
	})

	link := "https://proxy-user:proxy-pass@download.example.test/file?sign=origin-secret-sign&filename=private-name.mkv#access_token=fragment-secret"

	got, err := svc.fetchRealDownloadLink(ctx, link)
	if err == nil {
		t.Fatal("expected request error")
	}

	if got != "" {
		t.Fatalf("expected no link, got %q", got)
	}

	entries := logs.FilterMessage("请求云盘下载链接失败").All()
	if len(entries) != 1 {
		t.Fatalf("expected one error log, got %d", len(entries))
	}

	text := err.Error() + entries[0].Message + fmt.Sprint(entries[0].Context)
	for _, leaked := range []string{"proxy-user", "proxy-pass", "origin-secret-sign", "private-name.mkv", "fragment-secret", "abcd"} {
		if strings.Contains(text, leaked) {
			t.Fatalf("expected %q to be redacted from %s", leaked, text)
		}
	}

	if !strings.Contains(text, utils.RedactedSecret) {
		t.Fatalf("expected redacted marker in %s", text)
	}
}

type cloudbridgeContextMarkerKey struct{}

type captureDownloadLinkRoundTripper struct {
	contextValue string
	userAgent    string
}

func (rt *captureDownloadLinkRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	if value, ok := req.Context().Value(cloudbridgeContextMarkerKey{}).(string); ok {
		rt.contextValue = value
	}

	rt.userAgent = req.Header.Get("User-Agent")

	return &http.Response{
		StatusCode: http.StatusFound,
		Header: http.Header{
			"Location": []string{"https://download.example.test/file"},
		},
		Body:    io.NopCloser(strings.NewReader("")),
		Request: req,
	}, nil
}

func TestFetchRealDownloadLinkUsesRequestContextAndUserAgent(t *testing.T) {
	svc := &service{}
	baseCtx := stdctx.WithValue(stdctx.Background(), cloudbridgeContextMarkerKey{}, "request-marker")
	ctx := context.NewContext(baseCtx)
	roundTripper := &captureDownloadLinkRoundTripper{}

	oldClient := noFollowRedirectHttpClient
	noFollowRedirectHttpClient = &http.Client{
		Transport:     roundTripper,
		CheckRedirect: oldClient.CheckRedirect,
	}

	t.Cleanup(func() {
		noFollowRedirectHttpClient = oldClient
	})

	link, err := svc.fetchRealDownloadLink(ctx, "https://cloud.example.test/download")
	if err != nil {
		t.Fatalf("fetch real download link: %v", err)
	}

	if link != "https://download.example.test/file" {
		t.Fatalf("expected redirected link, got %q", link)
	}

	if roundTripper.contextValue != "request-marker" {
		t.Fatalf("expected request context marker to be propagated, got %q", roundTripper.contextValue)
	}

	if roundTripper.userAgent == "" {
		t.Fatal("expected browser user-agent to be sent to download link probe")
	}
}

func TestNoFollowRedirectHTTPClientHasTimeout(t *testing.T) {
	if noFollowRedirectHttpClient.Timeout <= 0 {
		t.Fatal("expected no-follow download link client to have a timeout")
	}
}

type failingDownloadLinkRoundTripper struct{}

func (failingDownloadLinkRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	return nil, fmt.Errorf("request failed: %s accessCode=abcd", req.URL.String())
}

func TestLogAndReturnDownloadLinkErrorRedactsError(t *testing.T) {
	core, logs := observer.New(zap.ErrorLevel)
	logger := zap.New(core)
	ctx := context.NewContext(stdctx.Background(), context.WithLogger(logger), context.WithTraceId("trace-test"))
	rawErr := fmt.Errorf("request failed: https://proxy-user:proxy-pass@example.test/file?access_token=query-secret&filename=private-name.mkv#token=fragment-secret Authorization: Bearer secret-token accessCode=abcd")

	got, err := logAndReturnDownloadLinkError(ctx, "获取个人文件下载地址失败", "file-id", rawErr)
	if err == nil {
		t.Fatal("expected error")
	}

	if got != "" {
		t.Fatalf("expected no link, got %q", got)
	}

	entries := logs.FilterMessage("获取个人文件下载地址失败").All()
	if len(entries) != 1 {
		t.Fatalf("expected one error log, got %d", len(entries))
	}

	text := err.Error() + entries[0].Message + fmt.Sprint(entries[0].Context)
	for _, leaked := range []string{"proxy-user", "proxy-pass", "query-secret", "private-name.mkv", "fragment-secret", "secret-token", "abcd"} {
		if strings.Contains(text, leaked) {
			t.Fatalf("expected %q to be redacted from %s", leaked, text)
		}
	}

	if !strings.Contains(text, utils.RedactedSecret) {
		t.Fatalf("expected redacted marker in %s", text)
	}
}

func TestLogCloudbridgeErrorRedactsShareInfoError(t *testing.T) {
	core, logs := observer.New(zap.ErrorLevel)
	logger := zap.New(core)
	ctx := context.NewContext(stdctx.Background(), context.WithLogger(logger), context.WithTraceId("trace-test"))
	rawErr := fmt.Errorf(`GET "https://cloud.189.cn/api/open/share/getShareInfo.action?shareCode=abcDEF&accessCode=wxyz&clientSecret=client-secret": accessCode=wxyz`)

	err := logCloudbridgeError(ctx, "查询分享信息失败", rawErr,
		zap.String("share_code", utils.MaskShareCodeForLog("abcDEF")),
		zap.Bool("has_access_code", true))
	if err == nil {
		t.Fatal("expected error")
	}

	entries := logs.FilterMessage("查询分享信息失败").All()
	if len(entries) != 1 {
		t.Fatalf("expected one error log, got %d", len(entries))
	}

	text := err.Error() + entries[0].Message + fmt.Sprint(entries[0].Context)
	for _, leaked := range []string{"abcDEF", "wxyz", "client-secret"} {
		if strings.Contains(text, leaked) {
			t.Fatalf("expected %q to be redacted from %s", leaked, text)
		}
	}

	if !strings.Contains(text, utils.RedactedSecret) {
		t.Fatalf("expected redacted marker in %s", text)
	}
}

func TestLogAndReturnCloudbridgeErrorUsesSafeReturnMessage(t *testing.T) {
	core, logs := observer.New(zap.ErrorLevel)
	logger := zap.New(core)
	ctx := context.NewContext(stdctx.Background(), context.WithLogger(logger), context.WithTraceId("trace-test"))
	rawErr := fmt.Errorf("request failed: /api/list?accessCode=wxyz&access_token=query-secret Authorization: Bearer secret-token")

	err := logAndReturnCloudbridgeError(ctx, "获取分享文件失败", "获取第2页分享文件失败", rawErr, zap.Int64("page_num", 2))
	if err == nil {
		t.Fatal("expected error")
	}

	if !strings.Contains(err.Error(), "获取第2页分享文件失败") {
		t.Fatalf("expected return message in %q", err.Error())
	}

	entries := logs.FilterMessage("获取分享文件失败").All()
	if len(entries) != 1 {
		t.Fatalf("expected one error log, got %d", len(entries))
	}

	text := err.Error() + entries[0].Message + fmt.Sprint(entries[0].Context)
	for _, leaked := range []string{"wxyz", "query-secret", "secret-token"} {
		if strings.Contains(text, leaked) {
			t.Fatalf("expected %q to be redacted from %s", leaked, text)
		}
	}

	if !strings.Contains(text, utils.RedactedSecret) {
		t.Fatalf("expected redacted marker in %s", text)
	}
}
