package client

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestCleanDropsWCFTypeAndConvertsDates(t *testing.T) {
	in := []any{map[string]any{"__type": "Site:#Microsoft.Bing.Webmaster.Api", "Url": "https://x/", "Date": "/Date(1791105285941)/",
		"Nested": map[string]any{"__type": "Inner", "Ok": true}}}
	b, _ := json.Marshal(Clean(in))
	if strings.Contains(string(b), "__type") || !strings.Contains(string(b), "2026-10-04T") || !strings.Contains(string(b), `"Ok":true`) {
		t.Fatalf("clean: %s", b)
	}
}
