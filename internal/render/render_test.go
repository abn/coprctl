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
