package cmd

import (
	"crypto/sha256"
	"fmt"
	"sort"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// TestAgentSafetyCommandManifest makes every runnable command part of a
// reviewed safety contract. Adding, removing, or reclassifying a command
// requires an intentional update of expectedDigest.
func TestAgentSafetyCommandManifest(t *testing.T) {
	dangerous := map[string]bool{
		"add": true, "remove": true, "submit": true, "verify": true, "request": true, "set-settings": true,
		"enable": true, "disable": true, "generate": true, "login": true, "logout": true, "use": true,
		"use-profile": true, "update": true, "submit-content": true, "delete": true,
	}
	guarded := func(c *cobra.Command) bool {
		a := c.Annotations
		return a["mutates"] == "true" || a["writes-local"] == "true" || a["writes-local"] == "conditional" || a["self-update"] == "true"
	}
	var entries []string
	var walk func(*cobra.Command)
	walk = func(parent *cobra.Command) {
		for _, c := range parent.Commands() {
			if c.Runnable() {
				leaf := strings.Fields(c.Use)[0]
				if dangerous[leaf] && !guarded(c) {
					t.Errorf("%q looks state-changing but has no mutates/writes-local annotation", c.CommandPath())
				}
				a := c.Annotations
				entries = append(entries, fmt.Sprintf("%s|mutates=%s|writes-local=%s|self-update=%s|interactive=%s|experimental=%s|ops=%s",
					c.CommandPath(), a["mutates"], a["writes-local"], a["self-update"], a["interactive"], a["experimental"], a["ops"]))
			}
			walk(c)
		}
	}
	walk(tree())
	sort.Strings(entries)
	digest := fmt.Sprintf("%x", sha256.Sum256([]byte(strings.Join(entries, "\n"))))
	const expectedDigest = "5d3f7a5e836bf6ed9e3e9323b1697b11730f604e96f5508d7dd56b1013fd4689"
	if digest != expectedDigest {
		t.Fatalf("agent-safety command manifest changed: got %s; review the annotations, then update expectedDigest\n%s", digest, strings.Join(entries, "\n"))
	}
}
