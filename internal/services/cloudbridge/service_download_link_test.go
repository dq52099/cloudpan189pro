package cloudbridge

import (
	stdctx "context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/patrickmn/go-cache"
	"github.com/xxcheng123/cloudpan189-share/internal/framework/context"
	"github.com/xxcheng123/cloudpan189-share/internal/shared"
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
