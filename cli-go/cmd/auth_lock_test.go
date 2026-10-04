package cmd

import (
	"bytes"
	"context"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/piyush-gambhir/bing-webmaster-cli/cli-go/internal/auth"
	"github.com/piyush-gambhir/bing-webmaster-cli/cli-go/internal/config"
	"github.com/zalando/go-keyring"
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
		t.Fatal("login persisted credentials while a another process held the profile lock")
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
		t.Fatal("logout deleted credentials while a another process held the profile lock")
	}
	unlock()
	if err := execWith(context.Background(), f, "", "auth", "logout"); err != nil {
		t.Fatalf("logout after the lock was released: %v", err)
	}
}

func TestLoginKeepsASiteChosenByAnotherProcess(t *testing.T) {
	isolate(t)
	path := os.Getenv("BWT_CONFIG")
	setSite := func(site string) {
		t.Helper()
		if err := config.Update(context.Background(), path, func(c *config.Config) error {
			c.Profiles["default"] = config.Profile{Site: site}
			return nil
		}); err != nil {
			t.Fatal(err)
		}
	}
	setSite("https://a.example.com/")
	two := `{"d":[{"Url":"https://a.example.com/"},{"Url":"https://b.example.com/"}]}`
	// `bwt sites use B` runs while login is checking the key with Bing.
	f := &fake{handle: func(r *http.Request, body string) *http.Response {
		setSite("https://b.example.com/")
		return reply(200, two)
	}}
	res := run(t, f, "secret-api-key\n", "auth", "login", "--no-input", "-o", "json")
	if res.err != nil || !strings.Contains(res.out, `"site": "https://b.example.com/"`) {
		t.Fatalf("login overwrote the concurrent site change: %v %s", res.err, res.out)
	}
}

func TestLoginAndLogoutDeleteLegacyOAuthTokens(t *testing.T) {
	isolate(t)
	path := os.Getenv("BWT_CONFIG")
	seed := func() {
		t.Helper()
		if err := config.Update(context.Background(), path, func(c *config.Config) error {
			c.Profiles["default"] = config.Profile{Auth: "oauth", TokenStore: "keychain"}
			c.CurrentProfile = "default"
			return nil
		}); err != nil {
			t.Fatal(err)
		}
		for _, kind := range auth.LegacyKinds {
			if err := keyring.Set(testService(t), "default:"+kind, "v0.1.1-token"); err != nil {
				t.Fatal(err)
			}
		}
	}
	gone := func(when string) {
		t.Helper()
		for _, kind := range auth.LegacyKinds {
			if _, err := keyring.Get(testService(t), "default:"+kind); err == nil {
				t.Fatalf("%s left the %s token behind", when, kind)
			}
		}
	}
	seed()
	if res := run(t, nil, "", "sites", "list"); res.err == nil || !strings.Contains(res.err.Error(), "no longer supports") {
		t.Fatalf("a v0.1.1 OAuth profile needs a clear next step: %v", res.err)
	}
	if res := run(t, nil, "", "auth", "logout"); res.err != nil {
		t.Fatalf("logout: %v", res.err)
	}
	gone("logout")
	seed()
	f := &fake{handle: func(r *http.Request, body string) *http.Response { return reply(200, sitesBody) }}
	if res := run(t, f, "secret-api-key\n", "auth", "login", "--no-input"); res.err != nil {
		t.Fatalf("login: %v", res.err)
	}
	gone("login")
	if v, _ := keyring.Get(testService(t), "default:api_key"); v != "secret-api-key" {
		t.Fatal("login did not save the new key")
	}
}
