package cli

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fTag is one header index entry plus its raw data.
type fTag struct {
	tag, typ, count uint32
	data            []byte
}

func fStr(s string) []byte {
	var b bytes.Buffer
	b.WriteString(s)
	b.WriteByte(0)
	return b.Bytes()
}

func fStrArr(ss ...string) []byte {
	var b bytes.Buffer
	for _, s := range ss {
		b.WriteString(s)
		b.WriteByte(0)
	}
	return b.Bytes()
}

func fI32(v ...uint32) []byte {
	var b bytes.Buffer
	for _, x := range v {
		_ = binary.Write(&b, binary.BigEndian, x)
	}
	return b.Bytes()
}

func fI16(v ...uint16) []byte {
	var b bytes.Buffer
	for _, x := range v {
		_ = binary.Write(&b, binary.BigEndian, x)
	}
	return b.Bytes()
}

func fAlign(n, a int) int { return (n + a - 1) &^ (a - 1) }

func fHeader(tags []fTag) []byte {
	var data []byte
	type ent struct{ tag, typ, off, count uint32 }
	ents := make([]ent, len(tags))
	for i, t := range tags {
		a := 1
		switch t.typ {
		case 3, 4:
			a = 4
		case 9:
			a = 2
		}
		for len(data)%a != 0 {
			data = append(data, 0)
		}
		off := len(data)
		data = append(data, t.data...)
		ents[i] = ent{t.tag, t.typ, uint32(off), t.count}
	}
	var b bytes.Buffer
	b.Write([]byte{0x8e, 0xad, 0xe8, 0x01, 0, 0, 0, 0})
	_ = binary.Write(&b, binary.BigEndian, uint32(len(ents)))
	_ = binary.Write(&b, binary.BigEndian, uint32(len(data)))
	for _, e := range ents {
		_ = binary.Write(&b, binary.BigEndian, e.tag)
		_ = binary.Write(&b, binary.BigEndian, e.typ)
		_ = binary.Write(&b, binary.BigEndian, e.off)
		_ = binary.Write(&b, binary.BigEndian, e.count)
	}
	b.Write(data)
	return b.Bytes()
}

// buildFixtureRPM writes a small RPM with two files using the rpm 6 header
// type numbering, and returns its path.
func buildFixtureRPM(t *testing.T) string {
	t.Helper()
	main := fHeader([]fTag{
		{1000, 6, 1, fStr("coprctl")},
		{1001, 6, 1, fStr("1.0.0")},
		{1002, 6, 1, fStr("1.fc44")},
		{1022, 6, 1, fStr("x86_64")},
		{5000, 8, 2, fStrArr("/usr/bin/coprctl", "/usr/share/man/man1/coprctl.1.gz")},
		{1028, 4, 2, fI32(8936440, 1234)},
		{1030, 3, 2, fI16(0o100755, 0o100644)},
	})
	lead := make([]byte, 96)
	binary.BigEndian.PutUint32(lead, 0xedabeedb)
	sig := make([]byte, 16)
	copy(sig, []byte{0x8e, 0xad, 0xe8, 0x01, 0, 0, 0, 0})
	var b bytes.Buffer
	b.Write(lead)
	b.Write(sig)
	b.Write(main)
	path := filepath.Join(t.TempDir(), "fixture.rpm")
	if err := os.WriteFile(path, b.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestRpmListFilesJSON(t *testing.T) {
	path := buildFixtureRPM(t)
	cmd := newRpmCmd(NewApp())
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"list-files", path, "--output", "json"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	var got rpmListFiles
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("invalid json output: %v\n%s", err, buf.String())
	}
	if got.Name != "coprctl" || got.Version != "1.0.0" || got.Release != "1.fc44" || got.Arch != "x86_64" {
		t.Errorf("metadata = %+v", got)
	}
	if len(got.Files) != 2 || got.Files[0].Name != "/usr/bin/coprctl" {
		t.Errorf("files = %+v", got.Files)
	}
	if got.Files[0].Size != 8936440 || got.Files[0].Mode.String() != "-rwxr-xr-x" {
		t.Errorf("first file = %+v", got.Files[0])
	}
}

func TestRpmListFilesTable(t *testing.T) {
	path := buildFixtureRPM(t)
	cmd := newRpmCmd(NewApp())
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"list-files", path, "--output", "table"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	out := buf.String()
	for _, want := range []string{"MODE", "SIZE", "PATH", "/usr/bin/coprctl", "-rwxr-xr-x", "8936440"} {
		if !strings.Contains(out, want) {
			t.Errorf("table output missing %q:\n%s", want, out)
		}
	}
}

func TestRpmListFilesRequiresPath(t *testing.T) {
	cmd := newRpmCmd(NewApp())
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"list-files"})
	if err := cmd.Execute(); err == nil {
		t.Fatal("expected a usage error without a path")
	}
}

func TestRpmListFilesRejectsNonRPM(t *testing.T) {
	path := filepath.Join(t.TempDir(), "not.rpm")
	if err := os.WriteFile(path, []byte("junk"), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := newRpmCmd(NewApp())
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"list-files", path})
	if err := cmd.Execute(); err == nil {
		t.Fatal("expected an error for a non-RPM file")
	}
}
