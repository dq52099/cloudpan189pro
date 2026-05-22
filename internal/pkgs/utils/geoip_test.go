package utils

import (
	"io"
	"net/http"
	"strings"
	"testing"
)

type geoIPRoundTripper func(*http.Request) (*http.Response, error)

func (f geoIPRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestFetchGeoLocationRejectsOversizedResponse(t *testing.T) {
	oldClient := geoIPClient

	t.Cleanup(func() {
		geoIPClient = oldClient
	})

	geoIPClient = &http.Client{
		Transport: geoIPRoundTripper(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(strings.Repeat("x", maxGeoIPResponseSize+1))),
			}, nil
		}),
	}

	if got := fetchGeoLocation("8.8.8.8"); got != "-" {
		t.Fatalf("expected oversized response to fail closed, got %q", got)
	}
}
