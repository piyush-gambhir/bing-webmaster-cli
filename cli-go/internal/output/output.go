// Package output renders command results as table, JSON, YAML, or CSV. JSON and
// YAML keep the full data; tables and CSV show rows with chosen columns.
package output

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
	"text/tabwriter"
	"unicode"

	"go.yaml.in/yaml/v3"
)

func Valid(format string) bool {
	return format == "table" || format == "json" || format == "yaml" || format == "csv"
}

// Col is one table or CSV column. Value derives the cell from the row; when nil
// the row's Key is used.
type Col struct {
	Header string
	Key    string
	Value  func(map[string]any) any
}

// Print renders data. For table and CSV, cols choose the columns of row-shaped
// data; without cols they are inferred.
func Print(w io.Writer, format string, data any, cols ...Col) error {
	return View(w, format, data, nil, cols...)
}

// View prints full for JSON and YAML, and rows (when non-nil) for table and CSV.
// Commands use it to give machines an envelope with context while people see a
// plain table.
func View(w io.Writer, format string, full any, rows []map[string]any, cols ...Col) error {
	if !Valid(format) {
		return fmt.Errorf("unsupported output %q; use table, json, yaml, or csv", format)
	}
	switch format {
	case "json":
		e := json.NewEncoder(w)
		e.SetIndent("", "  ")
		return e.Encode(full)
	case "yaml":
		return printYAML(w, full)
	}
	if rows == nil {
		normalized, err := normalize(full)
		if err != nil {
			return err
		}
		var ok bool
		rows, ok = asRows(normalized)
		if !ok {
			if format == "csv" {
				if m, isMap := normalized.(map[string]any); isMap {
					return keyValueCSV(w, m)
				}
				return fmt.Errorf("csv output needs row-shaped results; use -o json")
			}
			return plain(w, normalized)
		}
	}
	if len(cols) == 0 {
		cols = inferCols(rows)
	}
	if format == "csv" {
		return writeCSV(w, rows, cols)
	}
	return writeTable(w, rows, cols)
}

func normalize(data any) (any, error) {
	b, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}
	dec := json.NewDecoder(strings.NewReader(string(b)))
	dec.UseNumber()
	var v any
	err = dec.Decode(&v)
	return v, err
}

func asRows(v any) ([]map[string]any, bool) {
	switch x := v.(type) {
	case []any:
		rows := make([]map[string]any, 0, len(x))
		for _, item := range x {
			m, ok := item.(map[string]any)
			if !ok {
				return nil, false
			}
			rows = append(rows, m)
		}
		return rows, true
	case []map[string]any:
		return x, true
	}
	return nil, false
}

func inferCols(rows []map[string]any) []Col {
	seen := map[string]bool{}
	var keys []string
	for _, r := range rows {
		for k := range r {
			if !seen[k] && k != "__type" {
				seen[k] = true
				keys = append(keys, k)
			}
		}
	}
	sort.Strings(keys)
	cols := make([]Col, len(keys))
	for i, k := range keys {
		cols[i] = Col{Header: k, Key: k}
	}
	return cols
}

func value(c Col, r map[string]any) any {
	if c.Value != nil {
		return c.Value(r)
	}
	return r[c.Key]
}

func writeTable(w io.Writer, rows []map[string]any, cols []Col) error {
	if len(rows) == 0 {
		_, err := fmt.Fprintln(w, "No results.")
		return err
	}
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	headers := make([]string, len(cols))
	for i, c := range cols {
		headers[i] = cell(c.Header, "-")
	}
	fmt.Fprintln(tw, strings.Join(headers, "\t"))
	for _, r := range rows {
		cells := make([]string, len(cols))
		for i, c := range cols {
			cells[i] = cell(value(c, r), "-")
		}
		fmt.Fprintln(tw, strings.Join(cells, "\t"))
	}
	return tw.Flush()
}

func writeCSV(w io.Writer, rows []map[string]any, cols []Col) error {
	cw := csv.NewWriter(w)
	headers := make([]string, len(cols))
	for i, c := range cols {
		// Headers can come from remote JSON keys when columns are inferred.
		headers[i] = csvCell(c.Header)
	}
	if err := cw.Write(headers); err != nil {
		return err
	}
	for _, r := range rows {
		cells := make([]string, len(cols))
		for i, c := range cols {
			cells[i] = csvCell(value(c, r))
		}
		if err := cw.Write(cells); err != nil {
			return err
		}
	}
	cw.Flush()
	return cw.Error()
}

func keyValueCSV(w io.Writer, m map[string]any) error {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	cw := csv.NewWriter(w)
	_ = cw.Write([]string{"key", "value"})
	for _, k := range keys {
		_ = cw.Write([]string{csvCell(k), csvCell(m[k])})
	}
	cw.Flush()
	return cw.Error()
}

func plain(w io.Writer, v any) error {
	if m, ok := v.(map[string]any); ok {
		keys := make([]string, 0, len(m))
		for k := range m {
			if k != "__type" {
				keys = append(keys, k)
			}
		}
		sort.Strings(keys)
		tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
		for _, k := range keys {
			fmt.Fprintf(tw, "%s\t%s\n", cell(k, "-"), cell(m[k], "-"))
		}
		return tw.Flush()
	}
	if items, ok := v.([]any); ok {
		if len(items) == 0 {
			_, err := fmt.Fprintln(w, "No results.")
			return err
		}
		for _, item := range items {
			if _, err := fmt.Fprintln(w, cell(item, "-")); err != nil {
				return err
			}
		}
		return nil
	}
	_, err := fmt.Fprintln(w, cell(v, "-"))
	return err
}

func printYAML(w io.Writer, data any) error {
	b, err := json.Marshal(data)
	if err != nil {
		return err
	}
	var node yaml.Node
	if err := yaml.Unmarshal(b, &node); err != nil {
		return err
	}
	var block func(*yaml.Node)
	block = func(n *yaml.Node) {
		n.Style = 0
		for _, child := range n.Content {
			block(child)
		}
	}
	block(&node)
	e := yaml.NewEncoder(w)
	e.SetIndent(2)
	defer e.Close()
	return e.Encode(&node)
}

func text(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case json.Number:
		return x.String()
	case bool:
		if x {
			return "true"
		}
		return "false"
	default:
		b, _ := json.Marshal(v)
		return string(b)
	}
}

// cell sanitizes a value for a terminal table: control characters, including
// escape sequences, become spaces.
func cell(v any, empty string) string {
	if v == nil {
		return empty
	}
	return strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return ' '
		}
		return r
	}, text(v))
}

func csvCell(v any) string {
	if v == nil {
		return ""
	}
	s := text(v)
	// Neutralize spreadsheet formula injection from remote data.
	if s != "" && strings.ContainsRune("=+-@\t\r", rune(s[0])) {
		if _, err := json.Number(s).Float64(); err != nil {
			s = "'" + s
		}
	}
	return s
}
