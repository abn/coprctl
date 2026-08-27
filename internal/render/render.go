// Package render turns output structs into the supported formats. Human
// formats (table, plain) are renderings of the same structs JSON serializes,
// so machine output is never a separate code path.
package render

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
	"text/tabwriter"

	"gopkg.in/yaml.v3"
)

// Format is an output format.
type Format string

const (
	FormatTable Format = "table"
	FormatPlain Format = "plain"
	FormatJSON  Format = "json"
	FormatJSONL Format = "jsonl"
	FormatYAML  Format = "yaml"
)

// ParseFormat normalizes a format string.
func ParseFormat(s string) (Format, error) {
	switch Format(strings.ToLower(s)) {
	case "table", "":
		return FormatTable, nil
	case "plain":
		return FormatPlain, nil
	case "json":
		return FormatJSON, nil
	case "jsonl":
		return FormatJSONL, nil
	case "yaml":
		return FormatYAML, nil
	}
	return "", fmt.Errorf("unknown output format %q", s)
}

// Render writes v to w in the given format.
func Render(w io.Writer, format Format, v any) error {
	switch format {
	case FormatJSON:
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(v)
	case FormatJSONL:
		return renderJSONL(w, v)
	case FormatYAML:
		return renderYAML(w, v)
	case FormatTable, FormatPlain:
		return renderTable(w, v)
	}
	return fmt.Errorf("unsupported format %q", format)
}

func renderYAML(w io.Writer, v any) error {
	enc := yaml.NewEncoder(w)
	enc.SetIndent(2)
	if err := enc.Encode(v); err != nil {
		return err
	}
	return enc.Close()
}

func renderJSONL(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	return enc.Encode(v)
}

// Table is a generic two-dimensional table for human rendering.
type Table struct {
	Header []string
	Rows   [][]string
}

// NewTable builds a table with the given header.
func NewTable(header ...string) *Table { return &Table{Header: header} }

// Add appends a row.
func (t *Table) Add(row ...string) { t.Rows = append(t.Rows, row) }

func renderTable(w io.Writer, v any) error {
	tbl, ok := v.(*Table)
	if !ok {
		return fmt.Errorf("cannot render %T as a table", v)
	}
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	if len(tbl.Header) > 0 {
		fmt.Fprintln(tw, strings.Join(tbl.Header, "\t"))
	}
	for _, row := range tbl.Rows {
		fmt.Fprintln(tw, strings.Join(row, "\t"))
	}
	return tw.Flush()
}

// SortedKeys returns the sorted keys of a map for deterministic output.
func SortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
