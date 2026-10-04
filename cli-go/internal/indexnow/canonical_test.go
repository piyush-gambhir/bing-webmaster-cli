package indexnow

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

type redirects map[string]string

func (r redirects) RoundTrip(req *http.Request) (*http.Response, error) {
	res := &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader("ok")), Request: req}
	if to, ok := r[req.URL.Host]; ok {
		res.StatusCode = 308
		res.Header.Set("Location", to)
	}
	return res, nil
}

func TestCanonicalHostFollowsOnlyTheWWWVariant(t *testing.T) {
	ctx := context.Background()
	h := &http.Client{Transport: redirects{"example.com": "https://www.example.com/"}}
	if got, err := CanonicalHost(ctx, h, "example.com"); err != nil || got != "www.example.com" {
		t.Fatalf("apex to www: %q %v", got, err)
	}
	h = &http.Client{Transport: redirects{"www.example.com": "https://example.com/"}}
	if got, _ := CanonicalHost(ctx, h, "www.example.com"); got != "example.com" {
		t.Fatalf("www to apex: %q", got)
	}
	h = &http.Client{Transport: redirects{"example.com": "https://login.other.net/"}}
	if got, _ := CanonicalHost(ctx, h, "example.com"); got != "example.com" {
		t.Fatalf("unrelated redirect must be ignored: %q", got)
	}
	h = &http.Client{Transport: redirects{}}
	if got, _ := CanonicalHost(ctx, h, "example.com"); got != "example.com" {
		t.Fatalf("no redirect: %q", got)
	}
}
