package auth

import (
	"context"
	"path/filepath"

	"github.com/piyush-gambhir/bing-webmaster-cli/cli-go/internal/config"
)

func SecretKey(profile, kind string) string { return profile + ":" + kind }

var LegacyKinds = []string{"oauth", "access"}

func LockProfile(ctx context.Context, configPath, profile string) (func(), error) {
	return config.Lock(ctx, filepath.Join(filepath.Dir(configPath), "locks", profile+".lock"))
}
