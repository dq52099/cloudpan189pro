package utils

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

const maxGeoIPResponseSize = 64 << 10

// geoIPCache 地理信息缓存，避免对同一 IP 反复请求远程服务。
type geoIPEntry struct {
	location string
	expireAt time.Time
}

var (
	geoIPCache   sync.Map // map[string]geoIPEntry
	geoIPTimeout = 3 * time.Second
	geoIPTTL     = 24 * time.Hour
	geoIPClient  = &http.Client{Timeout: geoIPTimeout}
)

// GeoLocate 通过 IP 解析地理信息。
// 内网/环回 IP 会直接返回 "内网"，避免请求外部 API。
// 网络错误或查询失败时返回 "-"。
func GeoLocate(ip string) string {
	ip = strings.TrimSpace(ip)
	if ip == "" {
		return "-"
	}

	// 去除端口号
	if host, _, err := net.SplitHostPort(ip); err == nil {
		ip = host
	}

	parsed := net.ParseIP(ip)
	if parsed == nil {
		return "-"
	}

	if parsed.IsLoopback() || parsed.IsPrivate() || parsed.IsLinkLocalUnicast() || parsed.IsUnspecified() {
		return "内网"
	}

	if entry, ok := geoIPCache.Load(ip); ok {
		if v, ok := entry.(geoIPEntry); ok && time.Now().Before(v.expireAt) {
			return v.location
		}
	}

	location := fetchGeoLocation(ip)
	geoIPCache.Store(ip, geoIPEntry{
		location: location,
		expireAt: time.Now().Add(geoIPTTL),
	})

	return location
}

// fetchGeoLocation 调用 ip-api.com 的中文接口获取国家/省/市。
// 免费服务，匿名调用限制每分钟 45 次。
func fetchGeoLocation(ip string) string {
	apiURL := fmt.Sprintf("http://ip-api.com/json/%s?lang=zh-CN&fields=status,country,regionName,city", ip)

	resp, err := geoIPClient.Get(apiURL)
	if err != nil {
		return "-"
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		return "-"
	}

	data, err := io.ReadAll(io.LimitReader(resp.Body, maxGeoIPResponseSize+1))
	if err != nil || len(data) > maxGeoIPResponseSize {
		return "-"
	}

	var body struct {
		Status     string `json:"status"`
		Country    string `json:"country"`
		RegionName string `json:"regionName"`
		City       string `json:"city"`
	}

	if err := json.Unmarshal(data, &body); err != nil {
		return "-"
	}

	if body.Status != "success" {
		return "-"
	}

	parts := make([]string, 0, 3)

	for _, p := range []string{body.Country, body.RegionName, body.City} {
		p = strings.TrimSpace(p)
		if p != "" {
			parts = append(parts, p)
		}
	}

	if len(parts) == 0 {
		return "-"
	}

	return strings.Join(parts, " ")
}
