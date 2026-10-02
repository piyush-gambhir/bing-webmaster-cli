package output

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"
)

func TestFormatsPreserveTypesAndPrecision(t *testing.T) {
	data := map[string]any{"id": json.Number("9007199254740993"), "numeric_string": "0012", "name": "page", "ok": true}
	for _, format := range []string{"json", "yaml"} {
		var b bytes.Buffer
		if err := Print(&b, format, data); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(b.String(), "9007199254740993") {
			t.Fatal("lost precision", b.String())
		}
		if format == "yaml" {
			var got map[string]any
			if err := yaml.Unmarshal(b.Bytes(), &got); err != nil {
				t.Fatal(err)
			}
			if got["numeric_string"] != "0012" || got["ok"] != true {
				t.Fatalf("wrong types: %#v", got)
			}
		}
	}
}

func TestTableEscapesControlsAndColumns(t *testing.T) {
	var b bytes.Buffer
	rows := []map[string]any{{"Url": "\x1b[31mhttps://x\nrow", "Clicks": json.Number("5"), "__type": "x"}}
	cols := []Col{{Header: "url", Key: "Url"}, {Header: "clicks", Key: "Clicks"}, {Header: "double", Value: func(r map[string]any) any { return "computed" }}}
	if err := View(&b, "table", map[string]any{"rows": rows}, rows, cols...); err != nil {
		t.Fatal(err)
	}
	out := b.String()
	if strings.ContainsRune(out, '\x1b') || !strings.HasPrefix(out, "url") || !strings.Contains(out, "computed") {
		t.Fatalf("table: %q", out)
	}
	b.Reset()
	if err := Print(&b, "table", []any{}); err != nil || !strings.Contains(b.String(), "No results.") {
		t.Fatalf("empty: %q %v", b.String(), err)
	}
	if err := Print(&b, "bad", nil); err == nil {
		t.Fatal("bad format accepted")
	}
}

func TestViewJSONUsesEnvelope(t *testing.T) {
	var b bytes.Buffer
	rows := []map[string]any{{"a": 1}}
	if err := View(&b, "json", map[string]any{"filtered_locally": true, "rows": rows}, rows); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(b.String(), `"filtered_locally": true`) {
		t.Fatal(b.String())
	}
}

func TestCSV(t *testing.T) {
	var b bytes.Buffer
	rows := []any{map[string]any{"q": "=HYPERLINK(\"x\")", "n": json.Number("-3"), "s": "a,b"}}
	if err := Print(&b, "csv", rows); err != nil {
		t.Fatal(err)
	}
	if b.String() != "n,q,s\n-3,\"'=HYPERLINK(\"\"x\"\")\",\"a,b\"\n" {
		t.Fatalf("csv: %q", b.String())
	}
	b.Reset()
	if err := Print(&b, "csv", map[string]any{"daily": 10}); err != nil || !strings.HasPrefix(b.String(), "key,value\n") {
		t.Fatalf("key-value csv: %q %v", b.String(), err)
	}
	if err := Print(&b, "csv", "scalar"); err == nil {
		t.Fatal("scalar csv accepted")
	}
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, errors.New("broken pipe") }

func TestWriterErrorsPropagate(t *testing.T) {
	rows := []any{map[string]any{"a": 1}}
	for _, format := range []string{"json", "csv", "table"} {
		if err := Print(failingWriter{}, format, rows); err == nil {
			t.Errorf("%s: writer error swallowed", format)
		}
	}
}

func TestCSVGuardsInferredHeadersAndKeys(t *testing.T) {
	var b bytes.Buffer
	if err := Print(&b, "csv", []any{map[string]any{"=cmd()": "v"}}); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(b.String(), "'=cmd()\n") {
		t.Fatalf("inferred header not guarded: %q", b.String())
	}
	b.Reset()
	if err := Print(&b, "csv", map[string]any{"@SUM(A1)": 1}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(b.String(), "\n'@SUM(A1),1\n") {
		t.Fatalf("key column not guarded: %q", b.String())
	}
}
