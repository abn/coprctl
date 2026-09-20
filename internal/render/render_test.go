package render

import (
	"bytes"
	"strings"
	"testing"
)

func TestRenderJSON(t *testing.T) {
	var buf bytes.Buffer
	v := map[string]any{"name": "aetherpak", "id": 1}
	if err := Render(&buf, FormatJSON, v); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), `"name": "aetherpak"`) {
		t.Errorf("json output = %q", buf.String())
	}
}

func TestRenderTable(t *testing.T) {
	var buf bytes.Buffer
	tbl := NewTable("NAME", "ID")
	tbl.Add("aetherpak", "1")
	tbl.Add("cli", "2")
	if err := Render(&buf, FormatTable, tbl); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "aetherpak") || !strings.Contains(buf.String(), "cli") {
		t.Errorf("table output = %q", buf.String())
	}
}

func TestRenderYAML(t *testing.T) {
	var buf bytes.Buffer
	v := map[string]any{"name": "aetherpak"}
	if err := Render(&buf, FormatYAML, v); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "aetherpak") {
		t.Errorf("yaml output = %q", buf.String())
	}
}

func TestRenderJSONLSlice(t *testing.T) {
	var buf bytes.Buffer
	v := []map[string]any{{"id": 1}, {"id": 2}}
	if err := Render(&buf, FormatJSONL, v); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("jsonl lines = %q, want one object per line", buf.String())
	}
	if !strings.Contains(lines[0], `"id":1`) || !strings.Contains(lines[1], `"id":2`) {
		t.Errorf("jsonl output = %q", buf.String())
	}
}

func TestRenderJSONLSingle(t *testing.T) {
	var buf bytes.Buffer
	v := map[string]any{"id": 1}
	if err := Render(&buf, FormatJSONL, v); err != nil {
		t.Fatal(err)
	}
	if got := strings.Count(buf.String(), "\n"); got != 1 {
		t.Errorf("single-object jsonl should be one line, got %d newlines in %q", got, buf.String())
	}
}

func TestParseFormat(t *testing.T) {
	if f, err := ParseFormat("JSON"); err != nil || f != FormatJSON {
		t.Errorf("ParseFormat(JSON) = %v, %v", f, err)
	}
	if f, err := ParseFormat("table"); err != nil || f != FormatTable {
		t.Errorf("ParseFormat(table) = %v, %v", f, err)
	}
	if _, err := ParseFormat("nope"); err == nil {
		t.Errorf("expected error for unknown format")
	}
}

func TestRenderMapAsTable(t *testing.T) {
	var buf bytes.Buffer
	if err := Render(&buf, FormatTable, map[string]any{"deleted": "abn/demo", "count": 2}); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, want := range []string{"FIELD", "VALUE", "deleted", "abn/demo", "count", "2"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q: %q", want, out)
		}
	}
	// Keys are sorted, so the rendering is deterministic.
	if strings.Index(out, "count") > strings.Index(out, "deleted") {
		t.Errorf("keys not sorted: %q", out)
	}
}

func TestRenderStructAsTable(t *testing.T) {
	type payload struct {
		Name string `json:"name"`
		ID   int    `json:"id"`
		Note string `json:"note,omitempty"`
	}
	var buf bytes.Buffer
	if err := Render(&buf, FormatTable, payload{Name: "aetherpak", ID: 7}); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, want := range []string{"name", "aetherpak", "id", "7"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q: %q", want, out)
		}
	}
	if strings.Contains(out, "note") {
		t.Errorf("omitempty field should not render: %q", out)
	}
}

func TestRenderSliceOfObjectsAsTable(t *testing.T) {
	var buf bytes.Buffer
	rows := []map[string]any{
		{"id": 1, "name": "aetherpak", "state": "done"},
		{"id": 2, "name": "cli"},
	}
	if err := Render(&buf, FormatTable, rows); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, want := range []string{"id", "name", "state", "aetherpak", "cli", "done"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q: %q", want, out)
		}
	}
}

func TestRenderNestedValueAsJSON(t *testing.T) {
	var buf bytes.Buffer
	if err := Render(&buf, FormatTable, map[string]any{"chroots": []string{"a", "b"}}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), `["a","b"]`) {
		t.Errorf("nested value should be compact JSON: %q", buf.String())
	}
}

func TestRenderScalarAsTable(t *testing.T) {
	var buf bytes.Buffer
	if err := Render(&buf, FormatTable, "hello"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "hello") {
		t.Errorf("output = %q", buf.String())
	}
}

func TestRenderLargeNumberKeepsText(t *testing.T) {
	var buf bytes.Buffer
	if err := Render(&buf, FormatTable, map[string]any{"size": 1000000}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "1000000") || strings.Contains(buf.String(), "1e+06") {
		t.Errorf("number should keep its text form: %q", buf.String())
	}
}

func TestRenderGenericShapes(t *testing.T) {
	cases := []struct {
		name  string
		value any
		want  []string
		empty bool
	}{
		{name: "nil", value: nil, empty: true},
		{name: "empty map", value: map[string]any{}, empty: true},
		{name: "empty slice", value: []any{}, empty: true},
		{name: "string slice", value: []string{"a", "b"}, want: []string{"VALUE", "a", "b"}},
		{name: "mixed slice", value: []any{"a", 1}, want: []string{"VALUE", "a", "1"}},
		{name: "bool and null", value: map[string]any{"ok": true, "none": nil}, want: []string{"FIELD", "VALUE", "ok", "true", "none"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			if err := Render(&buf, FormatTable, tc.value); err != nil {
				t.Fatal(err)
			}
			out := buf.String()
			if tc.empty {
				if strings.TrimSpace(out) != "" {
					t.Errorf("expected empty output, got %q", out)
				}
				return
			}
			for _, want := range tc.want {
				if !strings.Contains(out, want) {
					t.Errorf("output missing %q: %q", want, out)
				}
			}
		})
	}
}

func TestRenderGenericDeterministic(t *testing.T) {
	v := map[string]any{"b": 2, "a": 1, "c": []string{"x", "y"}}
	var first string
	for i := 0; i < 5; i++ {
		var buf bytes.Buffer
		if err := Render(&buf, FormatTable, v); err != nil {
			t.Fatal(err)
		}
		if i == 0 {
			first = buf.String()
			continue
		}
		if buf.String() != first {
			t.Fatalf("render %d differs:\n%s\n---\n%s", i, first, buf.String())
		}
	}
}

func TestRenderGenericPlain(t *testing.T) {
	var buf bytes.Buffer
	if err := Render(&buf, FormatPlain, map[string]any{"deleted": "abn/demo"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "abn/demo") {
		t.Errorf("output = %q", buf.String())
	}
}

func TestRenderColoredTable(t *testing.T) {
	tbl := NewTable("NAME", "VALUE")
	tbl.Add("foo", "bar")
	var buf bytes.Buffer
	if err := RenderColored(&buf, FormatTable, tbl, true); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "\x1b[") {
		t.Errorf("expected ANSI codes, got %q", buf.String())
	}
	// Without color there are no escape sequences.
	var plain bytes.Buffer
	if err := Render(&buf, FormatTable, tbl); err != nil {
		t.Fatal(err)
	}
	_ = plain
	if err := Render(&plain, FormatTable, tbl); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(plain.String(), "\x1b[") {
		t.Errorf("unexpected ANSI codes: %q", plain.String())
	}
}
