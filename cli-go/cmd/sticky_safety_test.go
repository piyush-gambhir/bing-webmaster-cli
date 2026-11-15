package cmd

import (
	"bytes"
	"strings"
	"testing"
)

func TestEnvironmentReadOnlyIsSticky(t *testing.T) {
	t.Setenv("BWT_READ_ONLY", "true")
	t.Setenv("BWT_NO_UPDATE_NOTIFIER", "1")
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	var out, errOut bytes.Buffer
	root := NewRoot(strings.NewReader(""), &out, &errOut)
	root.SetArgs([]string{"sites", "add", "https://example.com/", "--read-only=false", "-o", "json"})
	if err := root.Execute(); err == nil || !strings.Contains(err.Error(), "read-only") {
		t.Fatalf("want a read-only refusal, got %v", err)
	}
}
