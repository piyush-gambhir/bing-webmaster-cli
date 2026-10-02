package cmd

import (
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/piyush-gambhir/bing-webmaster-cli/cli-go/internal/registry"
	"github.com/spf13/cobra"
)

func tree() *cobra.Command { return NewRoot(strings.NewReader(""), io.Discard, io.Discard) }

func TestEveryMethodHasACommand(t *testing.T) {
	cov := APICoverage(tree())
	for _, op := range registry.Ops {
		cmds := cov[op.Name]
		switch op.Status {
		case registry.Obsolete:
			if len(cmds) > 0 {
				t.Errorf("obsolete %s is reachable from %v", op.Name, cmds)
			}
		case registry.Experimental:
			if len(cmds) == 0 {
				t.Errorf("experimental %s has no command", op.Name)
			}
			for _, c := range cmds {
				if !strings.HasPrefix(c, "bwt experimental ") {
					t.Errorf("experimental %s is outside the experimental group: %s", op.Name, c)
				}
			}
		default:
			if len(cmds) == 0 {
				t.Errorf("%s has no command", op.Name)
			}
		}
	}
	for name := range cov {
		if _, ok := registry.Lookup(name); !ok {
			t.Errorf("command calls unknown method %s", name)
		}
	}
}

// A command is marked mutating exactly when one of its Bing methods writes.
func TestMutatesMatchesRegistry(t *testing.T) {
	var walk func(*cobra.Command)
	walk = func(c *cobra.Command) {
		if ops := c.Annotations["ops"]; c.Runnable() && ops != "" {
			writes := false
			for _, name := range strings.Split(ops, ",") {
				op, _ := registry.Lookup(name)
				writes = writes || op.Effect == registry.Write
			}
			if (c.Annotations["mutates"] == "true") != writes {
				t.Errorf("%s: mutates=%q but registry write=%v", c.CommandPath(), c.Annotations["mutates"], writes)
			}
		}
		for _, child := range c.Commands() {
			walk(child)
		}
	}
	walk(tree())
}

func TestCoverageDocIsCurrent(t *testing.T) {
	want := APICoverageMarkdown(tree())
	got, err := os.ReadFile("../../docs/api-coverage.md")
	if err != nil || string(got) != want {
		t.Fatalf("docs/api-coverage.md is out of date; run make docs (err=%v)", err)
	}
}

func TestCompatibilityRecordsSnapshotHash(t *testing.T) {
	b, err := os.ReadFile("../internal/registry/testdata/bing-webmaster-api.methods.json")
	if err != nil {
		t.Fatal(err)
	}
	doc, err := os.ReadFile("../../docs/compatibility.md")
	if err != nil {
		t.Fatal(err)
	}
	if sum := fmt.Sprintf("%x", sha256.Sum256(b)); !strings.Contains(string(doc), sum) {
		t.Fatalf("docs/compatibility.md does not record the snapshot sha256 %s", sum)
	}
}
