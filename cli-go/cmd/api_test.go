package cmd

import (
	"net/http"
	"strings"
	"testing"
)

func TestRawAPIWritesNeedConfirmation(t *testing.T) {
	isolate(t)
	t.Setenv("BWT_API_KEY", "k")
	f := &fake{handle: func(r *http.Request, body string) *http.Response { return reply(200, `{"d":null}`) }}
	data := `{"siteUrl":"https://www.example.com/"}`
	res := run(t, f, "", "api", "RemoveSite", "--data", data, "--no-input")
	if res.err == nil || !strings.Contains(res.err.Error(), "--yes") || f.count() != 0 {
		t.Fatalf("raw RemoveSite without --yes: err=%v calls=%d", res.err, f.count())
	}
	if res = run(t, f, "", "api", "RemoveSite", "--data", data, "--no-input", "--yes"); res.err != nil || f.count() != 1 {
		t.Fatalf("raw RemoveSite with --yes: err=%v calls=%d", res.err, f.count())
	}
	// Reads need no confirmation.
	f.handle = func(r *http.Request, body string) *http.Response { return reply(200, sitesBody) }
	if res = run(t, f, "", "api", "GetUserSites", "--no-input"); res.err != nil {
		t.Fatalf("raw read: %v", res.err)
	}
}
