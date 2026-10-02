// Package config stores named profiles in an XDG YAML file. Secrets (OAuth
// tokens, API keys, custom client secrets) live in the OS keychain, or in a
// separate 0600 file only when the user chose --insecure-storage.
package config

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gofrs/flock"
	"go.yaml.in/yaml/v3"
)

const (
	AuthOAuth  = "oauth"
	AuthAPIKey = "api_key"

	ClientBuiltin = "builtin"
	ClientCustom  = "custom"
)

// IndexNowKey is a site's IndexNow key. IndexNow keys are public by design
// (hosted at https://host/KEY.txt), so they are kept in the config file.
type IndexNowKey struct {
	Key         string `yaml:"key"`
	KeyLocation string `yaml:"key_location,omitempty"`
}

type Profile struct {
	// Auth is oauth or api_key; empty means the profile only holds IndexNow keys or a site.
	Auth string `yaml:"auth,omitempty"`
	// Client and ClientID record which OAuth client issued the tokens, so refresh
	// always uses the same client.
	Client       string                 `yaml:"client,omitempty"`
	ClientID     string                 `yaml:"client_id,omitempty"`
	RedirectPort int                    `yaml:"redirect_port,omitempty"`
	Scope        string                 `yaml:"scope,omitempty"`
	TokenStore   string                 `yaml:"token_store,omitempty"`
	Site         string                 `yaml:"site,omitempty"`
	IndexNow     map[string]IndexNowKey `yaml:"indexnow,omitempty"`
}

type Config struct {
	CurrentProfile string             `yaml:"current_profile,omitempty"`
	Profiles       map[string]Profile `yaml:"profiles"`
}

// Dir is the configuration directory: the directory of BWT_CONFIG when set,
// otherwise $XDG_CONFIG_HOME/bing-webmaster-cli.
func Dir() (string, error) {
	p, err := Path()
	if err != nil {
		return "", err
	}
	return filepath.Dir(p), nil
}

// DefaultPath is the configuration path when neither BWT_CONFIG nor
// XDG_CONFIG_HOME is set.
func DefaultPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "bing-webmaster-cli", "config.yaml"), nil
}

func Path() (string, error) {
	if path := os.Getenv("BWT_CONFIG"); path != "" {
		return path, nil
	}
	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		base = filepath.Join(home, ".config")
	}
	return filepath.Join(base, "bing-webmaster-cli", "config.yaml"), nil
}

// SecretsPath is the opt-in plaintext secrets file next to the config.
func SecretsPath() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "secrets.yaml"), nil
}

func Load(path string) (*Config, error) {
	c := &Config{Profiles: map[string]Profile{}}
	b, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return c, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}
	// Do not echo YAML parser errors: a malformed line could contain a credential.
	if err := yaml.Unmarshal(b, c); err != nil {
		return nil, fmt.Errorf("invalid YAML config at %s", path)
	}
	if c.Profiles == nil {
		c.Profiles = map[string]Profile{}
	}
	return c, nil
}

// Update applies mutate under an exclusive lock and writes atomically.
func Update(ctx context.Context, path string, mutate func(*Config) error) error {
	unlock, err := Lock(ctx, path+".lock")
	if err != nil {
		return err
	}
	defer unlock()
	c, err := Load(path)
	if err != nil {
		return err
	}
	if err := mutate(c); err != nil {
		return err
	}
	b, err := yaml.Marshal(c)
	if err != nil {
		return err
	}
	return atomicWrite(path, b)
}

// Lock takes an exclusive cross-process lock on lockPath, waiting up to 10s.
func Lock(ctx context.Context, lockPath string) (func(), error) {
	if err := os.MkdirAll(filepath.Dir(lockPath), 0700); err != nil {
		return nil, err
	}
	lock := flock.New(lockPath)
	lockCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	ok, err := lock.TryLockContext(lockCtx, 25*time.Millisecond)
	if err != nil {
		return nil, fmt.Errorf("lock %s: %w", filepath.Base(lockPath), err)
	}
	if !ok {
		return nil, fmt.Errorf("%s is locked by another process", filepath.Base(lockPath))
	}
	if err := os.Chmod(lockPath, 0600); err != nil {
		lock.Unlock()
		return nil, err
	}
	return func() { _ = lock.Unlock() }, nil
}

// ProfileName resolves the profile to use: flag, then BWT_PROFILE, then the saved current profile.
func (c *Config) ProfileName(flag string) string {
	if flag != "" {
		return flag
	}
	if env := os.Getenv("BWT_PROFILE"); env != "" {
		return env
	}
	return c.CurrentProfile
}

// ValidName checks a profile name; names are used in keychain keys and file names.
func ValidName(name string) error {
	if name == "" || strings.TrimSpace(name) != name || strings.ContainsAny(name, ":/\\\r\n\t") || len(name) > 64 {
		return fmt.Errorf("profile name must be 1-64 characters without spaces at the ends, slashes, colons, or control characters")
	}
	return nil
}
