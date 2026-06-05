package models

import (
	"slices"
	"testing"
)

func TestDefaultWebDAVAllowedSuffixesIncludeHLSPlaylist(t *testing.T) {
	if !slices.Contains(DefaultWebDAVAllowedSuffixes, ".m3u8") {
		t.Fatalf("expected default WebDAV suffixes to include .m3u8, got %v", DefaultWebDAVAllowedSuffixes)
	}
}

func TestSettingAdditionDefaultsIncludeHLSPlaylist(t *testing.T) {
	addition := SettingAddition{}
	addition.ApplyDefaultsForWrite()

	if !slices.Contains(addition.WebDAVAllowedSuffixes, ".m3u8") {
		t.Fatalf("expected defaulted WebDAV suffixes to include .m3u8, got %v", addition.WebDAVAllowedSuffixes)
	}
}
