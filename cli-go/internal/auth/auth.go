// Package auth holds the pieces of credential handling shared by commands:
// how saved secrets are named and the per-profile lock that serializes login
// and logout across processes.
//
// Bing Webmaster Tools logins use an API key. Bing's OAuth client registration
// rejects loopback redirect URIs (http://127.0.0.1 and http://localhost), so a
// browser OAuth flow cannot deliver its code to a command-line tool.
package auth

import (
	"context"
	"path/filepath"

	"github.com/piyush-gambhir/bing-webmaster-cli/cli-go/internal/config"
)

// SecretKey names a profile's secret in the store.
func SecretKey(profile, kind string) string { return profile + ":" + kind }

// LockProfile takes the per-profile cross-process lock, so concurrent logins
// and logouts of the same profile cannot interleave.
func LockProfile(ctx context.Context, configPath, profile string) (func(), error) {
	return config.Lock(ctx, filepath.Join(filepath.Dir(configPath), "locks", profile+".lock"))
}
