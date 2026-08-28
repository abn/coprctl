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

// ANSI color codes for human output. Applied only when color is enabled.
const (
	ansiReset  = "\x1b[0m"
	ansiBold   = "\x1b[1m"
	ansiDim    = "\x1b[2m"
	ansiCopper = "\x1b[38;5;179m" // warm copper accent for the prompt/keys
	ansiMuted  = "\x1b[38;5;244m" // muted grey for secondary values
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

// Render writes v to w in the given format, without color.
func Render(w io.Writer, format Format, v any) error {
	return RenderColored(w, format, v, false)
}

// RenderColored writes v to w in the given format, optionally with ANSI color
// for the human (table, plain) formats.
func RenderColored(w io.Writer, format Format, v any, color bool) error {
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
		return renderTable(w, v, color)
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

func renderTable(w io.Writer, v any, color bool) error {
	tbl, ok := v.(*Table)
	if !ok {
		return fmt.Errorf("cannot render %T as a table", v)
	}
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	if len(tbl.Header) > 0 {
		header := strings.Join(tbl.Header, "\t")
		if color {
			header = ansiBold + header + ansiReset
		}
		fmt.Fprintln(tw, header)
	}
	for _, row := range tbl.Rows {
		cells := row
		if color && len(cells) > 0 {
			// Emphasize the first (key/name) cell in copper, dim the rest.
			cells = make([]string, len(row))
			copy(cells, row)
			if len(cells) == 1 {
				cells[0] = ansiCopper + cells[0] + ansiReset
			} else {
				cells[0] = ansiCopper + cells[0] + ansiReset
				for i := 1; i < len(cells); i++ {
					cells[i] = ansiMuted + cells[i] + ansiReset
				}
			}
		}
		fmt.Fprintln(tw, strings.Join(cells, "\t"))
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
