package config

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
)

func TestProfileSelection(t *testing.T) {
	c := &Config{CurrentProfile: "saved"}
	t.Setenv("BWT_PROFILE", "env")
	if c.ProfileName("flag") != "flag" || c.ProfileName("") != "env" {
		t.Fatal("flag or env precedence wrong")
	}
	t.Setenv("BWT_PROFILE", "")
	if c.ProfileName("") != "saved" {
		t.Fatal("saved profile not used")
	}
	for _, bad := range []string{"", " x", "a:b", "a/b", "a\nb"} {
		if ValidName(bad) == nil {
			t.Errorf("accepted %q", bad)
		}
	}
	if ValidName("work-site") != nil {
		t.Fatal("rejected valid name")
	}
}

func TestConcurrentUpdatesAndPermissions(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "config.yaml")
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			if err := Update(context.Background(), path, func(c *Config) error {
				c.Profiles[fmt.Sprintf("p%d", i)] = Profile{Auth: AuthAPIKey, Site: "https://example.com/"}
				return nil
			}); err != nil {
				t.Error(err)
			}
		}(i)
	}
	wg.Wait()
	c, err := Load(path)
	if err != nil || len(c.Profiles) != 8 {
		t.Fatalf("profiles %d %v", len(c.Profiles), err)
	}
	if runtime.GOOS != "windows" {
		info, err := os.Stat(path)
		if err != nil || info.Mode().Perm() != 0600 {
			t.Fatalf("mode %v %v", info.Mode(), err)
		}
	}
}

func TestInvalidYAMLDoesNotEchoContents(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("profiles: [unclosed hunter2"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil || strings.Contains(err.Error(), "hunter2") {
		t.Fatalf("error: %v", err)
	}
}

func TestPaths(t *testing.T) {
	t.Setenv("BWT_CONFIG", "")
	t.Setenv("XDG_CONFIG_HOME", "/xdg")
	p, _ := Path()
	if p != filepath.Join("/xdg", "bing-webmaster-cli", "config.yaml") {
		t.Fatal(p)
	}
	t.Setenv("BWT_CONFIG", "/custom/c.yaml")
	s, _ := SecretsPath()
	if s != filepath.Join("/custom", "secrets.yaml") {
		t.Fatal(s)
	}
}
