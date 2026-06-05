package utils

import "testing"

func TestCheckIsPathRejectsDecodedTraversalSegments(t *testing.T) {
	tests := []struct {
		name string
		path string
	}{
		{name: "encoded parent", path: "/a/%2e%2e/b"},
		{name: "encoded current", path: "/a/%2E/b"},
		{name: "plain current", path: "/a/./b"},
		{name: "encoded slash", path: "/a%2Fb/c"},
		{name: "encoded backslash", path: "/a/%5c/b"},
		{name: "encoded null", path: "/a/%00/b"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if CheckIsPath(tt.path) {
				t.Fatalf("expected path %q to be rejected", tt.path)
			}
		})
	}
}

func TestCheckIsPathAllowsNormalEscapedPath(t *testing.T) {
	if !CheckIsPath("/a/%E4%B8%AD%E6%96%87/b") {
		t.Fatal("expected escaped normal path to be accepted")
	}
}

func TestCheckIsPathAllowsLiteralPercent(t *testing.T) {
	for _, path := range []string{"/a%b/file", "/a%25b/file"} {
		t.Run(path, func(t *testing.T) {
			if !CheckIsPath(path) {
				t.Fatalf("expected path %q to be accepted", path)
			}
		})
	}
}

func TestCheckIsPathRejectsInvalidEscapedUTF8(t *testing.T) {
	tests := []string{
		"/a/%ff/b",
		"/a/%e4%b8/b",
		"/a/%c0%af/b",
	}

	for _, path := range tests {
		t.Run(path, func(t *testing.T) {
			if CheckIsPath(path) {
				t.Fatalf("expected path %q to be rejected", path)
			}
		})
	}
}

func TestNormalizeStoragePath(t *testing.T) {
	tests := []struct {
		name string
		path string
		want string
	}{
		{name: "plain path", path: "/a/b", want: "/a/b"},
		{name: "trailing slash", path: "/a/b/", want: "/a/b"},
		{name: "repeated slash", path: "/a//b///c", want: "/a/b/c"},
		{name: "escaped segment", path: "/a/%E4%B8%AD%E6%96%87/b", want: "/a/中文/b"},
		{name: "trim segment", path: "/a%20/b%20", want: "/a/b"},
		{name: "sanitize reserved chars", path: "/a:b/file%3fname|ok", want: "/a_b/file_name丨ok"},
		{name: "literal percent", path: "/a%b/file", want: "/a%25b/file"},
		{name: "escaped percent", path: "/a%25b/file", want: "/a%25b/file"},
		{name: "double escaped separator text", path: "/a%252Fb/file", want: "/a%252Fb/file"},
		{name: "root path", path: "/", want: "/"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NormalizeStoragePath(tt.path)
			if err != nil {
				t.Fatalf("normalize path: %v", err)
			}

			if got != tt.want {
				t.Fatalf("expected %q, got %q", tt.want, got)
			}
		})
	}
}

func TestNormalizeStoragePathIsIdempotent(t *testing.T) {
	for _, path := range []string{
		"/a/b",
		"/a%25b/file",
		"/a%252Fb/file",
		"/media/中文/a_b",
	} {
		t.Run(path, func(t *testing.T) {
			once, err := NormalizeStoragePath(path)
			if err != nil {
				t.Fatalf("normalize once: %v", err)
			}

			twice, err := NormalizeStoragePath(once)
			if err != nil {
				t.Fatalf("normalize twice: %v", err)
			}

			if twice != once {
				t.Fatalf("expected idempotent normalized path %q, got %q", once, twice)
			}
		})
	}
}

func TestNormalizeStoragePathPartsReturnsDisplaySegments(t *testing.T) {
	normalizedPath, parts, err := NormalizeStoragePathParts("/literal/a%252Fb/file%25name")
	if err != nil {
		t.Fatalf("normalize path parts: %v", err)
	}

	if normalizedPath != "/literal/a%252Fb/file%25name" {
		t.Fatalf("expected canonical path, got %q", normalizedPath)
	}

	want := []string{"literal", "a%2Fb", "file%name"}
	if len(parts) != len(want) {
		t.Fatalf("expected display parts %v, got %v", want, parts)
	}

	for index := range want {
		if parts[index] != want[index] {
			t.Fatalf("expected display parts %v, got %v", want, parts)
		}
	}
}

func TestSplitNormalizedStoragePathKeepsEscapedPercent(t *testing.T) {
	got := SplitNormalizedStoragePath("/a%25b/file")
	want := []string{"a%25b", "file"}

	if len(got) != len(want) {
		t.Fatalf("expected %v, got %v", want, got)
	}

	for index := range want {
		if got[index] != want[index] {
			t.Fatalf("expected %v, got %v", want, got)
		}
	}
}

func TestJoinStoragePathEscapesDisplayNamePercent(t *testing.T) {
	got, err := JoinStoragePath("/subscriptions", "a%2Fb")
	if err != nil {
		t.Fatalf("join storage path: %v", err)
	}

	if got != "/subscriptions/a%252Fb" {
		t.Fatalf("expected escaped literal percent path, got %q", got)
	}
}

func TestJoinStoragePathNormalizesParentAndName(t *testing.T) {
	got, err := JoinStoragePath("/media//%E4%B8%AD%E6%96%87%20/a%3ab", "file%name")
	if err != nil {
		t.Fatalf("join storage path: %v", err)
	}

	if got != "/media/中文/a_b/file%25name" {
		t.Fatalf("expected normalized joined path, got %q", got)
	}
}

func TestJoinStoragePathRejectsInvalidInput(t *testing.T) {
	if _, err := JoinStoragePath("/a/%2F/b", "file"); err == nil {
		t.Fatal("expected invalid parent path to be rejected")
	}

	if _, err := JoinStoragePath("/a", "  "); err == nil {
		t.Fatal("expected blank display name to be rejected")
	}
}

func TestNormalizeStoragePathRejectsInvalidPath(t *testing.T) {
	if _, err := NormalizeStoragePath("/a/%2F/b"); err == nil {
		t.Fatal("expected invalid escaped slash to be rejected")
	}

	if _, err := NormalizeStoragePath("/a%2Fb/c"); err == nil {
		t.Fatal("expected invalid escaped slash in segment to be rejected")
	}
}

func TestNormalizeStoragePathRejectsInvalidEscapedUTF8(t *testing.T) {
	if _, err := NormalizeStoragePath("/a/%ff/b"); err == nil {
		t.Fatal("expected invalid escaped UTF-8 to be rejected")
	}
}

func TestNormalizeStoragePathRejectsBlankSanitizedSegment(t *testing.T) {
	if _, err := NormalizeStoragePath("/a/%20%20/b"); err == nil {
		t.Fatal("expected blank sanitized segment to be rejected")
	}
}

func TestPathEscapePreservesRootPath(t *testing.T) {
	if got := PathEscape("/"); got != "/" {
		t.Fatalf("expected root path to remain '/', got %q", got)
	}
}

func TestPathEscapeEncodesSegments(t *testing.T) {
	got := PathEscape("/api/file/open", "/中文 目录/movie #1.mkv")
	want := "/api/file/open/%E4%B8%AD%E6%96%87%20%E7%9B%AE%E5%BD%95/movie%20%231.mkv"

	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}
