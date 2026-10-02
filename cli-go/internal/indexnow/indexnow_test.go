package indexnow

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

type roundTrip func(*http.Request) (*http.Response, error)

func (f roundTrip) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func respond(status int, body []byte) *http.Response {
	return &http.Response{StatusCode: status, Header: http.Header{}, Body: io.NopCloser(bytes.NewReader(body))}
}

func TestKeys(t *testing.T) {
	k, err := GenerateKey()
	if err != nil || len(k) != 32 || !ValidKey(k) {
		t.Fatalf("%q %v", k, err)
	}
	for _, bad := range []string{"short", strings.Repeat("a", 129), "has space key", "slash/key1"} {
		if ValidKey(bad) {
			t.Errorf("accepted %q", bad)
		}
	}
	if DefaultKeyLocation("example.com", k) != "https://example.com/"+k+".txt" {
		t.Fatal("key location")
	}
}

func TestPlanGroupsAndBatches(t *testing.T) {
	var urls []string
	for i := 0; i < MaxBatch+1; i++ {
		urls = append(urls, fmt.Sprintf("https://a.example/%d", i))
	}
	urls = append(urls, "https://B.example/x", "https://a.example/0")
	batches, err := Plan(urls, 0)
	if err != nil || len(batches) != 3 || len(batches[0].URLs) != MaxBatch || len(batches[1].URLs) != 1 || batches[2].Host != "b.example" {
		t.Fatalf("%d batches %v", len(batches), err)
	}
	if _, err := Plan([]string{"ftp://x/y"}, 0); err == nil {
		t.Fatal("accepted non-http URL")
	}
	if _, err := Plan([]string{"  "}, 0); err == nil {
		t.Fatal("accepted empty input")
	}
}

func TestSubmitAndStatuses(t *testing.T) {
	var got map[string]any
	h := &http.Client{Transport: roundTrip(func(r *http.Request) (*http.Response, error) {
		if r.Method != "POST" || r.URL.String() != Endpoints["indexnow"] || r.Header.Get("Content-Type") != "application/json; charset=utf-8" {
			t.Fatalf("bad request %s %s", r.Method, r.URL)
		}
		_ = json.NewDecoder(r.Body).Decode(&got)
		return respond(202, nil), nil
	})}
	res, err := Submit(context.Background(), h, Endpoints["indexnow"], Batch{Host: "example.com", URLs: []string{"https://example.com/a"}}, "abcdef12", "https://example.com/abcdef12.txt")
	if err != nil || !res.OK || res.Status != 202 || got["keyLocation"] != "https://example.com/abcdef12.txt" || got["host"] != "example.com" {
		t.Fatalf("%+v %v %v", res, err, got)
	}
	for status, ok := range map[int]bool{200: true, 202: true, 400: false, 403: false, 422: false, 429: false} {
		if _, gotOK := Meaning(status); gotOK != ok {
			t.Errorf("status %d ok=%v", status, gotOK)
		}
	}
}

func TestCheckKey(t *testing.T) {
	for name, tc := range map[string]struct {
		status int
		body   string
		ok     bool
	}{"match": {200, "abcdef12\n", true}, "mismatch": {200, "other", false}, "missing": {404, "", false}, "redirect": {301, "", false}} {
		t.Run(name, func(t *testing.T) {
			h := &http.Client{Transport: roundTrip(func(r *http.Request) (*http.Response, error) {
				res := respond(tc.status, []byte(tc.body))
				res.Header.Set("Location", "https://elsewhere/")
				return res, nil
			})}
			err := CheckKey(context.Background(), h, "https://example.com/abcdef12.txt", "abcdef12")
			if (err == nil) != tc.ok {
				t.Fatalf("err=%v", err)
			}
		})
	}
}

func TestReadSitemapIndexGzipAndChangedSince(t *testing.T) {
	index := `<?xml version="1.0"?><sitemapindex xmlns="http://www.sitemaps.org/schemas/sitemap/0.9"><sitemap><loc>https://example.com/s1.xml.gz</loc></sitemap></sitemapindex>`
	set := `<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9"><url><loc>https://example.com/new</loc><lastmod>2026-09-20</lastmod></url><url><loc>https://example.com/old</loc><lastmod>2026-01-01T10:00:00+00:00</lastmod></url><url><loc>https://example.com/unknown</loc></url></urlset>`
	var gz bytes.Buffer
	z := gzip.NewWriter(&gz)
	z.Write([]byte(set))
	z.Close()
	h := &http.Client{Transport: roundTrip(func(r *http.Request) (*http.Response, error) {
		switch r.URL.Path {
		case "/sitemap.xml":
			return respond(200, []byte(index)), nil
		case "/s1.xml.gz":
			return respond(200, gz.Bytes()), nil
		}
		return respond(404, nil), nil
	})}
	entries, err := ReadSitemap(context.Background(), h, "https://example.com/sitemap.xml")
	if err != nil || len(entries) != 3 {
		t.Fatalf("%v %v", entries, err)
	}
	kept, missing := ChangedSince(entries, time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC))
	if len(kept) != 1 || kept[0] != "https://example.com/new" || missing != 1 {
		t.Fatalf("kept=%v missing=%d", kept, missing)
	}
	if _, err := ReadSitemap(context.Background(), h, "https://example.com/missing.xml"); err == nil {
		t.Fatal("404 sitemap accepted")
	}
}
