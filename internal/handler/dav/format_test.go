package dav

import "testing"

func TestGetContentTypeRecognizesWebDAVVideoSuffixes(t *testing.T) {
	engine := &workEngine{}

	tests := []struct {
		name     string
		filename string
		want     string
	}{
		{name: "hls playlist", filename: "movie.m3u8", want: "application/vnd.apple.mpegurl"},
		{name: "m4v video", filename: "movie.m4v", want: "video/x-m4v"},
		{name: "mpeg video", filename: "movie.mpeg", want: "video/mpeg"},
		{name: "mpeg uppercase", filename: "MOVIE.MPG", want: "video/mpeg"},
		{name: "transport stream", filename: "segment.ts", want: "video/mp2t"},
		{name: "m2ts stream", filename: "movie.m2ts", want: "video/mp2t"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := engine.getContentType(tt.filename); got != tt.want {
				t.Fatalf("expected %q, got %q", tt.want, got)
			}
		})
	}
}

func TestEncodeWebDAVPathEscapesPercentNamesOnce(t *testing.T) {
	engine := &workEngine{}

	tests := []struct {
		name string
		path string
		want string
	}{
		{name: "literal percent", path: "/dav/percent/a%b", want: "/dav/percent/a%25b"},
		{name: "literal escaped separator text", path: "/dav/literal/a%2Fb", want: "/dav/literal/a%252Fb"},
		{name: "directory trailing slash", path: "/dav/literal/a%2Fb/", want: "/dav/literal/a%252Fb/"},
		{name: "chinese and space", path: "/dav/中文 目录/movie #1.mkv", want: "/dav/%E4%B8%AD%E6%96%87%20%E7%9B%AE%E5%BD%95/movie%20%231.mkv"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := engine.encodeWebDAVPath(tt.path); got != tt.want {
				t.Fatalf("expected %q, got %q", tt.want, got)
			}
		})
	}
}

func TestBuildChildPathPreservesPercentDisplayNames(t *testing.T) {
	engine := &workEngine{}

	if got, want := engine.buildChildPath("/dav/literal/a%2Fb/", "child%name", false), "/dav/literal/a%2Fb/child%name"; got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}

	if got, want := engine.encodeWebDAVPath(engine.buildChildPath("/dav/literal/a%2Fb/", "child%2Fdir", true)), "/dav/literal/a%252Fb/child%252Fdir/"; got != want {
		t.Fatalf("expected encoded child path %q, got %q", want, got)
	}
}
