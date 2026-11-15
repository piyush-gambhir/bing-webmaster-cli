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

const AuthAPIKey = "api_key"

type IndexNowKey struct {
	Key         string `yaml:"key"`
	KeyLocation string `yaml:"key_location,omitempty"`
}

type Profile struct {
	Auth       string                 `yaml:"auth,omitempty"`
	TokenStore string                 `yaml:"token_store,omitempty"`
	Site       string                 `yaml:"site,omitempty"`
	IndexNow   map[string]IndexNowKey `yaml:"indexnow,omitempty"`
}

type Config struct {
	CurrentProfile string             `yaml:"current_profile,omitempty"`
	Profiles       map[string]Profile `yaml:"profiles"`
}

func Dir() (string, error) {
	p, err := Path()
	if err != nil {
		return "", err
	}
	return filepath.Dir(p), nil
}

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
	if err := yaml.Unmarshal(b, c); err != nil {
		return nil, fmt.Errorf("invalid YAML config at %s", path)
	}
	if c.Profiles == nil {
		c.Profiles = map[string]Profile{}
	}
	return c, nil
}

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

func (c *Config) ProfileName(flag string) string {
	if flag != "" {
		return flag
	}
	if env := os.Getenv("BWT_PROFILE"); env != "" {
		return env
	}
	return c.CurrentProfile
}

func ValidName(name string) error {
	if name == "" || strings.TrimSpace(name) != name || strings.ContainsAny(name, ":/\\\r\n\t") || len(name) > 64 {
		return fmt.Errorf("profile name must be 1-64 characters without spaces at the ends, slashes, colons, or control characters")
	}
	return nil
}
