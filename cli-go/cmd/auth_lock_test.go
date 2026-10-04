package cmd

import (
	"bytes"
	"context"
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
