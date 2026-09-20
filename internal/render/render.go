// Package render turns output structs into the supported formats. Human
// formats (table, plain) are renderings of the same structs JSON serializes,
// so machine output is never a separate code path.
package render

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"reflect"
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
	// A collection is one object per line, not a single array line, so a
	// multi-build submit streams one line per build.
	rv := reflect.ValueOf(v)
	if rv.Kind() == reflect.Slice || rv.Kind() == reflect.Array {
		for i := 0; i < rv.Len(); i++ {
			if err := enc.Encode(rv.Index(i).Interface()); err != nil {
				return err
			}
		}
		return nil
	}
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
		// Commands with a hand-built *Table keep it. Anything else is rendered
		// from its machine shape, so a command never fails merely because the
		// human format was selected.
		generic, err := tableFor(v)
		if err != nil {
			return err
		}
		tbl = generic
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

// tableFor renders an arbitrary value as a table by way of its JSON encoding,
// so the human output can never disagree with the machine output.
func tableFor(v any) (*Table, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	// Decode numbers as their original text: a float64 would render 1000000 as
	// 1e+06 in a cell.
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	var decoded any
	if err := dec.Decode(&decoded); err != nil {
		return nil, err
	}
	return tableFromJSON(decoded), nil
}

// tableFromJSON turns decoded JSON into a table: an object becomes FIELD/VALUE
// rows, a collection of objects becomes one row per object with a column per
// key, and anything else becomes a single column.
func tableFromJSON(v any) *Table {
	switch val := v.(type) {
	case nil:
		return NewTable()
	case map[string]any:
		if len(val) == 0 {
			return NewTable()
		}
		t := NewTable("FIELD", "VALUE")
		for _, k := range SortedKeys(val) {
			t.Add(k, cellValue(val[k]))
		}
		return t
	case []any:
		if len(val) == 0 {
			return NewTable()
		}
		if t := rowsFromObjects(val); t != nil {
			return t
		}
		t := NewTable("VALUE")
		for _, item := range val {
			t.Add(cellValue(item))
		}
		return t
	default:
		t := NewTable("VALUE")
		t.Add(cellValue(v))
		return t
	}
}

// rowsFromObjects builds a table from a collection of JSON objects, using the
// union of their keys as columns. It returns nil when the collection holds
// anything other than objects.
func rowsFromObjects(items []any) *Table {
	columns := map[string]bool{}
	for _, item := range items {
		obj, ok := item.(map[string]any)
		if !ok {
			return nil
		}
		for k := range obj {
			columns[k] = true
		}
	}
	keys := SortedKeys(columns)
	t := NewTable(keys...)
	for _, item := range items {
		obj := item.(map[string]any)
		row := make([]string, len(keys))
		for i, k := range keys {
			row[i] = cellValue(obj[k])
		}
		t.Add(row...)
	}
	return t
}

// cellValue renders one table cell: strings verbatim, nested structures as
// compact JSON so nothing is lost, and scalars as their natural text.
func cellValue(v any) string {
	switch val := v.(type) {
	case nil:
		return ""
	case string:
		return val
	case json.Number:
		return val.String()
	case map[string]any, []any:
		data, err := json.Marshal(val)
		if err != nil {
			return fmt.Sprint(val)
		}
		return string(data)
	default:
		return fmt.Sprint(val)
	}
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
