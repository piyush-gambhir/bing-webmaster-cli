package cmd

import (
	"bytes"
	"context"
	"net/http"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/piyush-gambhir/bing-webmaster-cli/cli-go/internal/auth"
)

// execWith runs the CLI under ctx so a test can bound how long it waits on a lock.
func execWith(ctx context.Context, f *fake, input string, args ...string) error {
	var out, errOut bytes.Buffer
	a := &app{in: strings.NewReader(input), out: &out, errOut: &errOut, transport: f.transport()}
	root := newRoot(a)
	root.SetArgs(args)
	return root.ExecuteContext(ctx)
}

func TestLoginAndLogoutWaitForTheRefreshLock(t *testing.T) {
	isolate(t)
	f := &fake{}
	unlock, err := auth.LockProfile(context.Background(), os.Getenv("BWT_CONFIG"), "default")
	if err != nil {
		t.Fatal(err)
	}
	short := func() error {
		ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
		defer cancel()
		return execWith(ctx, f, "secret-api-key\n", "auth", "login", "--with-api-key", "--no-input", "--no-verify")
	}
	if err := short(); err == nil {
		t.Fatal("login persisted credentials while a refresh held the profile lock")
	}
	unlock()
	if err := short(); err != nil {
		t.Fatalf("login after the lock was released: %v", err)
	}
	if unlock, err = auth.LockProfile(context.Background(), os.Getenv("BWT_CONFIG"), "default"); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	if err := execWith(ctx, f, "", "auth", "logout"); err == nil {
		t.Fatal("logout deleted credentials while a refresh held the profile lock")
	}
	unlock()
	if err := execWith(context.Background(), f, "", "auth", "logout"); err != nil {
		t.Fatalf("logout after the lock was released: %v", err)
	}
}

func TestReloginReplacesTheCachedAccessToken(t *testing.T) {
	isolate(t)
	t.Setenv("BWT_CLIENT_ID", "cid")
	t.Setenv("BWT_CLIENT_SECRET", "csecret")
	var issued string
	f := &fake{handle: func(r *http.Request, body string) *http.Response {
		switch {
		case r.URL.Path == "/webmasters/oauth/token":
			return reply(200, `{"access_token":"`+issued+`","token_type":"bearer","expires_in":3599,"refresh_token":"refresh-`+issued+`"}`)
		case strings.HasSuffix(r.URL.Path, "/GetUserSites"):
			if got := r.Header.Get("Authorization"); got != "Bearer "+issued {
				t.Errorf("used %q, want the token from the latest login (%s)", got, issued)
			}
			return reply(200, sitesBody)
		}
		t.Errorf("unexpected %s", r.URL)
		return reply(500, "{}")
	}}
	login := func(token string) {
		issued = token
		var out, errOut bytes.Buffer
		a := &app{in: strings.NewReader(""), out: &out, errOut: &errOut, transport: f.transport()}
		a.openBrowser = func(authURL string) error {
			u, _ := url.Parse(authURL)
			q := u.Query()
			go func() {
				if res, err := http.Get(q.Get("redirect_uri") + "?code=c&state=" + url.QueryEscape(q.Get("state"))); err == nil {
					res.Body.Close()
				}
			}()
			return nil
		}
		root := newRoot(a)
		root.SetArgs([]string{"auth", "login", "--no-verify"})
		if err := root.ExecuteContext(context.Background()); err != nil {
			t.Fatalf("login %s: %v %s", token, err, errOut.String())
		}
	}
	login("account-a")
	login("account-b")
	if res := run(t, f, "", "sites", "list"); res.err != nil {
		t.Fatalf("sites list: %v", res.err)
	}
}
