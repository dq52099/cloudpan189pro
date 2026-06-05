package shared

import (
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/xxcheng123/cloudpan189-share/internal/repository/models"
)

func TestJoinDownloadURLWithBaseUsesProvidedBaseURL(t *testing.T) {
	values := url.Values{"sign": []string{"ok"}}

	got := JoinDownloadURLWithBase("https://media.example.test", 123, values)

	if !strings.HasPrefix(got, "https://media.example.test/api/file/download/123?") {
		t.Fatalf("expected provided base URL in download URL, got %q", got)
	}

	if !strings.Contains(got, "sign=ok") {
		t.Fatalf("expected query values in download URL, got %q", got)
	}
}

func TestJoinDownloadURLWithBaseTrimsTrailingSlash(t *testing.T) {
	got := JoinDownloadURLWithBase("https://media.example.test/", 123, nil)

	if !strings.HasPrefix(got, "https://media.example.test/api/file/download/123?") {
		t.Fatalf("expected trailing slash to be trimmed from base URL, got %q", got)
	}

	if strings.Contains(got, "example.test//api") {
		t.Fatalf("expected no doubled path slash in download URL, got %q", got)
	}
}

func TestJoinDownloadURLWithBaseKeepsPathPrefix(t *testing.T) {
	got := JoinDownloadURLWithBase("https://media.example.test/cloudpan/", 123, nil)

	if !strings.HasPrefix(got, "https://media.example.test/cloudpan/api/file/download/123?") {
		t.Fatalf("expected base URL path prefix to remain, got %q", got)
	}

	if strings.Contains(got, "cloudpan//api") {
		t.Fatalf("expected no doubled path slash after path prefix, got %q", got)
	}
}

func TestJoinDownloadURLWithBaseFallsBackToDefaultBaseURL(t *testing.T) {
	got := JoinDownloadURLWithBase("", 123, nil)

	if !strings.HasPrefix(got, "http://localhost:12395/api/file/download/123?") {
		t.Fatalf("expected default base URL in download URL, got %q", got)
	}
}

func TestSharedConfigConcurrentAccess(t *testing.T) {
	oldSaltKey := GetSaltKey()
	oldBaseURL := GetBaseURL()
	oldEnableAuth := IsAuthEnabled()
	oldAddition := GetSettingAddition()
	oldMediaConfig := GetMediaConfig()

	t.Cleanup(func() {
		SetSetting(oldSaltKey, oldBaseURL, oldEnableAuth, oldAddition)
		SetMediaConfig(oldMediaConfig)
	})

	SetSetting("initial-salt", "https://initial.example.test", true, models.SettingAddition{
		WebDAVAllowedSuffixes: []string{".mp4"},
	})
	SetMediaConfig(&models.MediaConfig{
		Enable:           true,
		StoragePath:      "/tmp/media-initial",
		IncludedSuffixes: []string{".mp4"},
		BaseURL:          "https://media-initial.example.test",
	})

	var (
		wg    sync.WaitGroup
		start = make(chan struct{})
	)

	for writer := 0; writer < 4; writer++ {
		wg.Add(1)
		go func(writer int) {
			defer wg.Done()

			<-start

			for i := 0; i < 500; i++ {
				SetSetting(
					"salt",
					"https://example.test",
					i%2 == 0,
					models.SettingAddition{
						WebDAVAllowedSuffixes: []string{".mp4", ".mkv"},
						TaskThreadCount:       i + 1,
					},
				)
				SetMediaConfig(&models.MediaConfig{
					Enable:           true,
					StoragePath:      "/tmp/media",
					IncludedSuffixes: []string{".mp4", ".mkv"},
					BaseURL:          "https://media.example.test",
				})
				SetMediaConfigLastRebuildTime(time.Unix(int64(writer*500+i), 0))
			}
		}(writer)
	}

	for reader := 0; reader < 8; reader++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			<-start

			for i := 0; i < 1000; i++ {
				_ = JoinDownloadURL(123, url.Values{"sign": []string{"ok"}})
				_ = GetSaltKey()
				_ = GetBaseURL()
				_ = IsAuthEnabled()

				addition := GetSettingAddition()
				if len(addition.WebDAVAllowedSuffixes) > 0 {
					addition.WebDAVAllowedSuffixes[0] = ".mutated"
				}

				mediaConfig := GetMediaConfig()
				if mediaConfig != nil && len(mediaConfig.IncludedSuffixes) > 0 {
					mediaConfig.IncludedSuffixes[0] = ".mutated"
				}
			}
		}()
	}

	close(start)
	wg.Wait()

	addition := GetSettingAddition()
	if len(addition.WebDAVAllowedSuffixes) > 0 && addition.WebDAVAllowedSuffixes[0] == ".mutated" {
		t.Fatal("expected shared setting suffixes to be isolated from caller mutation")
	}

	mediaConfig := GetMediaConfig()
	if mediaConfig != nil && len(mediaConfig.IncludedSuffixes) > 0 && mediaConfig.IncludedSuffixes[0] == ".mutated" {
		t.Fatal("expected shared media suffixes to be isolated from caller mutation")
	}
}
