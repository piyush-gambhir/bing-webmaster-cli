// Package indexnow implements the IndexNow protocol: key generation and checks,
// host-grouped batch submission, and reading URLs from sitemaps. IndexNow needs
// no Bing credentials; its key is public and hosted on the website.
package indexnow

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

// Endpoints are the documented shared endpoints; a submission to one reaches
// every participating engine.
var Endpoints = map[string]string{
	"indexnow": "https://api.indexnow.org/indexnow",
	"bing":     "https://www.bing.com/indexnow",
}

const MaxBatch = 10000

var keyPattern = regexp.MustCompile(`^[A-Za-z0-9-]{8,128}$`)

// GenerateKey returns a 32-character hexadecimal key, which satisfies both the
// protocol page ("hexadecimal") and the FAQ (8-128 letters, digits, hyphens).
func GenerateKey() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func ValidKey(k string) bool { return keyPattern.MatchString(k) }

func DefaultKeyLocation(host, key string) string { return "https://" + host + "/" + key + ".txt" }

// HostOf returns the lowercased host of an absolute http(s) URL.
func HostOf(raw string) (string, error) {
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return "", fmt.Errorf("not an absolute http(s) URL: %q", raw)
	}
	return strings.ToLower(u.Host), nil
}

// CheckKey fetches the key file and compares its content. Redirects are reported
// rather than followed because engines may not follow them.
func CheckKey(ctx context.Context, h *http.Client, keyLocation, key string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, keyLocation, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "bwt")
	client := *h
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	res, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("could not fetch %s: %w", keyLocation, err)
	}
	defer res.Body.Close()
	if res.StatusCode >= 300 && res.StatusCode < 400 {
		loc := res.Header.Get("Location")
		msg := fmt.Sprintf("%s redirects to %s (HTTP %d); host the key file at the exact location", keyLocation, loc, res.StatusCode)
		if u, err := url.Parse(loc); err == nil && u.Host != "" {
			msg += fmt.Sprintf(". If your pages are served from %s, use --host %s so the key and the submitted URLs share a host", u.Host, u.Host)
		}
		return errors.New(msg)
	}
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("%s returned HTTP %d; upload a text file containing only the key", keyLocation, res.StatusCode)
	}
	b, err := io.ReadAll(io.LimitReader(res.Body, 1024))
	if err != nil {
		return err
	}
	if got := strings.TrimSpace(string(b)); got != key {
		return fmt.Errorf("%s does not contain the expected key", keyLocation)
	}
	return nil
}

type Batch struct {
	Host string   `json:"host" yaml:"host"`
	URLs []string `json:"urls" yaml:"urls"`
}

// Plan validates URLs, drops duplicates, and groups them by host into batches of
// at most max URLs, keeping input order.
func Plan(urls []string, max int) ([]Batch, error) {
	if max <= 0 || max > MaxBatch {
		max = MaxBatch
	}
	seen := map[string]bool{}
	byHost := map[string][]string{}
	var hosts []string
	for _, raw := range urls {
		raw = strings.TrimSpace(raw)
		if raw == "" || seen[raw] {
			continue
		}
		host, err := HostOf(raw)
		if err != nil {
			return nil, err
		}
		seen[raw] = true
		if _, ok := byHost[host]; !ok {
			hosts = append(hosts, host)
		}
		byHost[host] = append(byHost[host], raw)
	}
	var batches []Batch
	for _, host := range hosts {
		list := byHost[host]
		for len(list) > 0 {
			n := min(max, len(list))
			batches = append(batches, Batch{Host: host, URLs: list[:n]})
			list = list[n:]
		}
	}
	if len(batches) == 0 {
		return nil, errors.New("no URLs to submit")
	}
	return batches, nil
}

// Meaning explains an IndexNow response status; ok is true for 200 and 202.
func Meaning(status int) (string, bool) {
	switch status {
	case 200:
		return "received", true
	case 202:
		return "received; key validation pending", true
	case 400:
		return "bad request: invalid format", false
	case 403:
		return "key not valid: the key file was not found or does not match", false
	case 422:
		return "unprocessable: URLs do not belong to the host, or the key does not match the protocol", false
	case 429:
		return "too many requests (potential spam); slow down", false
	default:
		return http.StatusText(status), false
	}
}

type Result struct {
	Host    string `json:"host" yaml:"host"`
	Count   int    `json:"count" yaml:"count"`
	Status  int    `json:"http_status" yaml:"http_status"`
	Meaning string `json:"meaning" yaml:"meaning"`
	OK      bool   `json:"ok" yaml:"ok"`
}

// Submit posts one batch. It never retries.
func Submit(ctx context.Context, h *http.Client, endpoint string, b Batch, key, keyLocation string) (Result, error) {
	body := map[string]any{"host": b.Host, "key": key, "urlList": b.URLs}
	if keyLocation != "" {
		body["keyLocation"] = keyLocation
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return Result{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return Result{}, err
	}
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	req.Header.Set("User-Agent", "bwt")
	client := *h
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	res, err := client.Do(req)
	if err != nil {
		return Result{}, fmt.Errorf("IndexNow request failed: %w", err)
	}
	defer res.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(res.Body, 64<<10))
	meaning, ok := Meaning(res.StatusCode)
	return Result{Host: b.Host, Count: len(b.URLs), Status: res.StatusCode, Meaning: meaning, OK: ok}, nil
}

// Entry is one sitemap URL.
type Entry struct {
	Loc     string `xml:"loc"`
	LastMod string `xml:"lastmod"`
}

const (
	maxSitemapBytes = 50 << 20
	maxChildren     = 1000
	maxEntries      = 500000
)

// ReadSitemap fetches a sitemap or sitemap index (one level of nesting) and
// returns its URLs. Gzip content is detected by its magic bytes.
func ReadSitemap(ctx context.Context, h *http.Client, sitemapURL string) ([]Entry, error) {
	root, children, err := readOne(ctx, h, sitemapURL)
	if err != nil {
		return nil, err
	}
	if len(children) == 0 {
		return root, nil
	}
	if len(children) > maxChildren {
		return nil, fmt.Errorf("sitemap index lists %d sitemaps; the limit is %d", len(children), maxChildren)
	}
	var all []Entry
	for _, child := range children {
		entries, nested, err := readOne(ctx, h, child.Loc)
		if err != nil {
			return nil, err
		}
		if len(nested) > 0 {
			return nil, fmt.Errorf("%s is a nested sitemap index; only one level is supported", child.Loc)
		}
		all = append(all, entries...)
		if len(all) > maxEntries {
			return nil, fmt.Errorf("sitemaps list more than %d URLs", maxEntries)
		}
	}
	return all, nil
}

func readOne(ctx context.Context, h *http.Client, sitemapURL string) (urls, sitemaps []Entry, err error) {
	if _, err := HostOf(sitemapURL); err != nil {
		return nil, nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, sitemapURL, nil)
	if err != nil {
		return nil, nil, err
	}
	req.Header.Set("User-Agent", "bwt")
	res, err := h.Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("could not fetch sitemap %s: %w", sitemapURL, err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, nil, fmt.Errorf("sitemap %s returned HTTP %d", sitemapURL, res.StatusCode)
	}
	b, err := io.ReadAll(io.LimitReader(res.Body, maxSitemapBytes+1))
	if err != nil {
		return nil, nil, err
	}
	if len(b) >= 2 && b[0] == 0x1f && b[1] == 0x8b {
		z, err := gzip.NewReader(bytes.NewReader(b))
		if err != nil {
			return nil, nil, fmt.Errorf("sitemap %s: %w", sitemapURL, err)
		}
		b, err = io.ReadAll(io.LimitReader(z, maxSitemapBytes+1))
		if err != nil {
			return nil, nil, fmt.Errorf("sitemap %s: %w", sitemapURL, err)
		}
	}
	if len(b) > maxSitemapBytes {
		return nil, nil, fmt.Errorf("sitemap %s exceeds 50 MB", sitemapURL)
	}
	var doc struct {
		XMLName  xml.Name
		URLs     []Entry `xml:"url"`
		Sitemaps []Entry `xml:"sitemap"`
	}
	if err := xml.Unmarshal(b, &doc); err != nil {
		return nil, nil, fmt.Errorf("sitemap %s is not valid XML", sitemapURL)
	}
	switch doc.XMLName.Local {
	case "urlset":
		return doc.URLs, nil, nil
	case "sitemapindex":
		return nil, doc.Sitemaps, nil
	default:
		return nil, nil, fmt.Errorf("sitemap %s has root <%s>, expected urlset or sitemapindex", sitemapURL, doc.XMLName.Local)
	}
}

var lastmodLayouts = []string{time.RFC3339Nano, "2006-01-02T15:04Z07:00", "2006-01-02"}

// ParseLastMod reads a W3C datetime lastmod value.
func ParseLastMod(s string) (time.Time, bool) {
	s = strings.TrimSpace(s)
	for _, layout := range lastmodLayouts {
		if t, err := time.Parse(layout, s); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

// ChangedSince keeps entries whose lastmod is on or after since. Entries without
// a usable lastmod are dropped and counted, never assumed changed.
func ChangedSince(entries []Entry, since time.Time) (kept []string, missingLastmod int) {
	for _, e := range entries {
		t, ok := ParseLastMod(e.LastMod)
		if !ok {
			missingLastmod++
			continue
		}
		if !t.Before(since) {
			kept = append(kept, strings.TrimSpace(e.Loc))
		}
	}
	return kept, missingLastmod
}
