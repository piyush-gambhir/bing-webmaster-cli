package cmd

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/piyush-gambhir/bing-webmaster-cli/cli-go/internal/build"
	"github.com/piyush-gambhir/bing-webmaster-cli/cli-go/internal/config"
	"github.com/piyush-gambhir/bing-webmaster-cli/cli-go/internal/update"
)

const noticeText = "A new version of bwt is available"

// updateEnv isolates config, pins the running version, and clears every
// variable that turns the notice off.
func updateEnv(t *testing.T, version string) string {
	t.Helper()
	isolate(t)
	old := build.Version
	build.Version = version
	t.Cleanup(func() { build.Version = old })
	for _, name := range []string{"CI", "BWT_NO_UPDATE_NOTIFIER", "NO_UPDATE_NOTIFIER", "GOBIN", "GOPATH"} {
		t.Setenv(name, "")
	}
	dir, err := config.Dir()
	if err != nil {
		t.Fatal(err)
	}
	return dir
}

// gh fakes github.com: releases/latest redirects to tag (any other request,
// such as following that redirect, fails the test), and assets are served
// from files.
func gh(t *testing.T, tag string, files map[string][]byte) *fake {
	return &fake{handle: func(r *http.Request, body string) *http.Response {
		switch {
		case r.URL.Host == "github.com" && r.URL.Path == "/"+update.Repo+"/releases/latest":
			res := reply(http.StatusFound, "")
			res.Header.Set("Location", "https://github.com/"+update.Repo+"/releases/tag/"+tag)
			return res
		case r.URL.Host == "github.com" && strings.HasPrefix(r.URL.Path, "/"+update.Repo+"/releases/download/"+tag+"/"):
			if b, ok := files[filepath.Base(r.URL.Path)]; ok {
				return reply(200, string(b))
			}
		}
		t.Errorf("unexpected request %s", r.URL)
		return reply(404, "")
	}}
}

// runUpdateApp runs bwt with stderr treated as a terminal (or not) and waits
// for any background release check.
func runUpdateApp(t *testing.T, f *fake, tty bool, exe string, args ...string) result {
	t.Helper()
	var out, errOut bytes.Buffer
	a := &app{in: strings.NewReader(""), out: &out, errOut: &errOut, transport: f.transport(), exePath: exe,
		stderrTTY: func() bool { return tty }}
	root := newRoot(a)
	root.SetArgs(args)
	err := root.ExecuteContext(context.Background())
	a.checks.Wait()
	return result{err, out.String(), errOut.String()}
}

func TestUpdateNoticeChecksOnceAndShowsOncePerDay(t *testing.T) {
	dir := updateEnv(t, "0.1.3")
	f := gh(t, "v0.1.4", nil)
	exe := filepath.Join(t.TempDir(), "bwt")
	runUpdateApp(t, f, true, exe, "api", "methods", "-o", "json")
	c := update.ReadCache(dir)
	if f.count() != 1 || c.LatestVersion != "0.1.4" || c.CheckedAt.IsZero() {
		t.Fatalf("background check: %d requests, cache %+v", f.count(), c)
	}
	// Whether the first run printed depends on timing; start from "not shown".
	c.NotifiedVersion, c.NotifiedAt = "", time.Time{}
	update.WriteCache(dir, c)

	res := runUpdateApp(t, f, true, exe, "api", "methods", "-o", "json")
	want := "\nA new version of bwt is available: v0.1.3 -> v0.1.4\nUpdate with: bwt update\nRelease notes: https://github.com/" + update.Repo + "/releases/tag/v0.1.4\n"
	if res.err != nil || !strings.HasSuffix(res.errOut, want) || strings.Contains(res.out, noticeText) || f.count() != 1 {
		t.Fatalf("notice: %v stderr=%q requests=%d", res.err, res.errOut, f.count())
	}
	if res = runUpdateApp(t, f, true, exe, "api", "methods"); strings.Contains(res.errOut, noticeText) {
		t.Fatal("notice repeated within 24h")
	}
	c = update.ReadCache(dir)
	c.NotifiedAt = time.Now().Add(-25 * time.Hour)
	update.WriteCache(dir, c)
	if res = runUpdateApp(t, f, true, exe, "api", "methods"); !strings.Contains(res.errOut, noticeText) {
		t.Fatal("notice not shown again after 24h")
	}
	if f.count() != 1 {
		t.Fatalf("cached check went to the network: %d requests", f.count())
	}

	// In a Go bin directory the notice points at the source build.
	t.Setenv("GOBIN", filepath.Dir(exe))
	c = update.ReadCache(dir)
	c.NotifiedAt = time.Time{}
	update.WriteCache(dir, c)
	if res = runUpdateApp(t, f, true, exe, "api", "methods"); !strings.Contains(res.errOut, "Update with: "+update.SourceUpdate+"\n") {
		t.Fatalf("go install notice: %q", res.errOut)
	}
}

func TestUpdateNoticeSuppressed(t *testing.T) {
	cases := []struct {
		name    string
		tty     bool
		env     map[string]string
		version string
		args    []string
	}{
		{"stderr not a terminal", false, nil, "0.1.3", []string{"api", "methods"}},
		{"CI", true, map[string]string{"CI": "true"}, "0.1.3", []string{"api", "methods"}},
		{"BWT_NO_UPDATE_NOTIFIER", true, map[string]string{"BWT_NO_UPDATE_NOTIFIER": "1"}, "0.1.3", []string{"api", "methods"}},
		{"NO_UPDATE_NOTIFIER", true, map[string]string{"NO_UPDATE_NOTIFIER": "yes"}, "0.1.3", []string{"api", "methods"}},
		{"--quiet", true, nil, "0.1.3", []string{"api", "methods", "--quiet"}},
		{"BWT_QUIET", true, map[string]string{"BWT_QUIET": "1"}, "0.1.3", []string{"api", "methods"}},
		{"dev build", true, nil, "dev", []string{"api", "methods"}},
		{"empty version", true, nil, "", []string{"api", "methods"}},
		{"version", true, nil, "0.1.3", []string{"version"}},
		{"completion", true, nil, "0.1.3", []string{"completion", "bash"}},
		{"help", true, nil, "0.1.3", []string{"help"}},
		{"__complete", true, nil, "0.1.3", []string{"__complete", "api", ""}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := updateEnv(t, tc.version)
			for k, v := range tc.env {
				t.Setenv(k, v)
			}
			f := &fake{} // any request fails the run's check and is counted
			exe := filepath.Join(t.TempDir(), "bwt")
			if res := runUpdateApp(t, f, tc.tty, exe, tc.args...); res.err != nil || f.count() != 0 || strings.Contains(res.errOut, noticeText) {
				t.Fatalf("no cache: %v requests=%d stderr=%q", res.err, f.count(), res.errOut)
			}
			// A fresh cache with a newer release must not print either.
			update.WriteCache(dir, update.Cache{CheckedAt: time.Now(), LatestVersion: "9.9.9"})
			if res := runUpdateApp(t, f, tc.tty, exe, tc.args...); res.err != nil || f.count() != 0 || strings.Contains(res.errOut+res.out, noticeText) {
				t.Fatalf("cached: %v requests=%d stderr=%q", res.err, f.count(), res.errOut)
			}
		})
	}
}

func TestUpdateCheckJSON(t *testing.T) {
	dir := updateEnv(t, "0.1.3")
	// --check bypasses a fresh cache that says nothing is new.
	update.WriteCache(dir, update.Cache{CheckedAt: time.Now(), LatestVersion: "0.1.3"})
	f := gh(t, "v0.1.4", nil)
	exe := filepath.Join(t.TempDir(), "bwt")
	res := runUpdateApp(t, f, true, exe, "update", "--check", "-o", "json", "--read-only")
	var got map[string]any
	if err := json.Unmarshal([]byte(res.out), &got); err != nil || res.err != nil {
		t.Fatalf("%v %v %q", err, res.err, res.out)
	}
	want := map[string]any{"current_version": "0.1.3", "latest_version": "0.1.4", "update_available": true,
		"release_url": "https://github.com/" + update.Repo + "/releases/tag/v0.1.4", "install_method": "self"}
	if fmt.Sprint(got) != fmt.Sprint(want) || f.count() != 1 || strings.Contains(res.errOut, noticeText) {
		t.Fatalf("got %v requests=%d stderr=%q", got, f.count(), res.errOut)
	}
	if c := update.ReadCache(dir); c.LatestVersion != "0.1.4" {
		t.Fatalf("check not cached: %+v", c)
	}
	// version reads that cache and never the network.
	res = runUpdateApp(t, f, true, exe, "version", "-o", "json")
	if !strings.Contains(res.out, `"latest": "0.1.4"`) || !strings.Contains(res.out, `"update_available": true`) || f.count() != 1 {
		t.Fatalf("version: %s requests=%d", res.out, f.count())
	}
}

func TestUpdateAlreadyLatest(t *testing.T) {
	updateEnv(t, "0.1.4")
	f := gh(t, "v0.1.4", nil)
	exe := filepath.Join(t.TempDir(), "bwt")
	res := runUpdateApp(t, f, false, exe, "update", "--no-input")
	if res.err != nil || !strings.Contains(res.out, "bwt v0.1.4 is already the latest version.") || f.count() != 1 {
		t.Fatalf("%v %q requests=%d", res.err, res.out, f.count())
	}
}

func releaseArchive(t *testing.T, binary string) []byte {
	t.Helper()
	var b bytes.Buffer
	if runtime.GOOS == "windows" {
		zw := zip.NewWriter(&b)
		w, _ := zw.Create("bwt.exe")
		w.Write([]byte(binary))
		zw.Close()
		return b.Bytes()
	}
	z := gzip.NewWriter(&b)
	tw := tar.NewWriter(z)
	tw.WriteHeader(&tar.Header{Name: "bwt", Mode: 0755, Size: int64(len(binary))})
	tw.Write([]byte(binary))
	tw.Close()
	z.Close()
	return b.Bytes()
}

func installedRelease(t *testing.T, tamper bool) (*fake, string) {
	asset := update.AssetName(runtime.GOOS, runtime.GOARCH)
	archive := releaseArchive(t, "new bwt")
	sum := sha256.Sum256(archive)
	if tamper {
		sum = sha256.Sum256([]byte("other"))
	}
	f := gh(t, "v0.1.4", map[string][]byte{asset: archive, "checksums.txt": []byte(fmt.Sprintf("%x  %s\n", sum, asset))})
	exe := filepath.Join(t.TempDir(), update.BinaryName(runtime.GOOS))
	if err := os.WriteFile(exe, []byte("old bwt"), 0755); err != nil {
		t.Fatal(err)
	}
	return f, exe
}

func TestUpdateInstalls(t *testing.T) {
	dir := updateEnv(t, "0.1.3")
	update.WriteCache(dir, update.Cache{CheckedAt: time.Now(), LatestVersion: "0.1.4"})
	f, exe := installedRelease(t, false)
	res := runUpdateApp(t, f, false, exe, "update", "-y")
	if res.err != nil || res.out != "Updated bwt v0.1.3 -> v0.1.4\nRelease notes: https://github.com/"+update.Repo+"/releases/tag/v0.1.4\n" {
		t.Fatalf("%v %q %q", res.err, res.out, res.errOut)
	}
	if b, _ := os.ReadFile(exe); string(b) != "new bwt" {
		t.Fatalf("exe holds %q", b)
	}
	if _, err := os.Stat(filepath.Join(dir, update.CacheFile)); !os.IsNotExist(err) {
		t.Fatalf("update cache not cleared: %v", err)
	}
}

func TestUpdateRefusals(t *testing.T) {
	cases := []struct {
		name, want string
		tamper     bool
		gobin      bool
		args       []string
	}{
		{"checksum mismatch", "checksum", true, false, []string{"update", "--yes"}},
		{"no-input without yes", "pass --yes", false, false, []string{"update", "--no-input"}},
		{"read-only", "read-only", false, false, []string{"update", "--yes", "--read-only"}},
		{"go bin directory", "", false, true, []string{"update", "--yes"}},
		{"dry run", "", false, false, []string{"update", "--yes", "--dry-run"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			updateEnv(t, "0.1.3")
			f, exe := installedRelease(t, tc.tamper)
			if tc.gobin {
				t.Setenv("GOBIN", filepath.Dir(exe))
			}
			res := runUpdateApp(t, f, false, exe, tc.args...)
			if tc.want != "" && (res.err == nil || !strings.Contains(res.err.Error(), tc.want)) {
				t.Fatalf("want error %q, got %v", tc.want, res.err)
			}
			if tc.gobin && (res.err != nil || !strings.Contains(res.out, "Update with: "+update.SourceUpdate)) {
				t.Fatalf("go bin: %v %q", res.err, res.out)
			}
			if tc.name == "dry run" && (res.err != nil || !strings.Contains(res.out, "Would update bwt v0.1.3 -> v0.1.4")) {
				t.Fatalf("dry run: %v %q", res.err, res.out)
			}
			if b, _ := os.ReadFile(exe); string(b) != "old bwt" {
				t.Fatalf("exe changed to %q", b)
			}
		})
	}
}
