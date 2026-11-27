package cmd

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestCommandsResolve(t *testing.T) {
	for _, tc := range []struct {
		args   []string
		effect string
	}{{[]string{"sites", "list"}, "read"}, {[]string{"-o", "json", "submit", "urls", "https://x.com/a"}, "remote_write"},
		{[]string{"api", "GetUserSites"}, "read"}, {[]string{"api", "RemoveSite", "--param", "siteUrl=https://x.com/"}, "remote_write"},
		{[]string{"api", "NotABingMethod", "--http", "GET"}, "remote_write"}} {
		var out bytes.Buffer
		root := NewRoot(strings.NewReader(""), &out, &bytes.Buffer{})
		root.SetArgs(append([]string{"commands", "resolve", "--"}, tc.args...))
		if err := root.Execute(); err != nil {
			t.Fatalf("%v: %v", tc.args, err)
		}
		var got resolution
		if err := json.Unmarshal(out.Bytes(), &got); err != nil || got.Effect != tc.effect {
			t.Errorf("%v: got %+v (%v)", tc.args, got, err)
		}
	}
	root := NewRoot(strings.NewReader(""), &bytes.Buffer{}, &bytes.Buffer{})
	root.SetArgs([]string{"commands", "resolve", "--", "nonsense"})
	if root.Execute() == nil {
		t.Fatal("an unknown command resolved")
	}
}

func TestCommandsResolveRedactsCredentials(t *testing.T) {
	var out bytes.Buffer
	root := NewRoot(strings.NewReader(""), &out, &bytes.Buffer{})
	root.SetArgs([]string{"commands", "resolve", "--", "sites", "list", "--api-key", "secret-key-value", "--access-token", "secret-token-value", "-o", "json"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	var got resolution
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out.String(), "secret-") || got.Flags["api-key"] != "[REDACTED]" || got.Flags["access-token"] != "[REDACTED]" || got.Flags["output"] != "json" {
		t.Errorf("got %s", out.String())
	}
}
